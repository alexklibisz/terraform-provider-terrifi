package provider

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/alexklibisz/terrifi/internal/unifi"
)

// Compile-time interface checks.
var (
	_ resource.Resource                   = &portProfileResource{}
	_ resource.ResourceWithImportState    = &portProfileResource{}
	_ resource.ResourceWithValidateConfig = &portProfileResource{}
)

// Defaults of a port profile created in the UniFi UI (UniFi OS 5.1 /
// Network 10.x). Attributes with defaults are always sent explicitly, so
// removing one from the config reverts it to the UI default.
const (
	portProfileDefaultTaggedVLANMgmt   = "allow_all"
	portProfileDefaultPoeMode          = "auto"
	portProfileDefaultLldpmedEnabled   = true
	portProfileDefaultStormControlType = "percentage"
	portProfileDefaultFlowControl      = true
	portProfileDefaultPTP              = true
	portProfileDefaultStpEnabled       = true
)

// NewPortProfileResource is the factory function registered in provider.Resources().
func NewPortProfileResource() resource.Resource {
	return &portProfileResource{}
}

// portProfileResource holds the API client, injected by Configure().
type portProfileResource struct {
	client *Client
}

// portProfileResourceModel is the Terraform-side representation of a port profile.
type portProfileResourceModel struct {
	ID   types.String `tfsdk:"id"`
	Site types.String `tfsdk:"site"`
	Name types.String `tfsdk:"name"`

	NativeNetworkID  types.String `tfsdk:"native_network_id"`
	TaggedVLANMgmt   types.String `tfsdk:"tagged_vlan_mode"`
	TaggedNetworkIDs types.Set    `tfsdk:"tagged_network_ids"`

	PoeMode                    types.String `tfsdk:"poe_mode"`
	Isolation                  types.Bool   `tfsdk:"port_isolation_enabled"`
	LldpmedEnabled             types.Bool   `tfsdk:"lldpmed_enabled"`
	FlowControlEnabled         types.Bool   `tfsdk:"flow_control_enabled"`
	PTPEnabled                 types.Bool   `tfsdk:"ptp_enabled"`
	StpEnabled                 types.Bool   `tfsdk:"stp_enabled"`
	BpduGuardEnabled           types.Bool   `tfsdk:"bpdu_guard_enabled"`
	LoopProtectionEnabled      types.Bool   `tfsdk:"loop_protection_enabled"`
	EEEEnabled                 types.Bool   `tfsdk:"eee_enabled"`
	LinkDebounceMs             types.Int64  `tfsdk:"link_debounce_ms"`
	StormControlType           types.String `tfsdk:"storm_control_type"`
	StormControlBroadcast      types.Int64  `tfsdk:"storm_control_broadcast"`
	StormControlMulticast      types.Int64  `tfsdk:"storm_control_multicast"`
	StormControlUnknownUnicast types.Int64  `tfsdk:"storm_control_unknown_unicast"`
}

// Metadata sets the resource type name.
func (r *portProfileResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_port_profile"
}

func boolAttr(description string, def bool) schema.BoolAttribute {
	return schema.BoolAttribute{
		MarkdownDescription: fmt.Sprintf("%s Default: `%t`.", description, def),
		Optional:            true,
		Computed:            true,
		Default:             booldefault.StaticBool(def),
	}
}

