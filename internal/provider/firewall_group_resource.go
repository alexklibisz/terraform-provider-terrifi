package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/ubiquiti-community/go-unifi/unifi"
)

// Compile-time interface checks.
var (
	_ resource.Resource                = &firewallGroupResource{}
	_ resource.ResourceWithImportState = &firewallGroupResource{}
)

// NewFirewallGroupResource is the factory function registered in provider.Resources().
func NewFirewallGroupResource() resource.Resource {
	return &firewallGroupResource{}
}

// firewallGroupResource holds the API client, injected by Configure().
type firewallGroupResource struct {
	client *Client
}

// firewallGroupResourceModel is the Terraform-side representation of a firewall group.
type firewallGroupResourceModel struct {
	ID      types.String `tfsdk:"id"`
	Site    types.String `tfsdk:"site"`
	Name    types.String `tfsdk:"name"`
	Type    types.String `tfsdk:"type"`
	Members types.Set    `tfsdk:"members"`
}

// Metadata sets the resource type name.
func (r *firewallGroupResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_firewall_group"
}

// Schema defines the HCL schema for the terrifi_firewall_group resource.
func (r *firewallGroupResource) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a firewall group on the UniFi controller. Firewall groups are " +
			"named collections of ports or addresses that can be referenced by firewall policies and rules.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The ID of the firewall group.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"site": schema.StringAttribute{
				MarkdownDescription: "The site to associate the firewall group with. Defaults to the provider site.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the firewall group.",
				Required:            true,
			},

			"type": schema.StringAttribute{
				MarkdownDescription: "The type of firewall group. One of: `port-group`, `address-group`, `ipv6-address-group`.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("port-group", "address-group", "ipv6-address-group"),
				},
			},

			"members": schema.SetAttribute{
				MarkdownDescription: "The members of the firewall group. For `port-group`, these are port numbers " +
					"or port ranges (e.g. `\"80\"`, `\"8080-8090\"`). For `address-group`, these are IPv4 addresses " +
					"or CIDRs. For `ipv6-address-group`, these are IPv6 addresses or CIDRs.",
				Required:    true,
				ElementType: types.StringType,
			},
		},
	}
}

// Configure is called by the framework to inject the provider's API client.
func (r *firewallGroupResource) Configure(
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

// Create creates a new firewall group.
func (r *firewallGroupResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var plan firewallGroupResourceModel
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
		resp.Diagnostics.AddError("Error Creating Firewall Group", err.Error())
		return
	}

	r.apiToModel(created, &plan, site)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read refreshes the Terraform state from the actual API state.
func (r *firewallGroupResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var state firewallGroupResourceModel
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
			"Error Reading Firewall Group",
			fmt.Sprintf("Could not read firewall group %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	r.apiToModel(group, &state, site)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update updates an existing firewall group.
func (r *firewallGroupResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	var state, plan firewallGroupResourceModel
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
		resp.Diagnostics.AddError("Error Updating Firewall Group", err.Error())
		return
	}

	r.apiToModel(updated, &state, site)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Delete removes the firewall group from the UniFi controller.
func (r *firewallGroupResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	var state firewallGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	site := r.client.SiteOrDefault(state.Site)

	err := r.client.DeleteFirewallGroup(ctx, site, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error Deleting Firewall Group", err.Error())
	}
}

// ImportState handles `terraform import terrifi_firewall_group.name <id>`.
// Supports both "id" and "site:id" formats.
func (r *firewallGroupResource) ImportState(
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

// applyPlanToState merges the user's planned changes into the current state.
func (r *firewallGroupResource) applyPlanToState(plan, state *firewallGroupResourceModel) {
	if !plan.Name.IsNull() && !plan.Name.IsUnknown() {
		state.Name = plan.Name
	}
	if !plan.Type.IsNull() && !plan.Type.IsUnknown() {
		state.Type = plan.Type
	}
	if !plan.Members.IsNull() && !plan.Members.IsUnknown() {
		state.Members = plan.Members
	}
}

// modelToAPI converts our Terraform model to the go-unifi FirewallGroup struct.
func (r *firewallGroupResource) modelToAPI(ctx context.Context, m *firewallGroupResourceModel) (*unifi.FirewallGroup, diag.Diagnostics) {
	group := &unifi.FirewallGroup{
		Name:      m.Name.ValueString(),
		GroupType: m.Type.ValueString(),
	}

	var members []string
	diags := m.Members.ElementsAs(ctx, &members, false)
	if diags.HasError() {
		return nil, diags
	}
	group.GroupMembers = members

	return group, diags
}

// apiToModel converts the go-unifi FirewallGroup struct back to our Terraform model.
func (r *firewallGroupResource) apiToModel(group *unifi.FirewallGroup, m *firewallGroupResourceModel, site string) {
	m.ID = types.StringValue(group.ID)
	m.Site = types.StringValue(site)
	m.Name = types.StringValue(group.Name)
	m.Type = types.StringValue(group.GroupType)

	if group.GroupMembers != nil {
		vals := make([]attr.Value, len(group.GroupMembers))
		for i, member := range group.GroupMembers {
			vals[i] = types.StringValue(member)
		}
		m.Members = types.SetValueMust(types.StringType, vals)
	} else {
		m.Members = types.SetValueMust(types.StringType, []attr.Value{})
	}
}
