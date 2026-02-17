package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/ubiquiti-community/go-unifi/unifi"
)

// Compile-time interface checks.
//
// Go interfaces are satisfied implicitly — you don't write "implements Resource" like
// in Java. That's powerful but means you can silently miss a method. These var lines
// ask the compiler to verify that *dnsRecordResource satisfies the interfaces:
//
//	resource.Resource — requires Metadata, Schema, Configure, Create, Read, Update, Delete
//	resource.ResourceWithImportState — adds ImportState for `terraform import`
//
// The _ (blank identifier) discards the value; we only care about the type check.
// If you forget to implement a method, you'll get a clear compile error here pointing
// at the exact interface that's unsatisfied.
var (
	_ resource.Resource                = &dnsRecordResource{}
	_ resource.ResourceWithImportState = &dnsRecordResource{}
)

// NewDNSRecordResource is the factory function registered in provider.Resources().
// The framework calls this to create a new instance for each resource block in the config.
func NewDNSRecordResource() resource.Resource {
	return &dnsRecordResource{}
}

// dnsRecordResource holds the API client, injected by Configure().
type dnsRecordResource struct {
	client *Client
}

// dnsRecordResourceModel is the Terraform-side representation of a DNS record.
// Each field maps to an HCL attribute via the `tfsdk` struct tag.
//
// types.String, types.Bool, types.Int64 are Terraform's wrapper types. They're needed
// (instead of plain Go types) because Terraform distinguishes between:
//   - null    — attribute not set in config
//   - unknown — value not yet known (computed during apply)
//   - set     — attribute has an explicit value
//
// Plain Go types can't represent null, so we'd lose information.
type dnsRecordResourceModel struct {
	ID         types.String `tfsdk:"id"`
	Site       types.String `tfsdk:"site"`
	Name       types.String `tfsdk:"name"`
	Enabled    types.Bool   `tfsdk:"enabled"`
	Port       types.Int64  `tfsdk:"port"`
	Priority   types.Int64  `tfsdk:"priority"`
	RecordType types.String `tfsdk:"record_type"`
	TTL        types.Int64  `tfsdk:"ttl"`
	Value      types.String `tfsdk:"value"`
	Weight     types.Int64  `tfsdk:"weight"`
}

// Metadata sets the resource type name. Combined with the provider type name "terrifi",
// this becomes "terrifi_dns_record" — the name users write in their .tf files.
func (r *dnsRecordResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_dns_record"
}

// Schema defines the HCL schema for the terrifi_dns_record resource.
// This is what Terraform uses for validation, plan diffing, and documentation generation.
func (r *dnsRecordResource) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a DNS record on the UniFi controller.",

		Attributes: map[string]schema.Attribute{
			// Computed-only attributes are set by the API, not the user.
			// UseStateForUnknown tells Terraform to keep the previous value during plan
			// (instead of showing it as "(known after apply)") — since IDs don't change.
			"id": schema.StringAttribute{
				MarkdownDescription: "The ID of the DNS record.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			// Optional+Computed means: user can set it, but if they don't, we compute it
			// (from the provider's default site). RequiresReplace means changing this
			// attribute forces Terraform to destroy and recreate the resource.
			"site": schema.StringAttribute{
				MarkdownDescription: "The site to associate the DNS record with. Defaults to the provider site.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			// Required means the user must provide this. RequiresReplace because the UniFi
			// API treats the record key (hostname) as part of its identity.
			"name": schema.StringAttribute{
				MarkdownDescription: "The hostname for the DNS record.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			// Optional+Computed with a Default. The user can set it, but if they don't,
			// it defaults to true. booldefault.StaticBool sets the default value.
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the DNS record is enabled. Default: `true`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},

			// Optional attributes with validators. The framework validates these before
			// any CRUD operation runs, so our code can assume valid values.
			"port": schema.Int64Attribute{
				MarkdownDescription: "The port for SRV records.",
				Optional:            true,
				Validators: []validator.Int64{
					int64validator.Between(0, 65535),
				},
			},
			"priority": schema.Int64Attribute{
				MarkdownDescription: "The priority for MX/SRV records.",
				Optional:            true,
				Validators: []validator.Int64{
					int64validator.AtLeast(0),
				},
			},
			"record_type": schema.StringAttribute{
				MarkdownDescription: "The DNS record type (A, AAAA, CNAME, MX, TXT, SRV, PTR).",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("A", "AAAA", "CNAME", "MX", "TXT", "SRV", "PTR"),
				},
			},
			"ttl": schema.Int64Attribute{
				MarkdownDescription: "The TTL in seconds.",
				Optional:            true,
				Validators: []validator.Int64{
					int64validator.AtMost(65535),
				},
			},
			"value": schema.StringAttribute{
				MarkdownDescription: "The value of the DNS record (IP address, hostname, etc.).",
				Required:            true,
			},
			"weight": schema.Int64Attribute{
				MarkdownDescription: "The weight for SRV records.",
				Optional:            true,
				Validators: []validator.Int64{
					int64validator.AtLeast(0),
				},
			},
		},
	}
}

