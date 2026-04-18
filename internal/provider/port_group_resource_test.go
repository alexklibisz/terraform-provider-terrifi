package provider

import (
	"fmt"
	"regexp"
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

func TestPortGroupModelToAPI(t *testing.T) {
	r := &portGroupResource{}
	ctx := t.Context()

	t.Run("basic ports", func(t *testing.T) {
		model := &portGroupResourceModel{
			Name: types.StringValue("Web Ports"),
			Ports: types.SetValueMust(types.StringType, []attr.Value{
				types.StringValue("80"),
				types.StringValue("443"),
			}),
		}

		group, diags := r.modelToAPI(ctx, model)
		require.False(t, diags.HasError())

		assert.Equal(t, "Web Ports", group.Name)
		assert.Equal(t, "port-group", group.GroupType)
		assert.Len(t, group.GroupMembers, 2)
		assert.Contains(t, group.GroupMembers, "80")
		assert.Contains(t, group.GroupMembers, "443")
	})

	t.Run("port range", func(t *testing.T) {
		model := &portGroupResourceModel{
			Name: types.StringValue("High Ports"),
			Ports: types.SetValueMust(types.StringType, []attr.Value{
				types.StringValue("8080-8090"),
			}),
		}

		group, diags := r.modelToAPI(ctx, model)
		require.False(t, diags.HasError())

		assert.Equal(t, "High Ports", group.Name)
		assert.Equal(t, "port-group", group.GroupType)
		assert.Equal(t, []string{"8080-8090"}, group.GroupMembers)
	})

	t.Run("type is always port-group", func(t *testing.T) {
		model := &portGroupResourceModel{
			Name:  types.StringValue("Test"),
			Ports: types.SetValueMust(types.StringType, []attr.Value{types.StringValue("22")}),
		}

		group, diags := r.modelToAPI(ctx, model)
		require.False(t, diags.HasError())
		assert.Equal(t, "port-group", group.GroupType)
	})
}

func TestPortGroupAPIToModel(t *testing.T) {
	r := &portGroupResource{}

	t.Run("populated group", func(t *testing.T) {
		group := &unifi.FirewallGroup{
			ID:           "abc123",
			Name:         "Web Ports",
			GroupType:    "port-group",
			GroupMembers: []string{"80", "443", "8080"},
		}

		var model portGroupResourceModel
		r.apiToModel(group, &model, "default")

		assert.Equal(t, "abc123", model.ID.ValueString())
		assert.Equal(t, "default", model.Site.ValueString())
		assert.Equal(t, "Web Ports", model.Name.ValueString())
		assert.Equal(t, 3, len(model.Ports.Elements()))
	})

	t.Run("nil members returns empty set", func(t *testing.T) {
		group := &unifi.FirewallGroup{
			ID:           "xyz",
			Name:         "Empty",
			GroupType:    "port-group",
			GroupMembers: nil,
		}

		var model portGroupResourceModel
		r.apiToModel(group, &model, "default")

		assert.False(t, model.Ports.IsNull())
		assert.Equal(t, 0, len(model.Ports.Elements()))
	})
}

func TestPortGroupApplyPlanToState(t *testing.T) {
	r := &portGroupResource{}

	t.Run("name update preserves ports", func(t *testing.T) {
		state := &portGroupResourceModel{
			Name: types.StringValue("Old Name"),
			Ports: types.SetValueMust(types.StringType, []attr.Value{
				types.StringValue("80"),
			}),
		}

		plan := &portGroupResourceModel{
			Name:  types.StringValue("New Name"),
			Ports: types.SetNull(types.StringType),
		}

		r.applyPlanToState(plan, state)

		assert.Equal(t, "New Name", state.Name.ValueString())
		assert.Equal(t, 1, len(state.Ports.Elements()))
	})

	t.Run("ports update preserves name", func(t *testing.T) {
		state := &portGroupResourceModel{
			Name: types.StringValue("My Group"),
			Ports: types.SetValueMust(types.StringType, []attr.Value{
				types.StringValue("80"),
			}),
		}

		plan := &portGroupResourceModel{
			Name: types.StringNull(),
			Ports: types.SetValueMust(types.StringType, []attr.Value{
				types.StringValue("80"),
				types.StringValue("443"),
			}),
		}

		r.applyPlanToState(plan, state)

		assert.Equal(t, "My Group", state.Name.ValueString())
		assert.Equal(t, 2, len(state.Ports.Elements()))
	})
}

// ---------------------------------------------------------------------------
// Acceptance tests
// ---------------------------------------------------------------------------

func TestAccPortGroup_basic(t *testing.T) {
	name := fmt.Sprintf("tfacc-pg-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_port_group" "test" {
  name  = %q
  ports = ["80", "443"]
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_port_group.test", "name", name),
					resource.TestCheckResourceAttr("terrifi_port_group.test", "ports.#", "2"),
					resource.TestCheckResourceAttr("terrifi_port_group.test", "site", "default"),
					resource.TestCheckResourceAttrSet("terrifi_port_group.test", "id"),
				),
			},
		},
	})
}

