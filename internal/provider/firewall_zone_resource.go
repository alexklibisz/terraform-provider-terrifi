package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/ubiquiti-community/go-unifi/unifi"
)

var (
	_ resource.Resource                = &firewallZoneResource{}
	_ resource.ResourceWithImportState = &firewallZoneResource{}
)

func NewFirewallZoneResource() resource.Resource {
	return &firewallZoneResource{}
}

type firewallZoneResource struct {
	client *Client
}

type firewallZoneResourceModel struct {
	ID         types.String `tfsdk:"id"`
	Site       types.String `tfsdk:"site"`
	Name       types.String `tfsdk:"name"`
	NetworkIDs types.Set   `tfsdk:"network_ids"`
	ZoneKey    types.String `tfsdk:"zone_key"`
}

func (r *firewallZoneResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_firewall_zone"
}

func (r *firewallZoneResource) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a firewall zone on the UniFi controller. Firewall zones group networks together for firewall rule management.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The ID of the firewall zone.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"site": schema.StringAttribute{
				MarkdownDescription: "The site to associate the firewall zone with. Defaults to the provider site.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the firewall zone.",
				Required:            true,
			},

			"network_ids": schema.SetAttribute{
				MarkdownDescription: "Set of network IDs to associate with this firewall zone.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
			},

			"zone_key": schema.StringAttribute{
				MarkdownDescription: "The zone key assigned by the controller.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *firewallZoneResource) Configure(
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

func (r *firewallZoneResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var plan firewallZoneResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	site := r.client.SiteOrDefault(plan.Site)
	zone := r.modelToAPI(ctx, &plan)

	created, err := r.client.CreateFirewallZone(ctx, site, zone)
	if err != nil {
		resp.Diagnostics.AddError("Error Creating Firewall Zone", err.Error())
		return
	}

	r.apiToModel(created, &plan, site)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *firewallZoneResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var state firewallZoneResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	site := r.client.SiteOrDefault(state.Site)

	zone, err := r.client.GetFirewallZone(ctx, site, state.ID.ValueString())
	if err != nil {
		if _, ok := err.(*unifi.NotFoundError); ok {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Firewall Zone",
			fmt.Sprintf("Could not read firewall zone %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	r.apiToModel(zone, &state, site)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *firewallZoneResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	var state, plan firewallZoneResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.applyPlanToState(&plan, &state)

	site := r.client.SiteOrDefault(state.Site)
	zone := r.modelToAPI(ctx, &state)
	zone.ID = state.ID.ValueString()

	updated, err := r.client.UpdateFirewallZone(ctx, site, zone)
	if err != nil {
		resp.Diagnostics.AddError("Error Updating Firewall Zone", err.Error())
		return
	}

	r.apiToModel(updated, &state, site)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *firewallZoneResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	var state firewallZoneResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	site := r.client.SiteOrDefault(state.Site)

	err := r.client.DeleteFirewallZone(ctx, site, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error Deleting Firewall Zone", err.Error())
	}
}

func (r *firewallZoneResource) ImportState(
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

func (r *firewallZoneResource) applyPlanToState(plan, state *firewallZoneResourceModel) {
	if !plan.Name.IsNull() && !plan.Name.IsUnknown() {
		state.Name = plan.Name
	}
	if !plan.NetworkIDs.IsNull() && !plan.NetworkIDs.IsUnknown() {
		state.NetworkIDs = plan.NetworkIDs
	}
}

func (r *firewallZoneResource) modelToAPI(ctx context.Context, m *firewallZoneResourceModel) *unifi.FirewallZone {
	zone := &unifi.FirewallZone{
		Name: m.Name.ValueString(),
	}

	if !m.NetworkIDs.IsNull() && !m.NetworkIDs.IsUnknown() {
		var ids []string
		m.NetworkIDs.ElementsAs(ctx, &ids, false)
		zone.NetworkIDs = ids
	}

	return zone
}

func (r *firewallZoneResource) apiToModel(zone *unifi.FirewallZone, m *firewallZoneResourceModel, site string) {
	m.ID = types.StringValue(zone.ID)
	m.Site = types.StringValue(site)
	m.Name = types.StringValue(zone.Name)

	if zone.ZoneKey != "" {
		m.ZoneKey = types.StringValue(zone.ZoneKey)
	} else {
		m.ZoneKey = types.StringNull()
	}

	if zone.NetworkIDs != nil {
		vals := make([]attr.Value, len(zone.NetworkIDs))
		for i, id := range zone.NetworkIDs {
			vals[i] = types.StringValue(id)
		}
		m.NetworkIDs = types.SetValueMust(types.StringType, vals)
	} else {
		m.NetworkIDs = types.SetNull(types.StringType)
	}
}
