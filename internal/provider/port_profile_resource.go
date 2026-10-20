package provider

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
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
	portProfileDefaultTaggedVLANMgmt   = "auto"
	portProfileDefaultPoeMode          = "auto"
	portProfileDefaultDot1XCtrl        = "force_authorized"
	portProfileDefaultStormControlType = "level"
	portProfileDefaultAutoneg          = true
	portProfileDefaultFlowControl      = true
	portProfileDefaultPTP              = true
	portProfileDefaultStpEnabled       = true
	portProfileDefaultLldpmedEnabled   = true
)

var macAddressRE = regexp.MustCompile(`^([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}$`)

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

	NativeNetworkID    types.String `tfsdk:"native_network_id"`
	TaggedVLANMgmt     types.String `tfsdk:"tagged_vlan_mgmt"`
	TaggedNetworkIDs   types.Set    `tfsdk:"tagged_network_ids"`
	ExcludedNetworkIDs types.Set    `tfsdk:"excluded_network_ids"`
	VoiceNetworkID     types.String `tfsdk:"voice_network_id"`

	PoeMode    types.String `tfsdk:"poe_mode"`
	Autoneg    types.Bool   `tfsdk:"autoneg"`
	Speed      types.Int64  `tfsdk:"speed"`
	FullDuplex types.Bool   `tfsdk:"full_duplex"`

	EgressRateLimitKbps   types.Int64 `tfsdk:"egress_rate_limit_kbps"`
	FlowControlEnabled    types.Bool  `tfsdk:"flow_control_enabled"`
	PTPEnabled            types.Bool  `tfsdk:"ptp_enabled"`
	Isolation             types.Bool  `tfsdk:"isolation"`
	StpEnabled            types.Bool  `tfsdk:"stp_enabled"`
	StpUplink             types.Bool  `tfsdk:"stp_uplink"`
	BpduGuardEnabled      types.Bool  `tfsdk:"bpdu_guard_enabled"`
	LoopProtectionEnabled types.Bool  `tfsdk:"loop_protection_enabled"`
	EEEEnabled            types.Bool  `tfsdk:"eee_enabled"`
	LldpmedEnabled        types.Bool  `tfsdk:"lldpmed_enabled"`
	LinkDebounceMs        types.Int64 `tfsdk:"link_debounce_ms"`

	PortSecurityEnabled      types.Bool   `tfsdk:"port_security_enabled"`
	PortSecurityMACAddresses types.Set    `tfsdk:"port_security_mac_addresses"`
	Dot1XCtrl                types.String `tfsdk:"dot1x_ctrl"`

	StormControl *portProfileStormControlModel `tfsdk:"storm_control"`
}

