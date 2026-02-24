package provider

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/ubiquiti-community/go-unifi/unifi"
)

var macRegexp = regexp.MustCompile(`^([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}$`)

var (
	_ resource.Resource                = &clientDeviceResource{}
	_ resource.ResourceWithImportState = &clientDeviceResource{}
)

func NewClientDeviceResource() resource.Resource {
	return &clientDeviceResource{}
}

type clientDeviceResource struct {
	client *Client
}

type clientDeviceResourceModel struct {
	ID                types.String `tfsdk:"id"`
	Site              types.String `tfsdk:"site"`
	MAC               types.String `tfsdk:"mac"`
	Name              types.String `tfsdk:"name"`
	Note              types.String `tfsdk:"note"`
	FixedIP           types.String `tfsdk:"fixed_ip"`
	NetworkID         types.String `tfsdk:"network_id"`
	NetworkOverrideID types.String `tfsdk:"network_override_id"`
	LocalDNSRecord    types.String `tfsdk:"local_dns_record"`
	ClientGroupID     types.String `tfsdk:"client_group_id"`
	Blocked           types.Bool   `tfsdk:"blocked"`
}

func (r *clientDeviceResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_client_device"
}

func (r *clientDeviceResource) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a client device on the UniFi controller. Use this resource to set " +
			"aliases, notes, fixed IPs, VLAN overrides, local DNS records, and blocked status for known clients.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The ID of the client device.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"site": schema.StringAttribute{
				MarkdownDescription: "The site to associate the client device with. Defaults to the provider site.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"mac": schema.StringAttribute{
				MarkdownDescription: "The MAC address of the client device (e.g. `aa:bb:cc:dd:ee:ff`).",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						macRegexp,
						"must be a valid MAC address (e.g. aa:bb:cc:dd:ee:ff)",
					),
				},
			},

			"name": schema.StringAttribute{
				MarkdownDescription: "The alias/display name for the client device.",
				Optional:            true,
			},

			"note": schema.StringAttribute{
				MarkdownDescription: "A free-text note for the client device.",
				Optional:            true,
			},

			"fixed_ip": schema.StringAttribute{
				MarkdownDescription: "A fixed IP address to assign to this client via DHCP reservation. " +
					"Requires `network_id` to also be set.",
				Optional: true,
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.MatchRoot("network_id")),
				},
			},

			"network_id": schema.StringAttribute{
				MarkdownDescription: "The network ID for fixed IP assignment. Required when `fixed_ip` is set.",
				Optional:            true,
			},

			"network_override_id": schema.StringAttribute{
				MarkdownDescription: "The network ID for VLAN/network override. When set, the client " +
					"will be placed on this network regardless of the SSID or port profile it connects to.",
				Optional: true,
			},

			"local_dns_record": schema.StringAttribute{
				MarkdownDescription: "A local DNS hostname for this client device. " +
					"Requires `fixed_ip` to also be set (controller requirement).",
				Optional: true,
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.MatchRoot("fixed_ip")),
				},
			},

			"client_group_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the client group to assign this device to. " +
					"Use `terrifi_client_group` to manage groups.",
				Optional: true,
			},

			"blocked": schema.BoolAttribute{
				MarkdownDescription: "Whether the client device is blocked from network access.",
				Optional:            true,
			},
		},
	}
}

