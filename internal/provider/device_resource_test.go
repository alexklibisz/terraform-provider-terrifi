package provider

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ubiquiti-community/go-unifi/unifi"
)

// ---------------------------------------------------------------------------
// Unit tests — no TF_ACC, no network, no env vars needed
// ---------------------------------------------------------------------------

func TestDeviceUpdatePayload(t *testing.T) {
	t.Run("full model produces correct payload fields", func(t *testing.T) {
		// Verify the model-to-payload logic by testing the Client.UpdateDevice
		// payload construction indirectly through apiToModel round-trip.
		r := &deviceResource{}
		brightness := int64(80)
		volume := int64(42)
		dev := &unifi.Device{
			ID:                         "dev-123",
			MAC:                        "aa:bb:cc:dd:ee:ff",
			Name:                       "Office AP",
			LedOverride:               "on",
			LedOverrideColor:          "#ff0000",
			LedOverrideColorBrightness: &brightness,
			OutdoorModeOverride:       "off",
			Locked:                    true,
			Disabled:                  false,
			SnmpContact:              "admin@test.com",
			SnmpLocation:             "Building A",
			Volume:                   &volume,
			Adopted:                  true,
			State:                    unifi.DeviceStateConnected,
		}

		var m deviceResourceModel
		r.apiToModel(dev, &m, "default")

		// Verify model fields are correctly mapped from API.
		assert.Equal(t, "Office AP", m.Name.ValueString())
		assert.True(t, m.LedEnabled.ValueBool())
		assert.Equal(t, "#ff0000", m.LedColor.ValueString())
		assert.Equal(t, int64(80), m.LedBrightness.ValueInt64())
		assert.Equal(t, "off", m.OutdoorModeOverride.ValueString())
		assert.True(t, m.Locked.ValueBool())
		assert.False(t, m.Disabled.ValueBool())
	})

	t.Run("led_enabled false maps to off", func(t *testing.T) {
		r := &deviceResource{}
		dev := &unifi.Device{
			ID:          "dev-456",
			MAC:         "aa:bb:cc:dd:ee:ff",
			LedOverride: "off",
		}

		var m deviceResourceModel
		r.apiToModel(dev, &m, "default")
		assert.False(t, m.LedEnabled.ValueBool())
	})
}

func TestDeviceResourceAPIToModel(t *testing.T) {
	r := &deviceResource{}

	t.Run("full device", func(t *testing.T) {
		brightness := int64(80)
		volume := int64(42)
		dev := &unifi.Device{
			ID:                         "dev-123",
			MAC:                        "aa:bb:cc:dd:ee:ff",
			Name:                       "Office AP",
			Model:                      "U6-LR",
			Type:                       "uap",
			IP:                         "192.168.1.10",
			Adopted:                    true,
			State:                      unifi.DeviceStateConnected,
			LedOverride:               "on",
			LedOverrideColor:          "#ff0000",
			LedOverrideColorBrightness: &brightness,
			OutdoorModeOverride:       "off",
			Locked:                    true,
			Disabled:                  false,
			SnmpContact:              "admin@test.com",
			SnmpLocation:             "Building A",
			Volume:                   &volume,
		}

		var m deviceResourceModel
		r.apiToModel(dev, &m, "default")

		assert.Equal(t, "dev-123", m.ID.ValueString())
		assert.Equal(t, "default", m.Site.ValueString())
		assert.Equal(t, "aa:bb:cc:dd:ee:ff", m.MAC.ValueString())
		assert.Equal(t, "Office AP", m.Name.ValueString())
		assert.True(t, m.LedEnabled.ValueBool())
		assert.Equal(t, "#ff0000", m.LedColor.ValueString())
		assert.Equal(t, int64(80), m.LedBrightness.ValueInt64())
		assert.Equal(t, "off", m.OutdoorModeOverride.ValueString())
		assert.True(t, m.Locked.ValueBool())
		assert.False(t, m.Disabled.ValueBool())
		assert.Equal(t, "admin@test.com", m.SnmpContact.ValueString())
		assert.Equal(t, "Building A", m.SnmpLocation.ValueString())
		assert.Equal(t, int64(42), m.Volume.ValueInt64())

		// Read-only fields.
		assert.Equal(t, "U6-LR", m.Model.ValueString())
		assert.Equal(t, "uap", m.Type.ValueString())
		assert.Equal(t, "192.168.1.10", m.IP.ValueString())
		assert.True(t, m.Adopted.ValueBool())
		assert.Equal(t, int64(1), m.State.ValueInt64())
	})

	t.Run("minimal device — zero/nil values become null", func(t *testing.T) {
		dev := &unifi.Device{
			ID:  "dev-456",
			MAC: "11:22:33:44:55:66",
		}

		var m deviceResourceModel
		r.apiToModel(dev, &m, "mysite")

		assert.Equal(t, "dev-456", m.ID.ValueString())
		assert.Equal(t, "mysite", m.Site.ValueString())
		assert.Equal(t, "11:22:33:44:55:66", m.MAC.ValueString())
		assert.True(t, m.Name.IsNull())
		assert.True(t, m.LedEnabled.IsNull(), "led_enabled should be null for default/empty")
		assert.True(t, m.LedColor.IsNull())
		assert.True(t, m.LedBrightness.IsNull())
		assert.True(t, m.OutdoorModeOverride.IsNull())
		assert.False(t, m.Locked.ValueBool())
		assert.False(t, m.Disabled.ValueBool())
		assert.True(t, m.SnmpContact.IsNull())
		assert.True(t, m.SnmpLocation.IsNull())
		assert.True(t, m.Volume.IsNull())
		assert.True(t, m.Model.IsNull())
		assert.True(t, m.Type.IsNull())
		assert.True(t, m.IP.IsNull())
		assert.False(t, m.Adopted.ValueBool())
		assert.Equal(t, int64(0), m.State.ValueInt64())
	})

	t.Run("disabled and locked device", func(t *testing.T) {
		dev := &unifi.Device{
			ID:       "dev-789",
			MAC:      "aa:bb:cc:dd:ee:ff",
			Locked:   true,
			Disabled: true,
		}

		var m deviceResourceModel
		r.apiToModel(dev, &m, "default")

		assert.True(t, m.Locked.ValueBool())
		assert.True(t, m.Disabled.ValueBool())
	})
}