// portProfileStormControlModel is the nested storm_control block. A null
// traffic class means storm control is disabled for that class.
type portProfileStormControlModel struct {
	Type      types.String `tfsdk:"type"`
	Broadcast types.Int64  `tfsdk:"broadcast"`
	Multicast types.Int64  `tfsdk:"multicast"`
	Unicast   types.Int64  `tfsdk:"unicast"`
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
	emptyStringSet := types.SetValueMust(types.StringType, []attr.Value{})

	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a switch port profile on the UniFi controller. Port profiles are reusable " +
			"port configurations (VLANs, PoE, link speed and the UI's advanced settings) that can be " +
			"assigned to switch and gateway ports.",

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

			"tagged_vlan_mgmt": schema.StringAttribute{
				MarkdownDescription: "Tagged VLAN management. `auto` (UI \"Allow All\") tags every network, " +
					"`block_all` tags none, and `custom` tags the networks given by `tagged_network_ids` " +
					"or `excluded_network_ids`. Default: `auto`.",
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(portProfileDefaultTaggedVLANMgmt),
				Validators: []validator.String{
					stringvalidator.OneOf("auto", "block_all", "custom"),
				},
			},

			"tagged_network_ids": schema.SetAttribute{
				MarkdownDescription: "IDs of the networks tagged on the port, as selected in the UI. Only " +
					"with `tagged_vlan_mgmt = \"custom\"`; conflicts with `excluded_network_ids`. The " +
					"controller stores the inverse (every other corporate network is excluded), so a " +
					"network created later that isn't excluded shows up as a planned change.",
				Optional:    true,
				ElementType: types.StringType,
			},

			"excluded_network_ids": schema.SetAttribute{
				MarkdownDescription: "IDs of the networks **not** tagged on the port, exactly as the " +
					"controller stores them. Only with `tagged_vlan_mgmt = \"custom\"`; conflicts with " +
					"`tagged_network_ids`, which is usually the better choice: with this attribute, a " +
					"network created later is tagged on the port without any planned change.",
				Optional:    true,
				ElementType: types.StringType,
			},

			"voice_network_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the voice VLAN network, advertised over LLDP-MED. Requires " +
					"`lldpmed_enabled`, and must be a network tagged on the port.",
				Optional: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},

			// Link.

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

			"autoneg": boolAttr("Whether link speed and duplex are auto-negotiated. Set to `false` "+
				"to use `speed` and `full_duplex`.", portProfileDefaultAutoneg),

			"speed": schema.Int64Attribute{
				MarkdownDescription: "Fixed link speed in Mbps. Required when `autoneg` is `false`, not " +
					"allowed otherwise. One of: `10`, `100`, `1000`, `2500`, `5000`, `10000`, `20000`, " +
					"`25000`, `40000`, `50000`, `100000`.",
				Optional: true,
				Validators: []validator.Int64{
					int64validator.OneOf(10, 100, 1000, 2500, 5000, 10000, 20000, 25000, 40000, 50000, 100000),
				},
			},

			"full_duplex": boolAttr("Whether a fixed-speed link runs full duplex. Can only be `true` "+
				"when `autoneg` is `false`.", false),

			// Advanced (UI "Advanced: Manual").

			"egress_rate_limit_kbps": schema.Int64Attribute{
				MarkdownDescription: "Egress rate limit in kbps (64–9999999). When unset, egress rate " +
					"limiting is disabled.",
				Optional: true,
				Validators: []validator.Int64{
					int64validator.Between(64, 9999999),
				},
			},

			"flow_control_enabled": boolAttr("Whether flow control is enabled.", portProfileDefaultFlowControl),
			"ptp_enabled":          boolAttr("Whether Precision Time Protocol is enabled.", portProfileDefaultPTP),
			"isolation":            boolAttr("Whether port isolation is enabled.", false),
			"stp_enabled":          boolAttr("Whether spanning tree (STP) is enabled.", portProfileDefaultStpEnabled),
			"stp_uplink":           boolAttr("Whether the port is an STP uplink.", false),
			"bpdu_guard_enabled":   boolAttr("Whether BPDU guard is enabled.", false),
			"loop_protection_enabled": boolAttr("Whether non-STP loop protection is enabled "+
				"(`port_keepalive_enabled` in the API).", false),
			"eee_enabled":     boolAttr("Whether Energy Efficient Ethernet is enabled.", false),
			"lldpmed_enabled": boolAttr("Whether LLDP-MED is enabled.", portProfileDefaultLldpmedEnabled),

			"link_debounce_ms": schema.Int64Attribute{
				MarkdownDescription: "Link debounce. Unset is the UI's \"Auto\", `0` is \"Off\", and " +
					"100–5000 (in steps of 100) is \"Custom\" in milliseconds.",
				Optional: true,
			},

			"port_security_enabled": boolAttr("Whether the MAC address filter (port security) is enabled.", false),

			"port_security_mac_addresses": schema.SetAttribute{
				MarkdownDescription: "MAC addresses allowed by the MAC address filter. Default: `[]`.",
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				Default:             setdefault.StaticValue(emptyStringSet),
				Validators: []validator.Set{
					setvalidator.ValueStringsAre(
						stringvalidator.RegexMatches(macAddressRE, "must be a MAC address like 00:11:22:33:44:55"),
					),
				},
			},

			"dot1x_ctrl": schema.StringAttribute{
				MarkdownDescription: "802.1X control. One of: `auto`, `force_authorized`, `force_unauthorized`, " +
					"`mac_based`, `multi_host` (the UI's \"Multi-auth\"). Default: `force_authorized`.",
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(portProfileDefaultDot1XCtrl),
				Validators: []validator.String{
					stringvalidator.OneOf("auto", "force_authorized", "force_unauthorized", "mac_based", "multi_host"),
				},
			},

			"storm_control": schema.SingleNestedAttribute{
				MarkdownDescription: "Storm control. When unset, storm control is disabled.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{
						MarkdownDescription: "How thresholds are expressed: `level` (the UI's \"percentage\" " +
							"of link bandwidth, 0–100) or `rate` (packets per second, 0–14880000). " +
							"Default: `level`.",
						Optional: true,
						Computed: true,
						Default:  stringdefault.StaticString(portProfileDefaultStormControlType),
						Validators: []validator.String{
							stringvalidator.OneOf("level", "rate"),
						},
					},
					"broadcast": schema.Int64Attribute{
						MarkdownDescription: "Broadcast threshold. When unset, broadcast storm control is disabled.",
						Optional:            true,
					},
					"multicast": schema.Int64Attribute{
						MarkdownDescription: "Multicast threshold. When unset, multicast storm control is disabled.",
						Optional:            true,
					},
					"unicast": schema.Int64Attribute{
						MarkdownDescription: "Unknown-unicast threshold. When unset, unicast storm control is disabled.",
						Optional:            true,
					},
				},
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
	resp.Diagnostics.Append(validatePortProfileModel(ctx, &m)...)
}

