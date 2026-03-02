package provider

import (
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
// Unit tests — no TF_ACC, no network, no env vars needed
// ---------------------------------------------------------------------------

func TestFirewallGroupModelToAPI(t *testing.T) {
	r := &firewallGroupResource{}
	ctx := t.Context()

	t.Run("port group", func(t *testing.T) {
		model := &firewallGroupResourceModel{
			Name: types.StringValue("Web Ports"),
			Type: types.StringValue("port-group"),
			Members: types.SetValueMust(types.StringType, []attr.Value{
				types.StringValue("80"),
				types.StringValue("443"),
				types.StringValue("8080"),
			}),
		}

		group, diags := r.modelToAPI(ctx, model)
		require.False(t, diags.HasError())

		assert.Equal(t, "Web Ports", group.Name)
		assert.Equal(t, "port-group", group.GroupType)
		assert.Len(t, group.GroupMembers, 3)
		assert.Contains(t, group.GroupMembers, "80")
		assert.Contains(t, group.GroupMembers, "443")
		assert.Contains(t, group.GroupMembers, "8080")
	})

	t.Run("address group", func(t *testing.T) {
		model := &firewallGroupResourceModel{
			Name: types.StringValue("Trusted IPs"),
			Type: types.StringValue("address-group"),
			Members: types.SetValueMust(types.StringType, []attr.Value{
				types.StringValue("10.0.0.0/24"),
				types.StringValue("192.168.1.0/24"),
			}),
		}

		group, diags := r.modelToAPI(ctx, model)
		require.False(t, diags.HasError())

		assert.Equal(t, "Trusted IPs", group.Name)
		assert.Equal(t, "address-group", group.GroupType)
		assert.Len(t, group.GroupMembers, 2)
		assert.Contains(t, group.GroupMembers, "10.0.0.0/24")
		assert.Contains(t, group.GroupMembers, "192.168.1.0/24")
	})

	t.Run("ipv6 address group", func(t *testing.T) {
		model := &firewallGroupResourceModel{
			Name: types.StringValue("IPv6 Trusted"),
			Type: types.StringValue("ipv6-address-group"),
			Members: types.SetValueMust(types.StringType, []attr.Value{
				types.StringValue("fd00::/64"),
			}),
		}

		group, diags := r.modelToAPI(ctx, model)
		require.False(t, diags.HasError())

		assert.Equal(t, "IPv6 Trusted", group.Name)
		assert.Equal(t, "ipv6-address-group", group.GroupType)
		assert.Equal(t, []string{"fd00::/64"}, group.GroupMembers)
	})
}

func TestFirewallGroupAPIToModel(t *testing.T) {
	r := &firewallGroupResource{}

	t.Run("port group", func(t *testing.T) {
		group := &unifi.FirewallGroup{
			ID:           "abc123",
			Name:         "Web Ports",
			GroupType:    "port-group",
			GroupMembers: []string{"80", "443"},
		}

		var model firewallGroupResourceModel
		r.apiToModel(group, &model, "default")

		assert.Equal(t, "abc123", model.ID.ValueString())
		assert.Equal(t, "default", model.Site.ValueString())
		assert.Equal(t, "Web Ports", model.Name.ValueString())
		assert.Equal(t, "port-group", model.Type.ValueString())
		assert.False(t, model.Members.IsNull())
		assert.Equal(t, 2, len(model.Members.Elements()))
	})

	t.Run("address group", func(t *testing.T) {
		group := &unifi.FirewallGroup{
			ID:           "def456",
			Name:         "Trusted IPs",
			GroupType:    "address-group",
			GroupMembers: []string{"10.0.0.0/24", "192.168.1.0/24"},
		}

		var model firewallGroupResourceModel
		r.apiToModel(group, &model, "mysite")

		assert.Equal(t, "def456", model.ID.ValueString())
		assert.Equal(t, "mysite", model.Site.ValueString())
		assert.Equal(t, "Trusted IPs", model.Name.ValueString())
		assert.Equal(t, "address-group", model.Type.ValueString())
		assert.Equal(t, 2, len(model.Members.Elements()))
	})

	t.Run("nil members returns empty set", func(t *testing.T) {
		group := &unifi.FirewallGroup{
			ID:           "ghi789",
			Name:         "Empty Group",
			GroupType:    "port-group",
			GroupMembers: nil,
		}

		var model firewallGroupResourceModel
		r.apiToModel(group, &model, "default")

		assert.False(t, model.Members.IsNull())
		assert.Equal(t, 0, len(model.Members.Elements()))
	})
}

func TestFirewallGroupApplyPlanToState(t *testing.T) {
	r := &firewallGroupResource{}

	t.Run("partial update preserves unchanged fields", func(t *testing.T) {
		state := &firewallGroupResourceModel{
			Name: types.StringValue("Old Name"),
			Type: types.StringValue("port-group"),
			Members: types.SetValueMust(types.StringType, []attr.Value{
				types.StringValue("80"),
			}),
		}

		plan := &firewallGroupResourceModel{
			Name:    types.StringValue("New Name"),
			Type:    types.StringNull(),
			Members: types.SetNull(types.StringType),
		}

		r.applyPlanToState(plan, state)

		assert.Equal(t, "New Name", state.Name.ValueString())
		assert.Equal(t, "port-group", state.Type.ValueString())
		assert.Equal(t, 1, len(state.Members.Elements()))
	})

	t.Run("all fields updated", func(t *testing.T) {
		state := &firewallGroupResourceModel{
			Name: types.StringValue("Old Name"),
			Type: types.StringValue("port-group"),
			Members: types.SetValueMust(types.StringType, []attr.Value{
				types.StringValue("80"),
			}),
		}

		plan := &firewallGroupResourceModel{
			Name: types.StringValue("New Name"),
			Type: types.StringValue("address-group"),
			Members: types.SetValueMust(types.StringType, []attr.Value{
				types.StringValue("10.0.0.0/24"),
				types.StringValue("192.168.1.0/24"),
			}),
		}

		r.applyPlanToState(plan, state)

		assert.Equal(t, "New Name", state.Name.ValueString())
		assert.Equal(t, "address-group", state.Type.ValueString())
		assert.Equal(t, 2, len(state.Members.Elements()))
	})
}

// ---------------------------------------------------------------------------
// Acceptance tests — require TF_ACC=1 and a UniFi controller (Docker or hardware)
// ---------------------------------------------------------------------------

// TestAccFirewallGroup_portGroup tests creating a port group with the full lifecycle.
func TestAccFirewallGroup_portGroup(t *testing.T) {
	name := fmt.Sprintf("tfacc-ports-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_firewall_group" "test" {
  name    = %q
  type    = "port-group"
  members = ["80", "443"]
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_group.test", "name", name),
					resource.TestCheckResourceAttr("terrifi_firewall_group.test", "type", "port-group"),
					resource.TestCheckResourceAttr("terrifi_firewall_group.test", "members.#", "2"),
					resource.TestCheckResourceAttr("terrifi_firewall_group.test", "site", "default"),
					resource.TestCheckResourceAttrSet("terrifi_firewall_group.test", "id"),
				),
			},
		},
	})
}