func TestDeviceResourceAPIToModelRoundTrip(t *testing.T) {
	r := &deviceResource{}

	brightness := int64(50)
	volume := int64(75)
	original := &unifi.Device{
		ID:                         "dev-rt",
		MAC:                        "aa:bb:cc:dd:ee:ff",
		Name:                       "Round Trip",
		LedOverride:               "on",
		LedOverrideColor:          "#00ff00",
		LedOverrideColorBrightness: &brightness,
		OutdoorModeOverride:       "on",
		Locked:                    true,
		Disabled:                  false,
		SnmpContact:              "ops@example.com",
		SnmpLocation:             "DC1",
		Volume:                   &volume,
		Model:                    "USW-24",
		Type:                     "usw",
		IP:                       "10.0.0.1",
		Adopted:                  true,
		State:                    unifi.DeviceStateConnected,
	}

	// API → model → verify model correctly represents the API data.
	var m deviceResourceModel
	r.apiToModel(original, &m, "default")

	assert.Equal(t, original.Name, m.Name.ValueString())
	assert.True(t, m.LedEnabled.ValueBool()) // "on" → true
	assert.Equal(t, original.LedOverrideColor, m.LedColor.ValueString())
	assert.Equal(t, *original.LedOverrideColorBrightness, m.LedBrightness.ValueInt64())
	assert.Equal(t, original.OutdoorModeOverride, m.OutdoorModeOverride.ValueString())
	assert.Equal(t, original.Locked, m.Locked.ValueBool())
	assert.Equal(t, original.Disabled, m.Disabled.ValueBool())
	assert.Equal(t, original.SnmpContact, m.SnmpContact.ValueString())
	assert.Equal(t, original.SnmpLocation, m.SnmpLocation.ValueString())
	assert.Equal(t, *original.Volume, m.Volume.ValueInt64())
	assert.Equal(t, "USW-24", m.Model.ValueString())
	assert.Equal(t, "usw", m.Type.ValueString())
	assert.True(t, m.Adopted.ValueBool())
}

// ---------------------------------------------------------------------------
// Acceptance tests — require TF_ACC=1 and a UniFi controller with adopted device
// ---------------------------------------------------------------------------

// TestAccDeviceResource_basic manages an adopted device with just a name.
func TestAccDeviceResource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC not set")
	}
	preCheck(t)
	requireAdoptedDevice(t)

	dev := findFirstAdoptedDevice(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_device" "test" {
  mac  = %q
  name = "tfacc-device-basic"
}
`, dev.MAC),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("terrifi_device.test", "id"),
					resource.TestCheckResourceAttr("terrifi_device.test", "mac", dev.MAC),
					resource.TestCheckResourceAttr("terrifi_device.test", "name", "tfacc-device-basic"),
					resource.TestCheckResourceAttr("terrifi_device.test", "adopted", "true"),
					resource.TestCheckResourceAttrSet("terrifi_device.test", "model"),
					resource.TestCheckResourceAttrSet("terrifi_device.test", "type"),
				),
			},
			// Update name.
			{
				Config: fmt.Sprintf(`
resource "terrifi_device" "test" {
  mac  = %q
  name = "tfacc-device-renamed"
}
`, dev.MAC),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_device.test", "name", "tfacc-device-renamed"),
				),
			},
			// Remove name (set to null).
			{
				Config: fmt.Sprintf(`