// Schema defines the HCL schema for the terrifi_port_profile resource.
func (r *portProfileResource) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a switch port profile on the UniFi controller. Port profiles are reusable " +
			"port configurations (VLANs, PoE, link tuning, loop protection, storm control and LLDP-MED) that can be assigned to switch " +
			"and gateway ports. Other profile settings are not managed and keep their current values.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The ID of the port profile.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"site": schema.StringAttribute{
				MarkdownDescription: "The site to associate the port profile with. Defaults to the provider site.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the port profile.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 128),
				},
			},

			// VLANs.

			"native_network_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the native (untagged) network. When unset, the port has no " +
					"native network (the UI's \"None\"). Note that the UI defaults new profiles to the " +
					"Default network instead.",
				Optional: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},

			"tagged_vlan_mode": schema.StringAttribute{
				MarkdownDescription: "Tagged VLAN management. `allow_all` (UI \"Allow All\") tags every network, " +
					"`block_all` tags none, and `custom` tags the networks given by `tagged_network_ids`. " +
					"Default: `allow_all`.",
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(portProfileDefaultTaggedVLANMgmt),
				Validators: []validator.String{
					stringvalidator.OneOf("allow_all", "block_all", "custom"),
				},
			},

			"tagged_network_ids": schema.SetAttribute{
				MarkdownDescription: "IDs of the networks tagged on the port, as selected in the UI. Required " +
					"with `tagged_vlan_mode = \"custom\"`, not allowed otherwise. The controller stores the " +
					"inverse (every other LAN network is excluded). New networks are excluded automatically, " +
					"matching an unchanged allow-list once the controller finishes processing.",
				Optional:    true,
				ElementType: types.StringType,
			},

			"poe_mode": schema.StringAttribute{
				MarkdownDescription: "PoE mode. `auto` is the UI's \"Auto PoE\" checkbox ticked, `off` unticked. " +
					"Default: `auto`.",
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(portProfileDefaultPoeMode),
				Validators: []validator.String{
					stringvalidator.OneOf("auto", "off"),
				},
			},

			// Advanced (UI "Advanced: Manual").

			"flow_control_enabled":   boolAttr("Whether flow control is enabled.", portProfileDefaultFlowControl),
			"ptp_enabled":            boolAttr("Whether Precision Time Protocol is enabled.", portProfileDefaultPTP),
			"port_isolation_enabled": boolAttr("Whether port isolation is enabled.", false),
			"stp_enabled":            boolAttr("Whether spanning tree (STP) is enabled.", portProfileDefaultStpEnabled),
			"bpdu_guard_enabled":     boolAttr("Whether BPDU guard is enabled.", false),
			"loop_protection_enabled": boolAttr("Whether non-STP loop protection is enabled "+
				"(`port_keepalive_enabled` in the API).", false),
			"eee_enabled":     boolAttr("Whether Energy Efficient Ethernet is enabled.", false),
			"lldpmed_enabled": boolAttr("Whether LLDP-MED is enabled.", portProfileDefaultLldpmedEnabled),

			"link_debounce_ms": schema.Int64Attribute{
				MarkdownDescription: "Link debounce. Unset is the UI's \"Auto\", `0` is \"Off\", and " +
					"100–5000 (in steps of 100) is \"Custom\" in milliseconds.",
				Optional: true,
			},

			"storm_control_type": schema.StringAttribute{
				MarkdownDescription: "Storm-control threshold unit: `percentage` or `packets_per_second`. Default: `percentage`. With no thresholds, storm control is disabled.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(portProfileDefaultStormControlType),
				Validators:          []validator.String{stringvalidator.OneOf("percentage", "packets_per_second")},
			},
			"storm_control_broadcast": schema.Int64Attribute{
				MarkdownDescription: "Broadcast threshold (0–100 percent or 0–14880000 packets per second). Unset disables broadcast storm control.",
				Optional:            true,
			},
			"storm_control_multicast": schema.Int64Attribute{
				MarkdownDescription: "Multicast threshold (0–100 percent or 0–14880000 packets per second). Unset disables multicast storm control.",
				Optional:            true,
			},
			"storm_control_unknown_unicast": schema.Int64Attribute{
				MarkdownDescription: "Unknown-unicast threshold (0–100 percent or 0–14880000 packets per second). Unset disables unknown-unicast storm control.",
				Optional:            true,
			},
		},
	}
}

// ValidateConfig enforces cross-attribute constraints the UI enforces, which
// the controller would otherwise accept and then ignore or rewrite (showing
// up as perpetual diffs).
func (r *portProfileResource) ValidateConfig(
	ctx context.Context,
	req resource.ValidateConfigRequest,
	resp *resource.ValidateConfigResponse,
) {
	var m portProfileResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(validatePortProfileModel(&m)...)
}

