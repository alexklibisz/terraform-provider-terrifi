package provider

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/ubiquiti-community/go-unifi/unifi"
)

// requireHardware skips the test when running against the Docker simulation
// controller. The zone-based firewall v2 API requires a fully adopted gateway
// which the simulation mode doesn't provide.
func requireHardware(t *testing.T) {
	t.Helper()
	target := os.Getenv("TERRIFI_ACC_TARGET")
	if target == "" || target == "docker" {
		t.Skip("firewall zone tests require hardware (TERRIFI_ACC_TARGET=hardware)")
	}
}

// ---------------------------------------------------------------------------
// Unit tests
// ---------------------------------------------------------------------------

func TestFirewallZoneModelToAPI(t *testing.T) {
	r := &firewallZoneResource{}
	ctx := context.Background()

	t.Run("name only", func(t *testing.T) {
		model := &firewallZoneResourceModel{
			Name:       types.StringValue("My Zone"),
			NetworkIDs: types.SetNull(types.StringType),
		}

		zone := r.modelToAPI(ctx, model)

		assert.Equal(t, "My Zone", zone.Name)
		assert.Nil(t, zone.NetworkIDs)
	})

	t.Run("with network IDs", func(t *testing.T) {
		model := &firewallZoneResourceModel{
			Name: types.StringValue("My Zone"),
			NetworkIDs: types.SetValueMust(types.StringType, []attr.Value{
				types.StringValue("net-001"),
				types.StringValue("net-002"),
			}),
		}

		zone := r.modelToAPI(ctx, model)

		assert.Equal(t, "My Zone", zone.Name)
		assert.ElementsMatch(t, []string{"net-001", "net-002"}, zone.NetworkIDs)
	})

	t.Run("empty network IDs set", func(t *testing.T) {
		model := &firewallZoneResourceModel{
			Name:       types.StringValue("Empty Zone"),
			NetworkIDs: types.SetValueMust(types.StringType, []attr.Value{}),
		}

		zone := r.modelToAPI(ctx, model)

		assert.Equal(t, "Empty Zone", zone.Name)
		assert.Empty(t, zone.NetworkIDs)
	})

	t.Run("null network IDs vs unknown", func(t *testing.T) {
		model := &firewallZoneResourceModel{
			Name:       types.StringValue("Null Zone"),
			NetworkIDs: types.SetNull(types.StringType),
		}

		zone := r.modelToAPI(ctx, model)
		assert.Nil(t, zone.NetworkIDs)

		model.NetworkIDs = types.SetUnknown(types.StringType)
		zone = r.modelToAPI(ctx, model)
		assert.Nil(t, zone.NetworkIDs)
	})
}

func TestFirewallZoneAPIToModel(t *testing.T) {
	r := &firewallZoneResource{}

	t.Run("minimal zone", func(t *testing.T) {
		zone := &unifi.FirewallZone{
			ID:   "zone-001",
			Name: "My Zone",
		}

		var model firewallZoneResourceModel
		r.apiToModel(zone, &model, "default")

		assert.Equal(t, "zone-001", model.ID.ValueString())
		assert.Equal(t, "default", model.Site.ValueString())
		assert.Equal(t, "My Zone", model.Name.ValueString())
		assert.True(t, model.ZoneKey.IsNull())
		assert.True(t, model.NetworkIDs.IsNull())
	})

	t.Run("with network IDs and zone key", func(t *testing.T) {
		zone := &unifi.FirewallZone{
			ID:         "zone-002",
			Name:       "Full Zone",
			NetworkIDs: []string{"net-001", "net-002"},
			ZoneKey:    "zone_key_abc",
		}

		var model firewallZoneResourceModel
		r.apiToModel(zone, &model, "mysite")

		assert.Equal(t, "zone-002", model.ID.ValueString())
		assert.Equal(t, "mysite", model.Site.ValueString())
		assert.Equal(t, "Full Zone", model.Name.ValueString())
		assert.Equal(t, "zone_key_abc", model.ZoneKey.ValueString())
		assert.False(t, model.NetworkIDs.IsNull())
		assert.Equal(t, 2, len(model.NetworkIDs.Elements()))
	})

	t.Run("empty network IDs returns empty set", func(t *testing.T) {
		zone := &unifi.FirewallZone{
			ID:         "zone-003",
			Name:       "Empty Zone",
			NetworkIDs: []string{},
		}

		var model firewallZoneResourceModel
		r.apiToModel(zone, &model, "default")

		assert.False(t, model.NetworkIDs.IsNull())
		assert.Equal(t, 0, len(model.NetworkIDs.Elements()))
	})

	t.Run("nil network IDs returns null", func(t *testing.T) {
		zone := &unifi.FirewallZone{
			ID:   "zone-004",
			Name: "Nil Zone",
		}

		var model firewallZoneResourceModel
		r.apiToModel(zone, &model, "default")

		assert.True(t, model.NetworkIDs.IsNull())
	})
}

