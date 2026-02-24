package generate

import (
	"github.com/ubiquiti-community/go-unifi/unifi"
)

// ClientDeviceBlocks generates import + resource blocks for client devices.
func ClientDeviceBlocks(clients []unifi.Client) []ResourceBlock {
	blocks := make([]ResourceBlock, 0, len(clients))
	for _, c := range clients {
		name := c.Name
		if name == "" {
			name = c.MAC
		}
		block := ResourceBlock{
			ResourceType: "terrifi_client_device",
			ResourceName: ToTerraformName(name),
			ImportID:     c.ID,
		}

		block.Attributes = append(block.Attributes, Attr{Key: "mac", Value: HCLString(c.MAC)})

		if c.Name != "" {
			block.Attributes = append(block.Attributes, Attr{Key: "name", Value: HCLString(c.Name)})
		}
		if c.Note != "" {
			block.Attributes = append(block.Attributes, Attr{Key: "note", Value: HCLString(c.Note)})
		}
		if c.UseFixedIP && c.FixedIP != "" {
			block.Attributes = append(block.Attributes, Attr{Key: "fixed_ip", Value: HCLString(c.FixedIP)})
			block.Attributes = append(block.Attributes, Attr{
				Key:     "network_id",
				Value:   HCLString(c.NetworkID),
				Comment: "TODO: find and reference corresponding terrifi_network resource",
			})
		}
		if c.LocalDNSRecordEnabled && c.LocalDNSRecord != "" {
			block.Attributes = append(block.Attributes, Attr{Key: "local_dns_record", Value: HCLString(c.LocalDNSRecord)})
		}
		if c.VirtualNetworkOverrideEnabled != nil && *c.VirtualNetworkOverrideEnabled && c.VirtualNetworkOverrideID != "" {
			block.Attributes = append(block.Attributes, Attr{
				Key:     "network_override_id",
				Value:   HCLString(c.VirtualNetworkOverrideID),
				Comment: "TODO: find and reference corresponding terrifi_network resource",
			})
		}
		if c.Blocked != nil && *c.Blocked {
			block.Attributes = append(block.Attributes, Attr{Key: "blocked", Value: HCLBool(true)})
		}

		blocks = append(blocks, block)
	}
	DeduplicateNames(blocks)
	return blocks
}