// known reports whether an attribute value is usable for validation. Unknown
// values are validated again once known.
func known(v attr.Value) bool { return !v.IsUnknown() }

// setContains reports whether a known set contains s.
func setContains(ctx context.Context, set types.Set, s string) bool {
	var vals []string
	set.ElementsAs(ctx, &vals, false)
	return slices.Contains(vals, s)
}

// validatePortProfileModel holds the cross-attribute checks so they can be
// unit tested without building a tfsdk.Config. Attributes with defaults are
// null in config when the user relies on the default.
func validatePortProfileModel(ctx context.Context, m *portProfileResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	// Link speed.
	if known(m.Autoneg) {
		autoneg := m.Autoneg.IsNull() || m.Autoneg.ValueBool()
		if known(m.Speed) {
			if autoneg && !m.Speed.IsNull() {
				diags.AddAttributeError(path.Root("speed"), "Invalid Attribute Combination",
					"`speed` can only be set when `autoneg` is `false`.")
			}
			if !autoneg && m.Speed.IsNull() {
				diags.AddAttributeError(path.Root("speed"), "Missing Attribute",
					"`speed` is required when `autoneg` is `false`.")
			}
		}
		if autoneg && known(m.FullDuplex) && m.FullDuplex.ValueBool() {
			diags.AddAttributeError(path.Root("full_duplex"), "Invalid Attribute Combination",
				"`full_duplex` can only be `true` when `autoneg` is `false`.")
		}
	}

	// Tagged networks.
	if known(m.TaggedVLANMgmt) && known(m.TaggedNetworkIDs) && known(m.ExcludedNetworkIDs) {
		mgmt := m.TaggedVLANMgmt.ValueString()
		if m.TaggedVLANMgmt.IsNull() {
			mgmt = portProfileDefaultTaggedVLANMgmt
		}
		hasTagged, hasExcluded := !m.TaggedNetworkIDs.IsNull(), !m.ExcludedNetworkIDs.IsNull()
		switch {
		case mgmt != "custom" && (hasTagged || hasExcluded):
			diags.AddAttributeError(path.Root("tagged_vlan_mgmt"), "Invalid Attribute Combination",
				"`tagged_network_ids` and `excluded_network_ids` can only be set when "+
					"`tagged_vlan_mgmt` is `custom`.")
		case mgmt == "custom" && hasTagged == hasExcluded:
			diags.AddAttributeError(path.Root("tagged_vlan_mgmt"), "Invalid Attribute Combination",
				"With `tagged_vlan_mgmt = \"custom\"`, set exactly one of `tagged_network_ids` "+
					"or `excluded_network_ids`.")
		case mgmt == "custom" && hasTagged && len(m.TaggedNetworkIDs.Elements()) == 0:
			// The controller rewrites custom-with-nothing-tagged to block_all.
			diags.AddAttributeError(path.Root("tagged_network_ids"), "Invalid Attribute Value",
				"`tagged_network_ids` can't be empty: use `tagged_vlan_mgmt = \"block_all\"` to tag no networks.")
		}

		if known(m.NativeNetworkID) && !m.NativeNetworkID.IsNull() {
			native := m.NativeNetworkID.ValueString()
			if hasTagged && setContains(ctx, m.TaggedNetworkIDs, native) {
				diags.AddAttributeError(path.Root("tagged_network_ids"), "Invalid Attribute Value",
					"The native network can't also be tagged.")
			}
			if hasExcluded && setContains(ctx, m.ExcludedNetworkIDs, native) {
				diags.AddAttributeError(path.Root("excluded_network_ids"), "Invalid Attribute Value",
					"The native network can't be excluded.")
			}
		}

		// Voice VLAN: must be tagged on the port, and needs LLDP-MED.
		if known(m.VoiceNetworkID) && !m.VoiceNetworkID.IsNull() {
			voice := m.VoiceNetworkID.ValueString()
			switch {
			case mgmt == "block_all":
				diags.AddAttributeError(path.Root("voice_network_id"), "Invalid Attribute Combination",
					"A voice network can't be set when `tagged_vlan_mgmt` is `block_all`.")
			case hasTagged && !setContains(ctx, m.TaggedNetworkIDs, voice):
				diags.AddAttributeError(path.Root("voice_network_id"), "Invalid Attribute Value",
					"The voice network must be one of `tagged_network_ids`.")
			case hasExcluded && setContains(ctx, m.ExcludedNetworkIDs, voice):
				diags.AddAttributeError(path.Root("voice_network_id"), "Invalid Attribute Value",
					"The voice network can't be in `excluded_network_ids`.")
			}
		}
	}
	if known(m.VoiceNetworkID) && !m.VoiceNetworkID.IsNull() && known(m.LldpmedEnabled) &&
		!m.LldpmedEnabled.IsNull() && !m.LldpmedEnabled.ValueBool() {
		diags.AddAttributeError(path.Root("voice_network_id"), "Invalid Attribute Combination",
			"A voice network requires `lldpmed_enabled`.")
	}

	// Link debounce: 0 (off) or 100–5000 in steps of 100 (custom).
	if known(m.LinkDebounceMs) && !m.LinkDebounceMs.IsNull() {
		v := m.LinkDebounceMs.ValueInt64()
		if v != 0 && (v < 100 || v > 5000 || v%100 != 0) {
			diags.AddAttributeError(path.Root("link_debounce_ms"), "Invalid Attribute Value",
				fmt.Sprintf("`link_debounce_ms` must be 0 (off) or 100–5000 in steps of 100, got %d.", v))
		}
	}

	// Storm control.
	if sc := m.StormControl; sc != nil {
		classes := map[string]types.Int64{"broadcast": sc.Broadcast, "multicast": sc.Multicast, "unicast": sc.Unicast}
		anyConfigured := false
		for _, v := range classes {
			if !v.IsNull() {
				anyConfigured = true
			}
		}
		if !anyConfigured {
			diags.AddAttributeError(path.Root("storm_control"), "Missing Attribute",
				"`storm_control` must set at least one of `broadcast`, `multicast`, `unicast`. "+
					"Omit the block to disable storm control.")
		}

		if known(sc.Type) {
			scType := sc.Type.ValueString()
			if sc.Type.IsNull() {
				scType = portProfileDefaultStormControlType
			}
			limit := int64(100)
			if scType == "rate" {
				limit = 14880000
			}
			for name, v := range classes {
				if v.IsNull() || v.IsUnknown() {
					continue
				}
				if v.ValueInt64() < 0 || v.ValueInt64() > limit {
					diags.AddAttributeError(path.Root("storm_control").AtName(name), "Invalid Attribute Value",
						fmt.Sprintf("With `type = %q`, `%s` must be between 0 and %d, got %d.",
							scType, name, limit, v.ValueInt64()))
				}
			}
		}
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
	lan, err := r.lanNetworksIfNeeded(ctx, site, &plan, plan.TaggedVLANMgmt.ValueString())
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

	lan, err := r.lanNetworksIfNeeded(ctx, site, &state, profile.TaggedVLANMgmt)
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
	lan, err := r.lanNetworksIfNeeded(ctx, site, &state, state.TaggedVLANMgmt.ValueString())
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

// usesTaggedList reports whether tagged networks are expressed as the UI's
// allow-list (tagged_network_ids) rather than the literal deny-list. It is the
// default when neither is set yet (e.g. right after import).
func usesTaggedList(m *portProfileResourceModel) bool {
	return m.ExcludedNetworkIDs.IsNull() || !m.TaggedNetworkIDs.IsNull()
}

// lanNetworksIfNeeded lists the site's LAN networks when the allow-list
// conversion needs them: tagged_vlan_mgmt "custom" with tagged_network_ids.
func (r *portProfileResource) lanNetworksIfNeeded(ctx context.Context, site string, m *portProfileResourceModel, taggedVLANMgmt string) ([]string, error) {
	if taggedVLANMgmt != "custom" || !usesTaggedList(m) {
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
	if sent.TaggedVLANMgmt != "custom" || !usesTaggedList(m) || m.TaggedNetworkIDs.IsNull() || m.TaggedNetworkIDs.IsUnknown() {
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

	got, err := r.client.GetPortProfile(ctx, site, written.ID)
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
		TaggedVLANMgmt: m.TaggedVLANMgmt.ValueString(),
		// The controller represents "none" as an empty string, so send ""
		// rather than omitting the key; that clears a previously set value.
		NativeNetworkID: ptrTo(m.NativeNetworkID.ValueString()),
		VoiceNetworkID:  ptrTo(m.VoiceNetworkID.ValueString()),

		PoeMode:    ptrTo(m.PoeMode.ValueString()),
		Autoneg:    ptrTo(m.Autoneg.ValueBool()),
		FullDuplex: ptrTo(m.FullDuplex.ValueBool()),

		FlowControlEnabled:   ptrTo(m.FlowControlEnabled.ValueBool()),
		PTPEnabled:           ptrTo(m.PTPEnabled.ValueBool()),
		Isolation:            ptrTo(m.Isolation.ValueBool()),
		StpPortMode:          ptrTo(m.StpEnabled.ValueBool()),
		StpUplink:            ptrTo(m.StpUplink.ValueBool()),
		StpBpduGuardEnabled:  ptrTo(m.BpduGuardEnabled.ValueBool()),
		PortKeepaliveEnabled: ptrTo(m.LoopProtectionEnabled.ValueBool()),
		EEEEnabled:           ptrTo(m.EEEEnabled.ValueBool()),
		LldpmedEnabled:       ptrTo(m.LldpmedEnabled.ValueBool()),

		PortSecurityEnabled: ptrTo(m.PortSecurityEnabled.ValueBool()),
		Dot1XCtrl:           m.Dot1XCtrl.ValueString(),
	}

	if !m.Speed.IsNull() {
		p.Speed = ptrTo(m.Speed.ValueInt64())
	}

	p.EgressRateLimitKbpsEnabled = ptrTo(!m.EgressRateLimitKbps.IsNull())
	if !m.EgressRateLimitKbps.IsNull() {
		p.EgressRateLimitKbps = ptrTo(m.EgressRateLimitKbps.ValueInt64())
	}

	// Link debounce: unset = auto (the stored ms value is then ignored).
	p.LinkDebounceAuto = ptrTo(m.LinkDebounceMs.IsNull())
	if !m.LinkDebounceMs.IsNull() {
		p.LinkDebounce = ptrTo(m.LinkDebounceMs.ValueInt64())
	}

	// Tagged networks, always sent so switching away from custom clears them.
	p.ExcludedNetworkIDs = []string{}
	if p.TaggedVLANMgmt == "custom" {
		if usesTaggedList(m) {
			var tagged []string
			diags.Append(m.TaggedNetworkIDs.ElementsAs(ctx, &tagged, false)...)
			excluded, err := unifi.ExcludedFromTagged(lan, tagged, m.NativeNetworkID.ValueString())
			if err != nil {
				diags.AddAttributeError(path.Root("tagged_network_ids"), "Invalid Tagged Network", err.Error())
			} else {
				p.ExcludedNetworkIDs = excluded
			}
		} else {
			diags.Append(m.ExcludedNetworkIDs.ElementsAs(ctx, &p.ExcludedNetworkIDs, false)...)
		}
	}

	diags.Append(m.PortSecurityMACAddresses.ElementsAs(ctx, &p.PortSecurityMACAddress, false)...)

	p.StormctrlBcastEnabled = ptrTo(false)
	p.StormctrlMcastEnabled = ptrTo(false)
	p.StormctrlUcastEnabled = ptrTo(false)
	if sc := m.StormControl; sc != nil {
		p.StormctrlType = sc.Type.ValueString()
		setStormControlClass(p.StormctrlType, sc.Broadcast, &p.StormctrlBcastEnabled, &p.StormctrlBcastLevel, &p.StormctrlBcastRate)
		setStormControlClass(p.StormctrlType, sc.Multicast, &p.StormctrlMcastEnabled, &p.StormctrlMcastLevel, &p.StormctrlMcastRate)
		setStormControlClass(p.StormctrlType, sc.Unicast, &p.StormctrlUcastEnabled, &p.StormctrlUcastLevel, &p.StormctrlUcastRate)
	}

	return p, diags
}

// portProfileForward returns the forward mode the controller derives from
// tagged_vlan_mgmt: "native" when all tagged traffic is blocked, otherwise
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
	if scType == "rate" {
		v = rate
	}
	if v == nil {
		return types.Int64Null()
	}
	return types.Int64Value(*v)
}

// apiToModel converts the local PortProfile struct back to our Terraform
// model. Fields missing from the controller response fall back to the UI
// defaults, matching the schema defaults. Values the controller keeps but
// ignores (speed and duplex under autoneg, a voice network without LLDP-MED,
// disabled limits) are read as unset. lan is needed only when m uses
// tagged_network_ids with tagged_vlan_mgmt "custom".
func (r *portProfileResource) apiToModel(p *unifi.PortProfile, m *portProfileResourceModel, site string, lan []string) {
	m.ID = types.StringValue(p.ID)
	m.Site = types.StringValue(site)
	m.Name = types.StringValue(p.Name)

	native := ""
	if p.NativeNetworkID != nil {
		native = *p.NativeNetworkID
	}
	m.NativeNetworkID = nonEmptyStringOrNull(p.NativeNetworkID)
	m.TaggedVLANMgmt = types.StringValue(stringOr(p.TaggedVLANMgmt, portProfileDefaultTaggedVLANMgmt))
	switch {
	case m.TaggedVLANMgmt.ValueString() != "custom":
		m.TaggedNetworkIDs = types.SetNull(types.StringType)
		m.ExcludedNetworkIDs = types.SetNull(types.StringType)
	case usesTaggedList(m):
		m.TaggedNetworkIDs = stringSetValue(unifi.TaggedFromExcluded(lan, p.ExcludedNetworkIDs, native))
		m.ExcludedNetworkIDs = types.SetNull(types.StringType)
	default:
		m.TaggedNetworkIDs = types.SetNull(types.StringType)
		m.ExcludedNetworkIDs = stringSetValue(p.ExcludedNetworkIDs)
	}

	m.LldpmedEnabled = types.BoolValue(boolOr(p.LldpmedEnabled, portProfileDefaultLldpmedEnabled))
	if m.LldpmedEnabled.ValueBool() {
		m.VoiceNetworkID = nonEmptyStringOrNull(p.VoiceNetworkID)
	} else {
		m.VoiceNetworkID = types.StringNull()
	}

	m.PoeMode = types.StringValue(portProfileDefaultPoeMode)
	if p.PoeMode != nil && *p.PoeMode != "" {
		m.PoeMode = types.StringValue(*p.PoeMode)
	}

	m.Autoneg = types.BoolValue(boolOr(p.Autoneg, portProfileDefaultAutoneg))
	m.Speed = types.Int64Null()
	m.FullDuplex = types.BoolValue(false)
	if !m.Autoneg.ValueBool() {
		if p.Speed != nil {
			m.Speed = types.Int64Value(*p.Speed)
		}
		m.FullDuplex = types.BoolValue(boolOr(p.FullDuplex, false))
	}

	if boolOr(p.EgressRateLimitKbpsEnabled, false) && p.EgressRateLimitKbps != nil {
		m.EgressRateLimitKbps = types.Int64Value(*p.EgressRateLimitKbps)
	} else {
		m.EgressRateLimitKbps = types.Int64Null()
	}
	m.FlowControlEnabled = types.BoolValue(boolOr(p.FlowControlEnabled, portProfileDefaultFlowControl))
	m.PTPEnabled = types.BoolValue(boolOr(p.PTPEnabled, portProfileDefaultPTP))
	m.Isolation = types.BoolValue(boolOr(p.Isolation, false))
	m.StpEnabled = types.BoolValue(boolOr(p.StpPortMode, portProfileDefaultStpEnabled))
	m.StpUplink = types.BoolValue(boolOr(p.StpUplink, false))
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

	m.PortSecurityEnabled = types.BoolValue(boolOr(p.PortSecurityEnabled, false))
	m.PortSecurityMACAddresses = stringSetValue(p.PortSecurityMACAddress)
	m.Dot1XCtrl = types.StringValue(stringOr(p.Dot1XCtrl, portProfileDefaultDot1XCtrl))

	scType := stringOr(p.StormctrlType, portProfileDefaultStormControlType)
	sc := &portProfileStormControlModel{
		Type:      types.StringValue(scType),
		Broadcast: stormControlClassToModel(scType, p.StormctrlBcastEnabled, p.StormctrlBcastLevel, p.StormctrlBcastRate),
		Multicast: stormControlClassToModel(scType, p.StormctrlMcastEnabled, p.StormctrlMcastLevel, p.StormctrlMcastRate),
		Unicast:   stormControlClassToModel(scType, p.StormctrlUcastEnabled, p.StormctrlUcastLevel, p.StormctrlUcastRate),
	}
	if sc.Broadcast.IsNull() && sc.Multicast.IsNull() && sc.Unicast.IsNull() {
		m.StormControl = nil
	} else {
		m.StormControl = sc
	}
}

// ---------------------------------------------------------------------------
// Small conversion helpers
// ---------------------------------------------------------------------------

func ptrTo[T any](v T) *T { return &v }

func stringOr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

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
