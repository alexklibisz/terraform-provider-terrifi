package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// ---------------------------------------------------------------------------
// Acceptance tests
// ---------------------------------------------------------------------------

func TestAccFirewallPolicyOrder_basic(t *testing.T) {
	zone1Name := fmt.Sprintf("tfacc-ord-z1-%s", randomSuffix())
	zone2Name := fmt.Sprintf("tfacc-ord-z2-%s", randomSuffix())
	pol1Name := fmt.Sprintf("tfacc-ord-p1-%s", randomSuffix())
	pol2Name := fmt.Sprintf("tfacc-ord-p2-%s", randomSuffix())

	config := testAccFirewallPolicyOrderConfig(zone1Name, zone2Name, pol1Name, pol2Name, `[
    terrifi_firewall_policy.pol1.id,
    terrifi_firewall_policy.pol2.id,
  ]`)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t); requireHardware(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("terrifi_firewall_policy_order.test", "id"),
					resource.TestCheckResourceAttrSet("terrifi_firewall_policy_order.test", "site"),
					resource.TestCheckResourceAttrPair(
						"terrifi_firewall_policy_order.test", "source_zone_id",
						"terrifi_firewall_zone.zone1", "id",
					),
					resource.TestCheckResourceAttrPair(
						"terrifi_firewall_policy_order.test", "destination_zone_id",
						"terrifi_firewall_zone.zone2", "id",
					),
					resource.TestCheckResourceAttr("terrifi_firewall_policy_order.test", "policy_ids.#", "2"),
					resource.TestCheckResourceAttrPair(
						"terrifi_firewall_policy_order.test", "policy_ids.0",
						"terrifi_firewall_policy.pol1", "id",
					),
					resource.TestCheckResourceAttrPair(
						"terrifi_firewall_policy_order.test", "policy_ids.1",
						"terrifi_firewall_policy.pol2", "id",
					),
				),
			},
			// Verify idempotency — no drift on second plan.
			{
				Config:   config,
				PlanOnly: true,
			},
		},
	})
}

func TestAccFirewallPolicyOrder_reorder(t *testing.T) {
	zone1Name := fmt.Sprintf("tfacc-ord-ro-z1-%s", randomSuffix())
	zone2Name := fmt.Sprintf("tfacc-ord-ro-z2-%s", randomSuffix())
	pol1Name := fmt.Sprintf("tfacc-ord-ro-p1-%s", randomSuffix())
	pol2Name := fmt.Sprintf("tfacc-ord-ro-p2-%s", randomSuffix())
	pol3Name := fmt.Sprintf("tfacc-ord-ro-p3-%s", randomSuffix())

	zonesAndPolicies := testAccFirewallPolicyOrderZonesConfig(zone1Name, zone2Name) +
		testAccFirewallPolicyOrderPoliciesConfig(pol1Name, pol2Name) + fmt.Sprintf(`
resource "terrifi_firewall_policy" "pol3" {
  name   = %q
  action = "BLOCK"

  source {
    zone_id = terrifi_firewall_zone.zone1.id
  }

  destination {
    zone_id = terrifi_firewall_zone.zone2.id
  }
}
`, pol3Name)

	// Step 1: order [pol1, pol2, pol3]
	config1 := zonesAndPolicies + `
resource "terrifi_firewall_policy_order" "test" {
  source_zone_id      = terrifi_firewall_zone.zone1.id
  destination_zone_id = terrifi_firewall_zone.zone2.id

  policy_ids = [
    terrifi_firewall_policy.pol1.id,
    terrifi_firewall_policy.pol2.id,
    terrifi_firewall_policy.pol3.id,
  ]
}
`

	// Step 2: reorder to [pol3, pol1, pol2]
	config2 := zonesAndPolicies + `
resource "terrifi_firewall_policy_order" "test" {
  source_zone_id      = terrifi_firewall_zone.zone1.id
  destination_zone_id = terrifi_firewall_zone.zone2.id

  policy_ids = [
    terrifi_firewall_policy.pol3.id,
    terrifi_firewall_policy.pol1.id,
    terrifi_firewall_policy.pol2.id,
  ]
}
`

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t); requireHardware(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config1,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_policy_order.test", "policy_ids.#", "3"),
					resource.TestCheckResourceAttrPair(
						"terrifi_firewall_policy_order.test", "policy_ids.0",
						"terrifi_firewall_policy.pol1", "id",
					),
					resource.TestCheckResourceAttrPair(
						"terrifi_firewall_policy_order.test", "policy_ids.1",
						"terrifi_firewall_policy.pol2", "id",
					),
					resource.TestCheckResourceAttrPair(
						"terrifi_firewall_policy_order.test", "policy_ids.2",
						"terrifi_firewall_policy.pol3", "id",
					),
				),
			},
			// Verify no drift after initial ordering.
			{
				Config:   config1,
				PlanOnly: true,
			},
			// Reorder.
			{
				Config: config2,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_policy_order.test", "policy_ids.#", "3"),
					resource.TestCheckResourceAttrPair(
						"terrifi_firewall_policy_order.test", "policy_ids.0",
						"terrifi_firewall_policy.pol3", "id",
					),
					resource.TestCheckResourceAttrPair(
						"terrifi_firewall_policy_order.test", "policy_ids.1",
						"terrifi_firewall_policy.pol1", "id",
					),
					resource.TestCheckResourceAttrPair(
						"terrifi_firewall_policy_order.test", "policy_ids.2",
						"terrifi_firewall_policy.pol2", "id",
					),
				),
			},
			// Verify no drift after reordering.
			{
				Config:   config2,
				PlanOnly: true,
			},
		},
	})
}

