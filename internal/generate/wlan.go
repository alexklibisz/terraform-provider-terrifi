package generate

import (
	"github.com/ubiquiti-community/go-unifi/unifi"
)

// WLANBlocks generates import + resource blocks for WLANs.
func WLANBlocks(wlans []unifi.WLAN) []ResourceBlock {
	blocks := make([]ResourceBlock, 0, len(wlans))
	for _, w := range wlans {
		block := ResourceBlock{
			ResourceType: "terrifi_wlan",
			ResourceName: ToTerraformName(w.Name),
			ImportID:     w.ID,
		}

		block.Attributes = append(block.Attributes, Attr{Key: "name", Value: HCLString(w.Name)})
		block.Attributes = append(block.Attributes, Attr{
			Key:     "passphrase",
			Value:   HCLString("REPLACE_ME"),
			Comment: "SENSITIVE: not returned by API",
		})
		block.Attributes = append(block.Attributes, Attr{
			Key:     "network_id",
			Value:   HCLString(w.NetworkID),
			Comment: "TODO: find and reference corresponding terrifi_network resource",
		})

		if !w.Enabled {
			block.Attributes = append(block.Attributes, Attr{Key: "enabled", Value: HCLBool(false)})
		}
		if w.WLANBand != "" && w.WLANBand != "both" {
			block.Attributes = append(block.Attributes, Attr{Key: "wifi_band", Value: HCLString(w.WLANBand)})
		}
		if w.Security != "" && w.Security != "wpapsk" {
			block.Attributes = append(block.Attributes, Attr{Key: "security", Value: HCLString(w.Security)})
		}
		if w.HideSSID {
			block.Attributes = append(block.Attributes, Attr{Key: "hide_ssid", Value: HCLBool(true)})
		}
		if w.WPAMode != "" && w.WPAMode != "wpa2" {
			block.Attributes = append(block.Attributes, Attr{Key: "wpa_mode", Value: HCLString(w.WPAMode)})
		}
		if w.WPA3Support {
			block.Attributes = append(block.Attributes, Attr{Key: "wpa3_support", Value: HCLBool(true)})
		}
		if w.WPA3Transition {
			block.Attributes = append(block.Attributes, Attr{Key: "wpa3_transition", Value: HCLBool(true)})
		}

		blocks = append(blocks, block)
	}
	DeduplicateNames(blocks)
	return blocks
}
