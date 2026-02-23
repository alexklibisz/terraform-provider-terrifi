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

func TestWLANModelToAPI(t *testing.T) {
	r := &wlanResource{}

	t.Run("basic WLAN with all fields", func(t *testing.T) {
		model := &wlanResourceModel{
			Name:           types.StringValue("My WiFi"),
			Passphrase:     types.StringValue("supersecret"),
			NetworkID:      types.StringValue("net123"),
			WifiBand:       types.StringValue("both"),
			Security:       types.StringValue("wpapsk"),
			HideSSID:       types.BoolValue(false),
			WPAMode:        types.StringValue("wpa2"),
			WPA3Support:    types.BoolValue(false),
			WPA3Transition: types.BoolValue(false),
		}

		wlan := r.modelToAPI(model)

		assert.Equal(t, "My WiFi", wlan.Name)
		assert.Equal(t, "supersecret", wlan.XPassphrase)
		assert.Equal(t, "net123", wlan.NetworkID)
		assert.Equal(t, "both", wlan.WLANBand)
		assert.Equal(t, "wpapsk", wlan.Security)
		assert.False(t, wlan.HideSSID)
		assert.Equal(t, "wpa2", wlan.WPAMode)
		assert.False(t, wlan.WPA3Support)
		assert.False(t, wlan.WPA3Transition)
		assert.True(t, wlan.Enabled)
	})

	t.Run("5g band and hidden SSID", func(t *testing.T) {
		model := &wlanResourceModel{
			Name:           types.StringValue("Hidden 5G"),
			Passphrase:     types.StringValue("password123"),
			NetworkID:      types.StringValue("net456"),
			WifiBand:       types.StringValue("5g"),
			Security:       types.StringValue("wpapsk"),
			HideSSID:       types.BoolValue(true),
			WPAMode:        types.StringValue("wpa2"),
			WPA3Support:    types.BoolValue(false),
			WPA3Transition: types.BoolValue(false),
		}

		wlan := r.modelToAPI(model)

		assert.Equal(t, "5g", wlan.WLANBand)
		assert.True(t, wlan.HideSSID)
	})

	t.Run("open security omits passphrase", func(t *testing.T) {
		model := &wlanResourceModel{
			Name:           types.StringValue("Guest"),
			Passphrase:     types.StringNull(),
			NetworkID:      types.StringValue("net789"),
			WifiBand:       types.StringValue("both"),
			Security:       types.StringValue("open"),
			HideSSID:       types.BoolValue(false),
			WPAMode:        types.StringValue("wpa2"),
			WPA3Support:    types.BoolValue(false),
			WPA3Transition: types.BoolValue(false),
		}

		wlan := r.modelToAPI(model)

		assert.Equal(t, "open", wlan.Security)
		assert.Empty(t, wlan.XPassphrase)
	})

	t.Run("WPA3 transition mode", func(t *testing.T) {
		model := &wlanResourceModel{
			Name:           types.StringValue("WPA3 WiFi"),
			Passphrase:     types.StringValue("wpa3password"),
			NetworkID:      types.StringValue("net-wpa3"),
			WifiBand:       types.StringValue("both"),
			Security:       types.StringValue("wpapsk"),
			HideSSID:       types.BoolValue(false),
			WPAMode:        types.StringValue("wpa2"),
			WPA3Support:    types.BoolValue(true),
			WPA3Transition: types.BoolValue(true),
		}

		wlan := r.modelToAPI(model)

		assert.True(t, wlan.WPA3Support)
		assert.True(t, wlan.WPA3Transition)
	})
}

