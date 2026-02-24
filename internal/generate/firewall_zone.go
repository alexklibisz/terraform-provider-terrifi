package generate

import (
	"github.com/ubiquiti-community/go-unifi/unifi"
)

// FirewallZoneBlocks generates import + resource blocks for firewall zones.
func FirewallZoneBlocks(zones []unifi.FirewallZone) []ResourceBlock {
	blocks := make([]ResourceBlock, 0, len(zones))
	for _, z := range zones {
		block := ResourceBlock{
			ResourceType: "terrifi_firewall_zone",
			ResourceName: ToTerraformName(z.Name),
			ImportID:     z.ID,
		}

		block.Attributes = append(block.Attributes, Attr{Key: "name", Value: HCLString(z.Name)})

		if len(z.NetworkIDs) > 0 {
			block.Attributes = append(block.Attributes, Attr{
				Key:     "network_ids",
				Value:   HCLStringList(z.NetworkIDs),
				Comment: "TODO: find and reference corresponding terrifi_network resources",
			})
		}

		blocks = append(blocks, block)
	}
	DeduplicateNames(blocks)
	return blocks
}
