package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/ubiquiti-community/go-unifi/unifi"
)

var (
	_ resource.Resource                = &wlanResource{}
	_ resource.ResourceWithImportState = &wlanResource{}
)

func NewWLANResource() resource.Resource {
	return &wlanResource{}
}

type wlanResource struct {
	client *Client
}

type wlanResourceModel struct {
	ID             types.String `tfsdk:"id"`
	Site           types.String `tfsdk:"site"`
	Name           types.String `tfsdk:"name"`
	Enabled        types.Bool   `tfsdk:"enabled"`
	Passphrase     types.String `tfsdk:"passphrase"`
	NetworkID      types.String `tfsdk:"network_id"`
	WifiBand       types.String `tfsdk:"wifi_band"`
	Security       types.String `tfsdk:"security"`
	HideSSID       types.Bool   `tfsdk:"hide_ssid"`
	WPAMode        types.String `tfsdk:"wpa_mode"`
	WPA3Support    types.Bool   `tfsdk:"wpa3_support"`
	WPA3Transition types.Bool   `tfsdk:"wpa3_transition"`
}

func (r *wlanResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_wlan"
}

func (r *wlanResource) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a WLAN (WiFi network) on the UniFi controller.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The ID of the WLAN.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"site": schema.StringAttribute{
				MarkdownDescription: "The site to associate the WLAN with. Defaults to the provider site.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"name": schema.StringAttribute{
				MarkdownDescription: "The SSID (network name) of the WLAN. Must be 1-32 characters.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 32),
				},
			},

			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the WLAN is enabled. Default: `true`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},

			"passphrase": schema.StringAttribute{
				MarkdownDescription: "The WPA passphrase for the WLAN. Must be 8-255 characters. Required when security is `wpapsk`.",
				Optional:            true,
				Sensitive:           true,
				Validators: []validator.String{
					stringvalidator.LengthBetween(8, 255),
				},
			},

			"network_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the network to associate with this WLAN.",
				Required:            true,
			},

			"wifi_band": schema.StringAttribute{
				MarkdownDescription: "The WiFi band for this WLAN. Must be `2g`, `5g`, or `both`. Default: `both`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("both"),
				Validators: []validator.String{
					stringvalidator.OneOf("2g", "5g", "both"),
				},
			},

			"security": schema.StringAttribute{
				MarkdownDescription: "The security protocol for this WLAN. Must be `open` or `wpapsk`. Default: `wpapsk`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("wpapsk"),
				Validators: []validator.String{
					stringvalidator.OneOf("open", "wpapsk"),
				},
			},

			"hide_ssid": schema.BoolAttribute{
				MarkdownDescription: "Whether to hide the SSID from broadcast. Default: `false`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},

			"wpa_mode": schema.StringAttribute{
				MarkdownDescription: "The WPA mode for this WLAN. Must be `auto` or `wpa2`. Default: `wpa2`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("wpa2"),
				Validators: []validator.String{
					stringvalidator.OneOf("auto", "wpa2"),
				},
			},

			"wpa3_support": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable WPA3 support. Default: `false`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},

			"wpa3_transition": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable WPA3 transition mode (WPA2/WPA3 mixed). Default: `false`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
		},
	}
}

