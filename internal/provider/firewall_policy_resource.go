package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/ubiquiti-community/go-unifi/unifi"
)

var (
	_ resource.Resource                = &firewallPolicyResource{}
	_ resource.ResourceWithImportState = &firewallPolicyResource{}
)

func NewFirewallPolicyResource() resource.Resource {
	return &firewallPolicyResource{}
}

type firewallPolicyResource struct {
	client *Client
}

type firewallPolicyResourceModel struct {
	ID                  types.String `tfsdk:"id"`
	Site                types.String `tfsdk:"site"`
	Name                types.String `tfsdk:"name"`
	Description         types.String `tfsdk:"description"`
	Enabled             types.Bool   `tfsdk:"enabled"`
	Action              types.String `tfsdk:"action"`
	IPVersion           types.String `tfsdk:"ip_version"`
	Protocol            types.String `tfsdk:"protocol"`
	ConnectionStateType types.String `tfsdk:"connection_state_type"`
	ConnectionStates    types.Set    `tfsdk:"connection_states"`
	MatchIPSec          types.Bool   `tfsdk:"match_ipsec"`
	Logging             types.Bool   `tfsdk:"logging"`
	CreateAllowRespond  types.Bool   `tfsdk:"create_allow_respond"`
	Index               types.Int64  `tfsdk:"index"`
	Source              types.Object `tfsdk:"source"`
	Destination         types.Object `tfsdk:"destination"`
	Schedule            types.Object `tfsdk:"schedule"`
}

type firewallPolicyEndpointModel struct {
	ZoneID           types.String `tfsdk:"zone_id"`
	IPs              types.Set    `tfsdk:"ips"`
	MACAddresses     types.Set    `tfsdk:"mac_addresses"`
	NetworkIDs       types.Set    `tfsdk:"network_ids"`
	DeviceIDs        types.Set    `tfsdk:"device_ids"`
	PortMatchingType types.String `tfsdk:"port_matching_type"`
	Port             types.Int64  `tfsdk:"port"`
	PortGroupID      types.String `tfsdk:"port_group_id"`
}

type firewallPolicyScheduleModel struct {
	Mode           types.String `tfsdk:"mode"`
	Date           types.String `tfsdk:"date"`
	TimeAllDay     types.Bool   `tfsdk:"time_all_day"`
	TimeRangeStart types.String `tfsdk:"time_range_start"`
	TimeRangeEnd   types.String `tfsdk:"time_range_end"`
	RepeatOnDays   types.Set    `tfsdk:"repeat_on_days"`
}

// endpointAttrTypes defines the attribute types for source/destination nested objects.
var endpointAttrTypes = map[string]attr.Type{
	"zone_id":            types.StringType,
	"ips":                types.SetType{ElemType: types.StringType},
	"mac_addresses":      types.SetType{ElemType: types.StringType},
	"network_ids":        types.SetType{ElemType: types.StringType},
	"device_ids":         types.SetType{ElemType: types.StringType},
	"port_matching_type": types.StringType,
	"port":               types.Int64Type,
	"port_group_id":      types.StringType,
}

// scheduleAttrTypes defines the attribute types for the schedule nested object.
var scheduleAttrTypes = map[string]attr.Type{
	"mode":             types.StringType,
	"date":             types.StringType,
	"time_all_day":     types.BoolType,
	"time_range_start": types.StringType,
	"time_range_end":   types.StringType,
	"repeat_on_days":   types.SetType{ElemType: types.StringType},
}

func (r *firewallPolicyResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_firewall_policy"
}