func TestWLANAPIToModel(t *testing.T) {
	r := &wlanResource{}

	t.Run("basic WLAN from API", func(t *testing.T) {
		wlan := &unifi.WLAN{
			ID:        "wlan123",
			Name:      "Test WiFi",
			NetworkID: "net123",
			WLANBand:  "both",
			Security:  "wpapsk",
			HideSSID:  false,
			WPAMode:   "wpa2",
		}

		var model wlanResourceModel
		r.apiToModel(wlan, &model, "default")

		assert.Equal(t, "wlan123", model.ID.ValueString())
		assert.Equal(t, "default", model.Site.ValueString())
		assert.Equal(t, "Test WiFi", model.Name.ValueString())
		assert.Equal(t, "net123", model.NetworkID.ValueString())
		assert.Equal(t, "both", model.WifiBand.ValueString())
		assert.Equal(t, "wpapsk", model.Security.ValueString())
		assert.False(t, model.HideSSID.ValueBool())
		assert.Equal(t, "wpa2", model.WPAMode.ValueString())
		assert.False(t, model.WPA3Support.ValueBool())
		assert.False(t, model.WPA3Transition.ValueBool())
		// Passphrase is not returned by the API
		assert.True(t, model.Passphrase.IsNull())
	})

	t.Run("hidden SSID with WPA3", func(t *testing.T) {
		wlan := &unifi.WLAN{
			ID:             "wlan456",
			Name:           "Secure WiFi",
			NetworkID:      "net456",
			WLANBand:       "5g",
			Security:       "wpapsk",
			HideSSID:       true,
			WPAMode:        "wpa2",
			WPA3Support:    true,
			WPA3Transition: true,
		}

		var model wlanResourceModel
		r.apiToModel(wlan, &model, "mysite")

		assert.Equal(t, "mysite", model.Site.ValueString())
		assert.Equal(t, "5g", model.WifiBand.ValueString())
		assert.True(t, model.HideSSID.ValueBool())
		assert.True(t, model.WPA3Support.ValueBool())
		assert.True(t, model.WPA3Transition.ValueBool())
	})

	t.Run("open security", func(t *testing.T) {
		wlan := &unifi.WLAN{
			ID:        "wlan789",
			Name:      "Open Guest",
			NetworkID: "net789",
			WLANBand:  "2g",
			Security:  "open",
			WPAMode:   "wpa2",
		}

		var model wlanResourceModel
		r.apiToModel(wlan, &model, "default")

		assert.Equal(t, "open", model.Security.ValueString())
		assert.Equal(t, "2g", model.WifiBand.ValueString())
	})

	t.Run("empty band and security default correctly", func(t *testing.T) {
		wlan := &unifi.WLAN{
			ID:        "wlan-defaults",
			Name:      "Defaults",
			NetworkID: "net-defaults",
		}

		var model wlanResourceModel
		r.apiToModel(wlan, &model, "default")

		assert.Equal(t, "both", model.WifiBand.ValueString())
		assert.Equal(t, "wpapsk", model.Security.ValueString())
		assert.Equal(t, "wpa2", model.WPAMode.ValueString())
	})

	t.Run("passphrase from API is ignored", func(t *testing.T) {
		wlan := &unifi.WLAN{
			ID:           "wlan-pass",
			Name:         "Pass Test",
			NetworkID:    "net-pass",
			XPassphrase:  "returned-by-api",
			WLANBand:     "both",
			Security:     "wpapsk",
			WPAMode:      "wpa2",
		}

		var model wlanResourceModel
		model.Passphrase = types.StringValue("from-config")
		r.apiToModel(wlan, &model, "default")

		// apiToModel must never overwrite passphrase — it's managed by the caller
		assert.Equal(t, "from-config", model.Passphrase.ValueString())
	})
}

func TestWLANApplyPlanToState(t *testing.T) {
	r := &wlanResource{}

	t.Run("partial update preserves unchanged fields", func(t *testing.T) {
		state := &wlanResourceModel{
			Name:           types.StringValue("Original WiFi"),
			Passphrase:     types.StringValue("original123"),
			NetworkID:      types.StringValue("net123"),
			WifiBand:       types.StringValue("both"),
			Security:       types.StringValue("wpapsk"),
			HideSSID:       types.BoolValue(false),
			WPAMode:        types.StringValue("wpa2"),
			WPA3Support:    types.BoolValue(false),
			WPA3Transition: types.BoolValue(false),
		}

		plan := &wlanResourceModel{
			Name:           types.StringValue("Updated WiFi"),
			Passphrase:     types.StringNull(),
			NetworkID:      types.StringNull(),
			WifiBand:       types.StringNull(),
			Security:       types.StringNull(),
			HideSSID:       types.BoolNull(),
			WPAMode:        types.StringNull(),
			WPA3Support:    types.BoolNull(),
			WPA3Transition: types.BoolNull(),
		}

		r.applyPlanToState(plan, state)

		assert.Equal(t, "Updated WiFi", state.Name.ValueString())
		// Passphrase follows plan (null clears it); other null fields preserve state
		assert.True(t, state.Passphrase.IsNull())
		assert.Equal(t, "net123", state.NetworkID.ValueString())
		assert.Equal(t, "both", state.WifiBand.ValueString())
	})
}

// ---------------------------------------------------------------------------
// Acceptance tests
// ---------------------------------------------------------------------------