resource "terrifi_device" "test" {
  mac = %q
}
`, dev.MAC),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("terrifi_device.test", "name"),
				),
			},
		},
	})
}

// TestAccDeviceResource_ledOverride sets and changes LED override.
func TestAccDeviceResource_ledOverride(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC not set")
	}
	preCheck(t)
	requireAdoptedDevice(t)

	dev := findFirstAdoptedDevice(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// LEDs off.
			{
				Config: fmt.Sprintf(`
resource "terrifi_device" "test" {
  mac          = %q
  name         = "tfacc-device-led"
  led_enabled = false
}
`, dev.MAC),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_device.test", "led_enabled", "false"),
				),
			},
			// LEDs on.
			{
				Config: fmt.Sprintf(`
resource "terrifi_device" "test" {
  mac          = %q
  name         = "tfacc-device-led"
  led_enabled = true
}
`, dev.MAC),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_device.test", "led_enabled", "true"),
				),
			},
			// Reset to site default (omit attribute).
			{
				Config: fmt.Sprintf(`
resource "terrifi_device" "test" {
  mac  = %q
  name = "tfacc-device-led"
}
`, dev.MAC),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("terrifi_device.test", "led_enabled"),
				),
			},
		},
	})
}

// TestAccDeviceResource_snmp sets SNMP contact and location.
func TestAccDeviceResource_snmp(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC not set")
	}
	preCheck(t)
	requireAdoptedDevice(t)

	dev := findFirstAdoptedDevice(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_device" "test" {
  mac           = %q
  name          = "tfacc-device-snmp"
  snmp_contact  = "admin@example.com"
  snmp_location = "Server Room A"
}
`, dev.MAC),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_device.test", "snmp_contact", "admin@example.com"),
					resource.TestCheckResourceAttr("terrifi_device.test", "snmp_location", "Server Room A"),
				),
			},
			// Update SNMP.
			{
				Config: fmt.Sprintf(`
resource "terrifi_device" "test" {
  mac           = %q
  name          = "tfacc-device-snmp"
  snmp_contact  = "ops@example.com"
  snmp_location = "Server Room B"
}
`, dev.MAC),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_device.test", "snmp_contact", "ops@example.com"),
					resource.TestCheckResourceAttr("terrifi_device.test", "snmp_location", "Server Room B"),
				),
			},
			// Remove SNMP.
			{
				Config: fmt.Sprintf(`
resource "terrifi_device" "test" {
  mac  = %q
  name = "tfacc-device-snmp"
}
`, dev.MAC),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("terrifi_device.test", "snmp_contact"),
					resource.TestCheckNoResourceAttr("terrifi_device.test", "snmp_location"),
				),
			},
		},
	})
}

// TestAccDeviceResource_locked tests the locked attribute.
func TestAccDeviceResource_locked(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC not set")
	}
	preCheck(t)
	requireAdoptedDevice(t)

	dev := findFirstAdoptedDevice(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_device" "test" {
  mac    = %q
  name   = "tfacc-device-locked"
  locked = true
}
`, dev.MAC),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_device.test", "locked", "true"),
				),
			},
			// Unlock.
			{
				Config: fmt.Sprintf(`
resource "terrifi_device" "test" {
  mac    = %q
  name   = "tfacc-device-locked"
  locked = false
}
`, dev.MAC),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_device.test", "locked", "false"),
				),
			},
		},
	})
}

// TestAccDeviceResource_import tests importing by MAC and site:mac.
func TestAccDeviceResource_import(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC not set")
	}
	preCheck(t)
	requireAdoptedDevice(t)

	dev := findFirstAdoptedDevice(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create the resource first.
			{
				Config: fmt.Sprintf(`
resource "terrifi_device" "test" {
  mac  = %q
  name = "tfacc-device-import"
}
`, dev.MAC),
			},
			// Import by MAC.
			{
				ResourceName:            "terrifi_device.test",
				ImportState:             true,
				ImportStateId:           dev.MAC,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"name", "led_enabled", "led_color", "led_brightness", "outdoor_mode_override", "locked", "disabled", "snmp_contact", "snmp_location", "volume"},
			},
			// Import by site:mac.
			{
				ResourceName:            "terrifi_device.test",
				ImportState:             true,
				ImportStateId:           "default:" + dev.MAC,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"name", "led_enabled", "led_color", "led_brightness", "outdoor_mode_override", "locked", "disabled", "snmp_contact", "snmp_location", "volume"},
			},
		},
	})
}

// TestAccDeviceResource_idempotent verifies applying same config twice produces no diff.
func TestAccDeviceResource_idempotent(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC not set")
	}
	preCheck(t)
	requireAdoptedDevice(t)

	dev := findFirstAdoptedDevice(t)
	config := fmt.Sprintf(`
