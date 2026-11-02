package unifi

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
	"strings"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

// APIClient owns the controller transport, authentication session, and optional response cache.
type APIClient struct {
	Site    string
	BaseURL string
	APIPath string // API path prefix, e.g. "/proxy/network" for UniFi OS, empty for legacy
	APIKey  string
	HTTP    *retryablehttp.Client
	csrf    string         // CSRF token for session-authenticated requests
	cache   *responseCache // nil when response caching is disabled (zero overhead)
}

// Config holds the configuration needed to create an authenticated
// UniFi API client. It can be populated from Terraform attributes, env vars,
// or both (via ConfigFromEnv).
type Config struct {
	APIURL          string
	Username        string
	Password        string
	APIKey          string
	Site            string
	AllowInsecure   bool
	ResponseCaching bool
}

// ConfigFromEnv reads UniFi connection configuration from environment
// variables. This is the same set of env vars that the Terraform provider reads.
func ConfigFromEnv() Config {
	cfg := Config{
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

func New(ctx context.Context, cfg Config) (*APIClient, error) {
	cfg.APIURL = strings.TrimRight(cfg.APIURL, "/")
	if cfg.Site == "" {
		cfg.Site = "default"
	}
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
	if cfg.APIKey == "" && cfg.Username != "" && cfg.Password != "" {
		loginPath := "/api/login"
		if apiPath == "/proxy/network" {
			loginPath = "/api/auth/login"
		}
		csrf, err = login(ctx, httpClient, cfg.APIURL, loginPath, cfg.Username, cfg.Password)
		if err != nil {
			return nil, fmt.Errorf("custom client login failed: %w", err)
		}
	}

	var cache *responseCache
	if cfg.ResponseCaching {
		cache = newResponseCache()
	}

	return &APIClient{
		Site:    cfg.Site,
		BaseURL: cfg.APIURL,
		APIPath: apiPath,
		APIKey:  cfg.APIKey,
		HTTP:    httpClient,
		csrf:    csrf,
		cache:   cache,
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

// login authenticates with the UniFi controller using the given
// HTTP client, establishing a session for custom v2/v1 API requests. Returns
// the CSRF token from the login response (empty string for legacy controllers).
func login(ctx context.Context, httpClient *retryablehttp.Client, baseURL, loginPath, user, pass string) (string, error) {
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
	response, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading login response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("login returned status %d", resp.StatusCode)
	}

	if len(response) > 0 {
		var envelope struct {
			Meta json.RawMessage `json:"meta"`
		}
		if err := json.Unmarshal(response, &envelope); err != nil {
			return "", fmt.Errorf("decoding login response: %w", err)
		}
		if err := CheckV1Meta(envelope.Meta); err != nil {
			return "", err
		}
	}

	// Extract CSRF token from response headers. UniFi OS uses X-Csrf-Token;
	// newer firmware may use X-Updated-Csrf-Token.
	csrf := resp.Header.Get("X-Updated-Csrf-Token")
	if csrf == "" {
		csrf = resp.Header.Get("X-Csrf-Token")
	}

	return csrf, nil
}

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
