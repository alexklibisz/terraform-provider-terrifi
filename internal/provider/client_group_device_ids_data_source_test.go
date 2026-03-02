package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// ---------------------------------------------------------------------------
// Acceptance tests — require hardware (client groups need a real controller)
// ---------------------------------------------------------------------------

func TestAccClientGroupDeviceIDs_basic(t *testing.T) {
	requireHardware(t)
	suffix := randomSuffix()
	groupName := fmt.Sprintf("tfacc-cgdids-%s", suffix)
	mac1 := randomMAC()
	mac2 := randomMAC()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_client_group" "test" {
  name = %q
}

resource "terrifi_client_device" "dev1" {
  mac             = %q
  name            = "tfacc-cgdids-dev1"
  client_group_id = terrifi_client_group.test.id
}

resource "terrifi_client_device" "dev2" {
  mac             = %q
  name            = "tfacc-cgdids-dev2"
  client_group_id = terrifi_client_group.test.id
}

data "terrifi_client_group_device_ids" "test" {
  client_group_id = terrifi_client_group.test.id

  depends_on = [
    terrifi_client_device.dev1,
    terrifi_client_device.dev2,
  ]
}
`, groupName, mac1, mac2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.terrifi_client_group_device_ids.test", "ids.#", "2"),
					resource.TestCheckResourceAttr("data.terrifi_client_group_device_ids.test", "site", "default"),
					resource.TestCheckTypeSetElemAttrPair(
						"data.terrifi_client_group_device_ids.test", "ids.*",
						"terrifi_client_device.dev1", "id",
					),
					resource.TestCheckTypeSetElemAttrPair(
						"data.terrifi_client_group_device_ids.test", "ids.*",
						"terrifi_client_device.dev2", "id",
					),
				),
			},
		},
	})
}

func TestAccClientGroupDeviceIDs_empty(t *testing.T) {
	requireHardware(t)
	suffix := randomSuffix()
	groupName := fmt.Sprintf("tfacc-cgdids-empty-%s", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_client_group" "test" {
  name = %q
}

data "terrifi_client_group_device_ids" "test" {
  client_group_id = terrifi_client_group.test.id
}
`, groupName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.terrifi_client_group_device_ids.test", "ids.#", "0"),
				),
			},
		},
	})
}

func TestAccClientGroupDeviceIDs_multipleGroups(t *testing.T) {
	requireHardware(t)
	suffix := randomSuffix()
	groupNameA := fmt.Sprintf("tfacc-cgdids-a-%s", suffix)
	groupNameB := fmt.Sprintf("tfacc-cgdids-b-%s", suffix)
	macA := randomMAC()
	macB := randomMAC()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_client_group" "group_a" {
  name = %q
}

resource "terrifi_client_group" "group_b" {
  name = %q
}

resource "terrifi_client_device" "dev_a" {
  mac             = %q
  name            = "tfacc-cgdids-deva"
  client_group_id = terrifi_client_group.group_a.id
}

resource "terrifi_client_device" "dev_b" {
  mac             = %q
  name            = "tfacc-cgdids-devb"
  client_group_id = terrifi_client_group.group_b.id
}

data "terrifi_client_group_device_ids" "group_a" {
  client_group_id = terrifi_client_group.group_a.id

  depends_on = [
    terrifi_client_device.dev_a,
    terrifi_client_device.dev_b,
  ]
}

data "terrifi_client_group_device_ids" "group_b" {
  client_group_id = terrifi_client_group.group_b.id

  depends_on = [
    terrifi_client_device.dev_a,
    terrifi_client_device.dev_b,
  ]
}
`, groupNameA, groupNameB, macA, macB),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.terrifi_client_group_device_ids.group_a", "ids.#", "1"),
					resource.TestCheckResourceAttr("data.terrifi_client_group_device_ids.group_b", "ids.#", "1"),
					resource.TestCheckTypeSetElemAttrPair(
						"data.terrifi_client_group_device_ids.group_a", "ids.*",
						"terrifi_client_device.dev_a", "id",
					),
					resource.TestCheckTypeSetElemAttrPair(
						"data.terrifi_client_group_device_ids.group_b", "ids.*",
						"terrifi_client_device.dev_b", "id",
					),
				),
			},
		},
	})
}

func TestAccClientGroupDeviceIDs_addDevice(t *testing.T) {
	requireHardware(t)
	suffix := randomSuffix()
	groupName := fmt.Sprintf("tfacc-cgdids-add-%s", suffix)
	mac1 := randomMAC()
	mac2 := randomMAC()

	groupConfig := fmt.Sprintf(`
resource "terrifi_client_group" "test" {
  name = %q
}
`, groupName)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: One device in the group
			{
				Config: groupConfig + fmt.Sprintf(`
resource "terrifi_client_device" "dev1" {
  mac             = %q
  name            = "tfacc-cgdids-add1"
  client_group_id = terrifi_client_group.test.id
}

data "terrifi_client_group_device_ids" "test" {
  client_group_id = terrifi_client_group.test.id

  depends_on = [terrifi_client_device.dev1]
}
`, mac1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.terrifi_client_group_device_ids.test", "ids.#", "1"),
					resource.TestCheckTypeSetElemAttrPair(
						"data.terrifi_client_group_device_ids.test", "ids.*",
						"terrifi_client_device.dev1", "id",
					),
				),
			},
			// Step 2: Add a second device — data source should now return 2 IDs
			{
				Config: groupConfig + fmt.Sprintf(`
resource "terrifi_client_device" "dev1" {
  mac             = %q
  name            = "tfacc-cgdids-add1"
  client_group_id = terrifi_client_group.test.id
}

resource "terrifi_client_device" "dev2" {
  mac             = %q
  name            = "tfacc-cgdids-add2"
  client_group_id = terrifi_client_group.test.id
}

data "terrifi_client_group_device_ids" "test" {
  client_group_id = terrifi_client_group.test.id

  depends_on = [
    terrifi_client_device.dev1,
    terrifi_client_device.dev2,
  ]
}
`, mac1, mac2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.terrifi_client_group_device_ids.test", "ids.#", "2"),
					resource.TestCheckTypeSetElemAttrPair(
						"data.terrifi_client_group_device_ids.test", "ids.*",
						"terrifi_client_device.dev1", "id",
					),
					resource.TestCheckTypeSetElemAttrPair(
						"data.terrifi_client_group_device_ids.test", "ids.*",
						"terrifi_client_device.dev2", "id",
					),
				),
			},
		},
	})
}
