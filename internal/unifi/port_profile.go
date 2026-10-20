package unifi

import "fmt"

// PortProfile mirrors the controller's representation of a switch port
// profile at /api/s/{site}/rest/portconf. Only fields the provider manages are
// typed; the controller returns more (qos_profile, multicast router settings,
// dot1x_idle_timeout, lldpmed_notify_enabled, ...), which are ignored on read
// and preserved on update by a read-modify-write of the raw object (see
// UpdatePortProfile).
//
// Every optional field is a pointer or omitempty so that the provider decides
// exactly what goes on the wire. The go-unifi SDK's PortProfile serializes
// bools like autoneg and stp_port_mode without omitempty, so a partially
// populated struct silently sends `false` for settings the controller defaults
// to `true`.
//
// Field-to-UI mapping was established against UniFi OS 5.1 / Network 10.x by
// changing one UI control at a time and diffing the stored object.
type PortProfile struct {
	ID     string `json:"_id,omitempty"`
	SiteID string `json:"site_id,omitempty"`

	Name string `json:"name,omitempty"`
	// UI "Advanced: Auto/Manual". The advanced settings (everything from
	// storm control down) only apply under "manual".
	SettingPreference string `json:"setting_preference,omitempty"`

	// VLANs.
	//
	// Forward is derived by the controller: "all" (native = Default network,
	// tagged auto), "native" (tagged block_all), otherwise "customize".
	Forward         string  `json:"forward,omitempty"`
	NativeNetworkID *string `json:"native_networkconf_id,omitempty"` // "" = None
	TaggedVLANMgmt  string  `json:"tagged_vlan_mgmt,omitempty"`      // auto|block_all|custom
	// The UI shows an allow-list of tagged networks but stores this
	// deny-list: every LAN network except the tagged and native ones.
	ExcludedNetworkIDs []string `json:"excluded_networkconf_ids"`
	VoiceNetworkID     *string  `json:"voice_networkconf_id,omitempty"` // "" = none

	// Link.
	PoeMode    *string `json:"poe_mode,omitempty"` // auto|off (UI "Auto PoE")
	Autoneg    *bool   `json:"autoneg,omitempty"`
	Speed      *int64  `json:"speed,omitempty"`       // left stale when autoneg is re-enabled
	FullDuplex *bool   `json:"full_duplex,omitempty"` // left stale when autoneg is re-enabled

	// Advanced.
	EgressRateLimitKbpsEnabled *bool  `json:"egress_rate_limit_kbps_enabled,omitempty"`
	EgressRateLimitKbps        *int64 `json:"egress_rate_limit_kbps,omitempty"`
	FlowControlEnabled         *bool  `json:"flow_control_enabled,omitempty"`
	PTPEnabled                 *bool  `json:"precision_time_protocol_enabled,omitempty"`
	Isolation                  *bool  `json:"isolation,omitempty"`
	StpPortMode                *bool  `json:"stp_port_mode,omitempty"`
	StpUplink                  *bool  `json:"stp_uplink,omitempty"`
	StpBpduGuardEnabled        *bool  `json:"stp_bpdu_guard_enabled,omitempty"`
	PortKeepaliveEnabled       *bool  `json:"port_keepalive_enabled,omitempty"` // UI "Non-STP loop protection"
	EEEEnabled                 *bool  `json:"eee_enabled,omitempty"`
	LldpmedEnabled             *bool  `json:"lldpmed_enabled,omitempty"`
	// UI link debounce: Auto = (true, ignored), Off = (false, 0),
	// Custom = (false, 100–5000 ms in steps of 100).
	LinkDebounceAuto *bool  `json:"link_debounce_auto,omitempty"`
	LinkDebounce     *int64 `json:"link_debounce,omitempty"`

	// Port security. UI "MAC address filter" and "802.1X control".
	PortSecurityEnabled    *bool    `json:"port_security_enabled,omitempty"`
	PortSecurityMACAddress []string `json:"port_security_mac_address"`
	Dot1XCtrl              string   `json:"dot1x_ctrl,omitempty"` // auto|force_authorized|force_unauthorized|mac_based|multi_host ("Multi-auth")

	// Storm control. There is no master switch: storm control is on when any
	// traffic class is enabled. Type "level" is the UI's "percentage".
	StormctrlType         string `json:"stormctrl_type,omitempty"` // level|rate
	StormctrlBcastEnabled *bool  `json:"stormctrl_bcast_enabled,omitempty"`
	StormctrlBcastLevel   *int64 `json:"stormctrl_bcast_level,omitempty"`
	StormctrlBcastRate    *int64 `json:"stormctrl_bcast_rate,omitempty"`
	StormctrlMcastEnabled *bool  `json:"stormctrl_mcast_enabled,omitempty"`
	StormctrlMcastLevel   *int64 `json:"stormctrl_mcast_level,omitempty"`
	StormctrlMcastRate    *int64 `json:"stormctrl_mcast_rate,omitempty"`
	StormctrlUcastEnabled *bool  `json:"stormctrl_ucast_enabled,omitempty"` // UI "unknown unicast"
	StormctrlUcastLevel   *int64 `json:"stormctrl_ucast_level,omitempty"`
	StormctrlUcastRate    *int64 `json:"stormctrl_ucast_rate,omitempty"`
}

// ExcludedFromTagged converts the UI's tagged allow-list into the stored
// deny-list: every LAN network that is neither tagged nor the native network.
// It returns an error naming any tagged ID that isn't a LAN network, since
// such an ID could never round-trip.
func ExcludedFromTagged(lan, tagged []string, native string) ([]string, error) {
	lanSet := make(map[string]bool, len(lan))
	for _, id := range lan {
		lanSet[id] = true
	}
	keep := map[string]bool{native: true}
	for _, id := range tagged {
		if !lanSet[id] {
			return nil, fmt.Errorf("tagged network %q is not a LAN network on this site", id)
		}
		keep[id] = true
	}
	excluded := []string{}
	for _, id := range lan {
		if !keep[id] {
			excluded = append(excluded, id)
		}
	}
	return excluded, nil
}

// TaggedFromExcluded is the inverse of ExcludedFromTagged. A LAN network
// created after the profile was last written and missing from the deny-list
// shows up here as tagged, so Terraform reports it as drift instead of it
// silently being carried on the port.
func TaggedFromExcluded(lan, excluded []string, native string) []string {
	skip := map[string]bool{native: true}
	for _, id := range excluded {
		skip[id] = true
	}
	tagged := []string{}
	for _, id := range lan {
		if !skip[id] {
			tagged = append(tagged, id)
		}
	}
	return tagged
}
