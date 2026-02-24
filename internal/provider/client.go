package provider

import (
	"context"
	"crypto/tls"
	"fmt"
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
func NewClient(ctx context.Context, cfg ClientConfig) (*Client, error) {
	if cfg.APIURL == "" {
		return nil, fmt.Errorf("API URL is required (set UNIFI_API or pass api_url)")
	}
	if cfg.APIKey == "" && (cfg.Username == "" || cfg.Password == "") {
		return nil, fmt.Errorf("either API key or both username and password are required")
	}

	c := retryablehttp.NewClient()
	c.HTTPClient.Timeout = 30 * time.Second
	c.Logger = nil // No tflog outside the provider

	if cfg.AllowInsecure {
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

	client := &ui.ApiClient{}
	if err := client.SetHTTPClient(c); err != nil {
		return nil, fmt.Errorf("setting HTTP client: %w", err)
	}

	if err := client.SetBaseURL(cfg.APIURL); err != nil {
		return nil, fmt.Errorf("invalid API URL: %w", err)
	}

	if cfg.APIKey != "" {
		client.SetAPIKey(cfg.APIKey)
	}

	if err := client.Login(ctx, cfg.Username, cfg.Password); err != nil {
		return nil, fmt.Errorf("login failed: %w", err)
	}

	apiPath, err := discoverAPIPath(ctx, c, cfg.APIURL)
	if err != nil {
		return nil, fmt.Errorf("API path discovery failed: %w", err)
	}

	return &Client{
		ApiClient: client,
		Site:      cfg.Site,
		BaseURL:   cfg.APIURL,
		APIPath:   apiPath,
		APIKey:    cfg.APIKey,
		HTTP:      c,
	}, nil
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
