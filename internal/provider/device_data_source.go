package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/ubiquiti-community/go-unifi/unifi"
)

var _ datasource.DataSource = &deviceDataSource{}

func NewDeviceDataSource() datasource.DataSource {
	return &deviceDataSource{}
}

type deviceDataSource struct {
	client *Client
}

type deviceDataSourceModel struct {
	ID       types.String `tfsdk:"id"`
	Site     types.String `tfsdk:"site"`
	MAC      types.String `tfsdk:"mac"`
	Name     types.String `tfsdk:"name"`
	Model    types.String `tfsdk:"model"`
	Type     types.String `tfsdk:"type"`
	IP       types.String `tfsdk:"ip"`
	Disabled types.Bool   `tfsdk:"disabled"`
	Adopted  types.Bool   `tfsdk:"adopted"`
	State    types.Int64  `tfsdk:"state"`
}

func (d *deviceDataSource) Metadata(
	_ context.Context,
	req datasource.MetadataRequest,
	resp *datasource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_device"
}

func (d *deviceDataSource) Schema(
	_ context.Context,
	_ datasource.SchemaRequest,
	resp *datasource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a UniFi network device (access point, switch, or gateway) by name or MAC address.",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the device to look up. Exactly one of `name` or `mac` must be specified.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(path.MatchRoot("mac")),
					stringvalidator.LengthAtLeast(1),
				},
			},

			"mac": schema.StringAttribute{
				MarkdownDescription: "The MAC address of the device to look up (e.g. `aa:bb:cc:dd:ee:ff`). " +
					"Exactly one of `name` or `mac` must be specified.",
				Optional: true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						macRegexp,
						"must be a valid MAC address (e.g. aa:bb:cc:dd:ee:ff)",
					),
				},
			},

			"site": schema.StringAttribute{
				MarkdownDescription: "The site to look up the device in. Defaults to the provider site.",
				Optional:            true,
			},

			"id": schema.StringAttribute{
				MarkdownDescription: "The ID of the device.",
				Computed:            true,
			},

			"model": schema.StringAttribute{
				MarkdownDescription: "The hardware model of the device (e.g. `U6-LR`, `US-16-XG`).",
				Computed:            true,
			},

			"type": schema.StringAttribute{
				MarkdownDescription: "The device type (e.g. `uap` for access point, `usw` for switch, `ugw` for gateway).",
				Computed:            true,
			},

			"ip": schema.StringAttribute{
				MarkdownDescription: "The current IP address of the device.",
				Computed:            true,
			},

			"disabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the device is administratively disabled.",
				Computed:            true,
			},

			"adopted": schema.BoolAttribute{
				MarkdownDescription: "Whether the device has been adopted by the controller.",
				Computed:            true,
			},

			"state": schema.Int64Attribute{
				MarkdownDescription: "The device state. 0 = unknown, 1 = connected, 2 = pending, " +
					"4 = upgrading, 5 = provisioning, 6 = heartbeat missed.",
				Computed: true,
			},
		},
	}
}

func (d *deviceDataSource) Configure(
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

func (d *deviceDataSource) Read(
	ctx context.Context,
	req datasource.ReadRequest,
	resp *datasource.ReadResponse,
) {
	var config deviceDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	site := d.client.SiteOrDefault(config.Site)

	var device *unifi.Device
	var err error

	if !config.MAC.IsNull() && !config.MAC.IsUnknown() {
		mac := strings.ToLower(config.MAC.ValueString())
		device, err = d.client.GetDeviceByMAC(ctx, site, mac)
		if err != nil {
			resp.Diagnostics.AddError(
				"Device Not Found",
				fmt.Sprintf("No device found with MAC %q in site %q: %s", mac, site, err.Error()),
			)
			return
		}
	} else {
		name := config.Name.ValueString()
		device, err = d.findDeviceByName(ctx, site, name)
		if err != nil {
			resp.Diagnostics.AddError(
				"Device Not Found",
				fmt.Sprintf("No device found with name %q in site %q: %s", name, site, err.Error()),
			)
			return
		}
	}

	d.apiToModel(device, &config, site)
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

func (d *deviceDataSource) findDeviceByName(ctx context.Context, site, name string) (*unifi.Device, error) {
	devices, err := d.client.ListDevice(ctx, site)
	if err != nil {
		return nil, err
	}
	for i := range devices {
		if devices[i].Name == name {
			return &devices[i], nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func (d *deviceDataSource) apiToModel(dev *unifi.Device, m *deviceDataSourceModel, site string) {
	m.ID = types.StringValue(dev.ID)
	m.Site = types.StringValue(site)
	m.MAC = types.StringValue(dev.MAC)
	m.Name = stringValueOrNull(dev.Name)
	m.Model = stringValueOrNull(dev.Model)
	m.Type = stringValueOrNull(dev.Type)
	m.IP = stringValueOrNull(dev.IP)
	m.Disabled = types.BoolValue(dev.Disabled)
	m.Adopted = types.BoolValue(dev.Adopted)
	m.State = types.Int64Value(int64(dev.State))
}