// known reports whether an attribute value is usable for validation. Unknown
// values are validated again once known.
func known(v attr.Value) bool { return !v.IsUnknown() }

// setContains reports whether set definitely contains s. A known match is
// conclusive even if other elements are unknown.
func setContains(set types.Set, s string) bool {
	if set.IsUnknown() {
		return false
	}
	for _, element := range set.Elements() {
		if !element.IsUnknown() && !element.IsNull() && element.(types.String).ValueString() == s {
			return true
		}
	}
	return false
}

// validatePortProfileModel holds the cross-attribute checks so they can be
// unit tested without building a tfsdk.Config. Attributes with defaults are
// null in config when the user relies on the default.
func validatePortProfileModel(m *portProfileResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	// Link debounce: 0 (off) or 100–5000 in steps of 100 (custom).
	if known(m.LinkDebounceMs) && !m.LinkDebounceMs.IsNull() {
		v := m.LinkDebounceMs.ValueInt64()
		if v != 0 && (v < 100 || v > 5000 || v%100 != 0) {
			diags.AddAttributeError(path.Root("link_debounce_ms"), "Invalid Attribute Value",
				fmt.Sprintf("`link_debounce_ms` must be 0 (off) or 100–5000 in steps of 100, got %d.", v))
		}
	}

	if known(m.StormControlType) {
		unit := m.StormControlType.ValueString()
		if m.StormControlType.IsNull() {
			unit = portProfileDefaultStormControlType
		}
		limit := int64(100)
		if unit == "packets_per_second" {
			limit = 14880000
		}
		for name, v := range map[string]types.Int64{
			"storm_control_broadcast":       m.StormControlBroadcast,
			"storm_control_multicast":       m.StormControlMulticast,
			"storm_control_unknown_unicast": m.StormControlUnknownUnicast,
		} {
			if v.IsNull() || v.IsUnknown() {
				continue
			}
			if v.ValueInt64() < 0 || v.ValueInt64() > limit {
				diags.AddAttributeError(path.Root(name), "Invalid Attribute Value", fmt.Sprintf("With `storm_control_type = %q`, `%s` must be between 0 and %d, got %d.", unit, name, limit, v.ValueInt64()))
			}
		}
	}

	if !known(m.TaggedVLANMgmt) || !known(m.TaggedNetworkIDs) {
		return diags
	}

	mgmt := m.TaggedVLANMgmt.ValueString()
	if m.TaggedVLANMgmt.IsNull() {
		mgmt = portProfileDefaultTaggedVLANMgmt
	}
	hasTagged := !m.TaggedNetworkIDs.IsNull()
	switch {
	case mgmt != "custom" && hasTagged:
		diags.AddAttributeError(path.Root("tagged_network_ids"), "Invalid Attribute Combination",
			"`tagged_network_ids` can only be set when `tagged_vlan_mode` is `custom`.")
	case mgmt == "custom" && !hasTagged:
		diags.AddAttributeError(path.Root("tagged_network_ids"), "Missing Attribute",
			"`tagged_network_ids` is required when `tagged_vlan_mode` is `custom`.")
	case mgmt == "custom" && len(m.TaggedNetworkIDs.Elements()) == 0:
		// The controller rewrites custom-with-nothing-tagged to block_all.
		diags.AddAttributeError(path.Root("tagged_network_ids"), "Invalid Attribute Value",
			"`tagged_network_ids` can't be empty: use `tagged_vlan_mode = \"block_all\"` to tag no networks.")
	}

	if hasTagged && known(m.NativeNetworkID) && !m.NativeNetworkID.IsNull() &&
		setContains(m.TaggedNetworkIDs, m.NativeNetworkID.ValueString()) {
		diags.AddAttributeError(path.Root("tagged_network_ids"), "Invalid Attribute Value",
			"The native network can't also be tagged.")
	}
	return diags
}

