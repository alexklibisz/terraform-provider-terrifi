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
		addBool("flow_control_enabled", p.FlowControlEnabled, true)
		addBool("ptp_enabled", p.PTPEnabled, true)
		addBool("port_isolation_enabled", p.Isolation, false)
		addBool("stp_enabled", p.StpPortMode, true)
		addBool("bpdu_guard_enabled", p.StpBpduGuardEnabled, false)
		addBool("loop_protection_enabled", p.PortKeepaliveEnabled, false)
		addBool("eee_enabled", p.EEEEnabled, false)
		addBool("lldpmed_enabled", p.LldpmedEnabled, true)
		if p.LinkDebounceAuto != nil && !*p.LinkDebounceAuto {
			ms := int64(0)
			if p.LinkDebounce != nil {
				ms = *p.LinkDebounce
			}
			add("link_debounce_ms", HCLInt64(ms), "")
		}

		if unit := unifi.StormControlTypeFromAPI(p.StormctrlType); unit != "percentage" {
			add("storm_control_type", HCLString(unit), "")
		}
		for _, c := range []struct {
			name        string
			enabled     *bool
			level, rate *int64
		}{
			{"broadcast", p.StormctrlBcastEnabled, p.StormctrlBcastLevel, p.StormctrlBcastRate},
			{"multicast", p.StormctrlMcastEnabled, p.StormctrlMcastLevel, p.StormctrlMcastRate},
			{"unknown_unicast", p.StormctrlUcastEnabled, p.StormctrlUcastLevel, p.StormctrlUcastRate},
		} {
			v := c.level
			if p.StormctrlType == "rate" {
				v = c.rate
			}
			if c.enabled != nil && *c.enabled && v != nil {
				add("storm_control_"+c.name, HCLInt64(*v), "")
			}
		}

		blocks = append(blocks, block)
	}
	DeduplicateNames(blocks)
	return blocks
}