func (r *clientDeviceResource) Configure(
	_ context.Context,
	req resource.ConfigureRequest,
	resp *resource.ConfigureResponse,
) {
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

func (r *clientDeviceResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var plan clientDeviceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save client_group_id before the API call — the API doesn't return
	// usergroup_id in create/update responses, so we restore it after apiToModel.
	plannedGroupID := plan.ClientGroupID

	site := r.client.SiteOrDefault(plan.Site)
	apiObj := r.modelToAPI(&plan)

	created, err := r.client.CreateClientDevice(ctx, site, apiObj)
	if err != nil {
		resp.Diagnostics.AddError("Error Creating Client Device", err.Error())
		return
	}

	r.apiToModel(created, &plan, site)
	plan.ClientGroupID = plannedGroupID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *clientDeviceResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var state clientDeviceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save client_group_id before the API call — the API doesn't return
	// usergroup_id in responses, so we preserve it from prior state.
	priorGroupID := state.ClientGroupID

	site := r.client.SiteOrDefault(state.Site)

	apiObj, err := r.client.GetClientDevice(ctx, site, state.ID.ValueString())
	if err != nil {
		if _, ok := err.(*unifi.NotFoundError); ok {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Client Device",
			fmt.Sprintf("Could not read client device %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	r.apiToModel(apiObj, &state, site)
	state.ClientGroupID = priorGroupID
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *clientDeviceResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	var state, plan clientDeviceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save client_group_id before the API call — the API doesn't return
	// usergroup_id in create/update responses, so we restore it after apiToModel.
	plannedGroupID := plan.ClientGroupID

	r.applyPlanToState(&plan, &state)

	site := r.client.SiteOrDefault(state.Site)
	apiObj := r.modelToAPI(&state)
	apiObj.ID = state.ID.ValueString()

	updated, err := r.client.UpdateClientDevice(ctx, site, apiObj)
	if err != nil {
		resp.Diagnostics.AddError("Error Updating Client Device", err.Error())
		return
	}

	r.apiToModel(updated, &state, site)
	state.ClientGroupID = plannedGroupID
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *clientDeviceResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	var state clientDeviceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	site := r.client.SiteOrDefault(state.Site)

	// Clear all network bindings before deleting. The controller retains DHCP
	// reservations and DNS records even after the user record is removed, which
	// prevents referenced networks from being deleted. Sending an update with
	// all bindings cleared fixes this.
	mac := strings.ToLower(state.MAC.ValueString())
	clearObj := &unifi.Client{
		ID:  state.ID.ValueString(),
		MAC: mac,
	}
	_, err := r.client.UpdateClientDevice(ctx, site, clearObj)
	if err != nil {
		if _, ok := err.(*unifi.NotFoundError); ok {
			// Controller auto-cleaned the user record (common for non-connected
			// MACs), but network references may persist. Re-create a temporary
			// record with cleared bindings so the controller releases them.
			created, createErr := r.client.CreateClientDevice(ctx, site, &unifi.Client{MAC: mac})
			if createErr == nil {
				_ = r.client.DeleteClientDevice(ctx, site, created.ID)
			}
			return
		}
	}

	err = r.client.DeleteClientDevice(ctx, site, state.ID.ValueString())
	if err != nil {
		// Treat "not found" as success — the resource is already gone.
		if _, ok := err.(*unifi.NotFoundError); ok {
			return
		}
		resp.Diagnostics.AddError("Error Deleting Client Device", err.Error())
	}
}

func (r *clientDeviceResource) ImportState(
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

	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// ---------------------------------------------------------------------------
// Helper methods
// ---------------------------------------------------------------------------

func (r *clientDeviceResource) applyPlanToState(plan, state *clientDeviceResourceModel) {
	if !plan.MAC.IsNull() && !plan.MAC.IsUnknown() {
		state.MAC = plan.MAC
	}
	if !plan.Name.IsNull() && !plan.Name.IsUnknown() {
		state.Name = plan.Name
	} else {
		state.Name = types.StringNull()
	}
	if !plan.Note.IsNull() && !plan.Note.IsUnknown() {
		state.Note = plan.Note
	} else {
		state.Note = types.StringNull()
	}
	if !plan.FixedIP.IsNull() && !plan.FixedIP.IsUnknown() {
		state.FixedIP = plan.FixedIP
	} else {
		state.FixedIP = types.StringNull()
	}
	if !plan.NetworkID.IsNull() && !plan.NetworkID.IsUnknown() {
		state.NetworkID = plan.NetworkID
	} else {
		state.NetworkID = types.StringNull()
	}
	if !plan.NetworkOverrideID.IsNull() && !plan.NetworkOverrideID.IsUnknown() {
		state.NetworkOverrideID = plan.NetworkOverrideID
	} else {
		state.NetworkOverrideID = types.StringNull()
	}
	if !plan.LocalDNSRecord.IsNull() && !plan.LocalDNSRecord.IsUnknown() {
		state.LocalDNSRecord = plan.LocalDNSRecord
	} else {
		state.LocalDNSRecord = types.StringNull()
	}
	if !plan.ClientGroupID.IsNull() && !plan.ClientGroupID.IsUnknown() {
		state.ClientGroupID = plan.ClientGroupID
	} else {
		state.ClientGroupID = types.StringNull()
	}
	if !plan.Blocked.IsNull() && !plan.Blocked.IsUnknown() {
		state.Blocked = plan.Blocked
	} else {
		state.Blocked = types.BoolNull()
	}
}

func (r *clientDeviceResource) modelToAPI(m *clientDeviceResourceModel) *unifi.Client {
	c := &unifi.Client{
		MAC: strings.ToLower(m.MAC.ValueString()),
	}

	if !m.Name.IsNull() && !m.Name.IsUnknown() {
		c.Name = m.Name.ValueString()
	}

	if !m.Note.IsNull() && !m.Note.IsUnknown() {
		c.Note = m.Note.ValueString()
	}

	if !m.FixedIP.IsNull() && !m.FixedIP.IsUnknown() {
		c.FixedIP = m.FixedIP.ValueString()
		c.UseFixedIP = true
		if !m.NetworkID.IsNull() && !m.NetworkID.IsUnknown() {
			c.NetworkID = m.NetworkID.ValueString()
		}
	}

	if !m.NetworkOverrideID.IsNull() && !m.NetworkOverrideID.IsUnknown() {
		c.VirtualNetworkOverrideID = m.NetworkOverrideID.ValueString()
		c.VirtualNetworkOverrideEnabled = boolPtr(true)
	}

	if !m.LocalDNSRecord.IsNull() && !m.LocalDNSRecord.IsUnknown() {
		c.LocalDNSRecord = m.LocalDNSRecord.ValueString()
		c.LocalDNSRecordEnabled = true
	}

	if !m.ClientGroupID.IsNull() && !m.ClientGroupID.IsUnknown() {
		c.UserGroupID = m.ClientGroupID.ValueString()
	}

	if !m.Blocked.IsNull() && !m.Blocked.IsUnknown() {
		v := m.Blocked.ValueBool()
		c.Blocked = &v
	}

	return c
}

func (r *clientDeviceResource) apiToModel(c *unifi.Client, m *clientDeviceResourceModel, site string) {
	m.ID = types.StringValue(c.ID)
	m.Site = types.StringValue(site)
	m.MAC = types.StringValue(c.MAC)

	m.Name = stringValueOrNull(c.Name)
	m.Note = stringValueOrNull(c.Note)

	// Only populate fixed IP when the controller says it's enabled and has a value.
	if c.UseFixedIP && c.FixedIP != "" {
		m.FixedIP = types.StringValue(c.FixedIP)
		m.NetworkID = stringValueOrNull(c.NetworkID)
	} else {
		m.FixedIP = types.StringNull()
		m.NetworkID = types.StringNull()
	}

	// Only populate network override when enabled and has a value.
	if c.VirtualNetworkOverrideEnabled != nil && *c.VirtualNetworkOverrideEnabled && c.VirtualNetworkOverrideID != "" {
		m.NetworkOverrideID = types.StringValue(c.VirtualNetworkOverrideID)
	} else {
		m.NetworkOverrideID = types.StringNull()
	}

	// Only populate local DNS record when enabled and has a value.
	if c.LocalDNSRecordEnabled && c.LocalDNSRecord != "" {
		m.LocalDNSRecord = types.StringValue(c.LocalDNSRecord)
	} else {
		m.LocalDNSRecord = types.StringNull()
	}

	m.ClientGroupID = stringValueOrNull(c.UserGroupID)

	// Preserve blocked state faithfully: true or false when explicitly set by
	// the API, null when the API doesn't return the field at all.
	if c.Blocked != nil {
		m.Blocked = types.BoolValue(*c.Blocked)
	} else {
		m.Blocked = types.BoolNull()
	}
}
