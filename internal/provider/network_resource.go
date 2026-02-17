package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/ubiquiti-community/go-unifi/unifi"
)

var (
	_ resource.Resource                = &networkResource{}
	_ resource.ResourceWithImportState = &networkResource{}
)

func NewNetworkResource() resource.Resource {
	return &networkResource{}
}

type networkResource struct {
	client *Client
}

type networkResourceModel struct {
	ID                    types.String `tfsdk:"id"`
	Site                  types.String `tfsdk:"site"`
	Name                  types.String `tfsdk:"name"`
	Purpose               types.String `tfsdk:"purpose"`
	VLANId                types.Int64  `tfsdk:"vlan_id"`
	Subnet                types.String `tfsdk:"subnet"`
	NetworkGroup          types.String `tfsdk:"network_group"`
	DHCPEnabled           types.Bool   `tfsdk:"dhcp_enabled"`
	DHCPStart             types.String `tfsdk:"dhcp_start"`
	DHCPStop              types.String `tfsdk:"dhcp_stop"`
	DHCPLease             types.Int64  `tfsdk:"dhcp_lease"`
	DHCPDns               types.List   `tfsdk:"dhcp_dns"`
	InternetAccessEnabled types.Bool   `tfsdk:"internet_access_enabled"`
}

func (r *networkResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_network"
}

func (r *networkResource) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a network on the UniFi controller.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The ID of the network.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"site": schema.StringAttribute{
				MarkdownDescription: "The site to associate the network with. Defaults to the provider site.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the network.",
				Required:            true,
			},

			"purpose": schema.StringAttribute{
				MarkdownDescription: "The purpose of the network. Must be `corporate`.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("corporate"),
				},
			},

			"vlan_id": schema.Int64Attribute{
				MarkdownDescription: "The VLAN ID for the network. Must be between 2 and 4095.",
				Optional:            true,
				Validators: []validator.Int64{
					int64validator.Between(2, 4095),
				},
			},

			"subnet": schema.StringAttribute{
				MarkdownDescription: "The subnet for the network in CIDR notation (e.g., `192.168.33.0/24`).",
				Optional:            true,
			},

			"network_group": schema.StringAttribute{
				MarkdownDescription: "The network group. Default: `LAN`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("LAN"),
			},

			"dhcp_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether DHCP is enabled on this network. Default: `false`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},

			"dhcp_start": schema.StringAttribute{
				MarkdownDescription: "The starting IP address for the DHCP pool.",
				Optional:            true,
				Computed:            true,
			},

			"dhcp_stop": schema.StringAttribute{
				MarkdownDescription: "The ending IP address for the DHCP pool.",
				Optional:            true,
				Computed:            true,
			},

			"dhcp_lease": schema.Int64Attribute{
				MarkdownDescription: "The DHCP lease time in seconds. Default: `86400` (24 hours).",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(86400),
			},

			"dhcp_dns": schema.ListAttribute{
				MarkdownDescription: "List of DNS servers for DHCP clients. Maximum 4 servers.",
				ElementType:         types.StringType,
				Optional:            true,
				Validators: []validator.List{
					listvalidator.SizeAtMost(4),
				},
			},

			"internet_access_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether internet access is enabled on this network. Default: `true`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
		},
	}
}