func TestFirewallZoneApplyPlanToState(t *testing.T) {
	r := &firewallZoneResource{}

	t.Run("partial update preserves unchanged fields", func(t *testing.T) {
		state := &firewallZoneResourceModel{
			Name: types.StringValue("Old Zone"),
			NetworkIDs: types.SetValueMust(types.StringType, []attr.Value{
				types.StringValue("net-001"),
			}),
		}

		plan := &firewallZoneResourceModel{
			Name:       types.StringValue("New Zone"),
			NetworkIDs: types.SetNull(types.StringType),
		}

		r.applyPlanToState(plan, state)

		assert.Equal(t, "New Zone", state.Name.ValueString())
		// NetworkIDs is null in plan, so state should be preserved
		assert.Equal(t, 1, len(state.NetworkIDs.Elements()))
	})

	t.Run("full update", func(t *testing.T) {
		state := &firewallZoneResourceModel{
			Name: types.StringValue("Old Zone"),
			NetworkIDs: types.SetValueMust(types.StringType, []attr.Value{
				types.StringValue("net-001"),
			}),
		}

		plan := &firewallZoneResourceModel{
			Name: types.StringValue("New Zone"),
			NetworkIDs: types.SetValueMust(types.StringType, []attr.Value{
				types.StringValue("net-002"),
				types.StringValue("net-003"),
			}),
		}

		r.applyPlanToState(plan, state)

		assert.Equal(t, "New Zone", state.Name.ValueString())
		assert.Equal(t, 2, len(state.NetworkIDs.Elements()))
	})
}

// ---------------------------------------------------------------------------
// Acceptance tests
// ---------------------------------------------------------------------------

func TestAccFirewallZone_basic(t *testing.T) {
	name := fmt.Sprintf("tfacc-zone-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t); requireHardware(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_firewall_zone" "test" {
  name = %q
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_zone.test", "name", name),
					resource.TestCheckResourceAttr("terrifi_firewall_zone.test", "site", "default"),
					resource.TestCheckResourceAttrSet("terrifi_firewall_zone.test", "id"),
				),
			},
		},
	})
}

func TestAccFirewallZone_withNetworks(t *testing.T) {
	zoneName := fmt.Sprintf("tfacc-zone-nets-%s", randomSuffix())
	netName := fmt.Sprintf("tfacc-net-%s", randomSuffix())
	vlan := randomVLAN()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t); requireHardware(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_network" "test" {
  name    = %q
  purpose = "corporate"
  vlan_id = %d
  subnet  = "10.%d.%d.1/24"
}

