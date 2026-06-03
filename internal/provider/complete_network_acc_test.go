package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// TestAccCompleteNetwork_basic applies the examples/complete reference network
// end to end, proves it is idempotent (a second plan after apply is empty),
// and lets the framework destroy it. This is the §8 L07 coverage: a single
// test that exercises the breadth of implemented resources together, catching
// cross-resource regressions (dependency ordering, shared IDs, drift) that the
// per-resource tests miss.
//
// Target-agnostic: runs against whatever TERRIFI_ACC_TARGET selects (docker,
// uos, or hardware). The zone-based firewall slice of the example is gated
// OFF here via enable_zone_firewall=false, because it needs an adopted gateway
// with the default zones seeded (see testing/README.md and
// examples/complete/firewall-zbf.tf). A hardware run can flip it on with a
// separate test or a manual apply.
func TestAccCompleteNetwork_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ConfigDirectory: config.StaticDirectory("../../examples/complete"),
				ConfigVariables: config.Variables{
					"enable_zone_firewall": config.BoolVariable(false),
				},
				// Idempotency proof: after apply + refresh, the plan must be empty.
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}