func TestAccFirewallPolicyOrder_import(t *testing.T) {
	zone1Name := fmt.Sprintf("tfacc-ord-imp-z1-%s", randomSuffix())
	zone2Name := fmt.Sprintf("tfacc-ord-imp-z2-%s", randomSuffix())
	pol1Name := fmt.Sprintf("tfacc-ord-imp-p1-%s", randomSuffix())
	pol2Name := fmt.Sprintf("tfacc-ord-imp-p2-%s", randomSuffix())

	config := testAccFirewallPolicyOrderConfig(zone1Name, zone2Name, pol1Name, pol2Name, `[
    terrifi_firewall_policy.pol1.id,
    terrifi_firewall_policy.pol2.id,
  ]`)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t); requireHardware(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
			},
			{
				Config:            config,
				ResourceName:      "terrifi_firewall_policy_order.test",
				ImportState:       true,
				ImportStateVerify: true,
				// Import discovers all policies for the zone pair, which may
				// differ from the user's subset. Skip verifying policy_ids.
				ImportStateVerifyIgnore: []string{"policy_ids"},
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources["terrifi_firewall_policy_order.test"]
					if !ok {
						return "", fmt.Errorf("resource not found")
					}
					return rs.Primary.Attributes["source_zone_id"] + ":" + rs.Primary.Attributes["destination_zone_id"], nil
				},
			},
		},
	})
}

func TestAccFirewallPolicyOrder_forceNew(t *testing.T) {
	zone1Name := fmt.Sprintf("tfacc-ord-fn-z1-%s", randomSuffix())
	zone2Name := fmt.Sprintf("tfacc-ord-fn-z2-%s", randomSuffix())
	zone3Name := fmt.Sprintf("tfacc-ord-fn-z3-%s", randomSuffix())
	pol1Name := fmt.Sprintf("tfacc-ord-fn-p1-%s", randomSuffix())
	pol2Name := fmt.Sprintf("tfacc-ord-fn-p2-%s", randomSuffix())

	// Config 1: zone1 -> zone2
	config1 := testAccFirewallPolicyOrderConfig(zone1Name, zone2Name, pol1Name, pol2Name, `[
    terrifi_firewall_policy.pol1.id,
    terrifi_firewall_policy.pol2.id,
  ]`)

	// Config 2: zone1 -> zone3 (different destination zone = ForceNew)
	config2 := fmt.Sprintf(`
resource "terrifi_firewall_zone" "zone1" {
  name = %q
}

resource "terrifi_firewall_zone" "zone2" {
  name = %q
}

resource "terrifi_firewall_zone" "zone3" {
  name = %q
}

resource "terrifi_firewall_policy" "pol1" {
  name   = %q
  action = "BLOCK"

  source {
    zone_id = terrifi_firewall_zone.zone1.id
  }

  destination {
    zone_id = terrifi_firewall_zone.zone3.id
  }
}

resource "terrifi_firewall_policy" "pol2" {
  name   = %q
  action = "ALLOW"

  source {
    zone_id = terrifi_firewall_zone.zone1.id
  }

  destination {
    zone_id = terrifi_firewall_zone.zone3.id
  }
}

resource "terrifi_firewall_policy_order" "test" {
  source_zone_id      = terrifi_firewall_zone.zone1.id
  destination_zone_id = terrifi_firewall_zone.zone3.id

  policy_ids = [
    terrifi_firewall_policy.pol1.id,
    terrifi_firewall_policy.pol2.id,
  ]
}
`, zone1Name, zone2Name, zone3Name, pol1Name, pol2Name)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t); requireHardware(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config1,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"terrifi_firewall_policy_order.test", "destination_zone_id",
						"terrifi_firewall_zone.zone2", "id",
					),
				),
			},
			// Change destination zone — should trigger destroy+create.
			{
				Config: config2,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"terrifi_firewall_policy_order.test", "destination_zone_id",
						"terrifi_firewall_zone.zone3", "id",
					),
				),
			},
		},
	})
}

