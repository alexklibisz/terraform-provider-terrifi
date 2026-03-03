package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &firewallPolicyOrderResource{}
	_ resource.ResourceWithImportState = &firewallPolicyOrderResource{}
)

func NewFirewallPolicyOrderResource() resource.Resource {
	return &firewallPolicyOrderResource{}
}

type firewallPolicyOrderResource struct {
	client *Client
}

type firewallPolicyOrderResourceModel struct {
	ID                 types.String `tfsdk:"id"`
	Site               types.String `tfsdk:"site"`
	SourceZoneID       types.String `tfsdk:"source_zone_id"`
	DestinationZoneID  types.String `tfsdk:"destination_zone_id"`
	PolicyIDs          types.List   `tfsdk:"policy_ids"`
}

func (r *firewallPolicyOrderResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_firewall_policy_order"
}

func (r *firewallPolicyOrderResource) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the evaluation order of firewall policies for a specific zone pair. " +
			"Policies are evaluated in the order specified by `policy_ids`. " +
			"This resource uses the UniFi batch-reorder API to set the order of custom policies before predefined (system) policies.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Synthetic ID in the format `source_zone_id:destination_zone_id`.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"site": schema.StringAttribute{
				MarkdownDescription: "The site to associate the ordering with. Defaults to the provider site.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"source_zone_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the source firewall zone.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"destination_zone_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the destination firewall zone.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"policy_ids": schema.ListAttribute{
				MarkdownDescription: "Ordered list of firewall policy IDs. Policies are evaluated in this order, before any predefined (system) policies.",
				ElementType:         types.StringType,
				Required:            true,
			},
		},
	}
}

func (r *firewallPolicyOrderResource) Configure(
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

func (r *firewallPolicyOrderResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var plan firewallPolicyOrderResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	site := r.client.SiteOrDefault(plan.Site)
	sourceZoneID := plan.SourceZoneID.ValueString()
	destZoneID := plan.DestinationZoneID.ValueString()

	var policyIDs []string
	resp.Diagnostics.Append(plan.PolicyIDs.ElementsAs(ctx, &policyIDs, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.validatePolicyZones(ctx, site, sourceZoneID, destZoneID, policyIDs, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.ReorderFirewallPolicies(ctx, site, sourceZoneID, destZoneID, policyIDs)
	if err != nil {
		resp.Diagnostics.AddError("Error Reordering Firewall Policies", err.Error())
		return
	}

	plan.ID = types.StringValue(sourceZoneID + ":" + destZoneID)
	plan.Site = types.StringValue(site)

	// Read back the ordering from the API to populate state.
	r.readOrdering(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *firewallPolicyOrderResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var state firewallPolicyOrderResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.readOrdering(ctx, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *firewallPolicyOrderResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	var plan firewallPolicyOrderResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	site := r.client.SiteOrDefault(plan.Site)
	sourceZoneID := plan.SourceZoneID.ValueString()
	destZoneID := plan.DestinationZoneID.ValueString()

	var policyIDs []string
	resp.Diagnostics.Append(plan.PolicyIDs.ElementsAs(ctx, &policyIDs, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.validatePolicyZones(ctx, site, sourceZoneID, destZoneID, policyIDs, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.ReorderFirewallPolicies(ctx, site, sourceZoneID, destZoneID, policyIDs)
	if err != nil {
		resp.Diagnostics.AddError("Error Reordering Firewall Policies", err.Error())
		return
	}

	plan.Site = types.StringValue(site)

	// Read back the ordering from the API to populate state.
	r.readOrdering(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *firewallPolicyOrderResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	// No API call — ordering continues to exist on the controller.
	// Removing this resource simply removes Terraform's management of the order.
	tflog.Info(ctx, "Removing firewall policy order from state (ordering continues to exist on controller)")
}

func (r *firewallPolicyOrderResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	// Accepted formats:
	//   source_zone_id:destination_zone_id
	//   site:source_zone_id:destination_zone_id
	parts := strings.SplitN(req.ID, ":", 3)

	switch len(parts) {
	case 3:
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("site"), parts[0])...)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1]+":"+parts[2])...)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("source_zone_id"), parts[1])...)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("destination_zone_id"), parts[2])...)
	case 2:
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("source_zone_id"), parts[0])...)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("destination_zone_id"), parts[1])...)
	default:
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Expected format: source_zone_id:destination_zone_id or site:source_zone_id:destination_zone_id, got: %s", req.ID),
		)
	}
}