resource "terrifi_firewall_zone" "test" {
  name        = %q
  network_ids = [terrifi_network.test.id]
}
`, netName, vlan, vlan/256, vlan%256, zoneName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_zone.test", "name", zoneName),
					resource.TestCheckResourceAttr("terrifi_firewall_zone.test", "network_ids.#", "1"),
				),
			},
		},
	})
}

func TestAccFirewallZone_updateName(t *testing.T) {
	name1 := fmt.Sprintf("tfacc-zone-upd1-%s", randomSuffix())
	name2 := fmt.Sprintf("tfacc-zone-upd2-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t); requireHardware(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_firewall_zone" "test" {
  name = %q
}
`, name1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_zone.test", "name", name1),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "terrifi_firewall_zone" "test" {
  name = %q
}
`, name2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_zone.test", "name", name2),
				),
			},
		},
	})
}

func TestAccFirewallZone_addNetworks(t *testing.T) {
	zoneName := fmt.Sprintf("tfacc-zone-add-%s", randomSuffix())
	netName := fmt.Sprintf("tfacc-net-add-%s", randomSuffix())
	vlan := randomVLAN()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t); requireHardware(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_network" "test" {
  name    = %q
  purpose = "corporate"
  vlan_id = %d
  subnet  = "10.%d.%d.1/24"
}

resource "terrifi_firewall_zone" "test" {
  name = %q
}
`, netName, vlan, vlan/256, vlan%256, zoneName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_zone.test", "name", zoneName),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "terrifi_network" "test" {
  name    = %q
  purpose = "corporate"
  vlan_id = %d
  subnet  = "10.%d.%d.1/24"
}

resource "terrifi_firewall_zone" "test" {
  name        = %q
  network_ids = [terrifi_network.test.id]
}
`, netName, vlan, vlan/256, vlan%256, zoneName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_zone.test", "network_ids.#", "1"),
				),
			},
		},
	})
}

func TestAccFirewallZone_removeNetworks(t *testing.T) {
	zoneName := fmt.Sprintf("tfacc-zone-rm-%s", randomSuffix())
	netName := fmt.Sprintf("tfacc-net-rm-%s", randomSuffix())
	vlan := randomVLAN()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t); requireHardware(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_network" "test" {
  name    = %q
  purpose = "corporate"
  vlan_id = %d
  subnet  = "10.%d.%d.1/24"
}

resource "terrifi_firewall_zone" "test" {
  name        = %q
  network_ids = [terrifi_network.test.id]
}
`, netName, vlan, vlan/256, vlan%256, zoneName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_zone.test", "network_ids.#", "1"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "terrifi_network" "test" {
  name    = %q
  purpose = "corporate"
  vlan_id = %d
  subnet  = "10.%d.%d.1/24"
}

resource "terrifi_firewall_zone" "test" {
  name        = %q
  network_ids = []
}
`, netName, vlan, vlan/256, vlan%256, zoneName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_zone.test", "network_ids.#", "0"),
				),
			},
		},
	})
}

func TestAccFirewallZone_replaceNetworks(t *testing.T) {
	zoneName := fmt.Sprintf("tfacc-zone-repl-%s", randomSuffix())
	net1Name := fmt.Sprintf("tfacc-net-r1-%s", randomSuffix())
	net2Name := fmt.Sprintf("tfacc-net-r2-%s", randomSuffix())
	vlan1 := randomVLAN()
	vlan2 := randomVLAN()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t); requireHardware(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_network" "net1" {
  name    = %q
  purpose = "corporate"
  vlan_id = %d
  subnet  = "10.%d.%d.1/24"
}

resource "terrifi_network" "net2" {
  name    = %q
  purpose = "corporate"
  vlan_id = %d
  subnet  = "10.%d.%d.1/24"
}

resource "terrifi_firewall_zone" "test" {
  name        = %q
  network_ids = [terrifi_network.net1.id]
}
`, net1Name, vlan1, vlan1/256, vlan1%256, net2Name, vlan2, vlan2/256, vlan2%256, zoneName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_zone.test", "network_ids.#", "1"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "terrifi_network" "net1" {
  name    = %q
  purpose = "corporate"
  vlan_id = %d
  subnet  = "10.%d.%d.1/24"
}

