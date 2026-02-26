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
	ui "github.com/ubiquiti-community/go-unifi/unifi"
)

// Client wraps the go-unifi API client with site information.
// The go-unifi SDK (github.com/ubiquiti-community/go-unifi) provides typed Go structs
// and CRUD methods for every UniFi API endpoint. We wrap it to carry the default site
// name alongside the API client so resources can fall back to it.
type Client struct {
	*ui.ApiClient
	Site    string
	BaseURL string
	APIPath string // API path prefix, e.g. "/proxy/network" for UniFi OS, empty for legacy
	APIKey  string // Stored separately because the SDK's apiKey field is private
	HTTP    *retryablehttp.Client
	csrf    string // CSRF token for custom v2/v1 API requests that bypass the SDK
}

// SiteOrDefault returns the given site if non-empty, otherwise falls back to the
// provider's default site. Every resource calls this to resolve which site to operate on,
// since the site attribute is optional on individual resources.
func (c *Client) SiteOrDefault(site types.String) string {
	if v := site.ValueString(); v != "" {
		return v
	}
	return c.Site
}

// ClientConfig holds the configuration needed to create an authenticated
// UniFi API client. It can be populated from Terraform attributes, env vars,
// or both (via ClientConfigFromEnv).
type ClientConfig struct {
	APIURL        string
	Username      string
	Password      string
	APIKey        string
	Site          string
	AllowInsecure bool
}

// ClientConfigFromEnv reads UniFi connection configuration from environment
// variables. This is the same set of env vars that the Terraform provider reads.
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
	return cfg
}

// NewClient creates an authenticated UniFi API client from the given config.
// It handles HTTP client setup, TLS configuration, authentication (API key or
// username/password), and API path discovery.
//
// TODO(go-unifi): The SDK's New() constructor creates its own internal HTTP client
// and does not expose it or the CSRF token. Because we need to make custom HTTP
// requests for v2/v1 endpoints that bypass the SDK (firewall zones, firewall
// policies, client devices), we create a separate retryablehttp.Client and
// perform an independent login to obtain our own session cookie + CSRF token.
// If the SDK ever exposes a Do() method or the CSRF token, this dual-login
// approach can be eliminated.
func NewClient(ctx context.Context, cfg ClientConfig) (*Client, error) {
	if cfg.APIURL == "" {
		return nil, fmt.Errorf("API URL is required (set UNIFI_API or pass api_url)")
	}
	if cfg.APIKey == "" && (cfg.Username == "" || cfg.Password == "") {
		return nil, fmt.Errorf("either API key or both username and password are required")
	}

	// Create the SDK client using the v1.33.36+ constructor.
	// This handles HTTP setup, TLS, login, and API path discovery internally.
	sdkClient, err := ui.New(ctx, &ui.Config{
		BaseURL:       cfg.APIURL,
		APIKey:        cfg.APIKey,
		Username:      cfg.Username,
		Password:      cfg.Password,
		AllowInsecure: cfg.AllowInsecure,
	})
	if err != nil {
		return nil, fmt.Errorf("initializing SDK client: %w", err)
	}

	// Create a separate HTTP client for custom v2/v1 API requests that bypass
	// the SDK (firewall zones, firewall policies, client devices). The SDK
	// doesn't expose its internal HTTP client or CSRF token, so we maintain
	// our own authenticated session for these requests.
	httpClient := newRetryableHTTPClient(cfg.AllowInsecure)

	apiPath, err := discoverAPIPath(ctx, httpClient, cfg.APIURL)
	if err != nil {
		return nil, fmt.Errorf("API path discovery failed: %w", err)
	}

	var csrf string
	if cfg.APIKey == "" && cfg.Username != "" && cfg.Password != "" {
		loginPath := "/api/login"
		if apiPath == "/proxy/network" {
			loginPath = "/api/auth/login"
		}
		csrf, err = loginForCustomRequests(ctx, httpClient, cfg.APIURL, loginPath, cfg.Username, cfg.Password)
		if err != nil {
			return nil, fmt.Errorf("custom client login failed: %w", err)
		}
	}

	return &Client{
		ApiClient: sdkClient,
		Site:      cfg.Site,
		BaseURL:   cfg.APIURL,
		APIPath:   apiPath,
		APIKey:    cfg.APIKey,
		HTTP:      httpClient,
		csrf:      csrf,
	}, nil
}

// newRetryableHTTPClient creates a retryablehttp.Client configured for UniFi API access.
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

// loginForCustomRequests authenticates with the UniFi controller using the given
// HTTP client, establishing a session for custom v2/v1 API requests. Returns
// the CSRF token from the login response (empty string for legacy controllers).
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

	// Extract CSRF token from response headers. UniFi OS uses X-Csrf-Token;
	// newer firmware may use X-Updated-Csrf-Token.
	csrf := resp.Header.Get("X-Updated-Csrf-Token")
	if csrf == "" {
		csrf = resp.Header.Get("X-Csrf-Token")
	}

	return csrf, nil
}

// discoverAPIPath probes the UniFi controller to determine the API path prefix.
// UniFi OS controllers return HTTP 200 on GET / and use "/proxy/network" as the
// API path prefix. Legacy controllers return HTTP 302 and use no prefix.
// This replicates the logic in the go-unifi SDK's setAPIUrlStyle method.
func discoverAPIPath(ctx context.Context, c *retryablehttp.Client, baseURL string) (string, error) {
	req, err := retryablehttp.NewRequestWithContext(ctx, http.MethodGet, baseURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating probe request: %w", err)
	}

	// Disable redirects for this probe — we need to see the raw status code.
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