// wlanTestNetwork returns HCL for a supporting network used by WLAN tests.
// Each test gets a unique VLAN to avoid conflicts when tests run in parallel.
func wlanTestNetwork(name string, vlanID int) string {
	return fmt.Sprintf(`
resource "terrifi_network" "wlan_test" {
  name         = %q
  purpose      = "corporate"
  vlan_id      = %d
  subnet       = "10.%d.%d.1/24"
  network_group = "LAN"
  dhcp_enabled = false
}
`, name, vlanID, vlanID/256, vlanID%256)
}

func TestAccWLAN_basic(t *testing.T) {
	requireHardware(t)
	suffix := randomSuffix()
	netName := fmt.Sprintf("tfacc-wlan-net-%s", suffix)
	wlanName := fmt.Sprintf("tfacc-wlan-%s", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: wlanTestNetwork(netName, 500) + fmt.Sprintf(`
resource "terrifi_wlan" "test" {
  name       = %q
  passphrase = "testpassword123"
  network_id = terrifi_network.wlan_test.id
}
`, wlanName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_wlan.test", "name", wlanName),
					resource.TestCheckResourceAttr("terrifi_wlan.test", "security", "wpapsk"),
					resource.TestCheckResourceAttr("terrifi_wlan.test", "wifi_band", "both"),
					resource.TestCheckResourceAttr("terrifi_wlan.test", "hide_ssid", "false"),
					resource.TestCheckResourceAttr("terrifi_wlan.test", "wpa_mode", "wpa2"),
					resource.TestCheckResourceAttr("terrifi_wlan.test", "wpa3_support", "false"),
					resource.TestCheckResourceAttr("terrifi_wlan.test", "wpa3_transition", "false"),
					resource.TestCheckResourceAttr("terrifi_wlan.test", "site", "default"),
					resource.TestCheckResourceAttrSet("terrifi_wlan.test", "id"),
					resource.TestCheckResourceAttrPair("terrifi_wlan.test", "network_id", "terrifi_network.wlan_test", "id"),
				),
			},
		},
	})
}

func TestAccWLAN_updateName(t *testing.T) {
	requireHardware(t)
	suffix := randomSuffix()
	netName := fmt.Sprintf("tfacc-wlan-net-%s", suffix)
	wlanName1 := fmt.Sprintf("tfacc-wlan-%s", suffix)
	wlanName2 := fmt.Sprintf("tfacc-wlan-upd-%s", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: wlanTestNetwork(netName, 501) + fmt.Sprintf(`
resource "terrifi_wlan" "test" {
  name       = %q
  passphrase = "testpassword123"
  network_id = terrifi_network.wlan_test.id
}
`, wlanName1),
				Check: resource.TestCheckResourceAttr("terrifi_wlan.test", "name", wlanName1),
			},
			{
				Config: wlanTestNetwork(netName, 501) + fmt.Sprintf(`
resource "terrifi_wlan" "test" {
  name       = %q
  passphrase = "testpassword123"
  network_id = terrifi_network.wlan_test.id
}
`, wlanName2),
				Check: resource.TestCheckResourceAttr("terrifi_wlan.test", "name", wlanName2),
			},
		},
	})
}

func TestAccWLAN_updatePassphrase(t *testing.T) {
	requireHardware(t)
	suffix := randomSuffix()
	netName := fmt.Sprintf("tfacc-wlan-net-%s", suffix)
	wlanName := fmt.Sprintf("tfacc-wlan-%s", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: wlanTestNetwork(netName, 502) + fmt.Sprintf(`
resource "terrifi_wlan" "test" {
  name       = %q
  passphrase = "firstpassword1"
  network_id = terrifi_network.wlan_test.id
}
`, wlanName),
			},
			{
				Config: wlanTestNetwork(netName, 502) + fmt.Sprintf(`
resource "terrifi_wlan" "test" {
  name       = %q
  passphrase = "secondpassword2"
  network_id = terrifi_network.wlan_test.id
}
`, wlanName),
			},
		},
	})
}

