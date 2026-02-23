// Package provider implements the Terrifi Terraform provider for managing
// Ubiquiti UniFi network infrastructure.
//
// Architecture overview:
//
// Terraform Plugin Framework is HashiCorp's modern SDK for building providers.
// A provider has three responsibilities:
//  1. Schema — declare what configuration the provider block accepts (URL, credentials, etc.)
//  2. Configure — use that config to create an authenticated API client
//  3. Resources/DataSources — return the list of resource types this provider manages
//
// The framework calls these methods in order: Schema → Configure → then CRUD methods
// on individual resources as needed. The Configure method stores the authenticated client
// in resp.ResourceData, and each resource retrieves it in its own Configure method.
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
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	ui "github.com/ubiquiti-community/go-unifi/unifi"
)

// Compile-time check: terrifiProvider must implement the provider.Provider interface.
// This pattern is idiomatic Go — it catches interface mismatches at build time rather
// than at runtime. The _ means we don't actually use the variable.
var _ provider.Provider = &terrifiProvider{}

// terrifiProvider is the top-level provider struct. It's stateless — all configuration
// happens in Configure() which passes a Client to resources via resp.ResourceData.
type terrifiProvider struct{}

// terrifiProviderModel maps the HCL provider block to Go types. The `tfsdk` struct tags
// tell the framework which HCL attribute each field corresponds to.
// For example:
//
//	provider "terrifi" {
//	  api_url  = "https://192.168.1.12:8443"
//	  username = "admin"
//	}
//
// The framework automatically deserializes this HCL into a terrifiProviderModel struct.
// types.String/types.Bool are Terraform's wrapper types that track null vs empty vs set.
type terrifiProviderModel struct {
	ApiKey        types.String `tfsdk:"api_key"`
	Username      types.String `tfsdk:"username"`
	Password      types.String `tfsdk:"password"`
	ApiUrl        types.String `tfsdk:"api_url"`
	Site          types.String `tfsdk:"site"`
	AllowInsecure types.Bool   `tfsdk:"allow_insecure"`
}

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

// New creates a new provider instance. The framework calls this factory function
// for each Terraform operation, so providers should be cheap to create.
func New() provider.Provider {
	return &terrifiProvider{}
}

// Metadata sets the provider type name. This becomes the prefix for all resource types —
// e.g., "terrifi" means resources are named "terrifi_dns_record", "terrifi_network", etc.
func (p *terrifiProvider) Metadata(
	_ context.Context,
	_ provider.MetadataRequest,
	resp *provider.MetadataResponse,
) {
	resp.TypeName = "terrifi"
}

// Schema defines the provider block's HCL schema — the attributes users can configure.
// Each attribute has a type, description, and flags like Optional/Required/Sensitive.
func (p *terrifiProvider) Schema(
	_ context.Context,
	_ provider.SchemaRequest,
	resp *provider.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Terraform provider for managing Ubiquiti UniFi network infrastructure.",

		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				MarkdownDescription: "API key for the UniFi controller. Can be specified with the `UNIFI_API_KEY` " +
					"environment variable. If set, `username` and `password` are ignored.",
				Optional:  true,
				Sensitive: true, // Sensitive fields are redacted in plan output and logs
			},
			"username": schema.StringAttribute{
				MarkdownDescription: "Local username for the UniFi controller API. Can be specified with the " +
					"`UNIFI_USERNAME` environment variable.",
				Optional:  true,
				Sensitive: true,
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "Password for the UniFi controller API. Can be specified with the " +
					"`UNIFI_PASSWORD` environment variable.",
				Optional:  true,
				Sensitive: true,
			},
			"api_url": schema.StringAttribute{
				MarkdownDescription: "URL of the UniFi controller API. Can be specified with the `UNIFI_API` " +
					"environment variable. Do not include the `/api` path — the SDK discovers API paths automatically " +
					"to support both UDM-style and classic controller layouts.",
				Optional: true,
			},
			"site": schema.StringAttribute{
				MarkdownDescription: "The UniFi site to manage. Can be specified with the `UNIFI_SITE` " +
					"environment variable. Default: `default`.",
				Optional: true,
			},
			"allow_insecure": schema.BoolAttribute{
				MarkdownDescription: "Skip TLS certificate verification. Useful for local controllers with " +
					"self-signed certs. Can be specified with the `UNIFI_INSECURE` environment variable.",
				Optional: true,
			},
		},
	}
}

