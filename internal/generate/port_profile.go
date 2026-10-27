package generate

import (
	"github.com/alexklibisz/terrifi/internal/unifi"
)

const networkIDComment = "TODO: reference the corresponding terrifi_network resource"

// PortProfileBlocks generates import + resource blocks for port profiles.
// lanNetworkIDs are the site's LAN networks, used to express a custom tagged
// VLAN selection as tagged_network_ids (as the UI shows it) rather than the
// stored deny-list. Attributes equal to the resource's schema defaults are
// omitted. Settings the resource doesn't manage are not emitted.
func PortProfileBlocks(profiles []unifi.PortProfile, lanNetworkIDs []string) []ResourceBlock {
	blocks := make([]ResourceBlock, 0, len(profiles))
	for _, p := range profiles {
		block := ResourceBlock{
			ResourceType: "terrifi_port_profile",
			ResourceName: ToTerraformName(p.Name),
			ImportID:     p.ID,
		}
		add := func(key, value, comment string) {
			block.Attributes = append(block.Attributes, Attr{Key: key, Value: value, Comment: comment})
		}
		addBool := func(key string, v *bool, def bool) {
			if v != nil && *v != def {
				add(key, HCLBool(*v), "")
			}
		}

		add("name", HCLString(p.Name), "")

		native := ""
		if p.NativeNetworkID != nil {
			native = *p.NativeNetworkID
		}
		if native != "" {
			add("native_network_id", HCLString(native), networkIDComment)
		}
		if p.TaggedVLANMgmt != "" && p.TaggedVLANMgmt != "auto" {
			add("tagged_vlan_mode", HCLString(unifi.TaggedVLANModeFromAPI(p.TaggedVLANMgmt)), "")
		}
		if p.TaggedVLANMgmt == "custom" {
			tagged := unifi.TaggedFromExcluded(lanNetworkIDs, p.ExcludedNetworkIDs, native)
			add("tagged_network_ids", HCLStringList(tagged), networkIDComment)
		}
		if p.PoeMode != nil && *p.PoeMode != "" && *p.PoeMode != "auto" {
			add("poe_mode", HCLString(*p.PoeMode), "")
		}
		addBool("port_isolation_enabled", p.Isolation, false)
		addBool("lldpmed_enabled", p.LldpmedEnabled, true)

		blocks = append(blocks, block)
	}
	DeduplicateNames(blocks)
	return blocks
}
