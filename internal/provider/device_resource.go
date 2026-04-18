package provider

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/ubiquiti-community/go-unifi/unifi"
)

var ledColorRegexp = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3}){1,2}$`)

var (
	_ resource.Resource                     = &deviceResource{}
	_ resource.ResourceWithImportState      = &deviceResource{}
	_ resource.ResourceWithConfigValidators = &deviceResource{}
)

func NewDeviceResource() resource.Resource {
	return &deviceResource{}
}

type deviceResource struct {
	client *Client
}

type deviceResourceModel struct {
	ID                  types.String              `tfsdk:"id"`
	Site                types.String              `tfsdk:"site"`
	MAC                 types.String              `tfsdk:"mac"`
	Name                types.String              `tfsdk:"name"`
	LedEnabled          types.Bool                `tfsdk:"led_enabled"`
	LedColor            types.String              `tfsdk:"led_color"`
	LedBrightness       types.Int64               `tfsdk:"led_brightness"`
	OutdoorModeOverride types.String              `tfsdk:"outdoor_mode_override"`
	Locked              types.Bool                `tfsdk:"locked"`
	Disabled            types.Bool                `tfsdk:"disabled"`
	SnmpContact         types.String              `tfsdk:"snmp_contact"`
	SnmpLocation        types.String              `tfsdk:"snmp_location"`
	Volume              types.Int64               `tfsdk:"volume"`
	ConfigNetwork       *deviceConfigNetworkModel `tfsdk:"config_network"`
	// Computed/read-only.
	Model   types.String `tfsdk:"model"`
	Type    types.String `tfsdk:"type"`
	IP      types.String `tfsdk:"ip"`
	Adopted types.Bool   `tfsdk:"adopted"`
	State   types.Int64  `tfsdk:"state"`
}

type deviceConfigNetworkModel struct {
	Type    types.String `tfsdk:"type"`
	IP      types.String `tfsdk:"ip"`
	Netmask types.String `tfsdk:"netmask"`
	Gateway types.String `tfsdk:"gateway"`
	DNS1    types.String `tfsdk:"dns1"`
	DNS2    types.String `tfsdk:"dns2"`
}

func (r *deviceResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_device"
}

func (r *deviceResource) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages settings on an adopted UniFi network device (access point, switch, or gateway). " +
			"The device must already be adopted by the controller. This resource does not adopt or forget devices — " +
			"it only manages configurable properties like name, LED behavior, and SNMP settings. " +
			"Removing the resource from Terraform state does not affect the device on the controller.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The ID of the device.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"site": schema.StringAttribute{
				MarkdownDescription: "The site the device belongs to. Defaults to the provider site.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"mac": schema.StringAttribute{
				MarkdownDescription: "The MAC address of the device (e.g. `aa:bb:cc:dd:ee:ff`). " +
					"The device must already be adopted by the controller.",
				Required: true,
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
				MarkdownDescription: "The display name for the device.",
				Optional:            true,
			},

			"led_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether LEDs are enabled. `true` forces LEDs on, `false` forces LEDs off. " +
					"Omit to follow the site default.",
				Optional: true,
			},

			"led_color": schema.StringAttribute{
				MarkdownDescription: "LED color as a hex string (e.g. `#0000ff`).",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						ledColorRegexp,
						"must be a hex color (e.g. #0000ff)",
					),
				},
			},

			"led_brightness": schema.Int64Attribute{
				MarkdownDescription: "LED brightness (0–100).",
				Optional:            true,
				Validators: []validator.Int64{
					int64validator.Between(0, 100),
				},
			},

			"outdoor_mode_override": schema.StringAttribute{
				MarkdownDescription: "Outdoor mode override. `default` follows the device default, " +
					"`on` enables outdoor mode, `off` disables it.",
				Optional: true,
				Validators: []validator.String{
					stringvalidator.OneOf("default", "on", "off"),
				},
			},

			"locked": schema.BoolAttribute{
				MarkdownDescription: "Whether the device is locked to prevent accidental removal. Defaults to `false`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},

			"disabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the device is administratively disabled. Defaults to `false`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},

			"snmp_contact": schema.StringAttribute{
				MarkdownDescription: "[SNMP](https://en.wikipedia.org/wiki/Simple_Network_Management_Protocol) contact string (max 255 characters). " +
					"Identifies who is responsible for the device; read by network monitoring tools.",
				Optional: true,
				Validators: []validator.String{
					stringvalidator.LengthAtMost(255),
				},
			},

			"snmp_location": schema.StringAttribute{
				MarkdownDescription: "[SNMP](https://en.wikipedia.org/wiki/Simple_Network_Management_Protocol) location string (max 255 characters). " +
					"Describes where the device is physically located; read by network monitoring tools.",
				Optional: true,
				Validators: []validator.String{
					stringvalidator.LengthAtMost(255),
				},
			},

			"volume": schema.Int64Attribute{
				MarkdownDescription: "Speaker volume (0–100). Only applicable to devices with speakers.",
				Optional:            true,
				Validators: []validator.Int64{
					int64validator.Between(0, 100),
				},
			},

			"config_network": schema.SingleNestedAttribute{
				MarkdownDescription: "Management network configuration for the device. " +
					"Use `type = \"dhcp\"` to receive an address from DHCP, or `type = \"static\"` " +
					"with `ip`, `netmask`, and `gateway` set to assign a fixed management address. " +
					"Omit this block to leave the device's current configuration unchanged.",
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{
						MarkdownDescription: "Addressing mode: `dhcp` or `static`.",
						Required:            true,
						Validators: []validator.String{
							stringvalidator.OneOf("dhcp", "static"),
						},
					},
					"ip": schema.StringAttribute{
						MarkdownDescription: "Static IPv4 address. Required when `type = static`.",
						Optional:            true,
					},
					"netmask": schema.StringAttribute{
						MarkdownDescription: "Subnet mask (e.g. `255.255.255.0`). Required when `type = static`.",
						Optional:            true,
					},
					"gateway": schema.StringAttribute{
						MarkdownDescription: "Default gateway IPv4 address. Required when `type = static`.",
						Optional:            true,
					},
					"dns1": schema.StringAttribute{
						MarkdownDescription: "Primary DNS server.",
						Optional:            true,
					},
					"dns2": schema.StringAttribute{
						MarkdownDescription: "Secondary DNS server.",
						Optional:            true,
					},
				},
			},

			// Read-only attributes.
			"model": schema.StringAttribute{
				MarkdownDescription: "The hardware model of the device (e.g. `U6-LR`, `US-16-XG`).",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"type": schema.StringAttribute{
				MarkdownDescription: "The device type (e.g. `uap` for access point, `usw` for switch, `ugw` for gateway).",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"ip": schema.StringAttribute{
				MarkdownDescription: "The current IP address of the device.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"adopted": schema.BoolAttribute{
				MarkdownDescription: "Whether the device has been adopted by the controller.",
				Computed:            true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},

			"state": schema.Int64Attribute{
				MarkdownDescription: "The device state. 0 = unknown, 1 = connected, 2 = pending, " +
					"4 = upgrading, 5 = provisioning, 6 = heartbeat missed.",
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *deviceResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		deviceConfigNetworkValidator{},
	}
}

func (r *deviceResource) Configure(
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

func (r *deviceResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var plan deviceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save which optional fields were configured so we can preserve nulls after
	// apiToModel — the API returns all fields, but we only track what the user set.
	planned := plan

	site := r.client.SiteOrDefault(plan.Site)
	mac := strings.ToLower(plan.MAC.ValueString())

	// Look up the existing device by MAC — it must already be adopted.
	existing, err := r.client.ApiClient.GetDeviceByMAC(ctx, site, mac)
	if err != nil {
		resp.Diagnostics.AddError(
			"Device Not Found",
			fmt.Sprintf("No adopted device found with MAC %q in site %q. "+
				"The device must be adopted by the controller before it can be managed by Terraform: %s",
				mac, site, err.Error()),
		)
		return
	}

	// TODO(go-unifi): Bypass SDK's UpdateDevice — see device_api.go for details.
	err = r.client.UpdateDevice(ctx, site, existing.ID, &plan)
	if err != nil {
		resp.Diagnostics.AddError("Error Updating Device", err.Error())
		return
	}

	// Re-read to get full state including runtime fields (state, ip, etc.).
	device, err := r.client.ApiClient.GetDevice(ctx, site, existing.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Device After Create", err.Error())
		return
	}

	r.apiToModel(device, &plan, site)
	r.preserveNullOptionals(&planned, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *deviceResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var state deviceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save prior state to preserve nulls for unmanaged optional fields.
	prior := state

	site := r.client.SiteOrDefault(state.Site)

	device, err := r.client.ApiClient.GetDevice(ctx, site, state.ID.ValueString())
	if err != nil {
		if _, ok := err.(*unifi.NotFoundError); ok {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Device",
			fmt.Sprintf("Could not read device %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	r.apiToModel(device, &state, site)
	r.preserveNullOptionals(&prior, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *deviceResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	var state, plan deviceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	site := r.client.SiteOrDefault(state.Site)

	// TODO(go-unifi): Bypass SDK's UpdateDevice — see device_api.go for details.
	err := r.client.UpdateDevice(ctx, site, state.ID.ValueString(), &plan)
	if err != nil {
		resp.Diagnostics.AddError("Error Updating Device", err.Error())
		return
	}

	// Re-read to get full state including runtime fields.
	device, err := r.client.ApiClient.GetDevice(ctx, site, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Device After Update", err.Error())
		return
	}

	r.apiToModel(device, &state, site)
	r.preserveNullOptionals(&plan, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *deviceResource) Delete(
	_ context.Context,
	_ resource.DeleteRequest,
	_ *resource.DeleteResponse,
) {
	// Intentional no-op. Removing the resource from Terraform state does not
	// forget/unadopt the physical device. The device keeps its current settings.
}

func (r *deviceResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	// Support "site:mac" or just "mac" format.
	// Check if the full string is a MAC first, since MACs contain colons.
	var site, mac string
	if macRegexp.MatchString(req.ID) {
		mac = strings.ToLower(req.ID)
	} else if idx := strings.Index(req.ID, ":"); idx > 0 {
		site = req.ID[:idx]
		mac = strings.ToLower(req.ID[idx+1:])
	} else {
		mac = strings.ToLower(req.ID)
	}

	if !macRegexp.MatchString(mac) {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Expected a MAC address or site:mac, got %q", req.ID),
		)
		return
	}

	if site != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("site"), site)...)
	}

	resolvedSite := site
	if resolvedSite == "" {
		resolvedSite = r.client.SiteOrDefault(types.StringNull())
	}

	device, err := r.client.ApiClient.GetDeviceByMAC(ctx, resolvedSite, mac)
	if err != nil {
		resp.Diagnostics.AddError(
			"Device Not Found",
			fmt.Sprintf("No device found with MAC %q in site %q: %s", mac, resolvedSite, err.Error()),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), device.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("mac"), mac)...)
}

// ---------------------------------------------------------------------------
// Helper methods
// ---------------------------------------------------------------------------

// preserveNullOptionals ensures optional fields the user didn't configure stay
// null in the state, even if the API returned values for them. This prevents
// Terraform's "inconsistent result after apply" error.
func (r *deviceResource) preserveNullOptionals(plan, state *deviceResourceModel) {
	if plan.Name.IsNull() {
		state.Name = types.StringNull()
	}
	if plan.LedEnabled.IsNull() {
		state.LedEnabled = types.BoolNull()
	}
	if plan.LedColor.IsNull() {
		state.LedColor = types.StringNull()
	}
	if plan.LedBrightness.IsNull() {
		state.LedBrightness = types.Int64Null()
	}
	if plan.OutdoorModeOverride.IsNull() {
		state.OutdoorModeOverride = types.StringNull()
	}
	if plan.SnmpContact.IsNull() {
		state.SnmpContact = types.StringNull()
	}
	if plan.SnmpLocation.IsNull() {
		state.SnmpLocation = types.StringNull()
	}
	if plan.Volume.IsNull() {
		state.Volume = types.Int64Null()
	}
	if plan.ConfigNetwork == nil {
		state.ConfigNetwork = nil
	} else if state.ConfigNetwork != nil {
		// Preserve null sub-fields the user didn't configure so Terraform
		// doesn't see spurious diffs for values the API fills in by default.
		if plan.ConfigNetwork.IP.IsNull() {
			state.ConfigNetwork.IP = types.StringNull()
		}
		if plan.ConfigNetwork.Netmask.IsNull() {
			state.ConfigNetwork.Netmask = types.StringNull()
		}
		if plan.ConfigNetwork.Gateway.IsNull() {
			state.ConfigNetwork.Gateway = types.StringNull()
		}
		if plan.ConfigNetwork.DNS1.IsNull() {
			state.ConfigNetwork.DNS1 = types.StringNull()
		}
		if plan.ConfigNetwork.DNS2.IsNull() {
			state.ConfigNetwork.DNS2 = types.StringNull()
		}
	}
}

func (r *deviceResource) apiToModel(d *unifi.Device, m *deviceResourceModel, site string) {
	m.ID = types.StringValue(d.ID)
	m.Site = types.StringValue(site)
	m.MAC = types.StringValue(d.MAC)

	m.Name = stringValueOrNull(d.Name)
	switch d.LedOverride {
	case "on":
		m.LedEnabled = types.BoolValue(true)
	case "off":
		m.LedEnabled = types.BoolValue(false)
	default:
		m.LedEnabled = types.BoolNull()
	}
	m.LedColor = stringValueOrNull(d.LedOverrideColor)
	if d.LedOverrideColorBrightness != nil {
		m.LedBrightness = types.Int64Value(*d.LedOverrideColorBrightness)
	} else {
		m.LedBrightness = types.Int64Null()
	}
	m.OutdoorModeOverride = stringValueOrNull(d.OutdoorModeOverride)
	m.Locked = types.BoolValue(d.Locked)
	m.Disabled = types.BoolValue(d.Disabled)
	m.SnmpContact = stringValueOrNull(d.SnmpContact)
	m.SnmpLocation = stringValueOrNull(d.SnmpLocation)
	if d.Volume != nil {
		m.Volume = types.Int64Value(*d.Volume)
	} else {
		m.Volume = types.Int64Null()
	}

	if d.ConfigNetwork != nil && d.ConfigNetwork.Type != "" {
		m.ConfigNetwork = &deviceConfigNetworkModel{
			Type:    types.StringValue(d.ConfigNetwork.Type),
			IP:      stringValueOrNull(d.ConfigNetwork.IP),
			Netmask: stringValueOrNull(d.ConfigNetwork.Netmask),
			Gateway: stringValueOrNull(d.ConfigNetwork.Gateway),
			DNS1:    stringValueOrNull(d.ConfigNetwork.DNS1),
			DNS2:    stringValueOrNull(d.ConfigNetwork.DNS2),
		}
	} else {
		m.ConfigNetwork = nil
	}

	// Read-only fields.
	m.Model = stringValueOrNull(d.Model)
	m.Type = stringValueOrNull(d.Type)
	m.IP = stringValueOrNull(d.IP)
	m.Adopted = types.BoolValue(d.Adopted)
	m.State = types.Int64Value(int64(d.State))
}

// ---------------------------------------------------------------------------
// Config validators
// ---------------------------------------------------------------------------

// deviceConfigNetworkValidator ensures that when config_network.type is
// "static", the required addressing fields (ip, netmask, gateway) are set.
type deviceConfigNetworkValidator struct{}

func (v deviceConfigNetworkValidator) Description(_ context.Context) string {
	return "When config_network.type is \"static\", ip, netmask, and gateway must be set."
}

func (v deviceConfigNetworkValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v deviceConfigNetworkValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var cfg *deviceConfigNetworkModel
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("config_network"), &cfg)...)
	if resp.Diagnostics.HasError() || cfg == nil {
		return
	}

	if cfg.Type.IsNull() || cfg.Type.IsUnknown() {
		return
	}

	switch cfg.Type.ValueString() {
	case "static":
		for name, v := range map[string]types.String{
			"ip":      cfg.IP,
			"netmask": cfg.Netmask,
			"gateway": cfg.Gateway,
		} {
			if v.IsNull() {
				resp.Diagnostics.AddAttributeError(
					path.Root("config_network").AtName(name),
					"Missing required attribute",
					fmt.Sprintf("config_network.%s is required when config_network.type is \"static\".", name),
				)
			}
		}
	case "dhcp":
		for name, v := range map[string]types.String{
			"ip":      cfg.IP,
			"netmask": cfg.Netmask,
			"gateway": cfg.Gateway,
		} {
			if !v.IsNull() && !v.IsUnknown() {
				resp.Diagnostics.AddAttributeError(
					path.Root("config_network").AtName(name),
					"Attribute not allowed",
					fmt.Sprintf("config_network.%s must not be set when config_network.type is \"dhcp\".", name),
				)
			}
		}
	}
}
