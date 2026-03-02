package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &clientGroupDeviceIDsDataSource{}

func NewClientGroupDeviceIDsDataSource() datasource.DataSource {
	return &clientGroupDeviceIDsDataSource{}
}

type clientGroupDeviceIDsDataSource struct {
	client *Client
}

type clientGroupDeviceIDsDataSourceModel struct {
	Site          types.String `tfsdk:"site"`
	ClientGroupID types.String `tfsdk:"client_group_id"`
	IDs           types.Set    `tfsdk:"ids"`
}

func (d *clientGroupDeviceIDsDataSource) Metadata(
	_ context.Context,
	req datasource.MetadataRequest,
	resp *datasource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_client_group_device_ids"
}

func (d *clientGroupDeviceIDsDataSource) Schema(
	_ context.Context,
	_ datasource.SchemaRequest,
	resp *datasource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Returns the IDs of client devices belonging to a particular client group. " +
			"Use this to reference grouped devices in firewall policies.",

		Attributes: map[string]schema.Attribute{
			"client_group_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the client group to look up.",
				Required:            true,
			},
			"site": schema.StringAttribute{
				MarkdownDescription: "The site to query. Defaults to the provider site.",
				Optional:            true,
				Computed:            true,
			},
			"ids": schema.SetAttribute{
				MarkdownDescription: "The set of client device IDs belonging to the group.",
				Computed:            true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (d *clientGroupDeviceIDsDataSource) Configure(
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

func (d *clientGroupDeviceIDsDataSource) Read(
	ctx context.Context,
	req datasource.ReadRequest,
	resp *datasource.ReadResponse,
) {
	var config clientGroupDeviceIDsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	site := d.client.SiteOrDefault(config.Site)
	groupID := config.ClientGroupID.ValueString()

	devices, err := d.client.ListClientDevices(ctx, site)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Listing Client Devices",
			fmt.Sprintf("Could not list client devices for site %q: %s", site, err.Error()),
		)
		return
	}

	var ids []string
	for _, dev := range devices {
		if dev.UserGroupID == groupID {
			ids = append(ids, dev.ID)
		}
	}

	idSet, diags := types.SetValueFrom(ctx, types.StringType, ids)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	config.Site = types.StringValue(site)
	config.IDs = idSet
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
