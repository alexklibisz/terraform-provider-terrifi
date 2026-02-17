package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ubiquiti-community/go-unifi/unifi"
)

// ---------------------------------------------------------------------------
// Unit tests
// ---------------------------------------------------------------------------

func TestNetworkModelToAPI(t *testing.T) {
	r := &networkResource{}
	ctx := context.Background()

	t.Run("minimal corporate network", func(t *testing.T) {
		model := &networkResourceModel{
			Name:                  types.StringValue("Corporate"),
			Purpose:               types.StringValue("corporate"),
			DHCPEnabled:           types.BoolValue(false),
			NetworkGroup:          types.StringValue("LAN"),
			DHCPLease:             types.Int64Value(86400),
			InternetAccessEnabled: types.BoolValue(true),
		}

		net := r.modelToAPI(ctx, model)

		require.NotNil(t, net.Name)
		assert.Equal(t, "Corporate", *net.Name)
		assert.Equal(t, "corporate", net.Purpose)
		assert.True(t, net.Enabled)
		assert.False(t, net.DHCPDEnabled)
		assert.True(t, net.InternetAccessEnabled)
		require.NotNil(t, net.NetworkGroup)
		assert.Equal(t, "LAN", *net.NetworkGroup)
	})

	t.Run("corporate network with VLAN and DHCP", func(t *testing.T) {
		model := &networkResourceModel{
			Name:                  types.StringValue("IoT"),
			Purpose:               types.StringValue("corporate"),
			VLANId:                types.Int64Value(33),
			Subnet:                types.StringValue("192.168.33.0/24"),
			DHCPEnabled:           types.BoolValue(true),
			DHCPStart:             types.StringValue("192.168.33.10"),
			DHCPStop:              types.StringValue("192.168.33.250"),
			DHCPLease:             types.Int64Value(86400),
			NetworkGroup:          types.StringValue("LAN"),
			InternetAccessEnabled: types.BoolValue(true),
		}

		net := r.modelToAPI(ctx, model)

		require.NotNil(t, net.Name)
		assert.Equal(t, "IoT", *net.Name)
		assert.Equal(t, "corporate", net.Purpose)
		require.NotNil(t, net.VLAN)
		assert.Equal(t, int64(33), *net.VLAN)
		assert.True(t, net.VLANEnabled)
		require.NotNil(t, net.IPSubnet)
		assert.Equal(t, "192.168.33.0/24", *net.IPSubnet)
		assert.True(t, net.DHCPDEnabled)
		require.NotNil(t, net.DHCPDStart)
		assert.Equal(t, "192.168.33.10", *net.DHCPDStart)
		require.NotNil(t, net.DHCPDStop)
		assert.Equal(t, "192.168.33.250", *net.DHCPDStop)
		require.NotNil(t, net.DHCPDLeaseTime)
		assert.Equal(t, int64(86400), *net.DHCPDLeaseTime)
	})

	t.Run("dhcp dns list fans out to API fields", func(t *testing.T) {
		model := &networkResourceModel{
			Name:    types.StringValue("DNS Test"),
			Purpose: types.StringValue("corporate"),
			DHCPDns: types.ListValueMust(types.StringType, []attr.Value{
				types.StringValue("1.1.1.1"),
				types.StringValue("8.8.8.8"),
				types.StringValue("9.9.9.9"),
				types.StringValue("8.8.4.4"),
			}),
		}

		net := r.modelToAPI(ctx, model)

		assert.Equal(t, "1.1.1.1", net.DHCPDDNS1)
		assert.Equal(t, "8.8.8.8", net.DHCPDDNS2)
		assert.Equal(t, "9.9.9.9", net.DHCPDDNS3)
		assert.Equal(t, "8.8.4.4", net.DHCPDDNS4)
	})
}

