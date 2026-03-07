package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const testAccProviderResponseCaching = `
provider "terrifi" {
  response_caching = true
}
`

// TestAccResponseCaching_FirewallZone exercises the full CRUD lifecycle of a
// firewall zone with response caching enabled. The create, read-back, update,
// and destroy phases verify that the cache is populated on reads and correctly
// invalidated on writes.
func TestAccResponseCaching_FirewallZone(t *testing.T) {
	name1 := fmt.Sprintf("tfacc-cache-z-%s", randomSuffix())
	name2 := fmt.Sprintf("tfacc-cache-z2-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t); requireHardware(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderResponseCaching + fmt.Sprintf(`
resource "terrifi_firewall_zone" "test" {
  name = %q
}
`, name1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_zone.test", "name", name1),
					resource.TestCheckResourceAttrSet("terrifi_firewall_zone.test", "id"),
				),
			},
			// Update triggers a write (invalidating cache) then a read-back.
			{
				Config: testAccProviderResponseCaching + fmt.Sprintf(`
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

// TestAccResponseCaching_FirewallPolicy exercises the full CRUD lifecycle of a
// firewall policy with response caching enabled. Both zones and the policy use
// v2 list-all endpoints, so the cache is exercised heavily during refresh.
func TestAccResponseCaching_FirewallPolicy(t *testing.T) {
	zone1Name := fmt.Sprintf("tfacc-cache-pz1-%s", randomSuffix())
	zone2Name := fmt.Sprintf("tfacc-cache-pz2-%s", randomSuffix())
	policyName := fmt.Sprintf("tfacc-cache-pol-%s", randomSuffix())

	zonesConfig := testAccProviderResponseCaching + fmt.Sprintf(`
resource "terrifi_firewall_zone" "zone1" {
  name = %q
}

resource "terrifi_firewall_zone" "zone2" {
  name = %q
}
`, zone1Name, zone2Name)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t); requireHardware(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: zonesConfig + fmt.Sprintf(`
resource "terrifi_firewall_policy" "test" {
  name   = %q
  action = "BLOCK"

  source {
    zone_id = terrifi_firewall_zone.zone1.id
  }

  destination {
    zone_id = terrifi_firewall_zone.zone2.id
  }
}
`, policyName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_policy.test", "name", policyName),
					resource.TestCheckResourceAttr("terrifi_firewall_policy.test", "action", "BLOCK"),
					resource.TestCheckResourceAttr("terrifi_firewall_policy.test", "enabled", "true"),
					resource.TestCheckResourceAttrSet("terrifi_firewall_policy.test", "id"),
				),
			},
			// Update: disable the policy. This writes (invalidating cache),
			// then refresh reads back both zones and the policy from cache.
			{
				Config: zonesConfig + fmt.Sprintf(`
resource "terrifi_firewall_policy" "test" {
  name    = %q
  action  = "BLOCK"
  enabled = false

  source {
    zone_id = terrifi_firewall_zone.zone1.id
  }

  destination {
    zone_id = terrifi_firewall_zone.zone2.id
  }
}
`, policyName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_policy.test", "enabled", "false"),
				),
			},
		},
	})
}

// TestAccResponseCaching_ClientDevice exercises the full CRUD lifecycle of a
// client device with response caching enabled. Client device reads go through
// doV1Request → doV2Request, so the cache is exercised on this code path too.
func TestAccResponseCaching_ClientDevice(t *testing.T) {
	mac := randomMAC()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderResponseCaching + fmt.Sprintf(`
resource "terrifi_client_device" "test" {
  mac  = %q
  name = "tfacc-cache-client"
}
`, mac),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_client_device.test", "mac", mac),
					resource.TestCheckResourceAttr("terrifi_client_device.test", "name", "tfacc-cache-client"),
					resource.TestCheckResourceAttrSet("terrifi_client_device.test", "id"),
				),
			},
			// Update name to verify write-invalidation + read-back works.
			{
				Config: testAccProviderResponseCaching + fmt.Sprintf(`
resource "terrifi_client_device" "test" {
  mac  = %q
  name = "tfacc-cache-client-updated"
}
`, mac),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_client_device.test", "name", "tfacc-cache-client-updated"),
				),
			},
		},
	})
}

// TestAccResponseCaching_MultipleFirewallZones creates multiple firewall zones
// with caching enabled. During refresh, each zone's Read calls GetFirewallZone
// which lists all zones — with caching, only one list-all call should be made.
// This test verifies correct behavior when multiple resources share a cached
// list response.
func TestAccResponseCaching_MultipleFirewallZones(t *testing.T) {
	name1 := fmt.Sprintf("tfacc-cache-mz1-%s", randomSuffix())
	name2 := fmt.Sprintf("tfacc-cache-mz2-%s", randomSuffix())
	name3 := fmt.Sprintf("tfacc-cache-mz3-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t); requireHardware(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderResponseCaching + fmt.Sprintf(`
resource "terrifi_firewall_zone" "z1" {
  name = %q
}

resource "terrifi_firewall_zone" "z2" {
  name = %q
}

resource "terrifi_firewall_zone" "z3" {
  name = %q
}
`, name1, name2, name3),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_zone.z1", "name", name1),
					resource.TestCheckResourceAttr("terrifi_firewall_zone.z2", "name", name2),
					resource.TestCheckResourceAttr("terrifi_firewall_zone.z3", "name", name3),
				),
			},
		},
	})
}
