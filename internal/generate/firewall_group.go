package generate

import (
	"github.com/ubiquiti-community/go-unifi/unifi"
)

// FirewallGroupBlocks generates import + resource blocks for firewall groups.
func FirewallGroupBlocks(groups []unifi.FirewallGroup) []ResourceBlock {
	blocks := make([]ResourceBlock, 0, len(groups))
	for _, g := range groups {
		block := ResourceBlock{
			ResourceType: "terrifi_firewall_group",
			ResourceName: ToTerraformName(g.Name),
			ImportID:     g.ID,
		}

		block.Attributes = append(block.Attributes, Attr{Key: "name", Value: HCLString(g.Name)})
		block.Attributes = append(block.Attributes, Attr{Key: "type", Value: HCLString(g.GroupType)})
		block.Attributes = append(block.Attributes, Attr{Key: "members", Value: HCLStringList(g.GroupMembers)})

		blocks = append(blocks, block)
	}
	DeduplicateNames(blocks)
	return blocks
}