// TestAccFirewallGroup_addressGroup tests creating an address group.
func TestAccFirewallGroup_addressGroup(t *testing.T) {
	name := fmt.Sprintf("tfacc-addrs-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_firewall_group" "test" {
  name    = %q
  type    = "address-group"
  members = ["10.0.0.0/24", "192.168.1.0/24"]
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_group.test", "name", name),
					resource.TestCheckResourceAttr("terrifi_firewall_group.test", "type", "address-group"),
					resource.TestCheckResourceAttr("terrifi_firewall_group.test", "members.#", "2"),
					resource.TestCheckResourceAttrSet("terrifi_firewall_group.test", "id"),
				),
			},
		},
	})
}

// TestAccFirewallGroup_ipv6AddressGroup tests creating an IPv6 address group.
func TestAccFirewallGroup_ipv6AddressGroup(t *testing.T) {
	name := fmt.Sprintf("tfacc-ipv6-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_firewall_group" "test" {
  name    = %q
  type    = "ipv6-address-group"
  members = ["fd00::/64", "2001:db8::/32"]
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_group.test", "name", name),
					resource.TestCheckResourceAttr("terrifi_firewall_group.test", "type", "ipv6-address-group"),
					resource.TestCheckResourceAttr("terrifi_firewall_group.test", "members.#", "2"),
					resource.TestCheckResourceAttrSet("terrifi_firewall_group.test", "id"),
				),
			},
		},
	})
}

// TestAccFirewallGroup_updateMembers tests changing the members of a group.
func TestAccFirewallGroup_updateMembers(t *testing.T) {
	name := fmt.Sprintf("tfacc-updmem-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_firewall_group" "test" {
  name    = %q
  type    = "port-group"
  members = ["80", "443"]
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_group.test", "members.#", "2"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "terrifi_firewall_group" "test" {
  name    = %q
  type    = "port-group"
  members = ["80", "443", "8080"]
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_group.test", "members.#", "3"),
				),
			},
		},
	})
}

// TestAccFirewallGroup_updateName tests renaming a group in place.
func TestAccFirewallGroup_updateName(t *testing.T) {
	name1 := fmt.Sprintf("tfacc-rename1-%s", randomSuffix())
	name2 := fmt.Sprintf("tfacc-rename2-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_firewall_group" "test" {
  name    = %q
  type    = "port-group"
  members = ["80"]
}
`, name1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_group.test", "name", name1),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "terrifi_firewall_group" "test" {
  name    = %q
  type    = "port-group"
  members = ["80"]
}
`, name2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_group.test", "name", name2),
				),
			},
		},
	})
}