// Configure is called by the framework to inject the provider's API client.
// Every resource gets its own Configure call. The client was stored in
// resp.ResourceData by the provider's Configure method.
func (r *dnsRecordResource) Configure(
	_ context.Context,
	req resource.ConfigureRequest,
	resp *resource.ConfigureResponse,
) {
	// ProviderData is nil during plan-only operations before the provider is configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *Client, got: %T.", req.ProviderData),
		)
		return
	}

	r.client = client
}

// Create is called when Terraform needs to create a new DNS record (terraform apply
// on a new resource). The flow:
//  1. Read the planned values from req.Plan (what the user wrote in HCL)
//  2. Convert the Terraform model to a go-unifi API struct
//  3. Call the UniFi API to create the record
//  4. Convert the API response back to a Terraform model
//  5. Save to resp.State (Terraform's state file)
func (r *dnsRecordResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var plan dnsRecordResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	site := r.client.SiteOrDefault(plan.Site)
	record := r.modelToAPI(&plan)

	created, err := r.client.CreateDNSRecord(ctx, site, record)
	if err != nil {
		resp.Diagnostics.AddError("Error Creating DNS Record", err.Error())
		return
	}

	r.apiToModel(created, &plan, site)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read refreshes the Terraform state from the actual API state. Terraform calls this
// during plan to detect drift (changes made outside Terraform). If the resource was
// deleted externally, we call RemoveResource to tell Terraform it's gone.
func (r *dnsRecordResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var state dnsRecordResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	site := r.client.SiteOrDefault(state.Site)

	record, err := r.client.GetDNSRecord(ctx, site, state.ID.ValueString())
	if err != nil {
		// If the record was deleted outside Terraform, remove it from state
		// so Terraform knows it needs to be recreated.
		if _, ok := err.(*unifi.NotFoundError); ok {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading DNS Record",
			fmt.Sprintf("Could not read DNS record %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	r.apiToModel(record, &state, site)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is called when Terraform detects that the planned values differ from current
// state. The UniFi API requires sending complete objects on PUT (no partial updates),
// so we merge the planned changes into the current state before sending.
//
// The flow:
//  1. Read current state (what the API returned last time)
//  2. Read planned values (what the user wants)
//  3. Merge plan into state (applyPlanToState) — this preserves API-set fields
//     that the user didn't specify, while applying the user's changes
//  4. Convert to API struct and send the update
//  5. Save the API response back to state
func (r *dnsRecordResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	var state, plan dnsRecordResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.applyPlanToState(&plan, &state)

	site := r.client.SiteOrDefault(state.Site)
	record := r.modelToAPI(&state)
	record.ID = state.ID.ValueString()

	updated, err := r.client.UpdateDNSRecord(ctx, site, record)
	if err != nil {
		resp.Diagnostics.AddError("Error Updating DNS Record", err.Error())
		return
	}

	r.apiToModel(updated, &state, site)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Delete removes the DNS record from the UniFi controller.
func (r *dnsRecordResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	var state dnsRecordResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	site := r.client.SiteOrDefault(state.Site)

	err := r.client.DeleteDNSRecord(ctx, site, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error Deleting DNS Record", err.Error())
	}
}

// ImportState handles `terraform import terrifi_dns_record.name <id>`.
// We support two formats:
//   - "site:id" — import from a specific site
//   - "id"      — import from the provider's default site
func (r *dnsRecordResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	parts := strings.SplitN(req.ID, ":", 2)

	if len(parts) == 2 {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("site"), parts[0])...)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
		return
	}

	// ImportStatePassthroughID is a framework helper that sets the "id" attribute
	// from the import ID string. After this, Read() will be called to populate the rest.
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// ---------------------------------------------------------------------------
// Helper methods
// ---------------------------------------------------------------------------

// applyPlanToState merges the user's planned changes into the current state.
// For each field, if the plan has a concrete value (not null, not unknown), we
// copy it to state. Otherwise we keep the existing state value.
//
// This is necessary because the UniFi API returns complete objects, and some fields
// may be set by the API that the user didn't specify. We don't want to accidentally
// send zero values for those fields on update.
func (r *dnsRecordResource) applyPlanToState(plan, state *dnsRecordResourceModel) {
	if !plan.Name.IsNull() && !plan.Name.IsUnknown() {
		state.Name = plan.Name
	}
	if !plan.Enabled.IsNull() && !plan.Enabled.IsUnknown() {
		state.Enabled = plan.Enabled
	}
	if !plan.Port.IsNull() && !plan.Port.IsUnknown() {
		state.Port = plan.Port
	}
	if !plan.Priority.IsNull() && !plan.Priority.IsUnknown() {
		state.Priority = plan.Priority
	}
	if !plan.RecordType.IsNull() && !plan.RecordType.IsUnknown() {
		state.RecordType = plan.RecordType
	}
	if !plan.TTL.IsNull() && !plan.TTL.IsUnknown() {
		state.TTL = plan.TTL
	}
	if !plan.Value.IsNull() && !plan.Value.IsUnknown() {
		state.Value = plan.Value
	}
	if !plan.Weight.IsNull() && !plan.Weight.IsUnknown() {
		state.Weight = plan.Weight
	}
}

// modelToAPI converts our Terraform model to the go-unifi DNSRecord struct.
// The go-unifi SDK uses plain Go types (string, bool, int64) and pointers,
// so we need to unwrap the Terraform wrapper types.
//
// Note: the API struct field names don't always match our model names:
//   - model.Name  → API Key    (the hostname)
//   - model.Value → API Value  (the record target)
//   - model.TTL   → API Ttl    (different casing)
func (r *dnsRecordResource) modelToAPI(m *dnsRecordResourceModel) *unifi.DNSRecord {
	rec := &unifi.DNSRecord{
		Key:   m.Name.ValueString(),
		Value: m.Value.ValueString(),
	}

	if !m.Enabled.IsNull() {
		rec.Enabled = m.Enabled.ValueBool()
	}

	// Port is a pointer in the API struct (*int64) because it's truly optional.
	// ValueInt64Pointer returns nil when the Terraform value is null.
	rec.Port = m.Port.ValueInt64Pointer()

	if !m.Priority.IsNull() {
		rec.Priority = m.Priority.ValueInt64()
	}

	if !m.RecordType.IsNull() {
		rec.RecordType = m.RecordType.ValueString()
	}

	if !m.TTL.IsNull() {
		rec.Ttl = m.TTL.ValueInt64()
	}

	if !m.Weight.IsNull() {
		rec.Weight = m.Weight.ValueInt64()
	}

	return rec
}

// apiToModel converts the go-unifi DNSRecord struct back to our Terraform model.
// For optional fields, we set them to types.XXXNull() when the API returns zero values,
// so Terraform doesn't show spurious diffs (e.g., "port: 0 → null").
func (r *dnsRecordResource) apiToModel(rec *unifi.DNSRecord, m *dnsRecordResourceModel, site string) {
	m.ID = types.StringValue(rec.ID)
	m.Site = types.StringValue(site)
	m.Name = types.StringValue(rec.Key)
	m.Value = types.StringValue(rec.Value)
	m.Enabled = types.BoolValue(rec.Enabled)

	if rec.Port != nil && *rec.Port != 0 {
		m.Port = types.Int64PointerValue(rec.Port)
	} else {
		m.Port = types.Int64Null()
	}

	if rec.Priority != 0 {
		m.Priority = types.Int64Value(rec.Priority)
	} else {
		m.Priority = types.Int64Null()
	}

	if rec.RecordType != "" {
		m.RecordType = types.StringValue(rec.RecordType)
	} else {
		m.RecordType = types.StringNull()
	}

	if rec.Ttl != 0 {
		m.TTL = types.Int64Value(rec.Ttl)
	} else {
		m.TTL = types.Int64Null()
	}

	if rec.Weight != 0 {
		m.Weight = types.Int64Value(rec.Weight)
	} else {
		m.Weight = types.Int64Null()
	}
}