func TestNetworkAPIToModel(t *testing.T) {
	r := &networkResource{}
	ctx := context.Background()

	t.Run("minimal network", func(t *testing.T) {
		name := "Test Network"
		net := &unifi.Network{
			ID:                    "abc123",
			Purpose:               "corporate",
			Name:                  &name,
			DHCPDEnabled:          false,
			InternetAccessEnabled: true,
		}

		var model networkResourceModel
		r.apiToModel(ctx, net, &model, "default")

		assert.Equal(t, "abc123", model.ID.ValueString())
		assert.Equal(t, "default", model.Site.ValueString())
		assert.Equal(t, "Test Network", model.Name.ValueString())
		assert.Equal(t, "corporate", model.Purpose.ValueString())
		assert.False(t, model.DHCPEnabled.ValueBool())
		assert.True(t, model.InternetAccessEnabled.ValueBool())
		assert.True(t, model.VLANId.IsNull())
		assert.True(t, model.Subnet.IsNull())
	})

	t.Run("network with VLAN and DHCP", func(t *testing.T) {
		name := "IoT"
		vlan := int64(33)
		subnet := "192.168.33.0/24"
		group := "LAN"
		start := "192.168.33.10"
		stop := "192.168.33.250"
		lease := int64(86400)

		net := &unifi.Network{
			ID:                    "def456",
			Purpose:               "corporate",
			Name:                  &name,
			VLAN:                  &vlan,
			VLANEnabled:           true,
			IPSubnet:              &subnet,
			NetworkGroup:          &group,
			DHCPDEnabled:          true,
			DHCPDStart:            &start,
			DHCPDStop:             &stop,
			DHCPDLeaseTime:        &lease,
			InternetAccessEnabled: true,
		}

		var model networkResourceModel
		r.apiToModel(ctx, net, &model, "default")

		assert.Equal(t, int64(33), model.VLANId.ValueInt64())
		assert.Equal(t, "192.168.33.0/24", model.Subnet.ValueString())
		assert.Equal(t, "LAN", model.NetworkGroup.ValueString())
		assert.True(t, model.DHCPEnabled.ValueBool())
		assert.Equal(t, "192.168.33.10", model.DHCPStart.ValueString())
		assert.Equal(t, "192.168.33.250", model.DHCPStop.ValueString())
		assert.Equal(t, int64(86400), model.DHCPLease.ValueInt64())
	})

	t.Run("network with DNS servers", func(t *testing.T) {
		name := "Test Network"
		net := &unifi.Network{
			ID:                    "ghi789",
			Purpose:               "corporate",
			Name:                  &name,
			DHCPDEnabled:          true,
			DHCPDDNS1:             "8.8.8.8",
			DHCPDDNS2:             "8.8.4.4",
			InternetAccessEnabled: true,
		}

		var model networkResourceModel
		r.apiToModel(ctx, net, &model, "default")

		assert.False(t, model.DHCPDns.IsNull())
		assert.Equal(t, 2, len(model.DHCPDns.Elements()))
	})
}

func TestNetworkApplyPlanToState(t *testing.T) {
	r := &networkResource{}

	t.Run("partial update preserves unchanged fields", func(t *testing.T) {
		state := &networkResourceModel{
			Name:         types.StringValue("Test Network"),
			Purpose:      types.StringValue("corporate"),
			VLANId:       types.Int64Value(33),
			Subnet:       types.StringValue("192.168.33.0/24"),
			DHCPEnabled:  types.BoolValue(true),
			DHCPStart:    types.StringValue("192.168.33.10"),
			DHCPStop:     types.StringValue("192.168.33.250"),
			NetworkGroup: types.StringValue("LAN"),
		}

		plan := &networkResourceModel{
			Name:        types.StringValue("Updated Network"),
			DHCPEnabled: types.BoolValue(false),
			VLANId:      types.Int64Null(),
			Subnet:      types.StringNull(),
		}

		r.applyPlanToState(plan, state)

		assert.Equal(t, "Updated Network", state.Name.ValueString())
		assert.False(t, state.DHCPEnabled.ValueBool())
		assert.Equal(t, int64(33), state.VLANId.ValueInt64())
		assert.Equal(t, "192.168.33.0/24", state.Subnet.ValueString())
		assert.Equal(t, "LAN", state.NetworkGroup.ValueString())
	})
}

// ---------------------------------------------------------------------------
// Acceptance tests
// ---------------------------------------------------------------------------