func TestAccFirewallPolicyOrder_zoneMismatch(t *testing.T) {
	zone1Name := fmt.Sprintf("tfacc-ord-zm-z1-%s", randomSuffix())
	zone2Name := fmt.Sprintf("tfacc-ord-zm-z2-%s", randomSuffix())
	zone3Name := fmt.Sprintf("tfacc-ord-zm-z3-%s", randomSuffix())
	polName := fmt.Sprintf("tfacc-ord-zm-p1-%s", randomSuffix())

	// Policy goes zone1 -> zone2, but order resource specifies zone1 -> zone3.
	config := fmt.Sprintf(`
resource "terrifi_firewall_zone" "zone1" {
  name = %q
}

resource "terrifi_firewall_zone" "zone2" {
  name = %q
}

resource "terrifi_firewall_zone" "zone3" {
  name = %q
}

resource "terrifi_firewall_policy" "pol1" {
  name   = %q
  action = "BLOCK"

  source {
    zone_id = terrifi_firewall_zone.zone1.id
  }

  destination {
    zone_id = terrifi_firewall_zone.zone2.id
  }
}

resource "terrifi_firewall_policy_order" "test" {
  source_zone_id      = terrifi_firewall_zone.zone1.id
  destination_zone_id = terrifi_firewall_zone.zone3.id

  policy_ids = [
    terrifi_firewall_policy.pol1.id,
  ]
}
`, zone1Name, zone2Name, zone3Name, polName)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t); requireHardware(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      config,
				ExpectError: regexp.MustCompile(`Firewall Policy Zone Mismatch`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func testAccFirewallPolicyOrderZonesConfig(zone1Name, zone2Name string) string {
	return fmt.Sprintf(`
resource "terrifi_firewall_zone" "zone1" {
  name = %q
}

resource "terrifi_firewall_zone" "zone2" {
  name = %q
}
`, zone1Name, zone2Name)
}

func testAccFirewallPolicyOrderPoliciesConfig(pol1Name, pol2Name string) string {
	return fmt.Sprintf(`
resource "terrifi_firewall_policy" "pol1" {
  name   = %q
  action = "BLOCK"

  source {
    zone_id = terrifi_firewall_zone.zone1.id
  }

  destination {
    zone_id = terrifi_firewall_zone.zone2.id
  }
}

resource "terrifi_firewall_policy" "pol2" {
  name   = %q
  action = "ALLOW"

  source {
    zone_id = terrifi_firewall_zone.zone1.id
  }

  destination {
    zone_id = terrifi_firewall_zone.zone2.id
  }
}
`, pol1Name, pol2Name)
}

func testAccFirewallPolicyOrderConfig(zone1Name, zone2Name, pol1Name, pol2Name, policyIDsExpr string) string {
	return testAccFirewallPolicyOrderZonesConfig(zone1Name, zone2Name) +
		testAccFirewallPolicyOrderPoliciesConfig(pol1Name, pol2Name) + fmt.Sprintf(`
resource "terrifi_firewall_policy_order" "test" {
  source_zone_id      = terrifi_firewall_zone.zone1.id
  destination_zone_id = terrifi_firewall_zone.zone2.id

  policy_ids = %s
}
`, policyIDsExpr)
}