// validatePolicyZones checks that each policy in policyIDs belongs to the
// specified zone pair. The UniFi batch-reorder API returns a cryptic 404
// "Firewall Policy Not Found" when a policy's zones don't match, so this
// provides a clear error message instead.
func (r *firewallPolicyOrderResource) validatePolicyZones(
	ctx context.Context,
	site, sourceZoneID, destZoneID string,
	policyIDs []string,
	diags *diag.Diagnostics,
) {
	policies, err := r.client.ListFirewallPolicies(ctx, site)
	if err != nil {
		diags.AddError(
			"Error Validating Firewall Policies",
			fmt.Sprintf("Could not list firewall policies to validate zone membership: %s", err.Error()),
		)
		return
	}

	// Build a lookup by policy ID.
	type policyInfo struct {
		name         string
		sourceZoneID string
		destZoneID   string
	}
	byID := make(map[string]policyInfo, len(policies))
	for _, p := range policies {
		info := policyInfo{name: p.Name}
		if p.Source != nil {
			info.sourceZoneID = p.Source.ZoneID
		}
		if p.Destination != nil {
			info.destZoneID = p.Destination.ZoneID
		}
		byID[p.ID] = info
	}

	for _, id := range policyIDs {
		p, ok := byID[id]
		if !ok {
			diags.AddError(
				"Firewall Policy Not Found",
				fmt.Sprintf("Policy %q does not exist on the controller.", id),
			)
			continue
		}
		if p.sourceZoneID != sourceZoneID || p.destZoneID != destZoneID {
			diags.AddError(
				"Firewall Policy Zone Mismatch",
				fmt.Sprintf(
					"Policy %q (%s) has source_zone_id=%q and destination_zone_id=%q, "+
						"but this terrifi_firewall_policy_order expects source_zone_id=%q and destination_zone_id=%q. "+
						"Each policy's zones must match the ordering resource's zones.",
					id, p.name, p.sourceZoneID, p.destZoneID, sourceZoneID, destZoneID,
				),
			)
		}
	}
}

// readOrdering fetches the current policy ordering from the API and updates the
// model's PolicyIDs. It only updates PolicyIDs to include policies that are
// already in the user's configured list (to avoid importing unmanaged policies
// into state). Policies not in the API response are dropped.
func (r *firewallPolicyOrderResource) readOrdering(
	ctx context.Context,
	m *firewallPolicyOrderResourceModel,
	diags *diag.Diagnostics,
) {
	site := r.client.SiteOrDefault(m.Site)
	m.Site = types.StringValue(site)
	sourceZoneID := m.SourceZoneID.ValueString()
	destZoneID := m.DestinationZoneID.ValueString()

	apiOrder, err := r.client.GetFirewallPolicyOrdering(ctx, site, sourceZoneID, destZoneID)
	if err != nil {
		diags.AddError(
			"Error Reading Firewall Policy Ordering",
			fmt.Sprintf("Could not read firewall policy ordering for %s:%s: %s", sourceZoneID, destZoneID, err.Error()),
		)
		return
	}

	// Build a set of policy IDs the user has in their config.
	var configuredIDs []string
	if !m.PolicyIDs.IsNull() && !m.PolicyIDs.IsUnknown() {
		m.PolicyIDs.ElementsAs(ctx, &configuredIDs, false)
	}
	configuredSet := make(map[string]bool, len(configuredIDs))
	for _, id := range configuredIDs {
		configuredSet[id] = true
	}

	// Return API ordering filtered to only policies in the user's config.
	// This prevents unmanaged policies from appearing in state.
	var filtered []string
	for _, id := range apiOrder {
		if configuredSet[id] {
			filtered = append(filtered, id)
		}
	}

	vals := make([]attr.Value, len(filtered))
	for i, id := range filtered {
		vals[i] = types.StringValue(id)
	}
	m.PolicyIDs = types.ListValueMust(types.StringType, vals)
}