func TestAccPortGroup_updatePorts(t *testing.T) {
	name := fmt.Sprintf("tfacc-pg-upd-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_port_group" "test" {
  name  = %q
  ports = ["80", "443"]
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_port_group.test", "ports.#", "2"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "terrifi_port_group" "test" {
  name  = %q
  ports = ["80", "443", "8080", "8443"]
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_port_group.test", "ports.#", "4"),
				),
			},
		},
	})
}

func TestAccPortGroup_removePorts(t *testing.T) {
	name := fmt.Sprintf("tfacc-pg-rm-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_port_group" "test" {
  name  = %q
  ports = ["80", "443", "8080", "8443"]
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_port_group.test", "ports.#", "4"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "terrifi_port_group" "test" {
  name  = %q
  ports = ["443"]
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_port_group.test", "ports.#", "1"),
				),
			},
		},
	})
}

func TestAccPortGroup_updateName(t *testing.T) {
	name1 := fmt.Sprintf("tfacc-pg-n1-%s", randomSuffix())
	name2 := fmt.Sprintf("tfacc-pg-n2-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_port_group" "test" {
  name  = %q
  ports = ["80"]
}
`, name1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_port_group.test", "name", name1),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "terrifi_port_group" "test" {
  name  = %q
  ports = ["80"]
}
`, name2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_port_group.test", "name", name2),
				),
			},
		},
	})
}

func TestAccPortGroup_portRange(t *testing.T) {
	name := fmt.Sprintf("tfacc-pg-range-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_port_group" "test" {
  name  = %q
  ports = ["80", "8080-8090", "443"]
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_port_group.test", "name", name),
					resource.TestCheckResourceAttr("terrifi_port_group.test", "ports.#", "3"),
				),
			},
		},
	})
}

func TestAccPortGroup_manyPorts(t *testing.T) {
	name := fmt.Sprintf("tfacc-pg-many-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_port_group" "test" {
  name  = %q
  ports = ["22", "53", "80", "443", "993", "995", "3389", "5060", "8080", "8443"]
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_port_group.test", "ports.#", "10"),
				),
			},
		},
	})
}

func TestAccPortGroup_import(t *testing.T) {
	name := fmt.Sprintf("tfacc-pg-imp-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_port_group" "test" {
  name  = %q
  ports = ["80", "443"]
}
`, name),
			},
			{
				ResourceName:      "terrifi_port_group.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccPortGroup_importSiteID(t *testing.T) {
	name := fmt.Sprintf("tfacc-pg-impsid-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_port_group" "test" {
  name  = %q
  ports = ["80", "443"]
}
`, name),
			},
			{
				ResourceName:      "terrifi_port_group.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs := s.RootModule().Resources["terrifi_port_group.test"]
					if rs == nil {
						return "", fmt.Errorf("resource not found in state")
					}
					return fmt.Sprintf("%s:%s", rs.Primary.Attributes["site"], rs.Primary.Attributes["id"]), nil
				},
			},
		},
	})
}

func TestAccPortGroup_dataSource(t *testing.T) {
	name := fmt.Sprintf("tfacc-pg-ds-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_port_group" "test" {
  name  = %q
  ports = ["80", "443", "8080"]
}

data "terrifi_port_group" "lookup" {
  name = terrifi_port_group.test.name
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.terrifi_port_group.lookup", "name", name),
					resource.TestCheckResourceAttr("data.terrifi_port_group.lookup", "ports.#", "3"),
					resource.TestCheckResourceAttrSet("data.terrifi_port_group.lookup", "id"),
					resource.TestCheckResourceAttr("data.terrifi_port_group.lookup", "site", "default"),
				),
			},
		},
	})
}

func TestAccPortGroup_dataSourceNotFound(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "terrifi_port_group" "missing" {
  name = "this-port-group-does-not-exist-at-all"
}
`,
				ExpectError: regexp.MustCompile(`Port Group Not Found`),
			},
		},
	})
}
