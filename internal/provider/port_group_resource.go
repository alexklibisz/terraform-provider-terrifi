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
	"github.com/ubiquiti-community/go-unifi/unifi"
)

// Compile-time interface checks.
var (
	_ resource.Resource                = &portGroupResource{}
	_ resource.ResourceWithImportState = &portGroupResource{}
)

func NewPortGroupResource() resource.Resource {
	return &portGroupResource{}
}

type portGroupResource struct {
	client *Client
}

type portGroupResourceModel struct {
	ID    types.String `tfsdk:"id"`
	Site  types.String `tfsdk:"site"`
	Name  types.String `tfsdk:"name"`
	Ports types.Set    `tfsdk:"ports"`
}

func (r *portGroupResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_port_group"
}

func (r *portGroupResource) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a port group on the UniFi controller. Port groups are named collections " +
			"of port numbers or port ranges that can be referenced by firewall policies and rules.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The ID of the port group.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"site": schema.StringAttribute{
				MarkdownDescription: "The site to associate the port group with. Defaults to the provider site.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the port group.",
				Required:            true,
			},

			"ports": schema.SetAttribute{
				MarkdownDescription: "The ports in this group. Each entry is a port number (e.g. `\"80\"`) " +
					"or a port range (e.g. `\"8080-8090\"`).",
				Required:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func (r *portGroupResource) Configure(
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

func (r *portGroupResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var plan portGroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	site := r.client.SiteOrDefault(plan.Site)
	group, diags := r.modelToAPI(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreateFirewallGroup(ctx, site, group)
	if err != nil {
		resp.Diagnostics.AddError("Error Creating Port Group", err.Error())
		return
	}

	r.apiToModel(created, &plan, site)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *portGroupResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var state portGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	site := r.client.SiteOrDefault(state.Site)

	group, err := r.client.GetFirewallGroup(ctx, site, state.ID.ValueString())
	if err != nil {
		if _, ok := err.(*unifi.NotFoundError); ok {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Port Group",
			fmt.Sprintf("Could not read port group %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	r.apiToModel(group, &state, site)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *portGroupResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	var state, plan portGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.applyPlanToState(&plan, &state)

	site := r.client.SiteOrDefault(state.Site)
	group, diags := r.modelToAPI(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	group.ID = state.ID.ValueString()

	updated, err := r.client.UpdateFirewallGroup(ctx, site, group)
	if err != nil {
		resp.Diagnostics.AddError("Error Updating Port Group", err.Error())
		return
	}

	r.apiToModel(updated, &state, site)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *portGroupResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	var state portGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	site := r.client.SiteOrDefault(state.Site)

	err := r.client.DeleteFirewallGroup(ctx, site, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error Deleting Port Group", err.Error())
	}
}

// ImportState supports both "id" and "site:id" import formats.
func (r *portGroupResource) ImportState(
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

func (r *portGroupResource) applyPlanToState(plan, state *portGroupResourceModel) {
	if !plan.Name.IsNull() && !plan.Name.IsUnknown() {
		state.Name = plan.Name
	}
	if !plan.Ports.IsNull() && !plan.Ports.IsUnknown() {
		state.Ports = plan.Ports
	}
}

func (r *portGroupResource) modelToAPI(ctx context.Context, m *portGroupResourceModel) (*unifi.FirewallGroup, diag.Diagnostics) {
	group := &unifi.FirewallGroup{
		Name:      m.Name.ValueString(),
		GroupType: "port-group",
	}

	var ports []string
	diags := m.Ports.ElementsAs(ctx, &ports, false)
	if diags.HasError() {
		return nil, diags
	}
	group.GroupMembers = ports

	return group, diags
}

func (r *portGroupResource) apiToModel(group *unifi.FirewallGroup, m *portGroupResourceModel, site string) {
	m.ID = types.StringValue(group.ID)
	m.Site = types.StringValue(site)
	m.Name = types.StringValue(group.Name)

	if group.GroupMembers != nil {
		vals := make([]attr.Value, len(group.GroupMembers))
		for i, port := range group.GroupMembers {
			vals[i] = types.StringValue(port)
		}
		m.Ports = types.SetValueMust(types.StringType, vals)
	} else {
		m.Ports = types.SetValueMust(types.StringType, []attr.Value{})
	}
}