func TestAccWLAN_updateBand(t *testing.T) {
	requireHardware(t)
	suffix := randomSuffix()
	netName := fmt.Sprintf("tfacc-wlan-net-%s", suffix)
	wlanName := fmt.Sprintf("tfacc-wlan-%s", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: wlanTestNetwork(netName, 503) + fmt.Sprintf(`
resource "terrifi_wlan" "test" {
  name       = %q
  passphrase = "testpassword123"
  network_id = terrifi_network.wlan_test.id
  wifi_band  = "2g"
}
`, wlanName),
				Check: resource.TestCheckResourceAttr("terrifi_wlan.test", "wifi_band", "2g"),
			},
			{
				Config: wlanTestNetwork(netName, 503) + fmt.Sprintf(`
resource "terrifi_wlan" "test" {
  name       = %q
  passphrase = "testpassword123"
  network_id = terrifi_network.wlan_test.id
  wifi_band  = "5g"
}
`, wlanName),
				Check: resource.TestCheckResourceAttr("terrifi_wlan.test", "wifi_band", "5g"),
			},
			{
				Config: wlanTestNetwork(netName, 503) + fmt.Sprintf(`
resource "terrifi_wlan" "test" {
  name       = %q
  passphrase = "testpassword123"
  network_id = terrifi_network.wlan_test.id
  wifi_band  = "both"
}
`, wlanName),
				Check: resource.TestCheckResourceAttr("terrifi_wlan.test", "wifi_band", "both"),
			},
		},
	})
}

func TestAccWLAN_hiddenSSID(t *testing.T) {
	requireHardware(t)
	suffix := randomSuffix()
	netName := fmt.Sprintf("tfacc-wlan-net-%s", suffix)
	wlanName := fmt.Sprintf("tfacc-wlan-%s", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: wlanTestNetwork(netName, 504) + fmt.Sprintf(`
resource "terrifi_wlan" "test" {
  name       = %q
  passphrase = "testpassword123"
  network_id = terrifi_network.wlan_test.id
  hide_ssid  = true
}
`, wlanName),
				Check: resource.TestCheckResourceAttr("terrifi_wlan.test", "hide_ssid", "true"),
			},
			{
				Config: wlanTestNetwork(netName, 504) + fmt.Sprintf(`
resource "terrifi_wlan" "test" {
  name       = %q
  passphrase = "testpassword123"
  network_id = terrifi_network.wlan_test.id
  hide_ssid  = false
}
`, wlanName),
				Check: resource.TestCheckResourceAttr("terrifi_wlan.test", "hide_ssid", "false"),
			},
		},
	})
}

func TestAccWLAN_openSecurity(t *testing.T) {
	requireHardware(t)
	suffix := randomSuffix()
	netName := fmt.Sprintf("tfacc-wlan-net-%s", suffix)
	wlanName := fmt.Sprintf("tfacc-wlan-%s", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: wlanTestNetwork(netName, 505) + fmt.Sprintf(`
resource "terrifi_wlan" "test" {
  name       = %q
  network_id = terrifi_network.wlan_test.id
  security   = "open"
}
`, wlanName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_wlan.test", "security", "open"),
					resource.TestCheckResourceAttr("terrifi_wlan.test", "name", wlanName),
				),
			},
		},
	})
}

func TestAccWLAN_import(t *testing.T) {
	requireHardware(t)
	suffix := randomSuffix()
	netName := fmt.Sprintf("tfacc-wlan-net-%s", suffix)
	wlanName := fmt.Sprintf("tfacc-wlan-%s", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: wlanTestNetwork(netName, 506) + fmt.Sprintf(`
resource "terrifi_wlan" "test" {
  name       = %q
  passphrase = "testpassword123"
  network_id = terrifi_network.wlan_test.id
}
`, wlanName),
			},
			{
				ResourceName:            "terrifi_wlan.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"passphrase"},
			},
		},
	})
}

func TestAccWLAN_importSiteID(t *testing.T) {
	requireHardware(t)
	suffix := randomSuffix()
	netName := fmt.Sprintf("tfacc-wlan-net-%s", suffix)
	wlanName := fmt.Sprintf("tfacc-wlan-%s", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: wlanTestNetwork(netName, 507) + fmt.Sprintf(`
resource "terrifi_wlan" "test" {
  name       = %q
  passphrase = "testpassword123"
  network_id = terrifi_network.wlan_test.id
}
`, wlanName),
			},
			{
				ResourceName:            "terrifi_wlan.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"passphrase"},
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs := s.RootModule().Resources["terrifi_wlan.test"]
					if rs == nil {
						return "", fmt.Errorf("resource not found in state")
					}
					return fmt.Sprintf("%s:%s", rs.Primary.Attributes["site"], rs.Primary.Attributes["id"]), nil
				},
			},
		},
	})
}