resource "terrifi_network" "net2" {
  name    = %q
  purpose = "corporate"
  vlan_id = %d
  subnet  = "10.%d.%d.1/24"
}

resource "terrifi_firewall_zone" "test" {
  name        = %q
  network_ids = [terrifi_network.net2.id]
}
`, net1Name, vlan1, vlan1/256, vlan1%256, net2Name, vlan2, vlan2/256, vlan2%256, zoneName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_zone.test", "network_ids.#", "1"),
				),
			},
		},
	})
}

func TestAccFirewallZone_updateNameAndNetworks(t *testing.T) {
	name1 := fmt.Sprintf("tfacc-zone-both1-%s", randomSuffix())
	name2 := fmt.Sprintf("tfacc-zone-both2-%s", randomSuffix())
	netName := fmt.Sprintf("tfacc-net-both-%s", randomSuffix())
	vlan := randomVLAN()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t); requireHardware(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_network" "test" {
  name    = %q
  purpose = "corporate"
  vlan_id = %d
  subnet  = "10.%d.%d.1/24"
}

resource "terrifi_firewall_zone" "test" {
  name = %q
}
`, netName, vlan, vlan/256, vlan%256, name1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_zone.test", "name", name1),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "terrifi_network" "test" {
  name    = %q
  purpose = "corporate"
  vlan_id = %d
  subnet  = "10.%d.%d.1/24"
}

resource "terrifi_firewall_zone" "test" {
  name        = %q
  network_ids = [terrifi_network.test.id]
}
`, netName, vlan, vlan/256, vlan%256, name2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_zone.test", "name", name2),
					resource.TestCheckResourceAttr("terrifi_firewall_zone.test", "network_ids.#", "1"),
				),
			},
		},
	})
}

func TestAccFirewallZone_emptyNetworkList(t *testing.T) {
	name := fmt.Sprintf("tfacc-zone-empty-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t); requireHardware(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_firewall_zone" "test" {
  name        = %q
  network_ids = []
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_zone.test", "name", name),
					resource.TestCheckResourceAttr("terrifi_firewall_zone.test", "network_ids.#", "0"),
				),
			},
		},
	})
}

func TestAccFirewallZone_import(t *testing.T) {
	name := fmt.Sprintf("tfacc-zone-imp-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t); requireHardware(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_firewall_zone" "test" {
  name = %q
}
`, name),
			},
			{
				ResourceName:      "terrifi_firewall_zone.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccFirewallZone_importSiteID(t *testing.T) {
	name := fmt.Sprintf("tfacc-zone-impsid-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t); requireHardware(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_firewall_zone" "test" {
  name = %q
}
`, name),
			},
			{
				ResourceName:      "terrifi_firewall_zone.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs := s.RootModule().Resources["terrifi_firewall_zone.test"]
					if rs == nil {
						return "", fmt.Errorf("resource not found in state")
					}
					return fmt.Sprintf("%s:%s", rs.Primary.Attributes["site"], rs.Primary.Attributes["id"]), nil
				},
			},
		},
	})
}

func TestAccFirewallZone_multipleZones(t *testing.T) {
	name1 := fmt.Sprintf("tfacc-zone-m1-%s", randomSuffix())
	name2 := fmt.Sprintf("tfacc-zone-m2-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t); requireHardware(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_firewall_zone" "zone1" {
  name = %q
}

resource "terrifi_firewall_zone" "zone2" {
  name = %q
}
`, name1, name2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_zone.zone1", "name", name1),
					resource.TestCheckResourceAttr("terrifi_firewall_zone.zone2", "name", name2),
					resource.TestCheckResourceAttrSet("terrifi_firewall_zone.zone1", "id"),
					resource.TestCheckResourceAttrSet("terrifi_firewall_zone.zone2", "id"),
				),
			},
		},
	})
}