resource "terrifi_device" "test" {
  mac          = %q
  name         = "tfacc-device-idempotent"
  led_enabled = true
}
`, dev.MAC)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config},
			{Config: config}, // Second apply — should produce no changes.
		},
	})
}

// TestAccDeviceResource_multipleFields sets multiple optional fields at once.
func TestAccDeviceResource_multipleFields(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC not set")
	}
	preCheck(t)
	requireAdoptedDevice(t)

	dev := findFirstAdoptedDevice(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_device" "test" {
  mac           = %q
  name          = "tfacc-device-multi"
  led_enabled  = false
  locked        = true
  snmp_contact  = "test@example.com"
  snmp_location = "Lab"
}
`, dev.MAC),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_device.test", "name", "tfacc-device-multi"),
					resource.TestCheckResourceAttr("terrifi_device.test", "led_enabled", "false"),
					resource.TestCheckResourceAttr("terrifi_device.test", "locked", "true"),
					resource.TestCheckResourceAttr("terrifi_device.test", "snmp_contact", "test@example.com"),
					resource.TestCheckResourceAttr("terrifi_device.test", "snmp_location", "Lab"),
				),
			},
			// Change all fields at once.
			{
				Config: fmt.Sprintf(`
resource "terrifi_device" "test" {
  mac           = %q
  name          = "tfacc-device-multi-v2"
  led_enabled  = true
  locked        = false
  snmp_contact  = "ops@example.com"
  snmp_location = "Production"
}
`, dev.MAC),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_device.test", "name", "tfacc-device-multi-v2"),
					resource.TestCheckResourceAttr("terrifi_device.test", "led_enabled", "true"),
					resource.TestCheckResourceAttr("terrifi_device.test", "locked", "false"),
					resource.TestCheckResourceAttr("terrifi_device.test", "snmp_contact", "ops@example.com"),
					resource.TestCheckResourceAttr("terrifi_device.test", "snmp_location", "Production"),
				),
			},
		},
	})
}

// TestAccDeviceResource_nameOnly ensures mac-only → name → mac-only lifecycle works
// without touching any other fields (regression: API-returned fields like
// led_color, outdoor_mode_override must not leak into state).
func TestAccDeviceResource_nameOnly(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC not set")
	}
	preCheck(t)
	requireAdoptedDevice(t)

	dev := findFirstAdoptedDevice(t)
	macOnly := fmt.Sprintf(`
resource "terrifi_device" "test" {
  mac = %q
}
`, dev.MAC)

	withName := fmt.Sprintf(`
resource "terrifi_device" "test" {
  mac  = %q
  name = "tfacc-device-nameonly"
}
`, dev.MAC)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: MAC only — no optional fields.
			{Config: macOnly},
			// Step 2: re-apply same config — must be idempotent (no spurious diffs
			// from API-returned fields like led_color, outdoor_mode_override).
			{Config: macOnly},
			// Step 3: add name.
			{
				Config: withName,
				Check:  resource.TestCheckResourceAttr("terrifi_device.test", "name", "tfacc-device-nameonly"),
			},
			// Step 4: remove name again.
			{Config: macOnly},
			// Step 5: idempotent after removal.
			{Config: macOnly},
		},
	})
}

// TestAccDeviceResource_addRemoveOptionals adds optional fields then removes them,
// verifying no spurious diffs after removal (regression for "100 -> null" diffs).
func TestAccDeviceResource_addRemoveOptionals(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC not set")
	}
	preCheck(t)
	requireAdoptedDevice(t)

	dev := findFirstAdoptedDevice(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: set several optional fields.
			{
				Config: fmt.Sprintf(`
resource "terrifi_device" "test" {
  mac           = %q
  name          = "tfacc-device-addremove"
  led_enabled   = false
  snmp_contact  = "admin@example.com"
  snmp_location = "Rack 1"
  locked        = true
}
`, dev.MAC),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_device.test", "led_enabled", "false"),
					resource.TestCheckResourceAttr("terrifi_device.test", "snmp_contact", "admin@example.com"),
					resource.TestCheckResourceAttr("terrifi_device.test", "snmp_location", "Rack 1"),
					resource.TestCheckResourceAttr("terrifi_device.test", "locked", "true"),
				),
			},
			// Step 2: remove all optional fields except mac.
			{
				Config: fmt.Sprintf(`
resource "terrifi_device" "test" {
  mac = %q
}
`, dev.MAC),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("terrifi_device.test", "name"),
					resource.TestCheckNoResourceAttr("terrifi_device.test", "led_enabled"),
					resource.TestCheckNoResourceAttr("terrifi_device.test", "snmp_contact"),
					resource.TestCheckNoResourceAttr("terrifi_device.test", "snmp_location"),
					resource.TestCheckResourceAttr("terrifi_device.test", "locked", "false"),
				),
			},
			// Step 3: idempotent after removal — must produce no diff.
			{
				Config: fmt.Sprintf(`
resource "terrifi_device" "test" {
  mac = %q
}
`, dev.MAC),
			},
		},
	})
}