func (r *firewallPolicyResource) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	endpointAttributes := map[string]schema.Attribute{
		"zone_id": schema.StringAttribute{
			MarkdownDescription: "The ID of the firewall zone.",
			Required:            true,
		},
		"ips": schema.SetAttribute{
			MarkdownDescription: "IP addresses or CIDR ranges to match.",
			ElementType:         types.StringType,
			Optional:            true,
		},
		"mac_addresses": schema.SetAttribute{
			MarkdownDescription: "MAC addresses to match.",
			ElementType:         types.StringType,
			Optional:            true,
		},
		"network_ids": schema.SetAttribute{
			MarkdownDescription: "Network IDs to match.",
			ElementType:         types.StringType,
			Optional:            true,
		},
		"device_ids": schema.SetAttribute{
			MarkdownDescription: "Device IDs to match.",
			ElementType:         types.StringType,
			Optional:            true,
		},
		"port_matching_type": schema.StringAttribute{
			MarkdownDescription: "Port matching type. Valid values: `ANY`, `SPECIFIC`, `LIST`.",
			Optional:            true,
			Computed:            true,
			Default:             stringdefault.StaticString("ANY"),
			Validators: []validator.String{
				stringvalidator.OneOf("ANY", "SPECIFIC", "LIST"),
			},
		},
		"port": schema.Int64Attribute{
			MarkdownDescription: "Specific port number to match (when `port_matching_type` is `SPECIFIC`).",
			Optional:            true,
		},
		"port_group_id": schema.StringAttribute{
			MarkdownDescription: "Port group ID to match (when `port_matching_type` is `LIST`).",
			Optional:            true,
		},
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a firewall policy on the UniFi controller. Firewall policies define traffic rules between firewall zones.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The ID of the firewall policy.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"site": schema.StringAttribute{
				MarkdownDescription: "The site to associate the firewall policy with. Defaults to the provider site.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the firewall policy.",
				Required:            true,
			},

			"description": schema.StringAttribute{
				MarkdownDescription: "A description of the firewall policy.",
				Optional:            true,
			},

			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the firewall policy is enabled. Default: `true`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},

			"action": schema.StringAttribute{
				MarkdownDescription: "The action to take when traffic matches this policy. Valid values: `ALLOW`, `BLOCK`, `REJECT`.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("ALLOW", "BLOCK", "REJECT"),
				},
			},

			"ip_version": schema.StringAttribute{
				MarkdownDescription: "IP version to match. Valid values: `BOTH`, `IPV4`, `IPV6`. Default: `BOTH`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("BOTH"),
				Validators: []validator.String{
					stringvalidator.OneOf("BOTH", "IPV4", "IPV6"),
				},
			},

			"protocol": schema.StringAttribute{
				MarkdownDescription: "Protocol to match. Valid values: `all`, `tcp`, `udp`, `tcp_udp`. Default: `all`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("all"),
				Validators: []validator.String{
					stringvalidator.OneOf("all", "tcp", "udp", "tcp_udp"),
				},
			},

			"connection_state_type": schema.StringAttribute{
				MarkdownDescription: "Connection state type. Valid values: `ALL`, `RESPOND_ONLY`. Default: `ALL`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("ALL"),
				Validators: []validator.String{
					stringvalidator.OneOf("ALL", "RESPOND_ONLY"),
				},
			},

			"connection_states": schema.SetAttribute{
				MarkdownDescription: "Set of connection states to match (e.g. `NEW`, `ESTABLISHED`, `RELATED`, `INVALID`).",
				ElementType:         types.StringType,
				Optional:            true,
			},

			"match_ipsec": schema.BoolAttribute{
				MarkdownDescription: "Whether to match IPsec traffic.",
				Optional:            true,
			},

			"logging": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable syslog logging for matched traffic.",
				Optional:            true,
			},

			"create_allow_respond": schema.BoolAttribute{
				MarkdownDescription: "Whether to automatically create a corresponding allow-respond rule.",
				Optional:            true,
			},

			"index": schema.Int64Attribute{
				MarkdownDescription: "The ordering index of the policy. Assigned by the controller if not specified.",
				Optional:            true,
				Computed:            true,
			},
		},

		Blocks: map[string]schema.Block{
			"source": schema.SingleNestedBlock{
				MarkdownDescription: "Source endpoint configuration for the firewall policy.",
				Attributes:          endpointAttributes,
			},

			"destination": schema.SingleNestedBlock{
				MarkdownDescription: "Destination endpoint configuration for the firewall policy.",
				Attributes:          endpointAttributes,
			},

			"schedule": schema.SingleNestedBlock{
				MarkdownDescription: "Schedule configuration for when this policy is active.",
				Attributes: map[string]schema.Attribute{
					"mode": schema.StringAttribute{
						MarkdownDescription: "Schedule mode. Valid values: `ALWAYS`, `EVERY_DAY`, `EVERY_WEEK`, `ONE_TIME_ONLY`.",
						Optional:            true,
						Validators: []validator.String{
							stringvalidator.OneOf("ALWAYS", "EVERY_DAY", "EVERY_WEEK", "ONE_TIME_ONLY"),
						},
					},
					"date": schema.StringAttribute{
						MarkdownDescription: "Date for one-time schedules.",
						Optional:            true,
					},
					"time_all_day": schema.BoolAttribute{
						MarkdownDescription: "Whether the schedule applies all day.",
						Optional:            true,
					},
					"time_range_start": schema.StringAttribute{
						MarkdownDescription: "Start time for the schedule (e.g. `08:00`).",
						Optional:            true,
					},
					"time_range_end": schema.StringAttribute{
						MarkdownDescription: "End time for the schedule (e.g. `17:00`).",
						Optional:            true,
					},
					"repeat_on_days": schema.SetAttribute{
						MarkdownDescription: "Days of the week to repeat on. Valid values: `mon`, `tue`, `wed`, `thu`, `fri`, `sat`, `sun`.",
						ElementType:         types.StringType,
						Optional:            true,
					},
				},
			},
		},
	}
}

