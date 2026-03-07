package generate

import (
	"sort"

	"github.com/ubiquiti-community/go-unifi/unifi"
)

// FirewallPolicyOrderBlocks generates import + resource blocks for firewall
// policy ordering. It groups policies by (source_zone_id, destination_zone_id)
// pairs and generates one block per zone pair with the ordered policy IDs.
func FirewallPolicyOrderBlocks(policies []*unifi.FirewallPolicy) []ResourceBlock {
	// Group non-predefined policies by zone pair.
	type zonePair struct {
		sourceZoneID string
		destZoneID   string
	}
	type indexedPolicy struct {
		id    string
		name  string
		index int64
	}

	groups := make(map[zonePair][]indexedPolicy)
	for _, p := range policies {
		if p.Predefined {
			continue
		}
		if p.Source == nil || p.Destination == nil {
			continue
		}
		key := zonePair{
			sourceZoneID: p.Source.ZoneID,
			destZoneID:   p.Destination.ZoneID,
		}
		idx := int64(0)
		if p.Index != nil {
			idx = *p.Index
		}
		groups[key] = append(groups[key], indexedPolicy{id: p.ID, name: p.Name, index: idx})
	}

	var blocks []ResourceBlock
	for zp, pols := range groups {
		// Sort by index to get the correct order.
		sort.Slice(pols, func(i, j int) bool {
			return pols[i].index < pols[j].index
		})

		ids := make([]string, len(pols))
		for i, p := range pols {
			ids[i] = p.id
		}

		// Build a descriptive name from the first policy's name.
		name := "order"
		if len(pols) > 0 {
			name = ToTerraformName(pols[0].name) + "_and_others"
		}

		block := ResourceBlock{
			Comment:      "TODO: replace policy ID literals with references to terrifi_firewall_policy resources",
			ResourceType: "terrifi_firewall_policy_order",
			ResourceName: name,
			ImportID:     zp.sourceZoneID + ":" + zp.destZoneID,
		}

		block.Attributes = append(block.Attributes, Attr{
			Key:     "source_zone_id",
			Value:   HCLString(zp.sourceZoneID),
			Comment: "TODO: find and reference corresponding terrifi_firewall_zone resource",
		})
		block.Attributes = append(block.Attributes, Attr{
			Key:     "destination_zone_id",
			Value:   HCLString(zp.destZoneID),
			Comment: "TODO: find and reference corresponding terrifi_firewall_zone resource",
		})
		block.Attributes = append(block.Attributes, Attr{
			Key:   "policy_ids",
			Value: HCLStringList(ids),
		})

		blocks = append(blocks, block)
	}

	DeduplicateNames(blocks)
	return blocks
}