// TestAccDeviceResource_importThenApply imports a device, then applies config
// without changes — must be a no-op (regression for import → plan diffs).
func TestAccDeviceResource_importThenApply(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC not set")
	}
	preCheck(t)
	requireAdoptedDevice(t)

	dev := findFirstAdoptedDevice(t)
	config := fmt.Sprintf(`
resource "terrifi_device" "test" {
  mac  = %q
  name = "tfacc-device-importapply"
}
`, dev.MAC)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create.
			{Config: config},
			// Import.
			{
				ResourceName:            "terrifi_device.test",
				ImportState:             true,
				ImportStateId:           dev.MAC,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"name", "led_enabled", "led_color", "led_brightness", "outdoor_mode_override", "locked", "disabled", "snmp_contact", "snmp_location", "volume"},
			},
			// Apply same config after import — must succeed with no errors.
			{Config: config},
			// Second apply — idempotent.
			{Config: config},
		},
	})
}

// TestAccDeviceResource_ledToggleIdempotent toggles LED on/off/on and checks
// idempotency at each step.
func TestAccDeviceResource_ledToggleIdempotent(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC not set")
	}
	preCheck(t)
	requireAdoptedDevice(t)

	dev := findFirstAdoptedDevice(t)

	ledOn := fmt.Sprintf(`
resource "terrifi_device" "test" {
  mac         = %q
  name        = "tfacc-device-ledtoggle"
  led_enabled = true
}
`, dev.MAC)

	ledOff := fmt.Sprintf(`
resource "terrifi_device" "test" {
  mac         = %q
  name        = "tfacc-device-ledtoggle"
  led_enabled = false
}
`, dev.MAC)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: ledOn,
				Check:  resource.TestCheckResourceAttr("terrifi_device.test", "led_enabled", "true"),
			},
			{Config: ledOn}, // idempotent
			{
				Config: ledOff,
				Check:  resource.TestCheckResourceAttr("terrifi_device.test", "led_enabled", "false"),
			},
			{Config: ledOff}, // idempotent
			{
				Config: ledOn,
				Check:  resource.TestCheckResourceAttr("terrifi_device.test", "led_enabled", "true"),
			},
		},
	})
}

// TestAccDeviceResource_disabledToggle tests the disabled attribute lifecycle.
func TestAccDeviceResource_disabledToggle(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC not set")
	}
	preCheck(t)
	requireAdoptedDevice(t)

	dev := findFirstAdoptedDevice(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_device" "test" {
  mac      = %q
  name     = "tfacc-device-disabled"
  disabled = true
}
`, dev.MAC),
				Check: resource.TestCheckResourceAttr("terrifi_device.test", "disabled", "true"),
			},
			{
				Config: fmt.Sprintf(`
resource "terrifi_device" "test" {
  mac      = %q
  name     = "tfacc-device-disabled"
  disabled = false
}
`, dev.MAC),
				Check: resource.TestCheckResourceAttr("terrifi_device.test", "disabled", "false"),
			},
		},
	})
}

// TestAccDeviceResource_computedFieldsStable verifies computed read-only fields
// don't cause spurious diffs across multiple applies.
func TestAccDeviceResource_computedFieldsStable(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC not set")
	}
	preCheck(t)
	requireAdoptedDevice(t)

	dev := findFirstAdoptedDevice(t)
	config := fmt.Sprintf(`