func TestAccFirewallZone_multipleZonesWithDistinctNetworks(t *testing.T) {
	zone1Name := fmt.Sprintf("tfacc-zone-dn1-%s", randomSuffix())
	zone2Name := fmt.Sprintf("tfacc-zone-dn2-%s", randomSuffix())
	net1Name := fmt.Sprintf("tfacc-net-dn1-%s", randomSuffix())
	net2Name := fmt.Sprintf("tfacc-net-dn2-%s", randomSuffix())
	vlan1 := randomVLAN()
	vlan2 := randomVLAN()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t); requireHardware(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_network" "net1" {
  name    = %q
  purpose = "corporate"
  vlan_id = %d
  subnet  = "10.%d.%d.1/24"
}

resource "terrifi_network" "net2" {
  name    = %q
  purpose = "corporate"
  vlan_id = %d
  subnet  = "10.%d.%d.1/24"
}

resource "terrifi_firewall_zone" "zone1" {
  name        = %q
  network_ids = [terrifi_network.net1.id]
}

resource "terrifi_firewall_zone" "zone2" {
  name        = %q
  network_ids = [terrifi_network.net2.id]
}
`, net1Name, vlan1, vlan1/256, vlan1%256, net2Name, vlan2, vlan2/256, vlan2%256, zone1Name, zone2Name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_zone.zone1", "network_ids.#", "1"),
					resource.TestCheckResourceAttr("terrifi_firewall_zone.zone2", "network_ids.#", "1"),
				),
			},
		},
	})
}

func TestAccFirewallZone_importWithNetworks(t *testing.T) {
	zoneName := fmt.Sprintf("tfacc-zone-impn-%s", randomSuffix())
	netName := fmt.Sprintf("tfacc-net-impn-%s", randomSuffix())
	vlan := randomVLAN()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t); requireHardware(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_network" "test" {
  name    = %q
  purpose = "corporate"
  vlan_id = %d
  subnet  = "10.%d.%d.1/24"
}

resource "terrifi_firewall_zone" "test" {
  name        = %q
  network_ids = [terrifi_network.test.id]
}
`, netName, vlan, vlan/256, vlan%256, zoneName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_zone.test", "network_ids.#", "1"),
				),
			},
			{
				ResourceName:      "terrifi_firewall_zone.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccFirewallZone_multipleNetworksInZone(t *testing.T) {
	zoneName := fmt.Sprintf("tfacc-zone-mn-%s", randomSuffix())
	net1Name := fmt.Sprintf("tfacc-net-mn1-%s", randomSuffix())
	net2Name := fmt.Sprintf("tfacc-net-mn2-%s", randomSuffix())
	net3Name := fmt.Sprintf("tfacc-net-mn3-%s", randomSuffix())
	vlan1 := randomVLAN()
	vlan2 := randomVLAN()
	vlan3 := randomVLAN()

	netConfig := fmt.Sprintf(`
resource "terrifi_network" "net1" {
  name    = %q
  purpose = "corporate"
  vlan_id = %d
  subnet  = "10.%d.%d.1/24"
}

resource "terrifi_network" "net2" {
  name    = %q
  purpose = "corporate"
  vlan_id = %d
  subnet  = "10.%d.%d.1/24"
}

resource "terrifi_network" "net3" {
  name    = %q
  purpose = "corporate"
  vlan_id = %d
  subnet  = "10.%d.%d.1/24"
}
`, net1Name, vlan1, vlan1/256, vlan1%256, net2Name, vlan2, vlan2/256, vlan2%256, net3Name, vlan3, vlan3/256, vlan3%256)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t); requireHardware(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create zone with 2 networks
			{
				Config: netConfig + fmt.Sprintf(`
resource "terrifi_firewall_zone" "test" {
  name        = %q
  network_ids = [terrifi_network.net1.id, terrifi_network.net2.id]
}
`, zoneName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_zone.test", "network_ids.#", "2"),
				),
			},
			// Step 2: Add a third network
			{
				Config: netConfig + fmt.Sprintf(`
resource "terrifi_firewall_zone" "test" {
  name        = %q
  network_ids = [terrifi_network.net1.id, terrifi_network.net2.id, terrifi_network.net3.id]
}
`, zoneName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_zone.test", "network_ids.#", "3"),
				),
			},
			// Step 3: Remove the middle network
			{
				Config: netConfig + fmt.Sprintf(`
resource "terrifi_firewall_zone" "test" {
  name        = %q
  network_ids = [terrifi_network.net1.id, terrifi_network.net3.id]
}
`, zoneName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_zone.test", "network_ids.#", "2"),
				),
			},
		},
	})
}

