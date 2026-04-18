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

func TestDeviceDataSourceAPIToModel(t *testing.T) {
	d := &deviceDataSource{}

	t.Run("full device", func(t *testing.T) {
		dev := &unifi.Device{
			ID:       "dev-123",
			MAC:      "aa:bb:cc:dd:ee:ff",
			Name:     "Living Room AP",
			Model:    "U6-LR",
			Type:     "uap",
			IP:       "192.168.1.10",
			Disabled: false,
			Adopted:  true,
			State:    unifi.DeviceStateConnected,
		}

		var model deviceDataSourceModel
		d.apiToModel(dev, &model, "default")

		assert.Equal(t, "dev-123", model.ID.ValueString())
		assert.Equal(t, "default", model.Site.ValueString())
		assert.Equal(t, "aa:bb:cc:dd:ee:ff", model.MAC.ValueString())
		assert.Equal(t, "Living Room AP", model.Name.ValueString())
		assert.Equal(t, "U6-LR", model.Model.ValueString())
		assert.Equal(t, "uap", model.Type.ValueString())
		assert.Equal(t, "192.168.1.10", model.IP.ValueString())
		assert.False(t, model.Disabled.ValueBool())
		assert.True(t, model.Adopted.ValueBool())
		assert.Equal(t, int64(1), model.State.ValueInt64())
	})

	t.Run("minimal device", func(t *testing.T) {
		dev := &unifi.Device{
			ID:  "dev-456",
			MAC: "11:22:33:44:55:66",
		}

		var model deviceDataSourceModel
		d.apiToModel(dev, &model, "mysite")

		assert.Equal(t, "dev-456", model.ID.ValueString())
		assert.Equal(t, "mysite", model.Site.ValueString())
		assert.Equal(t, "11:22:33:44:55:66", model.MAC.ValueString())
		assert.True(t, model.Name.IsNull())
		assert.True(t, model.Model.IsNull())
		assert.True(t, model.Type.IsNull())
		assert.True(t, model.IP.IsNull())
		assert.False(t, model.Disabled.ValueBool())
		assert.False(t, model.Adopted.ValueBool())
		assert.Equal(t, int64(0), model.State.ValueInt64())
	})

	t.Run("disabled device", func(t *testing.T) {
		dev := &unifi.Device{
			ID:       "dev-789",
			MAC:      "aa:bb:cc:dd:ee:ff",
			Disabled: true,
			Adopted:  true,
			State:    unifi.DeviceStateConnected,
		}

		var model deviceDataSourceModel
		d.apiToModel(dev, &model, "default")

		assert.True(t, model.Disabled.ValueBool())
		assert.True(t, model.Adopted.ValueBool())
	})
}

// ---------------------------------------------------------------------------
// Acceptance tests — require TF_ACC=1 and a UniFi controller
// ---------------------------------------------------------------------------

// TestAccDeviceDataSource_byMAC looks up a device by MAC. This test requires
// at least one adopted device on the controller. It creates a client device
// resource first (which always works), then uses the data source to look up
// the controller's own devices by listing them.
func TestAccDeviceDataSource_byMAC(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC not set")
	}
	preCheck(t)
	requireAdoptedDevice(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDeviceDataSourceByMACConfig(t),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.terrifi_device.test", "id"),
					resource.TestCheckResourceAttrSet("data.terrifi_device.test", "mac"),
					resource.TestCheckResourceAttrSet("data.terrifi_device.test", "model"),
					resource.TestCheckResourceAttrSet("data.terrifi_device.test", "type"),
					resource.TestCheckResourceAttr("data.terrifi_device.test", "adopted", "true"),
				),
			},
		},
	})
}

func TestAccDeviceDataSource_byName(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC not set")
	}
	preCheck(t)
	requireAdoptedDevice(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDeviceDataSourceByNameConfig(t),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.terrifi_device.test", "id"),
					resource.TestCheckResourceAttrSet("data.terrifi_device.test", "mac"),
					resource.TestCheckResourceAttrSet("data.terrifi_device.test", "model"),
					resource.TestCheckResourceAttrSet("data.terrifi_device.test", "type"),
					resource.TestCheckResourceAttr("data.terrifi_device.test", "adopted", "true"),
				),
			},
		},
	})
}

