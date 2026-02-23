package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/ubiquiti-community/go-unifi/unifi"
)

// ---------------------------------------------------------------------------
// Unit tests
// ---------------------------------------------------------------------------

func TestClientGroupModelToAPI(t *testing.T) {
	r := &clientGroupResource{}

	t.Run("name mapping", func(t *testing.T) {
		model := &clientGroupResourceModel{
			Name: types.StringValue("IoT Devices"),
		}

		group := r.modelToAPI(model)

		assert.Equal(t, "IoT Devices", group.Name)
	})
}

func TestClientGroupAPIToModel(t *testing.T) {
	r := &clientGroupResource{}

	t.Run("ID, name, and site mapping", func(t *testing.T) {
		group := &unifi.ClientGroup{
			ID:   "group123",
			Name: "IoT Devices",
		}

		var model clientGroupResourceModel
		r.apiToModel(group, &model, "default")

		assert.Equal(t, "group123", model.ID.ValueString())
		assert.Equal(t, "IoT Devices", model.Name.ValueString())
		assert.Equal(t, "default", model.Site.ValueString())
	})

	t.Run("custom site", func(t *testing.T) {
		group := &unifi.ClientGroup{
			ID:   "group456",
			Name: "Cameras",
		}

		var model clientGroupResourceModel
		r.apiToModel(group, &model, "mysite")

		assert.Equal(t, "group456", model.ID.ValueString())
		assert.Equal(t, "Cameras", model.Name.ValueString())
		assert.Equal(t, "mysite", model.Site.ValueString())
	})
}

func TestClientGroupApplyPlanToState(t *testing.T) {
	r := &clientGroupResource{}

	t.Run("name update", func(t *testing.T) {
		state := &clientGroupResourceModel{
			Name: types.StringValue("Original"),
		}

		plan := &clientGroupResourceModel{
			Name: types.StringValue("Updated"),
		}

		r.applyPlanToState(plan, state)

		assert.Equal(t, "Updated", state.Name.ValueString())
	})

	t.Run("null plan preserves state", func(t *testing.T) {
		state := &clientGroupResourceModel{
			Name: types.StringValue("Original"),
		}

		plan := &clientGroupResourceModel{
			Name: types.StringNull(),
		}

		r.applyPlanToState(plan, state)

		assert.Equal(t, "Original", state.Name.ValueString())
	})

	t.Run("unknown plan preserves state", func(t *testing.T) {
		state := &clientGroupResourceModel{
			Name: types.StringValue("Original"),
		}

		plan := &clientGroupResourceModel{
			Name: types.StringUnknown(),
		}

		r.applyPlanToState(plan, state)

		assert.Equal(t, "Original", state.Name.ValueString())
	})
}

// ---------------------------------------------------------------------------
// Acceptance tests
// ---------------------------------------------------------------------------

func TestAccClientGroup_basic(t *testing.T) {
	requireHardware(t)
	suffix := randomSuffix()
	name := fmt.Sprintf("tfacc-cligrp-%s", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_client_group" "test" {
  name = %q
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_client_group.test", "name", name),
					resource.TestCheckResourceAttr("terrifi_client_group.test", "site", "default"),
					resource.TestCheckResourceAttrSet("terrifi_client_group.test", "id"),
				),
			},
		},
	})
}

func TestAccClientGroup_updateName(t *testing.T) {
	requireHardware(t)
	suffix := randomSuffix()
	name1 := fmt.Sprintf("tfacc-cligrp-%s", suffix)
	name2 := fmt.Sprintf("tfacc-cligrp-upd-%s", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_client_group" "test" {
  name = %q
}
`, name1),
				Check: resource.TestCheckResourceAttr("terrifi_client_group.test", "name", name1),
			},
			{
				Config: fmt.Sprintf(`
resource "terrifi_client_group" "test" {
  name = %q
}
`, name2),
				Check: resource.TestCheckResourceAttr("terrifi_client_group.test", "name", name2),
			},
		},
	})
}

func TestAccClientGroup_import(t *testing.T) {
	requireHardware(t)
	suffix := randomSuffix()
	name := fmt.Sprintf("tfacc-cligrp-%s", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_client_group" "test" {
  name = %q
}
`, name),
			},
			{
				ResourceName:      "terrifi_client_group.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccClientGroup_importSiteID(t *testing.T) {
	requireHardware(t)
	suffix := randomSuffix()
	name := fmt.Sprintf("tfacc-cligrp-%s", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_client_group" "test" {
  name = %q
}
`, name),
			},
			{
				ResourceName:      "terrifi_client_group.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs := s.RootModule().Resources["terrifi_client_group.test"]
					if rs == nil {
						return "", fmt.Errorf("resource not found in state")
					}
					return fmt.Sprintf("%s:%s", rs.Primary.Attributes["site"], rs.Primary.Attributes["id"]), nil
				},
			},
		},
	})
}

func TestAccClientGroup_withClientDevice(t *testing.T) {
	requireHardware(t)
	suffix := randomSuffix()
	groupName := fmt.Sprintf("tfacc-cligrp-%s", suffix)
	mac := randomMAC()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_client_group" "test" {
  name = %q
}

resource "terrifi_client_device" "test" {
  mac  = %q
  name = "tfacc-grouped-device"
}
`, groupName, mac),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_client_group.test", "name", groupName),
					resource.TestCheckResourceAttrSet("terrifi_client_group.test", "id"),
					resource.TestCheckResourceAttr("terrifi_client_device.test", "mac", mac),
					resource.TestCheckResourceAttr("terrifi_client_device.test", "name", "tfacc-grouped-device"),
					resource.TestCheckResourceAttrSet("terrifi_client_device.test", "id"),
				),
			},
		},
	})
}

func TestAccClientGroup_idempotentReapply(t *testing.T) {
	requireHardware(t)
	suffix := randomSuffix()
	name := fmt.Sprintf("tfacc-cligrp-%s", suffix)

	config := fmt.Sprintf(`
resource "terrifi_client_group" "test" {
  name = %q
}
`, name)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check:  resource.TestCheckResourceAttr("terrifi_client_group.test", "name", name),
			},
			{
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}