// Configure is called by the framework to inject the provider's API client.
func (r *portProfileResource) Configure(
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

// Create creates a new port profile.
func (r *portProfileResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var plan portProfileResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	site := r.client.SiteOrDefault(plan.Site)
	lan, err := r.lanNetworksIfNeeded(ctx, site, plan.TaggedVLANMgmt.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error Listing Networks", err.Error())
		return
	}

	profile, diags := r.modelToAPI(ctx, &plan, lan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreatePortProfile(ctx, site, profile)
	if err != nil {
		resp.Diagnostics.AddError("Error Creating Port Profile", err.Error())
		return
	}
	profile.ID = created.ID
	// Persist the successful creation before any fallible verification. Terraform
	// must retain the ID even if cancellation or a controller error interrupts it.
	createdState := plan
	r.apiToModel(created, &createdState, site, lan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &createdState)...)
	if resp.Diagnostics.HasError() {
		return
	}
	created, err = r.settleFreshTaggedNetworks(ctx, site, &plan, profile, created)
	if err != nil {
		resp.Diagnostics.AddError("Error Creating Port Profile", err.Error())
		return
	}

	r.apiToModel(created, &plan, site, lan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read refreshes the Terraform state from the actual API state.
func (r *portProfileResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var state portProfileResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	site := r.client.SiteOrDefault(state.Site)

	profile, err := r.client.GetPortProfile(ctx, site, state.ID.ValueString())
	if err != nil {
		var nf *unifi.NotFoundError
		if errors.As(err, &nf) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Port Profile",
			fmt.Sprintf("Could not read port profile %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	lan, err := r.lanNetworksIfNeeded(ctx, site, unifi.TaggedVLANModeFromAPI(profile.TaggedVLANMgmt))
	if err != nil {
		resp.Diagnostics.AddError("Error Listing Networks", err.Error())
		return
	}

	r.apiToModel(profile, &state, site, lan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update updates an existing port profile.
func (r *portProfileResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	var state, plan portProfileResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.applyPlanToState(&plan, &state)

	site := r.client.SiteOrDefault(state.Site)
	lan, err := r.lanNetworksIfNeeded(ctx, site, state.TaggedVLANMgmt.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error Listing Networks", err.Error())
		return
	}

	profile, diags := r.modelToAPI(ctx, &state, lan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	profile.ID = state.ID.ValueString()

	updated, err := r.client.UpdatePortProfile(ctx, site, profile)
	if err != nil {
		resp.Diagnostics.AddError("Error Updating Port Profile", err.Error())
		return
	}
	updated, err = r.settleFreshTaggedNetworks(ctx, site, &state, profile, updated)
	if err != nil {
		resp.Diagnostics.AddError("Error Updating Port Profile", err.Error())
		return
	}

	r.apiToModel(updated, &state, site, lan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Delete removes the port profile from the UniFi controller.
func (r *portProfileResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	var state portProfileResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	site := r.client.SiteOrDefault(state.Site)

	err := r.client.DeletePortProfile(ctx, site, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error Deleting Port Profile", err.Error())
	}
}

// ImportState handles `terraform import terrifi_port_profile.name <id>`.
// Supports both "id" and "site:id" formats.
func (r *portProfileResource) ImportState(
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

// lanNetworksIfNeeded lists the site's LAN networks when the allow-list
// conversion needs them: tagged_vlan_mode "custom".
func (r *portProfileResource) lanNetworksIfNeeded(ctx context.Context, site, taggedVLANMgmt string) ([]string, error) {
	if taggedVLANMgmt != "custom" {
		return nil, nil
	}
	return r.client.ListLANNetworkIDs(ctx, site)
}

// When a network is created, the controller excludes it from every custom
// port profile that exists at the time, in a background job that runs about a
// second later. A profile written within that window, typically one created
// in the same apply as a network it tags, loses the tag. Only networks created
// this recently can be affected, so nothing waits otherwise.
var (
	// portProfileFreshNetworkAge is how recently a tagged network must have
	// been created (per its ID) to be checked. It is generous so that clock
	// skew between Terraform and the controller doesn't hide a fresh network.
	portProfileFreshNetworkAge = time.Minute
	// portProfileSettleDelay is how long to wait after the write before
	// checking. The background job ran within about 1.2s in testing, and the
	// write always happens after the network is created.
	portProfileSettleDelay = 3 * time.Second
	portProfileNow         = time.Now
)

// settleFreshTaggedNetworks re-checks a profile just written (as sent, stored
// as written) when any of its tagged networks was just created. If the
// controller's background job has excluded one of them since, it writes the
// profile once more. It returns the profile as now stored.
func (r *portProfileResource) settleFreshTaggedNetworks(ctx context.Context, site string, m *portProfileResourceModel, sent, written *unifi.PortProfile) (*unifi.PortProfile, error) {
	if sent.TaggedVLANMgmt != "custom" || m.TaggedNetworkIDs.IsNull() || m.TaggedNetworkIDs.IsUnknown() {
		return written, nil
	}
	var tagged []string
	if diags := m.TaggedNetworkIDs.ElementsAs(ctx, &tagged, false); diags.HasError() {
		return written, nil
	}
	if !anyCreatedWithin(tagged, portProfileNow(), portProfileFreshNetworkAge) {
		return written, nil
	}

	tflog.Debug(ctx, "Port profile tags a newly created network, waiting for the controller to settle",
		map[string]any{"profile_id": written.ID, "delay": portProfileSettleDelay.String()})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(portProfileSettleDelay):
	}

	got, err := r.client.getPortProfile(ctx, site, written.ID, false)
	if err != nil {
		return nil, err
	}
	var lost []string
	for _, id := range tagged {
		if slices.Contains(got.ExcludedNetworkIDs, id) {
			lost = append(lost, id)
		}
	}
	if len(lost) == 0 {
		return got, nil
	}

	tflog.Info(ctx, "Controller excluded newly created networks from the port profile, writing it again",
		map[string]any{"profile_id": written.ID, "network_ids": lost})
	return r.client.UpdatePortProfile(ctx, site, sent)
}

// anyCreatedWithin reports whether any of the IDs was created within d of now,
// in either direction (the controller's clock may be ahead).
func anyCreatedWithin(ids []string, now time.Time, d time.Duration) bool {
	for _, id := range ids {
		if created, ok := unifi.IDTime(id); ok {
			if age := now.Sub(created); age < d && age > -d {
				return true
			}
		}
	}
	return false
}

// applyPlanToState copies the plan into the state. Unlike most resources, a
// null in the plan is copied too: every attribute is either defaulted or
// optional-with-null-meaning-off, so null is a real instruction. Controller
// fields the provider doesn't manage are preserved by UpdatePortProfile's
// read-modify-write instead.
func (r *portProfileResource) applyPlanToState(plan, state *portProfileResourceModel) {
	id, site := state.ID, state.Site
	*state = *plan
	state.ID = id
	if !plan.Site.IsNull() && !plan.Site.IsUnknown() {
		state.Site = plan.Site
	} else {
		state.Site = site
	}
}

// modelToAPI converts our Terraform model to the local PortProfile struct.
// lan is the site's LAN network IDs, needed only for tagged_network_ids.
func (r *portProfileResource) modelToAPI(ctx context.Context, m *portProfileResourceModel, lan []string) (*unifi.PortProfile, diag.Diagnostics) {
	var diags diag.Diagnostics

	p := &unifi.PortProfile{
		Name: m.Name.ValueString(),
		// The advanced settings only apply under "manual", so it is sent on
		// every create and update, including for profiles imported as "auto".
		SettingPreference: "manual",

		Forward:        portProfileForward(m.TaggedVLANMgmt.ValueString()),
		TaggedVLANMgmt: unifi.TaggedVLANModeToAPI(m.TaggedVLANMgmt.ValueString()),
		// The controller represents "none" as an empty string, so send ""
		// rather than omitting the key; that clears a previously set value.
		NativeNetworkID: ptrTo(m.NativeNetworkID.ValueString()),

		FlowControlEnabled:   ptrTo(m.FlowControlEnabled.ValueBool()),
		PTPEnabled:           ptrTo(m.PTPEnabled.ValueBool()),
		StpPortMode:          ptrTo(m.StpEnabled.ValueBool()),
		StpBpduGuardEnabled:  ptrTo(m.BpduGuardEnabled.ValueBool()),
		PortKeepaliveEnabled: ptrTo(m.LoopProtectionEnabled.ValueBool()),
		EEEEnabled:           ptrTo(m.EEEEnabled.ValueBool()),

		PoeMode:        ptrTo(m.PoeMode.ValueString()),
		Isolation:      ptrTo(m.Isolation.ValueBool()),
		LldpmedEnabled: ptrTo(m.LldpmedEnabled.ValueBool()),
	}

	p.LinkDebounceAuto = ptrTo(m.LinkDebounceMs.IsNull())
	if !m.LinkDebounceMs.IsNull() {
		p.LinkDebounce = ptrTo(m.LinkDebounceMs.ValueInt64())
	}
	p.StormctrlType = unifi.StormControlTypeToAPI(m.StormControlType.ValueString())
	p.StormctrlBcastEnabled = ptrTo(false)
	p.StormctrlMcastEnabled = ptrTo(false)
	p.StormctrlUcastEnabled = ptrTo(false)
	setStormControlClass(p.StormctrlType, m.StormControlBroadcast, &p.StormctrlBcastEnabled, &p.StormctrlBcastLevel, &p.StormctrlBcastRate)
	setStormControlClass(p.StormctrlType, m.StormControlMulticast, &p.StormctrlMcastEnabled, &p.StormctrlMcastLevel, &p.StormctrlMcastRate)
	setStormControlClass(p.StormctrlType, m.StormControlUnknownUnicast, &p.StormctrlUcastEnabled, &p.StormctrlUcastLevel, &p.StormctrlUcastRate)

	// Tagged networks, always sent so switching away from custom clears them.
	p.ExcludedNetworkIDs = []string{}
	if p.TaggedVLANMgmt == "custom" {
		var tagged []string
		diags.Append(m.TaggedNetworkIDs.ElementsAs(ctx, &tagged, false)...)
		excluded, err := unifi.ExcludedFromTagged(lan, tagged, m.NativeNetworkID.ValueString())
		if err != nil {
			diags.AddAttributeError(path.Root("tagged_network_ids"), "Invalid Tagged Network", err.Error())
		} else {
			p.ExcludedNetworkIDs = excluded
		}
	}

	return p, diags
}

// portProfileForward returns the forward mode the controller derives from
// tagged_vlan_mode: "native" when all tagged traffic is blocked, otherwise
// "customize". The controller computes the field itself (it also stores "all"
// for native = Default network with auto tagging), so the provider sends a
// consistent value but never exposes or reads it.
func portProfileForward(taggedVLANMgmt string) string {
	if taggedVLANMgmt == "block_all" {
		return "native"
	}
	return "customize"
}

// setStormControlClass enables one storm control traffic class and stores its
// threshold in the level or rate field, depending on the storm control type.
func setStormControlClass(scType string, v types.Int64, enabled **bool, level, rate **int64) {
	if v.IsNull() {
		return
	}
	*enabled = ptrTo(true)
	if scType == "rate" {
		*rate = ptrTo(v.ValueInt64())
	} else {
		*level = ptrTo(v.ValueInt64())
	}
}

// stormControlClassToModel is the inverse of setStormControlClass.
func stormControlClassToModel(scType string, enabled *bool, level, rate *int64) types.Int64 {
	if enabled == nil || !*enabled {
		return types.Int64Null()
	}
	v := level
	if scType == "packets_per_second" {
		v = rate
	}
	if v == nil {
		return types.Int64Null()
	}
	return types.Int64Value(*v)
}

// apiToModel converts the local PortProfile struct back to our Terraform
// model. Fields missing from the controller response fall back to the UI
// defaults, matching the schema defaults. lan is needed only with
// tagged_vlan_mode "custom".
func (r *portProfileResource) apiToModel(p *unifi.PortProfile, m *portProfileResourceModel, site string, lan []string) {
	m.ID = types.StringValue(p.ID)
	m.Site = types.StringValue(site)
	m.Name = types.StringValue(p.Name)

	native := ""
	if p.NativeNetworkID != nil {
		native = *p.NativeNetworkID
	}
	m.NativeNetworkID = nonEmptyStringOrNull(p.NativeNetworkID)
	m.TaggedVLANMgmt = types.StringValue(unifi.TaggedVLANModeFromAPI(p.TaggedVLANMgmt))
	m.TaggedNetworkIDs = types.SetNull(types.StringType)
	if m.TaggedVLANMgmt.ValueString() == "custom" {
		m.TaggedNetworkIDs = stringSetValue(unifi.TaggedFromExcluded(lan, p.ExcludedNetworkIDs, native))
	}

	m.PoeMode = types.StringValue(portProfileDefaultPoeMode)
	if p.PoeMode != nil && *p.PoeMode != "" {
		m.PoeMode = types.StringValue(*p.PoeMode)
	}
	m.FlowControlEnabled = types.BoolValue(boolOr(p.FlowControlEnabled, portProfileDefaultFlowControl))
	m.PTPEnabled = types.BoolValue(boolOr(p.PTPEnabled, portProfileDefaultPTP))
	m.StpEnabled = types.BoolValue(boolOr(p.StpPortMode, portProfileDefaultStpEnabled))
	m.BpduGuardEnabled = types.BoolValue(boolOr(p.StpBpduGuardEnabled, false))
	m.LoopProtectionEnabled = types.BoolValue(boolOr(p.PortKeepaliveEnabled, false))
	m.EEEEnabled = types.BoolValue(boolOr(p.EEEEnabled, false))

	if boolOr(p.LinkDebounceAuto, true) {
		m.LinkDebounceMs = types.Int64Null()
	} else if p.LinkDebounce != nil {
		m.LinkDebounceMs = types.Int64Value(*p.LinkDebounce)
	} else {
		m.LinkDebounceMs = types.Int64Value(0)
	}

	m.StormControlType = types.StringValue(unifi.StormControlTypeFromAPI(p.StormctrlType))
	m.StormControlBroadcast = stormControlClassToModel(m.StormControlType.ValueString(), p.StormctrlBcastEnabled, p.StormctrlBcastLevel, p.StormctrlBcastRate)
	m.StormControlMulticast = stormControlClassToModel(m.StormControlType.ValueString(), p.StormctrlMcastEnabled, p.StormctrlMcastLevel, p.StormctrlMcastRate)
	m.StormControlUnknownUnicast = stormControlClassToModel(m.StormControlType.ValueString(), p.StormctrlUcastEnabled, p.StormctrlUcastLevel, p.StormctrlUcastRate)

	m.Isolation = types.BoolValue(boolOr(p.Isolation, false))
	m.LldpmedEnabled = types.BoolValue(boolOr(p.LldpmedEnabled, portProfileDefaultLldpmedEnabled))
}

// ---------------------------------------------------------------------------
// Small conversion helpers
// ---------------------------------------------------------------------------

func ptrTo[T any](v T) *T { return &v }

func boolOr(v *bool, def bool) bool {
	if v == nil {
		return def
	}
	return *v
}

func nonEmptyStringOrNull(v *string) types.String {
	if v == nil || *v == "" {
		return types.StringNull()
	}
	return types.StringValue(*v)
}

func stringSetValue(vs []string) types.Set {
	vals := make([]attr.Value, len(vs))
	for i, v := range vs {
		vals[i] = types.StringValue(v)
	}
	return types.SetValueMust(types.StringType, vals)
}
