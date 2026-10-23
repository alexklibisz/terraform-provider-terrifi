package generate

import (
	"fmt"
	"strings"

	"github.com/alexklibisz/terrifi/internal/unifi"
)

const networkIDComment = "TODO: reference the corresponding terrifi_network resource"

// PortProfileBlocks generates import + resource blocks for port profiles.
// lanNetworkIDs are the site's LAN networks, used to express a custom tagged
// VLAN selection as tagged_network_ids (as the UI shows it) rather than the
// stored deny-list. Attributes equal to the resource's schema defaults are
// omitted, and values the controller keeps but ignores (speed and duplex
// under autoneg, a voice network without LLDP-MED, disabled limits) are
// skipped, mirroring how the resource reads them.
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
			add("tagged_vlan_mgmt", HCLString(p.TaggedVLANMgmt), "")
		}
		if p.TaggedVLANMgmt == "custom" {
			tagged := unifi.TaggedFromExcluded(lanNetworkIDs, p.ExcludedNetworkIDs, native)
			add("tagged_network_ids", HCLStringList(tagged), networkIDComment)
		}
		lldpmed := p.LldpmedEnabled == nil || *p.LldpmedEnabled
		if lldpmed && p.VoiceNetworkID != nil && *p.VoiceNetworkID != "" {
			add("voice_network_id", HCLString(*p.VoiceNetworkID), networkIDComment)
		}

		if p.PoeMode != nil && *p.PoeMode != "" && *p.PoeMode != "auto" {
			add("poe_mode", HCLString(*p.PoeMode), "")
		}
		if p.Autoneg != nil && !*p.Autoneg {
			add("autoneg", HCLBool(false), "")
			if p.Speed != nil {
				add("speed", HCLInt64(*p.Speed), "")
			}
			addBool("full_duplex", p.FullDuplex, false)
		}

		if p.EgressRateLimitKbpsEnabled != nil && *p.EgressRateLimitKbpsEnabled && p.EgressRateLimitKbps != nil {
			add("egress_rate_limit_kbps", HCLInt64(*p.EgressRateLimitKbps), "")
		}
		addBool("flow_control_enabled", p.FlowControlEnabled, true)
		addBool("ptp_enabled", p.PTPEnabled, true)
		addBool("isolation", p.Isolation, false)
		addBool("stp_enabled", p.StpPortMode, true)
		addBool("stp_uplink", p.StpUplink, false)
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

		addBool("port_security_enabled", p.PortSecurityEnabled, false)
		if len(p.PortSecurityMACAddress) > 0 {
			add("port_security_mac_addresses", HCLStringList(p.PortSecurityMACAddress), "")
		}
		if p.Dot1XCtrl != "" && p.Dot1XCtrl != "force_authorized" {
			add("dot1x_ctrl", HCLString(p.Dot1XCtrl), "")
		}

		if sc := stormControlHCL(&p); sc != "" {
			add("storm_control", sc, "")
		}

		blocks = append(blocks, block)
	}
	DeduplicateNames(blocks)
	return blocks
}

// stormControlHCL renders the storm_control object attribute, or "" when
// storm control is disabled for every traffic class.
func stormControlHCL(p *unifi.PortProfile) string {
	scType := p.StormctrlType
	if scType == "" {
		scType = "level"
	}
	var parts []string
	if scType != "level" {
		parts = append(parts, fmt.Sprintf("type = %s", HCLString(scType)))
	}
	classes := []struct {
		name        string
		enabled     *bool
		level, rate *int64
	}{
		{"broadcast", p.StormctrlBcastEnabled, p.StormctrlBcastLevel, p.StormctrlBcastRate},
		{"multicast", p.StormctrlMcastEnabled, p.StormctrlMcastLevel, p.StormctrlMcastRate},
		{"unicast", p.StormctrlUcastEnabled, p.StormctrlUcastLevel, p.StormctrlUcastRate},
	}
	anyEnabled := false
	for _, c := range classes {
		if c.enabled == nil || !*c.enabled {
			continue
		}
		v := c.level
		if scType == "rate" {
			v = c.rate
		}
		if v == nil {
			continue
		}
		anyEnabled = true
		parts = append(parts, fmt.Sprintf("%s = %s", c.name, HCLInt64(*v)))
	}
	if !anyEnabled {
		return ""
	}
	return "{ " + strings.Join(parts, ", ") + " }"
}