func TestAccWLAN_idempotentReapply(t *testing.T) {
	requireHardware(t)
	suffix := randomSuffix()
	netName := fmt.Sprintf("tfacc-wlan-net-%s", suffix)
	wlanName := fmt.Sprintf("tfacc-wlan-%s", suffix)

	config := wlanTestNetwork(netName, 508) + fmt.Sprintf(`
resource "terrifi_wlan" "test" {
  name       = %q
  passphrase = "testpassword123"
  network_id = terrifi_network.wlan_test.id
  wifi_band  = "both"
  security   = "wpapsk"
  hide_ssid  = false
  wpa_mode   = "wpa2"
}
`, wlanName)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check:  resource.TestCheckResourceAttr("terrifi_wlan.test", "name", wlanName),
			},
			{
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

func TestAccWLAN_updateSecurityOpenToWpapsk(t *testing.T) {
	requireHardware(t)
	suffix := randomSuffix()
	netName := fmt.Sprintf("tfacc-wlan-net-%s", suffix)
	wlanName := fmt.Sprintf("tfacc-wlan-%s", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: wlanTestNetwork(netName, 510) + fmt.Sprintf(`
resource "terrifi_wlan" "test" {
  name       = %q
  network_id = terrifi_network.wlan_test.id
  security   = "open"
}
`, wlanName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_wlan.test", "security", "open"),
				),
			},
			{
				Config: wlanTestNetwork(netName, 510) + fmt.Sprintf(`
resource "terrifi_wlan" "test" {
  name       = %q
  passphrase = "newpassword123"
  network_id = terrifi_network.wlan_test.id
  security   = "wpapsk"
}
`, wlanName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_wlan.test", "security", "wpapsk"),
				),
			},
		},
	})
}

func TestAccWLAN_updateSecurityWpapskToOpen(t *testing.T) {
	requireHardware(t)
	suffix := randomSuffix()
	netName := fmt.Sprintf("tfacc-wlan-net-%s", suffix)
	wlanName := fmt.Sprintf("tfacc-wlan-%s", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: wlanTestNetwork(netName, 511) + fmt.Sprintf(`
resource "terrifi_wlan" "test" {
  name       = %q
  passphrase = "testpassword123"
  network_id = terrifi_network.wlan_test.id
  security   = "wpapsk"
}
`, wlanName),
				Check: resource.TestCheckResourceAttr("terrifi_wlan.test", "security", "wpapsk"),
			},
			{
				Config: wlanTestNetwork(netName, 511) + fmt.Sprintf(`
resource "terrifi_wlan" "test" {
  name       = %q
  network_id = terrifi_network.wlan_test.id
  security   = "open"
}
`, wlanName),
				Check: resource.TestCheckResourceAttr("terrifi_wlan.test", "security", "open"),
			},
		},
	})
}

func TestAccWLAN_updateNetwork(t *testing.T) {
	requireHardware(t)
	suffix := randomSuffix()
	netName1 := fmt.Sprintf("tfacc-wlan-net1-%s", suffix)
	netName2 := fmt.Sprintf("tfacc-wlan-net2-%s", suffix)
	wlanName := fmt.Sprintf("tfacc-wlan-%s", suffix)

	net1 := fmt.Sprintf(`
resource "terrifi_network" "net1" {
  name          = %q
  purpose       = "corporate"
  vlan_id       = 512
  subnet        = "10.2.0.1/24"
  network_group = "LAN"
  dhcp_enabled  = false
}
`, netName1)

	net2 := fmt.Sprintf(`
resource "terrifi_network" "net2" {
  name          = %q
  purpose       = "corporate"
  vlan_id       = 513
  subnet        = "10.2.1.1/24"
  network_group = "LAN"
  dhcp_enabled  = false
}
`, netName2)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: net1 + net2 + fmt.Sprintf(`
resource "terrifi_wlan" "test" {
  name       = %q
  passphrase = "testpassword123"
  network_id = terrifi_network.net1.id
}
`, wlanName),
				Check: resource.TestCheckResourceAttrPair(
					"terrifi_wlan.test", "network_id",
					"terrifi_network.net1", "id",
				),
			},
			{
				Config: net1 + net2 + fmt.Sprintf(`
resource "terrifi_wlan" "test" {
  name       = %q
  passphrase = "testpassword123"
  network_id = terrifi_network.net2.id
}
`, wlanName),
				Check: resource.TestCheckResourceAttrPair(
					"terrifi_wlan.test", "network_id",
					"terrifi_network.net2", "id",
				),
			},
		},
	})
}

