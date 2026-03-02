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
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
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
	cfg := ClientConfig{
		APIURL:        stringValueOrEnv(config.ApiUrl, "UNIFI_API"),
		Username:      stringValueOrEnv(config.Username, "UNIFI_USERNAME"),
		Password:      stringValueOrEnv(config.Password, "UNIFI_PASSWORD"),
		APIKey:        stringValueOrEnv(config.ApiKey, "UNIFI_API_KEY"),
		Site:          stringValueOrEnv(config.Site, "UNIFI_SITE"),
		AllowInsecure: config.AllowInsecure.ValueBool(),
	}

	if !cfg.AllowInsecure {
		if v := os.Getenv("UNIFI_INSECURE"); v == "true" {
			cfg.AllowInsecure = true
		}
	}

	if cfg.Site == "" {
		cfg.Site = "default"
	}

	// tflog writes structured logs that appear when TF_LOG=DEBUG is set.
	// MaskFieldValuesWithFieldKeys redacts sensitive values in log output.
	ctx = tflog.SetField(ctx, "unifi_api_url", cfg.APIURL)
	ctx = tflog.SetField(ctx, "unifi_site", cfg.Site)
	ctx = tflog.MaskFieldValuesWithFieldKeys(ctx, "unifi_api_key")
	ctx = tflog.MaskFieldValuesWithFieldKeys(ctx, "unifi_password")
	tflog.Debug(ctx, "Configuring terrifi provider")

	// Validate that we have enough config to connect.
	// AddAttributeError highlights the specific attribute in Terraform's error output.
	if cfg.APIURL == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_url"),
			"Missing API URL",
			"The API URL must be provided via the api_url attribute or the UNIFI_API environment variable.",
		)
	}

	if cfg.APIKey == "" && (cfg.Username == "" || cfg.Password == "") {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Missing Authentication",
			"Either api_key or both username and password must be provided.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	configuredClient, err := NewClient(ctx, cfg)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create UniFi Client",
			fmt.Sprintf("An unexpected error occurred: %s", err.Error()),
		)
		return
	}

	// Route HTTP-level logs through tflog (see logger.go).
	configuredClient.HTTP.Logger = NewLogger(ctx)

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
		NewClientDeviceResource,
		NewClientGroupResource,
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
	return []func() datasource.DataSource{
		NewClientGroupDeviceIDsDataSource,
	}
}

// stringValueOrEnv returns the Terraform attribute value if non-empty, otherwise
// falls back to the named environment variable.
func stringValueOrEnv(val types.String, envVar string) string {
	if v := val.ValueString(); v != "" {
		return v
	}
	return os.Getenv(envVar)
}
