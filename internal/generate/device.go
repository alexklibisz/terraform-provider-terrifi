package generate

import (
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

		blocks = append(blocks, block)
	}
	DeduplicateNames(blocks)
	return blocks
}