// Configure is called by the framework after Schema. It reads the provider config,
// creates an authenticated UniFi API client, and stores it for resources to use.
//
// The flow is:
//  1. Read HCL config (with env var fallbacks)
//  2. Validate required fields
//  3. Create HTTP client with retry support and TLS config
//  4. Create go-unifi API client and authenticate
//  5. Store the client in resp.ResourceData so resources can access it
func (p *terrifiProvider) Configure(
	ctx context.Context,
	req provider.ConfigureRequest,
	resp *provider.ConfigureResponse,
) {
	var config terrifiProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Resolve each setting: prefer the HCL attribute, fall back to the env var.
	// This lets users configure the provider either way (or mix both).
	apiUrl := stringValueOrEnv(config.ApiUrl, "UNIFI_API")
	username := stringValueOrEnv(config.Username, "UNIFI_USERNAME")
	password := stringValueOrEnv(config.Password, "UNIFI_PASSWORD")
	apiKey := stringValueOrEnv(config.ApiKey, "UNIFI_API_KEY")

	allowInsecure := config.AllowInsecure.ValueBool()
	if !allowInsecure {
		if v := os.Getenv("UNIFI_INSECURE"); v == "true" {
			allowInsecure = true
		}
	}

	site := stringValueOrEnv(config.Site, "UNIFI_SITE")
	if site == "" {
		site = "default"
	}

	// tflog writes structured logs that appear when TF_LOG=DEBUG is set.
	// MaskFieldValuesWithFieldKeys redacts sensitive values in log output.
	ctx = tflog.SetField(ctx, "unifi_api_url", apiUrl)
	ctx = tflog.SetField(ctx, "unifi_site", site)
	ctx = tflog.MaskFieldValuesWithFieldKeys(ctx, "unifi_api_key")
	ctx = tflog.MaskFieldValuesWithFieldKeys(ctx, "unifi_password")
	tflog.Debug(ctx, "Configuring terrifi provider")

	// Validate that we have enough config to connect.
	// AddAttributeError highlights the specific attribute in Terraform's error output.
	if apiUrl == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_url"),
			"Missing API URL",
			"The API URL must be provided via the api_url attribute or the UNIFI_API environment variable.",
		)
	}

	if apiKey == "" && (username == "" || password == "") {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Missing Authentication",
			"Either api_key or both username and password must be provided.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// Create an HTTP client with automatic retry support.
	// go-retryablehttp wraps net/http and retries on transient failures (5xx, timeouts).
	// This is important because UniFi controllers can be flaky under load.
	c := retryablehttp.NewClient()
	c.HTTPClient.Timeout = 30 * time.Second
	c.Logger = NewLogger(ctx) // Route HTTP-level logs through tflog (see logger.go)

	// UniFi controllers typically use self-signed TLS certificates, so most local
	// setups need InsecureSkipVerify. This is controlled by the allow_insecure flag.
	if allowInsecure {
		c.HTTPClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
		}
	}

	// The UniFi API uses session cookies for authentication (not just bearer tokens).
	// A cookie jar stores the session cookie after login so subsequent requests are
	// authenticated automatically.
	jar, _ := cookiejar.New(nil)
	c.HTTPClient.Jar = jar

	// Create the go-unifi API client. This is the main SDK entry point that provides
	// typed methods like CreateDNSRecord(), GetNetwork(), etc.
	client := &ui.ApiClient{}
	if err := client.SetHTTPClient(c); err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create HTTP Client",
			fmt.Sprintf("An unexpected error occurred: %s", err.Error()),
		)
		return
	}

	// SetBaseURL tells the SDK where the controller lives. The SDK appends the correct
	// API paths automatically (e.g., /proxy/network/api for UDM, or /api for classic).
	if err := client.SetBaseURL(apiUrl); err != nil {
		resp.Diagnostics.AddError(
			"Invalid API URL",
			fmt.Sprintf("The provided API URL is invalid: %s", err.Error()),
		)
		return
	}

	// Authenticate: API key (stateless, set on every request as a header) or
	// username/password (creates a session via cookie).
	if apiKey != "" {
		client.SetAPIKey(apiKey)
	}

	// Login() serves double duty: it detects the API URL style (classic vs UniFi OS)
	// and, when no API key is set, authenticates via username/password. When an API key
	// IS set, Login() skips the POST but still probes the controller to set the correct
	// API path prefix (e.g. /proxy/network for UniFi OS).
	if err := client.Login(ctx, username, password); err != nil {
		resp.Diagnostics.AddError(
			"Login Failed",
			fmt.Sprintf("Could not log in to UniFi controller: %s", err.Error()),
		)
		return
	}

	// Discover the API path prefix. The go-unifi SDK does this internally during
	// Login() but stores it in a private field. We replicate the same probe: UniFi OS
	// returns 200 on GET / (and uses /proxy/network prefix), while legacy controllers
	// return 302 (and use no prefix).
	apiPath, err := discoverAPIPath(ctx, c, apiUrl)
	if err != nil {
		resp.Diagnostics.AddError(
			"API Path Discovery Failed",
			fmt.Sprintf("Could not determine UniFi API path style: %s", err.Error()),
		)
		return
	}

	// Wrap the SDK client with our site default. This is what every resource receives
	// in its Configure() method via req.ProviderData.
	configuredClient := &Client{
		ApiClient: client,
		Site:      site,
		BaseURL:   apiUrl,
		APIPath:   apiPath,
		APIKey:    apiKey,
		HTTP:      c,
	}

	// ResourceData and DataSourceData are how the framework passes the client to
	// individual resources and data sources. Each resource's Configure() method
	// casts req.ProviderData back to *Client.
	resp.DataSourceData = configuredClient
	resp.ResourceData = configuredClient
}

// Resources returns the list of resource types this provider supports.
// Each entry is a factory function that creates a new resource instance.
func (p *terrifiProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewDNSRecordResource,
		NewFirewallPolicyResource,
		NewFirewallZoneResource,
		NewNetworkResource,
		NewWLANResource,
	}
}

// DataSources returns the list of data source types. Empty for now — we'll add
// data sources (read-only lookups) as needed.
func (p *terrifiProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
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

// stringValueOrEnv returns the Terraform attribute value if non-empty, otherwise
// falls back to the named environment variable.
func stringValueOrEnv(val types.String, envVar string) string {
	if v := val.ValueString(); v != "" {
		return v
	}
	return os.Getenv(envVar)
}