func (r *firewallPolicyResource) Configure(
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

func (r *firewallPolicyResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var plan firewallPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	site := r.client.SiteOrDefault(plan.Site)
	policy := r.modelToAPI(ctx, &plan)

	created, err := r.client.CreateFirewallPolicy(ctx, site, policy)
	if err != nil {
		resp.Diagnostics.AddError("Error Creating Firewall Policy", err.Error())
		return
	}

	r.apiToModel(created, &plan, site)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *firewallPolicyResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var state firewallPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	site := r.client.SiteOrDefault(state.Site)

	policy, err := r.client.GetFirewallPolicy(ctx, site, state.ID.ValueString())
	if err != nil {
		if _, ok := err.(*unifi.NotFoundError); ok {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Firewall Policy",
			fmt.Sprintf("Could not read firewall policy %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	r.apiToModel(policy, &state, site)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *firewallPolicyResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	var state, plan firewallPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.applyPlanToState(&plan, &state)

	site := r.client.SiteOrDefault(state.Site)
	policy := r.modelToAPI(ctx, &state)
	policy.ID = state.ID.ValueString()

	updated, err := r.client.UpdateFirewallPolicy(ctx, site, policy)
	if err != nil {
		resp.Diagnostics.AddError("Error Updating Firewall Policy", err.Error())
		return
	}

	r.apiToModel(updated, &state, site)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *firewallPolicyResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	var state firewallPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	site := r.client.SiteOrDefault(state.Site)

	err := r.client.DeleteFirewallPolicy(ctx, site, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error Deleting Firewall Policy", err.Error())
	}
}

func (r *firewallPolicyResource) ImportState(
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

func (r *firewallPolicyResource) applyPlanToState(plan, state *firewallPolicyResourceModel) {
	if !plan.Name.IsNull() && !plan.Name.IsUnknown() {
		state.Name = plan.Name
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		state.Description = plan.Description
	}
	if !plan.Enabled.IsNull() && !plan.Enabled.IsUnknown() {
		state.Enabled = plan.Enabled
	}
	if !plan.Action.IsNull() && !plan.Action.IsUnknown() {
		state.Action = plan.Action
	}
	if !plan.IPVersion.IsNull() && !plan.IPVersion.IsUnknown() {
		state.IPVersion = plan.IPVersion
	}
	if !plan.Protocol.IsNull() && !plan.Protocol.IsUnknown() {
		state.Protocol = plan.Protocol
	}
	if !plan.ConnectionStateType.IsNull() && !plan.ConnectionStateType.IsUnknown() {
		state.ConnectionStateType = plan.ConnectionStateType
	}
	if !plan.ConnectionStates.IsNull() && !plan.ConnectionStates.IsUnknown() {
		state.ConnectionStates = plan.ConnectionStates
	}
	if !plan.MatchIPSec.IsNull() && !plan.MatchIPSec.IsUnknown() {
		state.MatchIPSec = plan.MatchIPSec
	}
	if !plan.Logging.IsNull() && !plan.Logging.IsUnknown() {
		state.Logging = plan.Logging
	}
	if !plan.CreateAllowRespond.IsNull() && !plan.CreateAllowRespond.IsUnknown() {
		state.CreateAllowRespond = plan.CreateAllowRespond
	}
	if !plan.Index.IsNull() && !plan.Index.IsUnknown() {
		state.Index = plan.Index
	}
	if !plan.Source.IsNull() && !plan.Source.IsUnknown() {
		state.Source = plan.Source
	}
	if !plan.Destination.IsNull() && !plan.Destination.IsUnknown() {
		state.Destination = plan.Destination
	}
	if !plan.Schedule.IsNull() && !plan.Schedule.IsUnknown() {
		state.Schedule = plan.Schedule
	}
}

func (r *firewallPolicyResource) modelToAPI(ctx context.Context, m *firewallPolicyResourceModel) *unifi.FirewallPolicy {
	policy := &unifi.FirewallPolicy{
		Name:                m.Name.ValueString(),
		Description:         m.Description.ValueString(),
		Enabled:             m.Enabled.ValueBool(),
		Action:              m.Action.ValueString(),
		IPVersion:           m.IPVersion.ValueString(),
		Protocol:            m.Protocol.ValueString(),
		ConnectionStateType: m.ConnectionStateType.ValueString(),
		Logging:             m.Logging.ValueBool(),
		MatchIPSec:          m.MatchIPSec.ValueBool(),
		CreateAllowRespond:  m.CreateAllowRespond.ValueBool(),
	}

	if !m.Index.IsNull() && !m.Index.IsUnknown() {
		v := m.Index.ValueInt64()
		policy.Index = &v
	}

	if !m.ConnectionStates.IsNull() && !m.ConnectionStates.IsUnknown() {
		var states []string
		m.ConnectionStates.ElementsAs(ctx, &states, false)
		policy.ConnectionStates = states
	}

	if !m.Source.IsNull() && !m.Source.IsUnknown() {
		var src firewallPolicyEndpointModel
		m.Source.As(ctx, &src, basetypes.ObjectAsOptions{})
		policy.Source = endpointModelToAPI(ctx, &src)
	}

	if !m.Destination.IsNull() && !m.Destination.IsUnknown() {
		var dst firewallPolicyEndpointModel
		m.Destination.As(ctx, &dst, basetypes.ObjectAsOptions{})
		policy.Destination = destinationModelToAPI(ctx, &dst)
	}

	if !m.Schedule.IsNull() && !m.Schedule.IsUnknown() {
		var sched firewallPolicyScheduleModel
		m.Schedule.As(ctx, &sched, basetypes.ObjectAsOptions{})
		policy.Schedule = scheduleModelToAPI(ctx, &sched)
	}

	return policy
}

func endpointModelToAPI(ctx context.Context, m *firewallPolicyEndpointModel) *unifi.FirewallPolicySource {
	ep := &unifi.FirewallPolicySource{
		ZoneID:           m.ZoneID.ValueString(),
		PortMatchingType: m.PortMatchingType.ValueString(),
		PortGroupID:      m.PortGroupID.ValueString(),
	}

	if !m.Port.IsNull() && !m.Port.IsUnknown() {
		v := m.Port.ValueInt64()
		ep.Port = &v
	}

	ep.MatchingTarget, ep.IPs = resolveMatchingTarget(ctx, m)

	return ep
}

func destinationModelToAPI(ctx context.Context, m *firewallPolicyEndpointModel) *unifi.FirewallPolicyDestination {
	ep := &unifi.FirewallPolicyDestination{
		ZoneID:           m.ZoneID.ValueString(),
		PortMatchingType: m.PortMatchingType.ValueString(),
		PortGroupID:      m.PortGroupID.ValueString(),
	}

	if !m.Port.IsNull() && !m.Port.IsUnknown() {
		v := m.Port.ValueInt64()
		ep.Port = &v
	}

	ep.MatchingTarget, ep.IPs = resolveMatchingTarget(ctx, m)

	return ep
}

// resolveMatchingTarget derives the API matching_target and ips values from the
// typed endpoint fields. Exactly one of ips, mac_addresses, network_ids, or
// device_ids should be set. If none is set, matching_target is ANY.
func resolveMatchingTarget(ctx context.Context, m *firewallPolicyEndpointModel) (string, []string) {
	type targetField struct {
		field  types.Set
		target string
	}
	for _, tf := range []targetField{
		{m.IPs, "IP"},
		{m.MACAddresses, "MAC"},
		{m.NetworkIDs, "NETWORK"},
		{m.DeviceIDs, "CLIENT"},
	} {
		if !tf.field.IsNull() && !tf.field.IsUnknown() {
			var vals []string
			tf.field.ElementsAs(ctx, &vals, false)
			return tf.target, vals
		}
	}
	return "ANY", nil
}

func scheduleModelToAPI(ctx context.Context, m *firewallPolicyScheduleModel) *unifi.FirewallPolicySchedule {
	sched := &unifi.FirewallPolicySchedule{
		Mode:           m.Mode.ValueString(),
		Date:           m.Date.ValueString(),
		TimeAllDay:     m.TimeAllDay.ValueBool(),
		TimeRangeStart: m.TimeRangeStart.ValueString(),
		TimeRangeEnd:   m.TimeRangeEnd.ValueString(),
	}

	if !m.RepeatOnDays.IsNull() && !m.RepeatOnDays.IsUnknown() {
		var days []string
		m.RepeatOnDays.ElementsAs(ctx, &days, false)
		sched.RepeatOnDays = days
	}

	return sched
}

func (r *firewallPolicyResource) apiToModel(policy *unifi.FirewallPolicy, m *firewallPolicyResourceModel, site string) {
	m.ID = types.StringValue(policy.ID)
	m.Site = types.StringValue(site)
	m.Name = types.StringValue(policy.Name)

	if policy.Description != "" {
		m.Description = types.StringValue(policy.Description)
	} else {
		m.Description = types.StringNull()
	}

	m.Enabled = types.BoolValue(policy.Enabled)
	m.Action = types.StringValue(policy.Action)

	if policy.IPVersion != "" {
		m.IPVersion = types.StringValue(policy.IPVersion)
	} else {
		m.IPVersion = types.StringValue("BOTH")
	}

	if policy.Protocol != "" {
		m.Protocol = types.StringValue(policy.Protocol)
	} else {
		m.Protocol = types.StringValue("all")
	}

	if policy.ConnectionStateType != "" {
		m.ConnectionStateType = types.StringValue(policy.ConnectionStateType)
	} else {
		m.ConnectionStateType = types.StringValue("ALL")
	}

	if len(policy.ConnectionStates) > 0 {
		vals := make([]attr.Value, len(policy.ConnectionStates))
		for i, s := range policy.ConnectionStates {
			vals[i] = types.StringValue(s)
		}
		m.ConnectionStates = types.SetValueMust(types.StringType, vals)
	} else {
		m.ConnectionStates = types.SetNull(types.StringType)
	}

	m.MatchIPSec = boolValueOrNull(policy.MatchIPSec)
	m.Logging = boolValueOrNull(policy.Logging)
	m.CreateAllowRespond = boolValueOrNull(policy.CreateAllowRespond)

	if policy.Index != nil {
		m.Index = types.Int64Value(*policy.Index)
	} else {
		m.Index = types.Int64Null()
	}

	if policy.Source != nil {
		m.Source = endpointAPIToModel(policy.Source)
	} else {
		m.Source = types.ObjectNull(endpointAttrTypes)
	}

	if policy.Destination != nil {
		m.Destination = destinationAPIToModel(policy.Destination)
	} else {
		m.Destination = types.ObjectNull(endpointAttrTypes)
	}

	if policy.Schedule != nil && !isDefaultSchedule(policy.Schedule) {
		m.Schedule = scheduleAPIToModel(policy.Schedule)
	} else {
		m.Schedule = types.ObjectNull(scheduleAttrTypes)
	}
}

func boolValueOrNull(b bool) types.Bool {
	if b {
		return types.BoolValue(true)
	}
	return types.BoolNull()
}

func endpointAPIToModel(src *unifi.FirewallPolicySource) types.Object {
	attrs := map[string]attr.Value{
		"zone_id":            types.StringValue(src.ZoneID),
		"port_matching_type": stringValueOrNull(src.PortMatchingType),
		"port_group_id":      stringValueOrNull(src.PortGroupID),
	}

	if src.Port != nil {
		attrs["port"] = types.Int64Value(*src.Port)
	} else {
		attrs["port"] = types.Int64Null()
	}

	populateTypedEndpointFields(attrs, src.MatchingTarget, src.IPs)

	return types.ObjectValueMust(endpointAttrTypes, attrs)
}

func destinationAPIToModel(dst *unifi.FirewallPolicyDestination) types.Object {
	attrs := map[string]attr.Value{
		"zone_id":            types.StringValue(dst.ZoneID),
		"port_matching_type": stringValueOrNull(dst.PortMatchingType),
		"port_group_id":      stringValueOrNull(dst.PortGroupID),
	}

	if dst.Port != nil {
		attrs["port"] = types.Int64Value(*dst.Port)
	} else {
		attrs["port"] = types.Int64Null()
	}

	populateTypedEndpointFields(attrs, dst.MatchingTarget, dst.IPs)

	return types.ObjectValueMust(endpointAttrTypes, attrs)
}

// populateTypedEndpointFields sets the correct typed field (ips, mac_addresses,
// network_ids, device_ids) based on the API's matching_target value, and sets
// the others to null.
func populateTypedEndpointFields(attrs map[string]attr.Value, matchingTarget string, ips []string) {
	setType := types.SetType{ElemType: types.StringType}
	nullSet := types.SetNull(types.StringType)

	// Default: all null.
	attrs["ips"] = nullSet
	attrs["mac_addresses"] = nullSet
	attrs["network_ids"] = nullSet
	attrs["device_ids"] = nullSet

	if ips == nil {
		return
	}

	vals := make([]attr.Value, len(ips))
	for i, v := range ips {
		vals[i] = types.StringValue(v)
	}
	sv := types.SetValueMust(setType.ElemType, vals)

	switch matchingTarget {
	case "IP":
		attrs["ips"] = sv
	case "MAC":
		attrs["mac_addresses"] = sv
	case "NETWORK":
		attrs["network_ids"] = sv
	case "CLIENT":
		attrs["device_ids"] = sv
	default:
		// ANY or unknown — leave all null.
	}
}

func scheduleAPIToModel(sched *unifi.FirewallPolicySchedule) types.Object {
	attrs := map[string]attr.Value{
		"mode":             stringValueOrNull(sched.Mode),
		"date":             stringValueOrNull(sched.Date),
		"time_all_day":     boolValueOrNull(sched.TimeAllDay),
		"time_range_start": stringValueOrNull(sched.TimeRangeStart),
		"time_range_end":   stringValueOrNull(sched.TimeRangeEnd),
	}

	if sched.RepeatOnDays != nil {
		vals := make([]attr.Value, len(sched.RepeatOnDays))
		for i, d := range sched.RepeatOnDays {
			vals[i] = types.StringValue(d)
		}
		attrs["repeat_on_days"] = types.SetValueMust(types.StringType, vals)
	} else {
		attrs["repeat_on_days"] = types.SetNull(types.StringType)
	}

	return types.ObjectValueMust(scheduleAttrTypes, attrs)
}

func stringValueOrNull(s string) types.String {
	if s != "" {
		return types.StringValue(s)
	}
	return types.StringNull()
}

// isDefaultSchedule returns true when the schedule is the API's default
// (mode=ALWAYS with no other fields set). We treat this as "no schedule
// configured" so that omitting the schedule block doesn't cause drift.
func isDefaultSchedule(s *unifi.FirewallPolicySchedule) bool {
	return s.Mode == "ALWAYS" &&
		s.Date == "" &&
		!s.TimeAllDay &&
		s.TimeRangeStart == "" &&
		s.TimeRangeEnd == "" &&
		len(s.RepeatOnDays) == 0
}