func TestAccDeviceDataSource_withClientDevice(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC not set")
	}
	preCheck(t)
	requireAdoptedDevice(t)
	mac := randomMAC()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDeviceDataSourceWithClientDeviceConfig(t, mac),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.terrifi_device.test", "mac"),
					resource.TestCheckResourceAttrPair(
						"terrifi_client_device.test", "fixed_ap_mac",
						"data.terrifi_device.test", "mac",
					),
				),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// requireAdoptedDevice skips the test if no adopted device exists on the controller.
func requireAdoptedDevice(t *testing.T) {
	t.Helper()
	dev := findFirstAdoptedDevice(t)
	if dev == nil {
		t.Skip("no adopted devices found on controller — skipping device data source test")
	}
}

// findFirstAdoptedDevice connects to the controller and returns the first
// adopted device, or nil if none exist.
func findFirstAdoptedDevice(t *testing.T) *unifi.Device {
	t.Helper()
	client := testAccGetClient(t)
	devices, err := client.ApiClient.ListDevice(t.Context(), "default")
	if err != nil {
		t.Fatalf("failed to list devices: %s", err)
	}
	for i := range devices {
		if devices[i].Adopted {
			return &devices[i]
		}
	}
	return nil
}

// findFirstAdoptedAP returns the first adopted access point (type="uap"), or
// nil if none exist. Radio settings are only meaningful on APs.
func findFirstAdoptedAP(t *testing.T) *unifi.Device {
	t.Helper()
	client := testAccGetClient(t)
	devices, err := client.ApiClient.ListDevice(t.Context(), "default")
	if err != nil {
		t.Fatalf("failed to list devices: %s", err)
	}
	for i := range devices {
		if devices[i].Adopted && devices[i].Type == "uap" {
			return &devices[i]
		}
	}
	return nil
}

// requireAdoptedAP skips the test if no adopted AP exists on the controller.
func requireAdoptedAP(t *testing.T) {
	t.Helper()
	if findFirstAdoptedAP(t) == nil {
		t.Skip("no adopted access points found on controller — skipping radio test")
	}
}

func testAccDeviceDataSourceByMACConfig(t *testing.T) string {
	t.Helper()
	dev := findFirstAdoptedDevice(t)
	return fmt.Sprintf(`
data "terrifi_device" "test" {
  mac = %q
}
`, dev.MAC)
}

func testAccDeviceDataSourceByNameConfig(t *testing.T) string {
	t.Helper()
	dev := findFirstAdoptedDevice(t)
	if dev.Name == "" {
		t.Skip("first adopted device has no name — skipping name lookup test")
	}
	return fmt.Sprintf(`
data "terrifi_device" "test" {
  name = %q
}
`, dev.Name)
}

func testAccDeviceDataSourceWithClientDeviceConfig(t *testing.T, clientMAC string) string {
	t.Helper()
	dev := findFirstAdoptedDevice(t)
	return fmt.Sprintf(`
data "terrifi_device" "test" {
  mac = %q
}

resource "terrifi_client_device" "test" {
  mac          = %q
  name         = "tfacc-ds-fixedap"
  fixed_ap_mac = data.terrifi_device.test.mac
}
`, dev.MAC, clientMAC)
}

// testAccGetClient builds an authenticated Client for use in test helpers.
// It reuses the same env-var-based config as the provider itself.
func testAccGetClient(t *testing.T) *Client {
	t.Helper()
	cfg := ClientConfigFromEnv()
	client, err := NewClient(t.Context(), cfg)
	if err != nil {
		t.Fatalf("failed to create test client: %s", err)
	}
	return client
}

// ---------------------------------------------------------------------------
// Validation tests (no controller needed)
// ---------------------------------------------------------------------------

func TestAccDeviceDataSource_validationBothNameAndMAC(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "terrifi_device" "test" {
  name = "some-device"
  mac  = "aa:bb:cc:dd:ee:ff"
}
`,
				ExpectError: regexp.MustCompile(`(?i)Invalid Attribute Combination`),
			},
		},
	})
}

func TestAccDeviceDataSource_validationNeitherNameNorMAC(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "terrifi_device" "test" {
}
`,
				ExpectError: regexp.MustCompile(`(?i)Invalid Attribute Combination`),
			},
		},
	})
}

func TestAccDeviceDataSource_validationInvalidMAC(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "terrifi_device" "test" {
  mac = "not-a-mac"
}
`,
				ExpectError: regexp.MustCompile(`must be a valid MAC address`),
			},
		},
	})
}
