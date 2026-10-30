package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// These cases verify controller persistence on virtual devices. They do not
// establish model support or physical effects. Keep them off hardware: static
// management addressing and radio changes require equipment-specific choices.
func TestAccDeviceResource_simulatedSettings(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC not set")
	}
	if os.Getenv("TERRIFI_ACC_TARGET") == "hardware" {
		t.Skip("virtual-device configuration tests")
	}
	preCheck(t)
	requireAdoptedAP(t)
	ap := findFirstAdoptedAP(t)
	for _, tc := range []struct {
		name, settings string
		attrs          map[string]string
	}{
		{"ledColor", `led_color = "#0000ff"`, map[string]string{"led_color": "#0000ff"}},
		{"ledBrightness", `led_brightness = 50`, map[string]string{"led_brightness": "50"}},
		{"outdoorMode", `outdoor_mode_override = "on"`, map[string]string{"outdoor_mode_override": "on"}},
		{"volume", `volume = 50`, map[string]string{"volume": "50"}},
		{"staticManagement", `config_network = {
 type = "static"
 ip = "192.0.2.10"
 netmask = "255.255.255.0"
 gateway = "192.0.2.1"
 dns1 = "192.0.2.53"
 dns2 = "192.0.2.54"
}`, map[string]string{"config_network.type": "static", "config_network.ip": "192.0.2.10", "config_network.netmask": "255.255.255.0", "config_network.gateway": "192.0.2.1", "config_network.dns1": "192.0.2.53", "config_network.dns2": "192.0.2.54"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := fmt.Sprintf("resource \"terrifi_device\" \"test\" {\n mac = %q\n %s\n}\n", ap.MAC, tc.settings)
			checks := []resource.TestCheckFunc{}
			for attr, value := range tc.attrs {
				checks = append(checks, resource.TestCheckResourceAttr("terrifi_device.test", attr, value))
			}
			resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
				{Config: config, Check: resource.ComposeTestCheckFunc(checks...)},
				{Config: config, PlanOnly: true},
			}})
		})
	}
}

func TestAccDeviceResource_simulatedRadioSettings(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC not set")
	}
	if os.Getenv("TERRIFI_ACC_TARGET") == "hardware" {
		t.Skip("virtual-device configuration tests")
	}
	preCheck(t)
	client := testAccGetClient(t)
	devices, err := client.ApiClient.ListDevice(t.Context(), "default")
	if err != nil {
		t.Fatal(err)
	}
	for _, band := range []struct {
		attr, radio string
		width       int
	}{{"radio_24", "ng", 20}, {"radio_5", "na", 80}, {"radio_6", "6e", 80}} {
		t.Run(band.attr, func(t *testing.T) {
			mac := ""
			for _, dev := range devices {
				if dev.Adopted && dev.Type == "uap" {
					for _, radio := range dev.RadioTable {
						if radio.Radio == band.radio {
							mac = dev.MAC
							break
						}
					}
				}
				if mac != "" {
					break
				}
			}
			if mac == "" {
				t.Skipf("simulator has no adopted AP with %s radio", band.radio)
			}
			config := fmt.Sprintf(`resource "terrifi_device" "test" {
 mac = %q
 %s = {
 channel = "auto"
 channel_width = %d
 transmit_power_mode = "custom"
 transmit_power = "10"
 min_rssi_enabled = true
 min_rssi = -80
 }
}`, mac, band.attr, band.width)
			checks := []resource.TestCheckFunc{}
			for attr, value := range map[string]string{"channel": "auto", "channel_width": fmt.Sprint(band.width), "transmit_power_mode": "custom", "transmit_power": "10", "min_rssi_enabled": "true", "min_rssi": "-80"} {
				checks = append(checks, resource.TestCheckResourceAttr("terrifi_device.test", band.attr+"."+attr, value))
			}
			resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{{Config: config, Check: resource.ComposeTestCheckFunc(checks...)}, {Config: config, PlanOnly: true}}})
		})
	}
}
