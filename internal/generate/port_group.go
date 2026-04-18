package generate

import (
	"github.com/ubiquiti-community/go-unifi/unifi"
)

// PortGroupBlocks generates import + resource blocks for port groups.
func PortGroupBlocks(groups []unifi.FirewallGroup) []ResourceBlock {
	blocks := make([]ResourceBlock, 0, len(groups))
	for _, g := range groups {
		if g.GroupType != "port-group" {
			continue
		}
		block := ResourceBlock{
			ResourceType: "terrifi_port_group",
			ResourceName: ToTerraformName(g.Name),
			ImportID:     g.ID,
		}

		block.Attributes = append(block.Attributes, Attr{Key: "name", Value: HCLString(g.Name)})
		block.Attributes = append(block.Attributes, Attr{Key: "ports", Value: HCLStringList(g.GroupMembers)})

		blocks = append(blocks, block)
	}
	DeduplicateNames(blocks)
	return blocks
}