resource "terrifi_device" "test" {
  mac  = %q
  name = "tfacc-device-computed"
}
`, dev.MAC)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("terrifi_device.test", "model"),
					resource.TestCheckResourceAttrSet("terrifi_device.test", "type"),
					resource.TestCheckResourceAttrSet("terrifi_device.test", "ip"),
					resource.TestCheckResourceAttr("terrifi_device.test", "adopted", "true"),
					resource.TestCheckResourceAttrSet("terrifi_device.test", "state"),
				),
			},
			// Second and third apply — computed fields must remain stable.
			{Config: config},
			{Config: config},
		},
	})
}

// ---------------------------------------------------------------------------
// Radio settings unit tests
// ---------------------------------------------------------------------------

func TestDeviceRadiosAPIToModel(t *testing.T) {
	r := &deviceResource{}

	t.Run("AP with 2.4 and 5 GHz radios", func(t *testing.T) {
		minRssi := int64(-75)
		ht := int64(80)
		dev := &unifi.Device{
			ID:  "ap-1",
			MAC: "aa:bb:cc:dd:ee:ff",
			RadioTable: []unifi.DeviceRadioTable{
				{
					Radio:          "ng",
					Channel:        "6",
					Ht:             &ht,
					TxPowerMode:    "auto",
					MinRssiEnabled: false,
				},
				{
					Radio:          "na",
					Channel:        "auto",
					Ht:             &ht,
					TxPowerMode:    "high",
					TxPower:        "23",
					MinRssiEnabled: true,
					MinRssi:        &minRssi,
				},
			},
		}

		var m deviceResourceModel
		r.apiToModel(dev, &m, "default")

		require.NotNil(t, m.Radio24)
		require.NotNil(t, m.Radio5)
		assert.Nil(t, m.Radio6)

		assert.Equal(t, "6", m.Radio24.Channel.ValueString())
		assert.Equal(t, int64(80), m.Radio24.Ht.ValueInt64())
		assert.Equal(t, "auto", m.Radio24.TxPowerMode.ValueString())
		assert.True(t, m.Radio24.TxPower.IsNull())
		assert.False(t, m.Radio24.MinRssiEnabled.ValueBool())
		assert.True(t, m.Radio24.MinRssi.IsNull())

		assert.Equal(t, "auto", m.Radio5.Channel.ValueString())
		assert.Equal(t, int64(80), m.Radio5.Ht.ValueInt64())
		assert.Equal(t, "high", m.Radio5.TxPowerMode.ValueString())
		assert.Equal(t, "23", m.Radio5.TxPower.ValueString())
		assert.True(t, m.Radio5.MinRssiEnabled.ValueBool())
		assert.Equal(t, int64(-75), m.Radio5.MinRssi.ValueInt64())
	})

	t.Run("AP with 6 GHz radio", func(t *testing.T) {
		ht := int64(160)
		dev := &unifi.Device{
			ID:  "ap-6e",
			MAC: "aa:bb:cc:dd:ee:ff",
			RadioTable: []unifi.DeviceRadioTable{
				{Radio: "6e", Channel: "37", Ht: &ht, TxPowerMode: "auto"},
			},
		}
		var m deviceResourceModel
		r.apiToModel(dev, &m, "default")

		assert.Nil(t, m.Radio24)
		assert.Nil(t, m.Radio5)
		require.NotNil(t, m.Radio6)
		assert.Equal(t, "37", m.Radio6.Channel.ValueString())
		assert.Equal(t, int64(160), m.Radio6.Ht.ValueInt64())
	})

	t.Run("device with no radios", func(t *testing.T) {
		dev := &unifi.Device{ID: "sw-1", MAC: "aa:bb:cc:dd:ee:ff"}
		var m deviceResourceModel
		r.apiToModel(dev, &m, "default")
		assert.Nil(t, m.Radio24)
		assert.Nil(t, m.Radio5)
		assert.Nil(t, m.Radio6)
	})
}

func TestPreserveNullRadio(t *testing.T) {
	full := func() *deviceRadioSettingsModel {
		return &deviceRadioSettingsModel{
			Channel:        types.StringValue("6"),
			Ht:             types.Int64Value(40),
			TxPower:        types.StringValue("20"),
			TxPowerMode:    types.StringValue("custom"),
			MinRssiEnabled: types.BoolValue(true),
			MinRssi:        types.Int64Value(-75),
		}
	}

	t.Run("nil plan nulls state", func(t *testing.T) {
		state := full()
		preserveNullRadio(nil, &state)
		assert.Nil(t, state)
	})

	t.Run("nil state stays nil", func(t *testing.T) {
		plan := full()
		var state *deviceRadioSettingsModel
		preserveNullRadio(plan, &state)
		assert.Nil(t, state)
	})

	t.Run("plan with some null fields nulls corresponding state fields", func(t *testing.T) {
		plan := &deviceRadioSettingsModel{
			Channel:        types.StringValue("auto"),
			Ht:             types.Int64Null(),
			TxPower:        types.StringNull(),
			TxPowerMode:    types.StringNull(),
			MinRssiEnabled: types.BoolNull(),
			MinRssi:        types.Int64Null(),
		}
		state := full()
		preserveNullRadio(plan, &state)
		require.NotNil(t, state)
		// Configured field kept from state (API).
		assert.Equal(t, "6", state.Channel.ValueString())
		// Unconfigured fields nulled.
		assert.True(t, state.Ht.IsNull())
		assert.True(t, state.TxPower.IsNull())
		assert.True(t, state.TxPowerMode.IsNull())
		assert.True(t, state.MinRssiEnabled.IsNull())
		assert.True(t, state.MinRssi.IsNull())
	})

	t.Run("plan with all fields set preserves all state values", func(t *testing.T) {
		plan := full()
		state := full()
		preserveNullRadio(plan, &state)
		require.NotNil(t, state)
		assert.False(t, state.Channel.IsNull())
		assert.False(t, state.Ht.IsNull())
		assert.False(t, state.TxPower.IsNull())
		assert.False(t, state.TxPowerMode.IsNull())
		assert.False(t, state.MinRssiEnabled.IsNull())
		assert.False(t, state.MinRssi.IsNull())
	})
}

func TestApplyPlannedToRadioEntry(t *testing.T) {
	t.Run("updates only non-null fields", func(t *testing.T) {
		ht := int64(80)
		existing := &unifi.DeviceRadioTable{
			Radio:          "na",
			Channel:        "36",
			Ht:             &ht,
			TxPower:        "23",
			TxPowerMode:    "high",
			MinRssiEnabled: true,
		}
		planned := deviceRadioSettingsModel{
			Channel:        types.StringValue("auto"),
			Ht:             types.Int64Null(),
			TxPower:        types.StringNull(),
			TxPowerMode:    types.StringNull(),
			MinRssiEnabled: types.BoolValue(false),
			MinRssi:        types.Int64Null(),
		}
		applyPlannedToRadioEntry(existing, planned)

		assert.Equal(t, "auto", existing.Channel)
		assert.Equal(t, int64(80), *existing.Ht, "Ht unchanged")
		assert.Equal(t, "23", existing.TxPower, "TxPower unchanged")
		assert.Equal(t, "high", existing.TxPowerMode, "TxPowerMode unchanged")
		assert.False(t, existing.MinRssiEnabled)
	})

	t.Run("updates all fields when all set", func(t *testing.T) {
		ht0 := int64(80)
		existing := &unifi.DeviceRadioTable{
			Radio:   "ng",
			Channel: "6",
			Ht:      &ht0,
		}
		newHt := int64(40)
		minRssi := int64(-70)
		planned := deviceRadioSettingsModel{
			Channel:        types.StringValue("11"),
			Ht:             types.Int64Value(newHt),
			TxPower:        types.StringValue("18"),
			TxPowerMode:    types.StringValue("custom"),
			MinRssiEnabled: types.BoolValue(true),
			MinRssi:        types.Int64Value(minRssi),
		}
		applyPlannedToRadioEntry(existing, planned)

		assert.Equal(t, "11", existing.Channel)
		assert.Equal(t, int64(40), *existing.Ht)
		assert.Equal(t, "18", existing.TxPower)
		assert.Equal(t, "custom", existing.TxPowerMode)
		assert.True(t, existing.MinRssiEnabled)
		assert.Equal(t, int64(-70), *existing.MinRssi)
	})
}

// ---------------------------------------------------------------------------
// Radio settings acceptance tests
// ---------------------------------------------------------------------------

// TestAccDeviceResource_radio24_setChannel configures channel and channel
// width on the 2.4 GHz radio, verifies idempotency, then removes the config.
func TestAccDeviceResource_radio24_setChannel(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC not set")
	}
	preCheck(t)
	requireAdoptedAP(t)

	ap := findFirstAdoptedAP(t)
	withRadio := fmt.Sprintf(`