func (r *networkResource) Configure(
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

func (r *networkResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var plan networkResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	site := r.client.SiteOrDefault(plan.Site)
	network := r.modelToAPI(ctx, &plan)

	created, err := r.client.CreateNetwork(ctx, site, network)
	if err != nil {
		resp.Diagnostics.AddError("Error Creating Network", err.Error())
		return
	}

	r.apiToModel(ctx, created, &plan, site)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *networkResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var state networkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	site := r.client.SiteOrDefault(state.Site)

	network, err := r.client.GetNetwork(ctx, site, state.ID.ValueString())
	if err != nil {
		if _, ok := err.(*unifi.NotFoundError); ok {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Network",
			fmt.Sprintf("Could not read network %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	r.apiToModel(ctx, network, &state, site)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *networkResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	var state, plan networkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.applyPlanToState(&plan, &state)

	site := r.client.SiteOrDefault(state.Site)
	network := r.modelToAPI(ctx, &state)
	network.ID = state.ID.ValueString()

	updated, err := r.client.UpdateNetwork(ctx, site, network)
	if err != nil {
		resp.Diagnostics.AddError("Error Updating Network", err.Error())
		return
	}

	r.apiToModel(ctx, updated, &state, site)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *networkResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	var state networkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	site := r.client.SiteOrDefault(state.Site)

	err := r.client.DeleteNetwork(ctx, site, state.ID.ValueString(), state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error Deleting Network", err.Error())
	}
}

func (r *networkResource) ImportState(
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

func (r *networkResource) applyPlanToState(plan, state *networkResourceModel) {
	if !plan.Name.IsNull() && !plan.Name.IsUnknown() {
		state.Name = plan.Name
	}
	if !plan.VLANId.IsNull() && !plan.VLANId.IsUnknown() {
		state.VLANId = plan.VLANId
	}
	if !plan.Subnet.IsNull() && !plan.Subnet.IsUnknown() {
		state.Subnet = plan.Subnet
	}
	if !plan.NetworkGroup.IsNull() && !plan.NetworkGroup.IsUnknown() {
		state.NetworkGroup = plan.NetworkGroup
	}
	if !plan.DHCPEnabled.IsNull() && !plan.DHCPEnabled.IsUnknown() {
		state.DHCPEnabled = plan.DHCPEnabled
	}
	if !plan.DHCPStart.IsNull() && !plan.DHCPStart.IsUnknown() {
		state.DHCPStart = plan.DHCPStart
	}
	if !plan.DHCPStop.IsNull() && !plan.DHCPStop.IsUnknown() {
		state.DHCPStop = plan.DHCPStop
	}
	if !plan.DHCPLease.IsNull() && !plan.DHCPLease.IsUnknown() {
		state.DHCPLease = plan.DHCPLease
	}
	if !plan.DHCPDns.IsNull() && !plan.DHCPDns.IsUnknown() {
		state.DHCPDns = plan.DHCPDns
	}
	if !plan.InternetAccessEnabled.IsNull() && !plan.InternetAccessEnabled.IsUnknown() {
		state.InternetAccessEnabled = plan.InternetAccessEnabled
	}
}

func (r *networkResource) modelToAPI(ctx context.Context, m *networkResourceModel) *unifi.Network {
	net := &unifi.Network{
		Purpose: m.Purpose.ValueString(),
		Enabled: true,
	}

	if !m.Name.IsNull() {
		name := m.Name.ValueString()
		net.Name = &name
	}

	if !m.VLANId.IsNull() {
		vlan := m.VLANId.ValueInt64()
		net.VLAN = &vlan
		if vlan > 0 {
			net.VLANEnabled = true
		}
	}

	if !m.Subnet.IsNull() {
		subnet := m.Subnet.ValueString()
		net.IPSubnet = &subnet
	}

	if !m.NetworkGroup.IsNull() {
		group := m.NetworkGroup.ValueString()
		net.NetworkGroup = &group
	}

	if !m.DHCPEnabled.IsNull() {
		net.DHCPDEnabled = m.DHCPEnabled.ValueBool()
	}

	if !m.DHCPStart.IsNull() {
		start := m.DHCPStart.ValueString()
		net.DHCPDStart = &start
	}

	if !m.DHCPStop.IsNull() {
		stop := m.DHCPStop.ValueString()
		net.DHCPDStop = &stop
	}

	if !m.DHCPLease.IsNull() {
		lease := m.DHCPLease.ValueInt64()
		net.DHCPDLeaseTime = &lease
	}

	if !m.DHCPDns.IsNull() {
		var dnsServers []types.String
		m.DHCPDns.ElementsAs(ctx, &dnsServers, false)

		for i, dns := range dnsServers {
			if i >= 4 {
				break
			}
			dnsVal := dns.ValueString()
			switch i {
			case 0:
				net.DHCPDDNS1 = dnsVal
			case 1:
				net.DHCPDDNS2 = dnsVal
			case 2:
				net.DHCPDDNS3 = dnsVal
			case 3:
				net.DHCPDDNS4 = dnsVal
			}
		}
	}

	if !m.InternetAccessEnabled.IsNull() {
		net.InternetAccessEnabled = m.InternetAccessEnabled.ValueBool()
	}

	return net
}

func (r *networkResource) apiToModel(ctx context.Context, net *unifi.Network, m *networkResourceModel, site string) {
	m.ID = types.StringValue(net.ID)
	m.Site = types.StringValue(site)
	m.Purpose = types.StringValue(net.Purpose)

	if net.Name != nil {
		m.Name = types.StringValue(*net.Name)
	} else {
		m.Name = types.StringNull()
	}

	if net.VLAN != nil && *net.VLAN != 0 {
		m.VLANId = types.Int64PointerValue(net.VLAN)
	} else {
		m.VLANId = types.Int64Null()
	}

	if net.IPSubnet != nil && *net.IPSubnet != "" {
		m.Subnet = types.StringPointerValue(net.IPSubnet)
	} else {
		m.Subnet = types.StringNull()
	}

	if net.NetworkGroup != nil && *net.NetworkGroup != "" {
		m.NetworkGroup = types.StringPointerValue(net.NetworkGroup)
	} else {
		m.NetworkGroup = types.StringNull()
	}

	m.DHCPEnabled = types.BoolValue(net.DHCPDEnabled)

	if net.DHCPDStart != nil && *net.DHCPDStart != "" {
		m.DHCPStart = types.StringPointerValue(net.DHCPDStart)
	} else {
		m.DHCPStart = types.StringNull()
	}

	if net.DHCPDStop != nil && *net.DHCPDStop != "" {
		m.DHCPStop = types.StringPointerValue(net.DHCPDStop)
	} else {
		m.DHCPStop = types.StringNull()
	}

	if net.DHCPDLeaseTime != nil && *net.DHCPDLeaseTime != 0 {
		m.DHCPLease = types.Int64PointerValue(net.DHCPDLeaseTime)
	} else {
		m.DHCPLease = types.Int64Null()
	}

	// Collect non-empty DNS servers into a list
	var dnsServers []string
	if net.DHCPDDNS1 != "" {
		dnsServers = append(dnsServers, net.DHCPDDNS1)
	}
	if net.DHCPDDNS2 != "" {
		dnsServers = append(dnsServers, net.DHCPDDNS2)
	}
	if net.DHCPDDNS3 != "" {
		dnsServers = append(dnsServers, net.DHCPDDNS3)
	}
	if net.DHCPDDNS4 != "" {
		dnsServers = append(dnsServers, net.DHCPDDNS4)
	}

	if len(dnsServers) > 0 {
		var dnsValues []types.String
		for _, dns := range dnsServers {
			dnsValues = append(dnsValues, types.StringValue(dns))
		}
		m.DHCPDns = types.ListValueMust(types.StringType, toAttrValues(dnsValues))
	} else {
		m.DHCPDns = types.ListNull(types.StringType)
	}

	m.InternetAccessEnabled = types.BoolValue(net.InternetAccessEnabled)
}

func toAttrValues(vals []types.String) []attr.Value {
	result := make([]attr.Value, len(vals))
	for i, v := range vals {
		result[i] = v
	}
	return result
}
