package generate

import (
	"github.com/ubiquiti-community/go-unifi/unifi"
)

// ClientGroupBlocks generates import + resource blocks for client groups.
func ClientGroupBlocks(groups []unifi.ClientGroup) []ResourceBlock {
	blocks := make([]ResourceBlock, 0, len(groups))
	for _, g := range groups {
		block := ResourceBlock{
			ResourceType: "terrifi_client_group",
			ResourceName: ToTerraformName(g.Name),
			ImportID:     g.ID,
		}

		block.Attributes = append(block.Attributes, Attr{Key: "name", Value: HCLString(g.Name)})

		blocks = append(blocks, block)
	}
	DeduplicateNames(blocks)
	return blocks
}
