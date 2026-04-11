package provider

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/assert"
	"github.com/ubiquiti-community/go-unifi/unifi"
)

// ---------------------------------------------------------------------------
// Unit tests — no TF_ACC, no network, no env vars needed
// ---------------------------------------------------------------------------

func TestDeviceResourceModelToAPI(t *testing.T) {
	r := &deviceResource{}

	t.Run("full model", func(t *testing.T) {
		brightness := int64(75)
		volume := int64(50)
		m := &deviceResourceModel{
			Name:                       types.StringValue("Living Room AP"),
			LedOverride:               types.StringValue("on"),
			LedOverrideColor:          types.StringValue("#0000ff"),
			LedOverrideColorBrightness: types.Int64Value(brightness),
			OutdoorModeOverride:       types.StringValue("off"),
			Locked:                    types.BoolValue(true),
			Disabled:                  types.BoolValue(false),
			SnmpContact:              types.StringValue("admin@example.com"),
			SnmpLocation:             types.StringValue("Rack 1"),
			Volume:                   types.Int64Value(volume),
		}

		d := r.modelToAPI(m)

		assert.Equal(t, "Living Room AP", d.Name)
		assert.Equal(t, "on", d.LedOverride)
		assert.Equal(t, "#0000ff", d.LedOverrideColor)
		assert.NotNil(t, d.LedOverrideColorBrightness)
		assert.Equal(t, int64(75), *d.LedOverrideColorBrightness)
		assert.Equal(t, "off", d.OutdoorModeOverride)
		assert.True(t, d.Locked)
		assert.False(t, d.Disabled)
		assert.Equal(t, "admin@example.com", d.SnmpContact)
		assert.Equal(t, "Rack 1", d.SnmpLocation)
		assert.NotNil(t, d.Volume)
		assert.Equal(t, int64(50), *d.Volume)
	})

	t.Run("minimal model — all null", func(t *testing.T) {
		m := &deviceResourceModel{
			Name:                       types.StringNull(),
			LedOverride:               types.StringNull(),
			LedOverrideColor:          types.StringNull(),
			LedOverrideColorBrightness: types.Int64Null(),
			OutdoorModeOverride:       types.StringNull(),
			Locked:                    types.BoolNull(),
			Disabled:                  types.BoolNull(),
			SnmpContact:              types.StringNull(),
			SnmpLocation:             types.StringNull(),
			Volume:                   types.Int64Null(),
		}

		d := r.modelToAPI(m)

		assert.Empty(t, d.Name)
		assert.Empty(t, d.LedOverride)
		assert.Empty(t, d.LedOverrideColor)
		assert.Nil(t, d.LedOverrideColorBrightness)
		assert.Empty(t, d.OutdoorModeOverride)
		assert.False(t, d.Locked)
		assert.False(t, d.Disabled)
		assert.Empty(t, d.SnmpContact)
		assert.Empty(t, d.SnmpLocation)
		assert.Nil(t, d.Volume)
	})

	t.Run("unknown values treated as unset", func(t *testing.T) {
		m := &deviceResourceModel{
			Name:                       types.StringUnknown(),
			LedOverride:               types.StringUnknown(),
			LedOverrideColor:          types.StringUnknown(),
			LedOverrideColorBrightness: types.Int64Unknown(),
			OutdoorModeOverride:       types.StringUnknown(),
			Locked:                    types.BoolUnknown(),
			Disabled:                  types.BoolUnknown(),
			SnmpContact:              types.StringUnknown(),
			SnmpLocation:             types.StringUnknown(),
			Volume:                   types.Int64Unknown(),
		}

		d := r.modelToAPI(m)

		assert.Empty(t, d.Name)
		assert.Empty(t, d.LedOverride)
		assert.Nil(t, d.LedOverrideColorBrightness)
		assert.Nil(t, d.Volume)
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
		assert.Equal(t, "on", m.LedOverride.ValueString())
		assert.Equal(t, "#ff0000", m.LedOverrideColor.ValueString())
		assert.Equal(t, int64(80), m.LedOverrideColorBrightness.ValueInt64())
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
		assert.True(t, m.LedOverride.IsNull())
		assert.True(t, m.LedOverrideColor.IsNull())
		assert.True(t, m.LedOverrideColorBrightness.IsNull())
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

func TestDeviceResourceModelToAPIRoundTrip(t *testing.T) {
	r := &deviceResource{}

	brightness := int64(50)
	volume := int64(75)
	original := &unifi.Device{
		ID:                         "dev-rt",
		MAC:                        "aa:bb:cc:dd:ee:ff",
		Name:                       "Round Trip",
		LedOverride:               "off",
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

	var m deviceResourceModel
	r.apiToModel(original, &m, "default")
	rebuilt := r.modelToAPI(&m)

	assert.Equal(t, original.Name, rebuilt.Name)
	assert.Equal(t, original.LedOverride, rebuilt.LedOverride)
	assert.Equal(t, original.LedOverrideColor, rebuilt.LedOverrideColor)
	assert.Equal(t, *original.LedOverrideColorBrightness, *rebuilt.LedOverrideColorBrightness)
	assert.Equal(t, original.OutdoorModeOverride, rebuilt.OutdoorModeOverride)
	assert.Equal(t, original.Locked, rebuilt.Locked)
	assert.Equal(t, original.Disabled, rebuilt.Disabled)
	assert.Equal(t, original.SnmpContact, rebuilt.SnmpContact)
	assert.Equal(t, original.SnmpLocation, rebuilt.SnmpLocation)
	assert.Equal(t, *original.Volume, *rebuilt.Volume)
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
			{
				Config: fmt.Sprintf(`
resource "terrifi_device" "test" {
  mac          = %q
  name         = "tfacc-device-led"
  led_override = "off"
}
`, dev.MAC),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_device.test", "led_override", "off"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "terrifi_device" "test" {
  mac          = %q
  name         = "tfacc-device-led"
  led_override = "on"
}
`, dev.MAC),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_device.test", "led_override", "on"),
				),
			},
			// Reset to default.
			{
				Config: fmt.Sprintf(`
resource "terrifi_device" "test" {
  mac          = %q
  name         = "tfacc-device-led"
  led_override = "default"
}
`, dev.MAC),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_device.test", "led_override", "default"),
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
				ImportStateVerifyIgnore: []string{"name", "led_override", "led_override_color", "led_override_color_brightness", "outdoor_mode_override", "locked", "disabled", "snmp_contact", "snmp_location", "volume"},
			},
			// Import by site:mac.
			{
				ResourceName:            "terrifi_device.test",
				ImportState:             true,
				ImportStateId:           "default:" + dev.MAC,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"name", "led_override", "led_override_color", "led_override_color_brightness", "outdoor_mode_override", "locked", "disabled", "snmp_contact", "snmp_location", "volume"},
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
  led_override = "on"
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
  led_override  = "off"
  locked        = true
  snmp_contact  = "test@example.com"
  snmp_location = "Lab"
}
`, dev.MAC),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_device.test", "name", "tfacc-device-multi"),
					resource.TestCheckResourceAttr("terrifi_device.test", "led_override", "off"),
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
  led_override  = "on"
  locked        = false
  snmp_contact  = "ops@example.com"
  snmp_location = "Production"
}
`, dev.MAC),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_device.test", "name", "tfacc-device-multi-v2"),
					resource.TestCheckResourceAttr("terrifi_device.test", "led_override", "on"),
					resource.TestCheckResourceAttr("terrifi_device.test", "locked", "false"),
					resource.TestCheckResourceAttr("terrifi_device.test", "snmp_contact", "ops@example.com"),
					resource.TestCheckResourceAttr("terrifi_device.test", "snmp_location", "Production"),
				),
			},
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

func TestAccDeviceResource_validationInvalidLedOverride(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "terrifi_device" "test" {
  mac          = "aa:bb:cc:dd:ee:ff"
  led_override = "blink"
}
`,
				ExpectError: regexp.MustCompile(`must be one of`),
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
  led_override_color = "red"
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
