package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/ubiquiti-community/go-unifi/unifi"
)

var _ datasource.DataSource = &portGroupDataSource{}

func NewPortGroupDataSource() datasource.DataSource {
	return &portGroupDataSource{}
}

type portGroupDataSource struct {
	client *Client
}

type portGroupDataSourceModel struct {
	ID    types.String `tfsdk:"id"`
	Site  types.String `tfsdk:"site"`
	Name  types.String `tfsdk:"name"`
	Ports types.Set    `tfsdk:"ports"`
}

func (d *portGroupDataSource) Metadata(
	_ context.Context,
	req datasource.MetadataRequest,
	resp *datasource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_port_group"
}

func (d *portGroupDataSource) Schema(
	_ context.Context,
	_ datasource.SchemaRequest,
	resp *datasource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a UniFi port group by name.",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the port group to look up.",
				Required:            true,
			},

			"site": schema.StringAttribute{
				MarkdownDescription: "The site to look up the port group in. Defaults to the provider site.",
				Optional:            true,
			},

			"id": schema.StringAttribute{
				MarkdownDescription: "The ID of the port group.",
				Computed:            true,
			},

			"ports": schema.SetAttribute{
				MarkdownDescription: "The ports in this group. Each entry is a port number (e.g. `\"80\"`) " +
					"or a port range (e.g. `\"8080-8090\"`).",
				Computed:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func (d *portGroupDataSource) Configure(
	_ context.Context,
	req datasource.ConfigureRequest,
	resp *datasource.ConfigureResponse,
) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *Client, got: %T.", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *portGroupDataSource) Read(
	ctx context.Context,
	req datasource.ReadRequest,
	resp *datasource.ReadResponse,
) {
	var config portGroupDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	site := d.client.SiteOrDefault(config.Site)

	group, err := d.findPortGroupByName(ctx, site, config.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Port Group Not Found",
			fmt.Sprintf("No port group found with name %q in site %q: %s", config.Name.ValueString(), site, err.Error()),
		)
		return
	}

	d.apiToModel(group, &config, site)
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

func (d *portGroupDataSource) findPortGroupByName(ctx context.Context, site, name string) (*unifi.FirewallGroup, error) {
	groups, err := d.client.ListFirewallGroup(ctx, site)
	if err != nil {
		return nil, err
	}
	for i := range groups {
		if groups[i].GroupType == "port-group" && groups[i].Name == name {
			return &groups[i], nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func (d *portGroupDataSource) apiToModel(group *unifi.FirewallGroup, m *portGroupDataSourceModel, site string) {
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