func TestAccNetwork_corporateWithVLANAndDHCP(t *testing.T) {
	name := fmt.Sprintf("tfacc-corp-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_network" "test" {
  name                     = %q
  purpose                  = "corporate"
  vlan_id                  = 33
  subnet                   = "192.168.33.1/24"
  network_group            = "LAN"
  dhcp_enabled             = true
  dhcp_start               = "192.168.33.6"
  dhcp_stop                = "192.168.33.254"
  dhcp_lease               = 86400
  dhcp_dns                 = ["8.8.8.8", "8.8.4.4"]
  internet_access_enabled  = true
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_network.test", "name", name),
					resource.TestCheckResourceAttr("terrifi_network.test", "purpose", "corporate"),
					resource.TestCheckResourceAttr("terrifi_network.test", "vlan_id", "33"),
					resource.TestCheckResourceAttr("terrifi_network.test", "subnet", "192.168.33.1/24"),
					resource.TestCheckResourceAttr("terrifi_network.test", "network_group", "LAN"),
					resource.TestCheckResourceAttr("terrifi_network.test", "dhcp_enabled", "true"),
					resource.TestCheckResourceAttr("terrifi_network.test", "dhcp_start", "192.168.33.6"),
					resource.TestCheckResourceAttr("terrifi_network.test", "dhcp_stop", "192.168.33.254"),
					resource.TestCheckResourceAttr("terrifi_network.test", "dhcp_lease", "86400"),
					resource.TestCheckResourceAttr("terrifi_network.test", "dhcp_dns.#", "2"),
					resource.TestCheckResourceAttr("terrifi_network.test", "dhcp_dns.0", "8.8.8.8"),
					resource.TestCheckResourceAttr("terrifi_network.test", "dhcp_dns.1", "8.8.4.4"),
					resource.TestCheckResourceAttr("terrifi_network.test", "internet_access_enabled", "true"),
					resource.TestCheckResourceAttr("terrifi_network.test", "site", "default"),
					resource.TestCheckResourceAttrSet("terrifi_network.test", "id"),
				),
			},
		},
	})
}

func TestAccNetwork_update(t *testing.T) {
	name := fmt.Sprintf("tfacc-update-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_network" "test" {
  name                     = %q
  purpose                  = "corporate"
  vlan_id                  = 50
  subnet                   = "192.168.50.1/24"
  dhcp_enabled             = true
  dhcp_start               = "192.168.50.6"
  dhcp_stop                = "192.168.50.254"
  dhcp_lease               = 86400
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_network.test", "dhcp_lease", "86400"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "terrifi_network" "test" {
  name                     = %q
  purpose                  = "corporate"
  vlan_id                  = 50
  subnet                   = "192.168.50.1/24"
  dhcp_enabled             = true
  dhcp_start               = "192.168.50.6"
  dhcp_stop                = "192.168.50.254"
  dhcp_lease               = 43200
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_network.test", "dhcp_lease", "43200"),
				),
			},
		},
	})
}

func TestAccNetwork_import(t *testing.T) {
	name := fmt.Sprintf("tfacc-import-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_network" "test" {
  name                     = %q
  purpose                  = "corporate"
  vlan_id                  = 60
  subnet                   = "192.168.60.1/24"
  dhcp_enabled             = true
  dhcp_start               = "192.168.60.6"
  dhcp_stop                = "192.168.60.254"
}
`, name),
			},
			{
				ResourceName:      "terrifi_network.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccNetwork_importSiteID(t *testing.T) {
	name := fmt.Sprintf("tfacc-impsid-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_network" "test" {
  name                     = %q
  purpose                  = "corporate"
  vlan_id                  = 80
  subnet                   = "192.168.80.1/24"
  dhcp_enabled             = true
  dhcp_start               = "192.168.80.6"
  dhcp_stop                = "192.168.80.254"
}
`, name),
			},
			{
				ResourceName:      "terrifi_network.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs := s.RootModule().Resources["terrifi_network.test"]
					if rs == nil {
						return "", fmt.Errorf("resource not found in state")
					}
					return fmt.Sprintf("%s:%s", rs.Primary.Attributes["site"], rs.Primary.Attributes["id"]), nil
				},
			},
		},
	})
}
