package provider

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/alexklibisz/terrifi/internal/unifi"
)

// Compile-time interface checks.
var (
	_ resource.Resource                = &devicePortsResource{}
	_ resource.ResourceWithImportState = &devicePortsResource{}
)

// NewDevicePortsResource is the factory function registered in provider.Resources().
func NewDevicePortsResource() resource.Resource {
	return &devicePortsResource{}
}

// devicePortsResource holds the API client, injected by Configure().
type devicePortsResource struct {
	client *Client
}

type devicePortsResourceModel struct {
	ID        types.String               `tfsdk:"id"`
	Site      types.String               `tfsdk:"site"`
	DeviceMAC types.String               `tfsdk:"device_mac"`
	Ports     map[string]devicePortModel `tfsdk:"ports"`
}

type devicePortModel struct {
	Name          types.String `tfsdk:"name"`
	PortProfileID types.String `tfsdk:"port_profile_id"`
}

func (r *devicePortsResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_device_ports"
}

func (r *devicePortsResource) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Assigns port profiles (and port names) to the ports of an adopted UniFi switch or gateway. " +
			"Only the ports listed in `ports` are managed; every other port is left as it is. " +
			"Use one `terrifi_device_ports` per device: two for the same device overwrite each other. " +
			"Device-level settings (name, LEDs, ...) belong in `terrifi_device`.",

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

			"device_mac": schema.StringAttribute{
				MarkdownDescription: "The MAC address of the adopted device (e.g. `aa:bb:cc:dd:ee:ff`).",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.RegexMatches(macRegexp, "must be a MAC address like aa:bb:cc:dd:ee:ff"),
				},
			},

			"ports": schema.MapNestedAttribute{
				MarkdownDescription: "The managed ports, keyed by port number (`\"1\"`, `\"2\"`, ...). " +
					"Removing a port stops managing it: its name and profile are removed and any per-port " +
					"settings made in the UI are kept.",
				Required: true,
				Validators: []validator.Map{
					mapvalidator.KeysAre(stringvalidator.RegexMatches(portKeyRE, "must be a port number such as \"1\"")),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							MarkdownDescription: "The port name. Omit to use the controller's default (`Port N`).",
							Optional:            true,
							Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
						},
						"port_profile_id": schema.StringAttribute{
							MarkdownDescription: "The ID of the `terrifi_port_profile` to assign. The profile then controls " +
								"every setting of the port, PoE included, and per-port settings are cleared. " +
								"Omit to remove the profile: the port becomes a default port (all networks).",
							Optional:   true,
							Validators: []validator.String{stringvalidator.LengthAtLeast(1)},
						},
					},
				},
			},
		},
	}
}

var portKeyRE = regexp.MustCompile(`^[1-9][0-9]*$`)

func (r *devicePortsResource) Configure(
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

func (r *devicePortsResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var plan devicePortsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	plan.Site = types.StringValue(r.client.SiteOrDefault(plan.Site))
	id, err := r.write(ctx, plan.Site.ValueString(), plan.DeviceMAC.ValueString(), portWants(plan.Ports, nil))
	if err != nil {
		resp.Diagnostics.AddError("Error Assigning Device Ports", err.Error())
		return
	}
	plan.ID = types.StringValue(id)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *devicePortsResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var state devicePortsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	site := r.client.SiteOrDefault(state.Site)
	d, err := r.client.GetDevicePorts(ctx, site, state.DeviceMAC.ValueString())
	if err != nil {
		var nf *unifi.NotFoundError
		if errors.As(err, &nf) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Device Ports",
			fmt.Sprintf("Could not read device %s: %s", state.DeviceMAC.ValueString(), err.Error()),
		)
		return
	}

	state.ID = types.StringValue(d.ID)
	state.Site = types.StringValue(site)
	state.Ports = portsToModel(portsFromOverrides(d.PortOverrides), state.Ports)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *devicePortsResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	var state, plan devicePortsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	plan.Site = state.Site
	id, err := r.write(ctx, plan.Site.ValueString(), plan.DeviceMAC.ValueString(), portWants(plan.Ports, state.Ports))
	if err != nil {
		resp.Diagnostics.AddError("Error Assigning Device Ports", err.Error())
		return
	}
	plan.ID = types.StringValue(id)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete stops managing every port: names and profiles are removed, the
// device itself is untouched.
func (r *devicePortsResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	var state devicePortsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.write(ctx, r.client.SiteOrDefault(state.Site), state.DeviceMAC.ValueString(), portWants(nil, state.Ports))
	var nf *unifi.NotFoundError
	if err != nil && !errors.As(err, &nf) {
		resp.Diagnostics.AddError("Error Releasing Device Ports", err.Error())
	}
}

// ImportState accepts "mac" or "site:mac". Every port that has a name or a
// profile is imported.
func (r *devicePortsResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	mac := req.ID
	if !macRegexp.MatchString(mac) {
		if i := strings.Index(req.ID, ":"); i > 0 {
			resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("site"), req.ID[:i])...)
			mac = req.ID[i+1:]
		}
	}
	if !macRegexp.MatchString(mac) {
		resp.Diagnostics.AddError("Invalid Import ID", fmt.Sprintf("Expected a MAC address or site:mac, got %q", req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("device_mac"), mac)...)
}

// write merges want into the device's port_overrides and PUTs the result. It
// returns the device ID.
func (r *devicePortsResource) write(ctx context.Context, site, mac string, want map[int64]*portWant) (string, error) {
	d, err := r.client.GetDevicePorts(ctx, site, mac)
	if err != nil {
		return "", fmt.Errorf("reading device %s: %w", mac, err)
	}
	if err := checkPortsAssignable(d, want); err != nil {
		return "", err
	}
	overrides, err := mergePortOverrides(d.PortOverrides, want)
	if err != nil {
		return "", err
	}
	if err := r.client.PutPortOverrides(ctx, site, d.ID, overrides); err != nil {
		return "", fmt.Errorf("updating port overrides on device %s: %w", mac, err)
	}
	return d.ID, nil
}

// portWants converts the planned ports into merge input. Ports only in prior
// (removed from the config) map to nil: no longer managed.
func portWants(planned, prior map[string]devicePortModel) map[int64]*portWant {
	want := map[int64]*portWant{}
	for k := range prior {
		if idx, err := strconv.ParseInt(k, 10, 64); err == nil {
			want[idx] = nil
		}
	}
	for k, p := range planned {
		idx, err := strconv.ParseInt(k, 10, 64)
		if err != nil {
			continue // rejected by the schema validator
		}
		want[idx] = &portWant{Name: p.Name.ValueStringPointer(), ProfileID: p.PortProfileID.ValueStringPointer()}
	}
	return want
}

// portsToModel builds the ports attribute from the controller's view. Only
// the ports in prior are reported; a nil prior (right after import) reports
// every port that has a name or a profile.
func portsToModel(got map[int64]portWant, prior map[string]devicePortModel) map[string]devicePortModel {
	out := map[string]devicePortModel{}
	for idx, w := range got {
		k := strconv.FormatInt(idx, 10)
		if _, managed := prior[k]; managed || (prior == nil && (w.Name != nil || w.ProfileID != nil)) {
			out[k] = devicePortModel{Name: types.StringPointerValue(w.Name), PortProfileID: types.StringPointerValue(w.ProfileID)}
		}
	}
	for k := range prior {
		if _, ok := out[k]; !ok {
			out[k] = devicePortModel{Name: types.StringNull(), PortProfileID: types.StringNull()}
		}
	}
	return out
}