func TestAccWLAN_updateMultipleProperties(t *testing.T) {
	requireHardware(t)
	suffix := randomSuffix()
	netName := fmt.Sprintf("tfacc-wlan-net-%s", suffix)
	wlanName1 := fmt.Sprintf("tfacc-wlan-%s", suffix)
	wlanName2 := fmt.Sprintf("tfacc-wlan-upd-%s", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: wlanTestNetwork(netName, 514) + fmt.Sprintf(`
resource "terrifi_wlan" "test" {
  name       = %q
  passphrase = "testpassword123"
  network_id = terrifi_network.wlan_test.id
  wifi_band  = "both"
  hide_ssid  = false
}
`, wlanName1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_wlan.test", "name", wlanName1),
					resource.TestCheckResourceAttr("terrifi_wlan.test", "wifi_band", "both"),
					resource.TestCheckResourceAttr("terrifi_wlan.test", "hide_ssid", "false"),
				),
			},
			{
				Config: wlanTestNetwork(netName, 514) + fmt.Sprintf(`
resource "terrifi_wlan" "test" {
  name       = %q
  passphrase = "changedpassword1"
  network_id = terrifi_network.wlan_test.id
  wifi_band  = "5g"
  hide_ssid  = true
}
`, wlanName2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_wlan.test", "name", wlanName2),
					resource.TestCheckResourceAttr("terrifi_wlan.test", "wifi_band", "5g"),
					resource.TestCheckResourceAttr("terrifi_wlan.test", "hide_ssid", "true"),
				),
			},
		},
	})
}

func TestAccWLAN_updateWpaMode(t *testing.T) {
	requireHardware(t)
	suffix := randomSuffix()
	netName := fmt.Sprintf("tfacc-wlan-net-%s", suffix)
	wlanName := fmt.Sprintf("tfacc-wlan-%s", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: wlanTestNetwork(netName, 515) + fmt.Sprintf(`
resource "terrifi_wlan" "test" {
  name       = %q
  passphrase = "testpassword123"
  network_id = terrifi_network.wlan_test.id
  wpa_mode   = "wpa2"
}
`, wlanName),
				Check: resource.TestCheckResourceAttr("terrifi_wlan.test", "wpa_mode", "wpa2"),
			},
			{
				Config: wlanTestNetwork(netName, 515) + fmt.Sprintf(`
resource "terrifi_wlan" "test" {
  name       = %q
  passphrase = "testpassword123"
  network_id = terrifi_network.wlan_test.id
  wpa_mode   = "auto"
}
`, wlanName),
				Check: resource.TestCheckResourceAttr("terrifi_wlan.test", "wpa_mode", "auto"),
			},
			{
				Config: wlanTestNetwork(netName, 515) + fmt.Sprintf(`
resource "terrifi_wlan" "test" {
  name       = %q
  passphrase = "testpassword123"
  network_id = terrifi_network.wlan_test.id
  wpa_mode   = "wpa2"
}
`, wlanName),
				Check: resource.TestCheckResourceAttr("terrifi_wlan.test", "wpa_mode", "wpa2"),
			},
		},
	})
}

func TestAccWLAN_wpa3(t *testing.T) {
	requireHardware(t)
	suffix := randomSuffix()
	netName := fmt.Sprintf("tfacc-wlan-net-%s", suffix)
	wlanName := fmt.Sprintf("tfacc-wlan-%s", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: wlanTestNetwork(netName, 509) + fmt.Sprintf(`
resource "terrifi_wlan" "test" {
  name            = %q
  passphrase      = "testpassword123"
  network_id      = terrifi_network.wlan_test.id
  wpa3_support    = true
  wpa3_transition = true
}
`, wlanName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_wlan.test", "wpa3_support", "true"),
					resource.TestCheckResourceAttr("terrifi_wlan.test", "wpa3_transition", "true"),
				),
			},
			{
				Config: wlanTestNetwork(netName, 509) + fmt.Sprintf(`
resource "terrifi_wlan" "test" {
  name            = %q
  passphrase      = "testpassword123"
  network_id      = terrifi_network.wlan_test.id
  wpa3_support    = false
  wpa3_transition = false
}
`, wlanName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_wlan.test", "wpa3_support", "false"),
					resource.TestCheckResourceAttr("terrifi_wlan.test", "wpa3_transition", "false"),
				),
			},
		},
	})
}
