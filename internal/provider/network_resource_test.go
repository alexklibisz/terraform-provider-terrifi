package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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

	t.Run("vlan-only network skips DHCP and subnet", func(t *testing.T) {
		model := &networkResourceModel{
			Name:         types.StringValue("IoT VLAN"),
			Purpose:      types.StringValue("vlan-only"),
			VLANId:       types.Int64Value(100),
			NetworkGroup: types.StringValue("LAN"),
			// These fields would be nulled by ModifyPlan at plan time, but we
			// test modelToAPI in isolation so pass null explicitly.
			Subnet:      types.StringNull(),
			DHCPEnabled: types.BoolValue(false),
			DHCPStart:   types.StringNull(),
			DHCPStop:    types.StringNull(),
			DHCPLease:   types.Int64Null(),
			DHCPDns:     types.ListNull(types.StringType),
		}

		net := r.modelToAPI(ctx, model)

		assert.Equal(t, "vlan-only", net.Purpose)
		require.NotNil(t, net.VLAN)
		assert.Equal(t, int64(100), *net.VLAN)
		assert.True(t, net.VLANEnabled)
		// Subnet and DHCP must NOT be set for vlan-only.
		assert.Nil(t, net.IPSubnet)
		assert.False(t, net.DHCPDEnabled)
		assert.Nil(t, net.DHCPDStart)
		assert.Nil(t, net.DHCPDStop)
		assert.Nil(t, net.DHCPDLeaseTime)
		// Setting-preference workaround must NOT be applied for vlan-only.
		assert.Nil(t, net.SettingPreference)
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

// TestNetworkAPIToModelEmptyDNS distinguishes an explicit empty list from an
// unconfigured list while still accepting DNS servers returned by the controller.
func TestNetworkAPIToModelEmptyDNS(t *testing.T) {
	for _, tc := range []struct {
		name  string
		prior types.List
		want  types.List
	}{
		{"null", types.ListNull(types.StringType), types.ListNull(types.StringType)},
		{"unknown", types.ListUnknown(types.StringType), types.ListNull(types.StringType)},
		{"explicit empty", types.ListValueMust(types.StringType, nil), types.ListValueMust(types.StringType, nil)},
		{"previous servers removed externally", types.ListValueMust(types.StringType, []attr.Value{types.StringValue("1.1.1.1")}), types.ListNull(types.StringType)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &networkResource{}
			m := networkResourceModel{DHCPDns: tc.prior}
			r.apiToModel(t.Context(), &unifi.Network{Purpose: "corporate"}, &m, "default")
			require.Equal(t, tc.want, m.DHCPDns)
			r.apiToModel(t.Context(), &unifi.Network{Purpose: "corporate"}, &m, "default")
			require.Equal(t, tc.want, m.DHCPDns, "refresh must preserve the representation")
			r.apiToModel(t.Context(), &unifi.Network{Purpose: "corporate", DHCPDDNS1: "9.9.9.9"}, &m, "default")
			require.Equal(t, types.ListValueMust(types.StringType, []attr.Value{types.StringValue("9.9.9.9")}), m.DHCPDns)
		})
	}
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

	t.Run("vlan-only network nulls out DHCP and subnet", func(t *testing.T) {
		name := "IoT VLAN"
		vlan := int64(100)
		group := "LAN"

		net := &unifi.Network{
			ID:           "vlan123",
			Purpose:      "vlan-only",
			Name:         &name,
			VLAN:         &vlan,
			VLANEnabled:  true,
			NetworkGroup: &group,
		}

		var model networkResourceModel
		r.apiToModel(ctx, net, &model, "default")

		assert.Equal(t, "vlan-only", model.Purpose.ValueString())
		assert.Equal(t, int64(100), model.VLANId.ValueInt64())
		assert.Equal(t, "LAN", model.NetworkGroup.ValueString())
		// All IP/DHCP fields must be null for vlan-only.
		assert.True(t, model.Subnet.IsNull())
		assert.False(t, model.DHCPEnabled.ValueBool())
		assert.True(t, model.DHCPStart.IsNull())
		assert.True(t, model.DHCPStop.IsNull())
		assert.True(t, model.DHCPLease.IsNull())
		assert.True(t, model.DHCPDns.IsNull())
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

func TestAccNetwork_updateLease(t *testing.T) {
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

func TestAccNetwork_updateDHCP(t *testing.T) {
	name := fmt.Sprintf("tfacc-dhcp-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_network" "test" {
  name                     = %q
  purpose                  = "corporate"
  vlan_id                  = 40
  subnet                   = "192.168.40.1/24"
  dhcp_enabled             = true
  dhcp_start               = "192.168.40.6"
  dhcp_stop                = "192.168.40.254"
  dhcp_lease               = 86400
  dhcp_dns                 = ["8.8.8.8", "8.8.4.4"]
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_network.test", "dhcp_enabled", "true"),
					resource.TestCheckResourceAttr("terrifi_network.test", "dhcp_start", "192.168.40.6"),
					resource.TestCheckResourceAttr("terrifi_network.test", "dhcp_stop", "192.168.40.254"),
					resource.TestCheckResourceAttr("terrifi_network.test", "dhcp_lease", "86400"),
					resource.TestCheckResourceAttr("terrifi_network.test", "dhcp_dns.#", "2"),
					resource.TestCheckResourceAttr("terrifi_network.test", "dhcp_dns.0", "8.8.8.8"),
					resource.TestCheckResourceAttr("terrifi_network.test", "dhcp_dns.1", "8.8.4.4"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "terrifi_network" "test" {
  name                     = %q
  purpose                  = "corporate"
  vlan_id                  = 40
  subnet                   = "192.168.40.1/24"
  dhcp_enabled             = true
  dhcp_start               = "192.168.40.100"
  dhcp_stop                = "192.168.40.200"
  dhcp_lease               = 3600
  dhcp_dns                 = ["1.1.1.1", "9.9.9.9", "208.67.222.222"]
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_network.test", "dhcp_enabled", "true"),
					resource.TestCheckResourceAttr("terrifi_network.test", "dhcp_start", "192.168.40.100"),
					resource.TestCheckResourceAttr("terrifi_network.test", "dhcp_stop", "192.168.40.200"),
					resource.TestCheckResourceAttr("terrifi_network.test", "dhcp_lease", "3600"),
					resource.TestCheckResourceAttr("terrifi_network.test", "dhcp_dns.#", "3"),
					resource.TestCheckResourceAttr("terrifi_network.test", "dhcp_dns.0", "1.1.1.1"),
					resource.TestCheckResourceAttr("terrifi_network.test", "dhcp_dns.1", "9.9.9.9"),
					resource.TestCheckResourceAttr("terrifi_network.test", "dhcp_dns.2", "208.67.222.222"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "terrifi_network" "test" {
  name                     = %q
  purpose                  = "corporate"
  vlan_id                  = 40
  subnet                   = "192.168.40.1/24"
  dhcp_enabled             = false
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_network.test", "dhcp_enabled", "false"),
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

func TestAccNetwork_vlanOnly(t *testing.T) {
	name := fmt.Sprintf("tfacc-vlan-%s", randomSuffix())
	nameUpdated := fmt.Sprintf("tfacc-vlan-upd-%s", randomSuffix())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: create minimal vlan-only network (no subnet, DHCP, internet fields).
			{
				Config: fmt.Sprintf(`
resource "terrifi_network" "test" {
  name    = %q
  purpose = "vlan-only"
  vlan_id = 200
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_network.test", "name", name),
					resource.TestCheckResourceAttr("terrifi_network.test", "purpose", "vlan-only"),
					resource.TestCheckResourceAttr("terrifi_network.test", "vlan_id", "200"),
					resource.TestCheckResourceAttr("terrifi_network.test", "network_group", "LAN"),
					resource.TestCheckResourceAttr("terrifi_network.test", "dhcp_enabled", "false"),
					resource.TestCheckNoResourceAttr("terrifi_network.test", "subnet"),
					resource.TestCheckNoResourceAttr("terrifi_network.test", "dhcp_start"),
					resource.TestCheckNoResourceAttr("terrifi_network.test", "dhcp_stop"),
					resource.TestCheckNoResourceAttr("terrifi_network.test", "dhcp_lease"),
					resource.TestCheckResourceAttrSet("terrifi_network.test", "id"),
					resource.TestCheckResourceAttr("terrifi_network.test", "site", "default"),
				),
			},
			// Step 2: update name only — no plan diff on DHCP/subnet fields.
			{
				Config: fmt.Sprintf(`
resource "terrifi_network" "test" {
  name    = %q
  purpose = "vlan-only"
  vlan_id = 200
}
`, nameUpdated),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_network.test", "name", nameUpdated),
					resource.TestCheckResourceAttr("terrifi_network.test", "purpose", "vlan-only"),
					resource.TestCheckResourceAttr("terrifi_network.test", "vlan_id", "200"),
					resource.TestCheckResourceAttr("terrifi_network.test", "dhcp_enabled", "false"),
					resource.TestCheckNoResourceAttr("terrifi_network.test", "subnet"),
				),
			},
			// Step 3: import round-trip — state must round-trip cleanly with no diff.
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

// TestAccNetwork_updatePreservesUnmanagedFields changes settings the resource
// doesn't manage directly on the controller, the way a user would in the UniFi
// UI, then changes managed attributes through Terraform. The update must keep
// the unmanaged settings (it used to send the SDK's defaults for them:
// mdns_enabled/igmp_snooping false, ipv6_interface_type "none", ...) and must
// clear DNS servers dropped from dhcp_dns.
func TestAccNetwork_updatePreservesUnmanagedFields(t *testing.T) {
	name := fmt.Sprintf("tfacc-net-keep-%s", randomSuffix())
	vlan := randomVLAN()
	a, b := vlan/256, vlan%256

	config := func(name, dhcpStop string, dns ...string) string {
		quoted := make([]string, len(dns))
		for i, d := range dns {
			quoted[i] = fmt.Sprintf("%q", d)
		}
		return fmt.Sprintf(`
resource "terrifi_network" "test" {
  name         = %q
  purpose      = "corporate"
  vlan_id      = %d
  subnet       = "10.%d.%d.1/24"
  dhcp_enabled = true
  dhcp_start   = "10.%d.%d.6"
  dhcp_stop    = "10.%d.%d.%s"
  dhcp_dns     = [%s]
}
`, name, vlan, a, b, a, b, a, b, dhcpStop, strings.Join(quoted, ", "))
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(name, "254", "1.1.1.1", "8.8.8.8", "9.9.9.9"),
				Check: resource.ComposeTestCheckFunc(
					testAccNetworkSetUnmanaged("terrifi_network.test"),
					testAccNetworkCheckUnmanaged("terrifi_network.test"),
				),
			},
			// Managed changes: name, DHCP range, DNS servers replaced.
			{
				Config: config(name+"-renamed", "200", "9.9.9.9", "1.0.0.1", "8.8.4.4"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_network.test", "name", name+"-renamed"),
					resource.TestCheckResourceAttr("terrifi_network.test", "dhcp_stop", fmt.Sprintf("10.%d.%d.200", a, b)),
					testAccNetworkCheckUnmanaged("terrifi_network.test"),
					testAccNetworkCheckDNS("terrifi_network.test", "9.9.9.9", "1.0.0.1", "8.8.4.4", ""),
				),
			},
			// A shorter list clears the dropped servers.
			{
				Config: config(name+"-renamed", "200", "9.9.9.9"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_network.test", "dhcp_dns.#", "1"),
					testAccNetworkCheckUnmanaged("terrifi_network.test"),
					testAccNetworkCheckDNS("terrifi_network.test", "9.9.9.9", "", "", ""),
				),
			},
			// Clearing the final server must retain [] in state, including refresh.
			{
				Config: config(name+"-renamed", "200"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_network.test", "dhcp_dns.#", "0"),
					testAccNetworkCheckUnmanaged("terrifi_network.test"),
					testAccNetworkCheckDNS("terrifi_network.test", "", "", "", ""),
				),
			},
			{RefreshState: true, Check: resource.TestCheckResourceAttr("terrifi_network.test", "dhcp_dns.#", "0")},
			{Config: config(name+"-renamed", "200"), PlanOnly: true},
			{
				Config: config(name+"-renamed", "200", "1.1.1.1", "8.8.8.8"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_network.test", "dhcp_dns.#", "2"),
					testAccNetworkCheckUnmanaged("terrifi_network.test"),
					testAccNetworkCheckDNS("terrifi_network.test", "1.1.1.1", "8.8.8.8", "", ""),
				),
			},
		},
	})
}

// testAccRawNetwork reads the stored network as raw JSON, including fields the
// SDK doesn't decode.
func testAccRawNetwork(s *terraform.State, addr string) (*Client, string, string, map[string]any, error) {
	rs, ok := s.RootModule().Resources[addr]
	if !ok {
		return nil, "", "", nil, fmt.Errorf("resource %s not found in state", addr)
	}
	client := testAccGetClientNoT()
	site := client.SiteOrDefault(types.StringNull())
	var resp struct {
		Data []map[string]any `json:"data"`
	}
	if err := client.doV2Request(context.Background(), http.MethodGet, client.networkURL(site, rs.Primary.ID), nil, &resp); err != nil {
		return nil, "", "", nil, err
	}
	if len(resp.Data) != 1 {
		return nil, "", "", nil, fmt.Errorf("network %s not found", rs.Primary.ID)
	}
	return client, site, rs.Primary.ID, resp.Data[0], nil
}

// unmanagedNetworkSettings are set outside Terraform and must survive updates.
// They are a mix of fields the SDK marshals with defaults (mdns_enabled,
// igmp_snooping, domain_name) and fields it doesn't marshal at all
// (upnp_lan_enabled, ipv6_ra_preferred_lifetime). (setting_preference "auto"
// is left out: in auto mode the controller itself recomputes igmp_snooping and
// domain_name.)
var unmanagedNetworkSettings = map[string]any{
	"mdns_enabled":               true,
	"igmp_snooping":              true,
	"domain_name":                "tfacc.example",
	"upnp_lan_enabled":           true,
	"ipv6_ra_preferred_lifetime": float64(7200),
}

func testAccNetworkSetUnmanaged(addr string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, site, id, _, err := testAccRawNetwork(s, addr)
		if err != nil {
			return err
		}
		// The controller merges a PUT into the stored network.
		var resp struct {
			Meta struct {
				RC  string `json:"rc"`
				Msg string `json:"msg"`
			} `json:"meta"`
		}
		if err := client.doV2Request(context.Background(), http.MethodPut, client.networkURL(site, id), unmanagedNetworkSettings, &resp); err != nil {
			return err
		}
		if resp.Meta.RC != "ok" {
			return fmt.Errorf("setting unmanaged fields: rc=%s msg=%s", resp.Meta.RC, resp.Meta.Msg)
		}
		return nil
	}
}

func testAccNetworkCheckUnmanaged(addr string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		_, _, _, n, err := testAccRawNetwork(s, addr)
		if err != nil {
			return err
		}
		for k, want := range unmanagedNetworkSettings {
			if n[k] != want {
				return fmt.Errorf("%s = %v, want %v (reset by the update)", k, n[k], want)
			}
		}
		return nil
	}
}

func testAccNetworkCheckDNS(addr string, want ...string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		_, _, _, n, err := testAccRawNetwork(s, addr)
		if err != nil {
			return err
		}
		for i, w := range want {
			k := fmt.Sprintf("dhcpd_dns_%d", i+1)
			got, _ := n[k].(string) // absent counts as ""
			if got != w {
				return fmt.Errorf("%s = %q, want %q", k, got, w)
			}
		}
		return nil
	}
}

func TestNetworkManagedFields(t *testing.T) {
	ctx := context.Background()
	r := &networkResource{}

	t.Run("corporate: managed keys only, all DNS slots explicit", func(t *testing.T) {
		dns, _ := types.ListValueFrom(ctx, types.StringType, []string{"192.168.10.2"})
		f := r.managedFields(ctx, &networkResourceModel{
			Purpose:               types.StringValue("corporate"),
			Name:                  types.StringValue("servers"),
			VLANId:                types.Int64Value(999),
			Subnet:                types.StringValue("192.168.10.1/24"),
			DHCPEnabled:           types.BoolValue(true),
			DHCPStart:             types.StringValue("192.168.10.6"),
			DHCPStop:              types.StringValue("192.168.10.254"),
			DHCPLease:             types.Int64Value(86400),
			DHCPDns:               dns,
			InternetAccessEnabled: types.BoolValue(true),
			NetworkGroup:          types.StringNull(),
		})
		assert.Equal(t, map[string]any{
			"purpose":                 "corporate",
			"name":                    "servers",
			"vlan":                    int64(999),
			"vlan_enabled":            true,
			"ip_subnet":               "192.168.10.1/24",
			"dhcpd_enabled":           true,
			"dhcpd_start":             "192.168.10.6",
			"dhcpd_stop":              "192.168.10.254",
			"dhcpd_leasetime":         int64(86400),
			"dhcpd_dns_1":             "192.168.10.2",
			"dhcpd_dns_2":             "",
			"dhcpd_dns_3":             "",
			"dhcpd_dns_4":             "",
			"internet_access_enabled": true,
			"setting_preference":      "manual",
		}, f)
		assert.NotContains(t, f, "enabled", "enabled is not managed")
	})

	t.Run("unset and unknown attributes are left out", func(t *testing.T) {
		f := r.managedFields(ctx, &networkResourceModel{
			Purpose:   types.StringValue("corporate"),
			Name:      types.StringValue("n"),
			DHCPStart: types.StringUnknown(),
			DHCPDns:   types.ListNull(types.StringType),
		})
		assert.Equal(t, map[string]any{"purpose": "corporate", "name": "n", "setting_preference": "manual"}, f)
	})

	t.Run("vlan-only: no IP or DHCP fields", func(t *testing.T) {
		f := r.managedFields(ctx, &networkResourceModel{
			Purpose: types.StringValue("vlan-only"),
			Name:    types.StringValue("v"),
			VLANId:  types.Int64Value(30),
			Subnet:  types.StringValue("10.0.0.1/24"),
		})
		assert.Equal(t, map[string]any{"purpose": "vlan-only", "name": "v", "vlan": int64(30), "vlan_enabled": true}, f)
	})
}

// TestMergeNetwork merges managed fields into a stored network shaped like a
// UDR's prefix-delegated LAN: unmanaged fields come back byte for byte.
func TestMergeNetwork(t *testing.T) {
	stored := `{
		"_id": "net-1", "name": "IOT-CONTROLLER", "purpose": "corporate", "enabled": false,
		"ipv6_interface_type": "pd", "ipv6_pd_interface": "wan", "ipv6_ra_enabled": true,
		"ipv6_ra_preferred_lifetime": "14400", "mdns_enabled": true, "igmp_snooping": true,
		"setting_preference": "auto", "dhcp_relay_servers": ["10.0.0.5"],
		"dhcpd_dns_1": "1.1.1.1", "dhcpd_dns_2": "8.8.8.8", "dhcpd_dns_3": "9.9.9.9"
	}`
	var existing map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(stored), &existing))

	merged, err := mergeNetwork(existing, map[string]any{
		"purpose": "corporate", "name": "servers", "vlan": int64(999), "vlan_enabled": true,
		"dhcpd_dns_1": "192.168.10.2", "dhcpd_dns_2": "", "dhcpd_dns_3": "", "dhcpd_dns_4": "",
	})
	require.NoError(t, err)
	b, err := json.Marshal(merged)
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(b, &body))

	// Managed: overwritten, dropped DNS servers sent as "".
	assert.Equal(t, "servers", body["name"])
	assert.Equal(t, float64(999), body["vlan"])
	assert.Equal(t, "192.168.10.2", body["dhcpd_dns_1"])
	assert.Equal(t, "", body["dhcpd_dns_2"])
	assert.Equal(t, "", body["dhcpd_dns_3"])
	// Unmanaged: untouched, including types (string-encoded number) and lists.
	assert.Equal(t, "pd", body["ipv6_interface_type"])
	assert.Equal(t, "wan", body["ipv6_pd_interface"])
	assert.Equal(t, true, body["ipv6_ra_enabled"])
	assert.Equal(t, "14400", body["ipv6_ra_preferred_lifetime"])
	assert.Equal(t, true, body["mdns_enabled"])
	assert.Equal(t, true, body["igmp_snooping"])
	assert.Equal(t, "auto", body["setting_preference"])
	assert.Equal(t, false, body["enabled"])
	assert.Equal(t, []any{"10.0.0.5"}, body["dhcp_relay_servers"])
	assert.Equal(t, "net-1", body["_id"])
}

func TestAccNetwork_internetAccessToggle(t *testing.T) {
	name := "tfacc-internet-" + randomSuffix()
	config := func(enabled bool) string {
		return fmt.Sprintf(`
resource "terrifi_network" "test" {
 name = %q
 purpose = "corporate"
 vlan_id = 49
 subnet = "192.168.49.1/24"
 dhcp_enabled = false
 internet_access_enabled = %t
}
`, name, enabled)
	}
	resource.Test(t, resource.TestCase{
		PreCheck: func() { preCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config(false), Check: resource.TestCheckResourceAttr("terrifi_network.test", "internet_access_enabled", "false")},
			{Config: config(true), Check: resource.TestCheckResourceAttr("terrifi_network.test", "internet_access_enabled", "true")},
			{Config: config(false), Check: resource.TestCheckResourceAttr("terrifi_network.test", "internet_access_enabled", "false")},
		},
	})
}

// TestAccNetwork_emptyDNSOnCreate covers an explicitly empty DNS list before
// the controller has ever stored any user-supplied servers.
func TestAccNetwork_emptyDNSOnCreate(t *testing.T) {
	vlan := randomVLAN()
	config := fmt.Sprintf(`
resource "terrifi_network" "test" {
 name = %q
 purpose = "corporate"
 vlan_id = %d
 subnet = "10.%d.%d.1/24"
 dhcp_dns = []
}
`, "tfacc-empty-dns-"+randomSuffix(), vlan, vlan/256, vlan%256)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config, Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("terrifi_network.test", "dhcp_dns.#", "0"),
				testAccNetworkCheckDNS("terrifi_network.test", "", "", "", ""),
			)},
			{RefreshState: true, Check: resource.TestCheckResourceAttr("terrifi_network.test", "dhcp_dns.#", "0")},
			{Config: config, PlanOnly: true},
		},
	})
}

// TestAccNetwork_importEmptyDNS persists the imported state so the subsequent
// apply reconciles the imported DNS value with an explicitly configured [].
func TestAccNetwork_importEmptyDNS(t *testing.T) {
	vlan := randomVLAN()
	config := fmt.Sprintf(`
resource "terrifi_network" "test" {
  name     = %q
  purpose  = "corporate"
  vlan_id  = %d
  subnet   = "10.%d.%d.1/24"
  dhcp_dns = []
}
`, "tfacc-import-empty-dns-"+randomSuffix(), vlan, vlan/256, vlan%256)
	var originalID string
	checkEmptyDNS := resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr("terrifi_network.test", "dhcp_dns.#", "0"),
		testAccNetworkCheckDNS("terrifi_network.test", "", "", "", ""),
	)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(checkEmptyDNS, func(s *terraform.State) error {
					originalID = s.RootModule().Resources["terrifi_network.test"].Primary.ID
					return nil
				}),
			},
			{
				ResourceName:       "terrifi_network.test",
				ImportState:        true,
				ImportStatePersist: true,
				ImportStateVerify:  true,
				// Import has no prior empty-list state and may read null.
				// Verify the configured [] after applying the imported state.
				ImportStateVerifyIgnore: []string{"dhcp_dns"},
			},
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(checkEmptyDNS, func(s *terraform.State) error {
					if got := s.RootModule().Resources["terrifi_network.test"].Primary.ID; got != originalID {
						return fmt.Errorf("network was replaced after import: ID = %q, want %q", got, originalID)
					}
					return nil
				}),
			},
			{RefreshState: true, Check: checkEmptyDNS},
			{Config: config, PlanOnly: true},
		},
	})
}
