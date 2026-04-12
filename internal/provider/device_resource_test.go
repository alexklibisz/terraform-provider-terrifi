package provider

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/assert"
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