// TestAccFirewallGroup_updateNameAndMembers tests changing both name and members.
func TestAccFirewallGroup_updateNameAndMembers(t *testing.T) {
	name1 := fmt.Sprintf("tfacc-both1-%s", randomSuffix())
	name2 := fmt.Sprintf("tfacc-both2-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_firewall_group" "test" {
  name    = %q
  type    = "address-group"
  members = ["10.0.0.0/24"]
}
`, name1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_group.test", "name", name1),
					resource.TestCheckResourceAttr("terrifi_firewall_group.test", "members.#", "1"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "terrifi_firewall_group" "test" {
  name    = %q
  type    = "address-group"
  members = ["10.0.0.0/24", "192.168.1.0/24", "172.16.0.0/12"]
}
`, name2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_group.test", "name", name2),
					resource.TestCheckResourceAttr("terrifi_firewall_group.test", "members.#", "3"),
				),
			},
		},
	})
}

// TestAccFirewallGroup_removeMembers tests reducing the member list.
func TestAccFirewallGroup_removeMembers(t *testing.T) {
	name := fmt.Sprintf("tfacc-rmmem-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_firewall_group" "test" {
  name    = %q
  type    = "port-group"
  members = ["80", "443", "8080", "8443"]
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_group.test", "members.#", "4"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "terrifi_firewall_group" "test" {
  name    = %q
  type    = "port-group"
  members = ["443"]
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_group.test", "members.#", "1"),
				),
			},
		},
	})
}

// TestAccFirewallGroup_portRange tests using port ranges in a port group.
func TestAccFirewallGroup_portRange(t *testing.T) {
	name := fmt.Sprintf("tfacc-range-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_firewall_group" "test" {
  name    = %q
  type    = "port-group"
  members = ["80", "8080-8090", "443"]
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_group.test", "name", name),
					resource.TestCheckResourceAttr("terrifi_firewall_group.test", "members.#", "3"),
				),
			},
		},
	})
}

// TestAccFirewallGroup_import tests importing a firewall group by ID.
func TestAccFirewallGroup_import(t *testing.T) {
	name := fmt.Sprintf("tfacc-import-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_firewall_group" "test" {
  name    = %q
  type    = "port-group"
  members = ["80", "443"]
}
`, name),
			},
			{
				ResourceName:      "terrifi_firewall_group.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccFirewallGroup_importSiteID tests importing using the "site:id" format.
func TestAccFirewallGroup_importSiteID(t *testing.T) {
	name := fmt.Sprintf("tfacc-impsid-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_firewall_group" "test" {
  name    = %q
  type    = "address-group"
  members = ["10.0.0.0/24"]
}
`, name),
			},
			{
				ResourceName:      "terrifi_firewall_group.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs := s.RootModule().Resources["terrifi_firewall_group.test"]
					if rs == nil {
						return "", fmt.Errorf("resource not found in state")
					}
					return fmt.Sprintf("%s:%s", rs.Primary.Attributes["site"], rs.Primary.Attributes["id"]), nil
				},
			},
		},
	})
}

// TestAccFirewallGroup_singleMember tests a group with a single member.
func TestAccFirewallGroup_singleMember(t *testing.T) {
	name := fmt.Sprintf("tfacc-single-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_firewall_group" "test" {
  name    = %q
  type    = "address-group"
  members = ["192.168.1.1"]
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_group.test", "name", name),
					resource.TestCheckResourceAttr("terrifi_firewall_group.test", "members.#", "1"),
				),
			},
		},
	})
}

// TestAccFirewallGroup_manyMembers tests a group with many members.
func TestAccFirewallGroup_manyMembers(t *testing.T) {
	name := fmt.Sprintf("tfacc-many-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_firewall_group" "test" {
  name    = %q
  type    = "port-group"
  members = ["22", "53", "80", "443", "993", "995", "3389", "5060", "8080", "8443"]
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_group.test", "members.#", "10"),
				),
			},
		},
	})
}

// TestAccFirewallGroup_replaceMembers tests completely replacing all members.
func TestAccFirewallGroup_replaceMembers(t *testing.T) {
	name := fmt.Sprintf("tfacc-replace-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_firewall_group" "test" {
  name    = %q
  type    = "address-group"
  members = ["10.0.0.0/24", "10.0.1.0/24"]
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_group.test", "members.#", "2"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "terrifi_firewall_group" "test" {
  name    = %q
  type    = "address-group"
  members = ["172.16.0.0/16", "192.168.0.0/16"]
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_group.test", "members.#", "2"),
				),
			},
		},
	})
}