func (r *wlanResource) Configure(
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

func (r *wlanResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var plan wlanResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	site := r.client.SiteOrDefault(plan.Site)

	// Look up the default WLAN group and user group — the API requires both.
	wlanGroupID, err := r.lookupDefaultWLANGroup(ctx, site)
	if err != nil {
		resp.Diagnostics.AddError("Error Looking Up WLAN Group", err.Error())
		return
	}

	userGroupID, err := r.lookupDefaultUserGroup(ctx, site)
	if err != nil {
		resp.Diagnostics.AddError("Error Looking Up User Group", err.Error())
		return
	}

	apGroupID, err := r.lookupDefaultAPGroup(ctx, site)
	if err != nil {
		resp.Diagnostics.AddError("Error Looking Up AP Group", err.Error())
		return
	}

	// Save passphrase before API call — the API never returns x_passphrase,
	// so we must restore it from the plan after apiToModel.
	plannedPassphrase := plan.Passphrase

	wlan := r.modelToAPI(&plan)
	wlan.WLANGroupID = wlanGroupID
	wlan.UserGroupID = userGroupID
	wlan.ApGroupIDs = []string{apGroupID}
	wlan.ApGroupMode = "all"

	created, err := r.client.CreateWLAN(ctx, site, wlan)
	if err != nil {
		resp.Diagnostics.AddError("Error Creating WLAN", err.Error())
		return
	}

	r.apiToModel(created, &plan, site)
	plan.Passphrase = plannedPassphrase
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *wlanResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var state wlanResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	site := r.client.SiteOrDefault(state.Site)

	wlan, err := r.client.GetWLAN(ctx, site, state.ID.ValueString())
	if err != nil {
		if _, ok := err.(*unifi.NotFoundError); ok {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading WLAN",
			fmt.Sprintf("Could not read WLAN %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	r.apiToModel(wlan, &state, site)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *wlanResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	var state, plan wlanResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save passphrase before API call — the API never returns x_passphrase,
	// so we must restore it from the plan after apiToModel.
	plannedPassphrase := plan.Passphrase

	r.applyPlanToState(&plan, &state)

	site := r.client.SiteOrDefault(state.Site)

	// Read the existing WLAN to preserve fields we don't manage (like wlangroup_id).
	existing, err := r.client.GetWLAN(ctx, site, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error Reading WLAN for Update", err.Error())
		return
	}

	wlan := r.modelToAPI(&state)
	wlan.ID = state.ID.ValueString()
	wlan.WLANGroupID = existing.WLANGroupID
	wlan.UserGroupID = existing.UserGroupID

	updated, err := r.client.UpdateWLAN(ctx, site, wlan)
	if err != nil {
		resp.Diagnostics.AddError("Error Updating WLAN", err.Error())
		return
	}

	r.apiToModel(updated, &state, site)
	state.Passphrase = plannedPassphrase
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *wlanResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	var state wlanResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	site := r.client.SiteOrDefault(state.Site)

	err := r.client.DeleteWLAN(ctx, site, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error Deleting WLAN", err.Error())
	}
}

func (r *wlanResource) ImportState(
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

func (r *wlanResource) lookupDefaultWLANGroup(ctx context.Context, site string) (string, error) {
	groups, err := r.client.ListWLANGroup(ctx, site)
	if err != nil {
		return "", fmt.Errorf("listing WLAN groups: %w", err)
	}
	if len(groups) == 0 {
		return "", fmt.Errorf("no WLAN groups found for site %q", site)
	}
	return groups[0].ID, nil
}

func (r *wlanResource) lookupDefaultUserGroup(ctx context.Context, site string) (string, error) {
	groups, err := r.client.ListClientGroup(ctx, site)
	if err != nil {
		return "", fmt.Errorf("listing client groups: %w", err)
	}
	if len(groups) == 0 {
		return "", fmt.Errorf("no client groups found for site %q", site)
	}
	return groups[0].ID, nil
}

func (r *wlanResource) lookupDefaultAPGroup(ctx context.Context, site string) (string, error) {
	groups, err := r.client.ListAPGroup(ctx, site)
	if err != nil {
		return "", fmt.Errorf("listing AP groups: %w", err)
	}
	if len(groups) == 0 {
		return "", fmt.Errorf("no AP groups found for site %q", site)
	}
	return groups[0].ID, nil
}

func (r *wlanResource) applyPlanToState(plan, state *wlanResourceModel) {
	if !plan.Name.IsNull() && !plan.Name.IsUnknown() {
		state.Name = plan.Name
	}
	if !plan.Enabled.IsNull() && !plan.Enabled.IsUnknown() {
		state.Enabled = plan.Enabled
	}
	// Always apply passphrase from plan — when switching from wpapsk to open,
	// the plan will be null, and we must clear the state value to match.
	if !plan.Passphrase.IsUnknown() {
		state.Passphrase = plan.Passphrase
	}
	if !plan.NetworkID.IsNull() && !plan.NetworkID.IsUnknown() {
		state.NetworkID = plan.NetworkID
	}
	if !plan.WifiBand.IsNull() && !plan.WifiBand.IsUnknown() {
		state.WifiBand = plan.WifiBand
	}
	if !plan.Security.IsNull() && !plan.Security.IsUnknown() {
		state.Security = plan.Security
	}
	if !plan.HideSSID.IsNull() && !plan.HideSSID.IsUnknown() {
		state.HideSSID = plan.HideSSID
	}
	if !plan.WPAMode.IsNull() && !plan.WPAMode.IsUnknown() {
		state.WPAMode = plan.WPAMode
	}
	if !plan.WPA3Support.IsNull() && !plan.WPA3Support.IsUnknown() {
		state.WPA3Support = plan.WPA3Support
	}
	if !plan.WPA3Transition.IsNull() && !plan.WPA3Transition.IsUnknown() {
		state.WPA3Transition = plan.WPA3Transition
	}
}

func (r *wlanResource) modelToAPI(m *wlanResourceModel) *unifi.WLAN {
	wlan := &unifi.WLAN{
		Name:                 m.Name.ValueString(),
		NetworkID:            m.NetworkID.ValueString(),
		ScheduleWithDuration: []unifi.WLANScheduleWithDuration{},
	}

	if !m.Enabled.IsNull() {
		wlan.Enabled = m.Enabled.ValueBool()
	}

	if !m.Passphrase.IsNull() && !m.Passphrase.IsUnknown() {
		wlan.XPassphrase = m.Passphrase.ValueString()
	}

	if !m.WifiBand.IsNull() {
		wlan.WLANBand = m.WifiBand.ValueString()
	}

	if !m.Security.IsNull() {
		wlan.Security = m.Security.ValueString()
	}

	if !m.HideSSID.IsNull() {
		wlan.HideSSID = m.HideSSID.ValueBool()
	}

	if !m.WPAMode.IsNull() {
		wlan.WPAMode = m.WPAMode.ValueString()
	}

	if !m.WPA3Support.IsNull() {
		wlan.WPA3Support = m.WPA3Support.ValueBool()
	}

	if !m.WPA3Transition.IsNull() {
		wlan.WPA3Transition = m.WPA3Transition.ValueBool()
	}

	return wlan
}

func (r *wlanResource) apiToModel(wlan *unifi.WLAN, m *wlanResourceModel, site string) {
	m.ID = types.StringValue(wlan.ID)
	m.Site = types.StringValue(site)
	m.Name = types.StringValue(wlan.Name)
	m.Enabled = types.BoolValue(wlan.Enabled)
	m.NetworkID = types.StringValue(wlan.NetworkID)

	// Never set passphrase from the API response. The passphrase is managed
	// exclusively from the Terraform config/plan. Some controller versions return
	// x_passphrase on GET, others don't — either way, we preserve the value from
	// prior state (in Read) or from the plan (in Create/Update).

	if wlan.WLANBand != "" {
		m.WifiBand = types.StringValue(wlan.WLANBand)
	} else {
		m.WifiBand = types.StringValue("both")
	}

	if wlan.Security != "" {
		m.Security = types.StringValue(wlan.Security)
	} else {
		m.Security = types.StringValue("wpapsk")
	}

	m.HideSSID = types.BoolValue(wlan.HideSSID)

	if wlan.WPAMode != "" {
		m.WPAMode = types.StringValue(wlan.WPAMode)
	} else {
		m.WPAMode = types.StringValue("wpa2")
	}

	m.WPA3Support = types.BoolValue(wlan.WPA3Support)
	m.WPA3Transition = types.BoolValue(wlan.WPA3Transition)
}