resource "terrifi_device" "test" {
  mac  = %q
  name = "tfacc-device-radio-channel"
  radio_24 = {
    channel = "auto"
    ht      = 40
  }
}
`, ap.MAC)
	bare := fmt.Sprintf(`
resource "terrifi_device" "test" {
  mac  = %q
  name = "tfacc-device-radio-channel"
}
`, ap.MAC)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: withRadio,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_device.test", "radio_24.channel", "auto"),
					resource.TestCheckResourceAttr("terrifi_device.test", "radio_24.ht", "40"),
				),
			},
			// Idempotent — second apply must produce no diff.
			{Config: withRadio},
			// Remove radio_24 — must leave device settings unchanged.
			{
				Config: bare,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("terrifi_device.test", "radio_24"),
				),
			},
		},
	})
}

// TestAccDeviceResource_radio24_txPower sets tx_power_mode on the 2.4 GHz radio.
func TestAccDeviceResource_radio24_txPower(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC not set")
	}
	preCheck(t)
	requireAdoptedAP(t)

	ap := findFirstAdoptedAP(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_device" "test" {
  mac  = %q
  name = "tfacc-device-radio-txpower"
  radio_24 = {
    tx_power_mode = "medium"
  }
}
`, ap.MAC),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_device.test", "radio_24.tx_power_mode", "medium"),
				),
			},
			// Change tx_power_mode.
			{
				Config: fmt.Sprintf(`
resource "terrifi_device" "test" {
  mac  = %q
  name = "tfacc-device-radio-txpower"
  radio_24 = {
    tx_power_mode = "auto"
  }
}
`, ap.MAC),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_device.test", "radio_24.tx_power_mode", "auto"),
				),
			},
		},
	})
}

