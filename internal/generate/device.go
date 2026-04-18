package generate

import (
	"fmt"
	"strings"

	"github.com/ubiquiti-community/go-unifi/unifi"
)

// DeviceBlocks generates import + resource blocks for adopted UniFi devices.
func DeviceBlocks(devices []unifi.Device) []ResourceBlock {
	blocks := make([]ResourceBlock, 0, len(devices))
	for _, d := range devices {
		if !d.Adopted {
			continue
		}

		name := d.Name
		if name == "" {
			name = d.MAC
		}
		block := ResourceBlock{
			ResourceType: "terrifi_device",
			ResourceName: ToTerraformName(name),
			ImportID:     d.MAC,
		}

		block.Attributes = append(block.Attributes, Attr{Key: "mac", Value: HCLString(d.MAC)})

		if d.Name != "" {
			block.Attributes = append(block.Attributes, Attr{Key: "name", Value: HCLString(d.Name)})
		}
		switch d.LedOverride {
		case "on":
			block.Attributes = append(block.Attributes, Attr{Key: "led_enabled", Value: HCLBool(true)})
		case "off":
			block.Attributes = append(block.Attributes, Attr{Key: "led_enabled", Value: HCLBool(false)})
		}
		if d.LedOverrideColor != "" {
			block.Attributes = append(block.Attributes, Attr{Key: "led_color", Value: HCLString(d.LedOverrideColor)})
		}
		if d.LedOverrideColorBrightness != nil {
			block.Attributes = append(block.Attributes, Attr{Key: "led_brightness", Value: HCLInt64(*d.LedOverrideColorBrightness)})
		}
		if d.OutdoorModeOverride != "" {
			block.Attributes = append(block.Attributes, Attr{Key: "outdoor_mode_override", Value: HCLString(d.OutdoorModeOverride)})
		}
		if d.Locked {
			block.Attributes = append(block.Attributes, Attr{Key: "locked", Value: HCLBool(true)})
		}
		if d.Disabled {
			block.Attributes = append(block.Attributes, Attr{Key: "disabled", Value: HCLBool(true)})
		}
		if d.SnmpContact != "" {
			block.Attributes = append(block.Attributes, Attr{Key: "snmp_contact", Value: HCLString(d.SnmpContact)})
		}
		if d.SnmpLocation != "" {
			block.Attributes = append(block.Attributes, Attr{Key: "snmp_location", Value: HCLString(d.SnmpLocation)})
		}
		if d.Volume != nil {
			block.Attributes = append(block.Attributes, Attr{Key: "volume", Value: HCLInt64(*d.Volume)})
		}
		if v := formatDeviceConfigNetwork(d.ConfigNetwork); v != "" {
			block.Attributes = append(block.Attributes, Attr{Key: "config_network", Value: v})
		}

		blocks = append(blocks, block)
	}
	DeduplicateNames(blocks)
	return blocks
}

// formatDeviceConfigNetwork renders the device's config_network as a nested
// HCL object literal (for use as an Attribute value). Returns an empty string
// when the device has no explicit configuration, so the attribute is omitted.
func formatDeviceConfigNetwork(cn *unifi.DeviceConfigNetwork) string {
	if cn == nil || cn.Type == "" {
		return ""
	}
	var lines []string
	lines = append(lines, fmt.Sprintf("    type    = %s", HCLString(cn.Type)))
	if cn.Type == "static" {
		if cn.IP != "" {
			lines = append(lines, fmt.Sprintf("    ip      = %s", HCLString(cn.IP)))
		}
		if cn.Netmask != "" {
			lines = append(lines, fmt.Sprintf("    netmask = %s", HCLString(cn.Netmask)))
		}
		if cn.Gateway != "" {
			lines = append(lines, fmt.Sprintf("    gateway = %s", HCLString(cn.Gateway)))
		}
		if cn.DNS1 != "" {
			lines = append(lines, fmt.Sprintf("    dns1    = %s", HCLString(cn.DNS1)))
		}
		if cn.DNS2 != "" {
			lines = append(lines, fmt.Sprintf("    dns2    = %s", HCLString(cn.DNS2)))
		}
	}
	return "{\n" + strings.Join(lines, "\n") + "\n  }"
}
