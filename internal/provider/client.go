package provider

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Client wraps a retryablehttp.Client plus the controller's discovered API
// path prefix and the per-session CSRF token. Resources call CRUD methods
// declared in the various *_api.go files via this type.
type Client struct {
	Site    string
	BaseURL string
	APIPath string // "/proxy/network" on UniFi OS, "" on legacy controllers
	APIKey  string
	HTTP    *retryablehttp.Client
	csrf    string         // session CSRF token; empty when authenticating via API key
	cache   *responseCache // nil when response caching is disabled
}

// SiteOrDefault returns the given site if non-empty, otherwise the provider's
// default site. Every resource calls this to resolve which site to operate on.
func (c *Client) SiteOrDefault(site types.String) string {
	if v := site.ValueString(); v != "" {
		return v
	}
	return c.Site
}

// ClientConfig holds the configuration needed to create an authenticated
// UniFi API client.
type ClientConfig struct {
	APIURL          string
	Username        string
	Password        string
	APIKey          string
	Site            string
	AllowInsecure   bool
	ResponseCaching bool
}

// ClientConfigFromEnv reads UniFi connection configuration from environment
// variables. The same set of vars the Terraform provider reads at configure time.
func ClientConfigFromEnv() ClientConfig {
	cfg := ClientConfig{
		APIURL:   os.Getenv("UNIFI_API"),
		Username: os.Getenv("UNIFI_USERNAME"),
		Password: os.Getenv("UNIFI_PASSWORD"),
		APIKey:   os.Getenv("UNIFI_API_KEY"),
		Site:     os.Getenv("UNIFI_SITE"),
	}
	if cfg.Site == "" {
		cfg.Site = "default"
	}
	if os.Getenv("UNIFI_INSECURE") == "true" {
		cfg.AllowInsecure = true
	}
	if os.Getenv("UNIFI_RESPONSE_CACHING") == "true" {
		cfg.ResponseCaching = true
	}
	return cfg
}

// NewClient creates an authenticated UniFi API client. It probes the
// controller to discover the API path prefix (UniFi OS vs. legacy) and, when
// authenticating with username/password, performs a login to obtain a session
// cookie and CSRF token. API-key auth needs no login — the key is sent as a
// header on every request.
func NewClient(ctx context.Context, cfg ClientConfig) (*Client, error) {
	if cfg.APIURL == "" {
		return nil, fmt.Errorf("API URL is required (set UNIFI_API or pass api_url)")
	}
	if cfg.APIKey == "" && (cfg.Username == "" || cfg.Password == "") {
		return nil, fmt.Errorf("either API key or both username and password are required")
	}

	httpClient := newRetryableHTTPClient(cfg.AllowInsecure)

	apiPath, err := discoverAPIPath(ctx, httpClient, cfg.APIURL)
	if err != nil {
		return nil, fmt.Errorf("API path discovery failed: %w", err)
	}

	var csrf string
	if cfg.APIKey == "" {
		loginPath := "/api/login"
		if apiPath == "/proxy/network" {
			loginPath = "/api/auth/login"
		}
		csrf, err = loginForCustomRequests(ctx, httpClient, cfg.APIURL, loginPath, cfg.Username, cfg.Password)
		if err != nil {
			return nil, fmt.Errorf("login failed: %w", err)
		}
	}

	var cache *responseCache
	if cfg.ResponseCaching {
		cache = newResponseCache()
	}

	return &Client{
		Site:    cfg.Site,
		BaseURL: cfg.APIURL,
		APIPath: apiPath,
		APIKey:  cfg.APIKey,
		HTTP:    httpClient,
		csrf:    csrf,
		cache:   cache,
	}, nil
}

// newRetryableHTTPClient creates a retryablehttp.Client configured for UniFi
// API access: 30s request timeout, optional TLS-bypass, in-memory cookie jar
// for session persistence.
func newRetryableHTTPClient(allowInsecure bool) *retryablehttp.Client {
	c := retryablehttp.NewClient()
	c.HTTPClient.Timeout = 30 * time.Second
	c.Logger = nil

	if allowInsecure {
		c.HTTPClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
		}
	}

	jar, _ := cookiejar.New(nil)
	c.HTTPClient.Jar = jar

	return c
}

// loginForCustomRequests authenticates with the UniFi controller, establishing
// the session cookie used by all subsequent HTTP calls. Returns the CSRF token
// from the response headers (empty string for legacy controllers).
func loginForCustomRequests(ctx context.Context, httpClient *retryablehttp.Client, baseURL, loginPath, user, pass string) (string, error) {
	payload, _ := json.Marshal(struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}{Username: user, Password: pass})

	req, err := retryablehttp.NewRequestWithContext(ctx, http.MethodPost, baseURL+loginPath, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("creating login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("performing login: %w", err)
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("login returned status %d", resp.StatusCode)
	}

	csrf := resp.Header.Get("X-Updated-Csrf-Token")
	if csrf == "" {
		csrf = resp.Header.Get("X-Csrf-Token")
	}
	return csrf, nil
}

// discoverAPIPath probes the UniFi controller to determine the API path prefix.
// UniFi OS controllers return HTTP 200 on GET / and use "/proxy/network" as the
// API path prefix. Legacy controllers redirect (302) and use no prefix.
func discoverAPIPath(ctx context.Context, c *retryablehttp.Client, baseURL string) (string, error) {
	req, err := retryablehttp.NewRequestWithContext(ctx, http.MethodGet, baseURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating probe request: %w", err)
	}

	origCheckRedirect := c.HTTPClient.CheckRedirect
	c.HTTPClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	defer func() { c.HTTPClient.CheckRedirect = origCheckRedirect }()

	resp, err := c.Do(req)
	if err != nil {
		return "", fmt.Errorf("probing controller: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return "/proxy/network", nil
	}
	return "", nil
}