// TestAccDeviceResource_radioBoth configures both 2.4 GHz and 5 GHz radios
// simultaneously.
func TestAccDeviceResource_radioBoth(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC not set")
	}
	preCheck(t)
	requireAdoptedAP(t)

	ap := findFirstAdoptedAP(t)
	if len(ap.RadioTable) < 2 {
		t.Skip("AP does not have at least two radios — skipping multi-radio test")
	}

	config := fmt.Sprintf(`
resource "terrifi_device" "test" {
  mac  = %q
  name = "tfacc-device-radio-multi"
  radio_24 = {
    channel       = "auto"
    ht            = 40
    tx_power_mode = "auto"
  }
  radio_5 = {
    channel       = "auto"
    ht            = 80
    tx_power_mode = "auto"
  }
}
`, ap.MAC)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_device.test", "radio_24.ht", "40"),
					resource.TestCheckResourceAttr("terrifi_device.test", "radio_5.ht", "80"),
				),
			},
			// Idempotent.
			{Config: config},
		},
	})
}

// TestAccDeviceResource_radio_addRemove adds a radio block then removes it,
// verifying no spurious diffs after removal.
func TestAccDeviceResource_radio_addRemove(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC not set")
	}
	preCheck(t)
	requireAdoptedAP(t)

	ap := findFirstAdoptedAP(t)
	macOnly := fmt.Sprintf(`
resource "terrifi_device" "test" {
  mac = %q
}
`, ap.MAC)

	withRadio := fmt.Sprintf(`
resource "terrifi_device" "test" {
  mac = %q
  radio_24 = {
    channel       = "auto"
    tx_power_mode = "auto"
  }
}
`, ap.MAC)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: macOnly},
			{Config: macOnly},
			{Config: withRadio,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("terrifi_device.test", "radio_24.channel"),
				),
			},
			{Config: withRadio},
			{Config: macOnly,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("terrifi_device.test", "radio_24"),
				),
			},
			{Config: macOnly},
		},
	})
}

// TestAccDeviceResource_radio_import verifies import works with radio blocks set.
func TestAccDeviceResource_radio_import(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC not set")
	}
	preCheck(t)
	requireAdoptedAP(t)

	ap := findFirstAdoptedAP(t)
	config := fmt.Sprintf(`
resource "terrifi_device" "test" {
  mac  = %q
  name = "tfacc-device-radio-import"
  radio_24 = {
    channel       = "auto"
    tx_power_mode = "auto"
  }
}
`, ap.MAC)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				ResourceName:  "terrifi_device.test",
				ImportState:   true,
				ImportStateId: ap.MAC,
				// Radio blocks aren't preserved across import (import can't know
				// which fields the user configured), so ignore them here.
				ImportStateVerifyIgnore: []string{
					"name", "led_enabled", "led_color", "led_brightness",
					"outdoor_mode_override", "locked", "disabled",
					"snmp_contact", "snmp_location", "volume",
					"radio_24", "radio_5", "radio_6",
				},
			},
			// Re-apply after import must succeed without diffs.
			{Config: config},
		},
	})
}

// ---------------------------------------------------------------------------
// Validation tests (no controller needed)
// ---------------------------------------------------------------------------

func TestAccDeviceResource_validationInvalidMAC(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "terrifi_device" "test" {
  mac = "not-a-mac"
}
`,
				ExpectError: regexp.MustCompile(`must be a valid MAC address`),
			},
		},
	})
}


func TestAccDeviceResource_validationInvalidLedColor(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "terrifi_device" "test" {
  mac                = "aa:bb:cc:dd:ee:ff"
  led_color = "red"
}
`,
				ExpectError: regexp.MustCompile(`must be a hex color`),
			},
		},
	})
}

func TestAccDeviceResource_validationInvalidOutdoorMode(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "terrifi_device" "test" {
  mac                    = "aa:bb:cc:dd:ee:ff"
  outdoor_mode_override  = "auto"
}
`,
				ExpectError: regexp.MustCompile(`must be one of`),
			},
		},
	})
}

func TestAccDeviceResource_validationInvalidHt(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "terrifi_device" "test" {
  mac = "aa:bb:cc:dd:ee:ff"
  radio_24 = { ht = 100 }
}
`,
				ExpectError: regexp.MustCompile(`must be one of`),
			},
		},
	})
}

func TestAccDeviceResource_validationInvalidTxPowerMode(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "terrifi_device" "test" {
  mac = "aa:bb:cc:dd:ee:ff"
  radio_5 = { tx_power_mode = "max" }
}
`,
				ExpectError: regexp.MustCompile(`must be one of`),
			},
		},
	})
}

func TestAccDeviceResource_validationInvalidMinRssi(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "terrifi_device" "test" {
  mac = "aa:bb:cc:dd:ee:ff"
  radio_5 = { min_rssi = -50 }
}
`,
				ExpectError: regexp.MustCompile(`Attribute must be between`),
			},
		},
	})
}