func TestAccFirewallZone_idempotentReapply(t *testing.T) {
	zoneName := fmt.Sprintf("tfacc-zone-idem-%s", randomSuffix())
	netName := fmt.Sprintf("tfacc-net-idem-%s", randomSuffix())
	vlan := randomVLAN()

	config := fmt.Sprintf(`
resource "terrifi_network" "test" {
  name    = %q
  purpose = "corporate"
  vlan_id = %d
  subnet  = "10.%d.%d.1/24"
}

resource "terrifi_firewall_zone" "test" {
  name        = %q
  network_ids = [terrifi_network.test.id]
}
`, netName, vlan, vlan/256, vlan%256, zoneName)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t); requireHardware(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_zone.test", "name", zoneName),
					resource.TestCheckResourceAttr("terrifi_firewall_zone.test", "network_ids.#", "1"),
				),
			},
			// Step 2: Reapply same config — should be a no-op (no diff)
			{
				Config:   config,
				PlanOnly: true,
			},
		},
	})
}

func TestAccFirewallZone_idempotentReapplyNoNetworks(t *testing.T) {
	zoneName := fmt.Sprintf("tfacc-zone-idemn-%s", randomSuffix())

	config := fmt.Sprintf(`
resource "terrifi_firewall_zone" "test" {
  name = %q
}
`, zoneName)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t); requireHardware(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_zone.test", "name", zoneName),
				),
			},
			{
				Config:   config,
				PlanOnly: true,
			},
		},
	})
}

func TestAccFirewallZone_moveNetworkBetweenZones(t *testing.T) {
	zone1Name := fmt.Sprintf("tfacc-zone-mv1-%s", randomSuffix())
	zone2Name := fmt.Sprintf("tfacc-zone-mv2-%s", randomSuffix())
	netName := fmt.Sprintf("tfacc-net-mv-%s", randomSuffix())
	vlan := randomVLAN()

	netConfig := fmt.Sprintf(`
resource "terrifi_network" "test" {
  name    = %q
  purpose = "corporate"
  vlan_id = %d
  subnet  = "10.%d.%d.1/24"
}
`, netName, vlan, vlan/256, vlan%256)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t); requireHardware(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: net belongs to zone1
			{
				Config: netConfig + fmt.Sprintf(`
resource "terrifi_firewall_zone" "zone1" {
  name        = %q
  network_ids = [terrifi_network.test.id]
}

resource "terrifi_firewall_zone" "zone2" {
  name        = %q
  network_ids = []
}
`, zone1Name, zone2Name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_zone.zone1", "network_ids.#", "1"),
					resource.TestCheckResourceAttr("terrifi_firewall_zone.zone2", "network_ids.#", "0"),
				),
			},
			// Step 2: Move net from zone1 to zone2
			{
				Config: netConfig + fmt.Sprintf(`
resource "terrifi_firewall_zone" "zone1" {
  name        = %q
  network_ids = []
}

resource "terrifi_firewall_zone" "zone2" {
  name        = %q
  network_ids = [terrifi_network.test.id]
}
`, zone1Name, zone2Name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_zone.zone1", "network_ids.#", "0"),
					resource.TestCheckResourceAttr("terrifi_firewall_zone.zone2", "network_ids.#", "1"),
				),
			},
		},
	})
}
