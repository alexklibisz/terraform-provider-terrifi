package unifi

import "fmt"

// PortProfile mirrors the controller's representation of a switch port
// profile at /api/s/{site}/rest/portconf. Only fields the provider manages are
// typed; the controller returns more (link speed, port
// security, 802.1X, voice VLAN, qos_profile, ...), which are ignored on read
// and preserved on update by a read-modify-write of the raw object (see
// UpdatePortProfile).
//
// Every optional field is a pointer or omitempty so that the provider decides
// exactly what goes on the wire. The go-unifi SDK's PortProfile serializes
// bools without omitempty, so a partially populated struct silently sends
// `false` for settings the controller defaults to `true`.
//
// Field-to-UI mapping was established against UniFi OS 5.1 / Network 10.x by
// changing one UI control at a time and diffing the stored object.
type PortProfile struct {
	ID     string `json:"_id,omitempty"`
	SiteID string `json:"site_id,omitempty"`

	Name string `json:"name,omitempty"`
	// UI "Advanced: Auto/Manual". The advanced settings (port isolation and
	// LLDP-MED among the managed ones) only apply under "manual".
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

	PoeMode              *string `json:"poe_mode,omitempty"` // auto|off (UI "Auto PoE")
	Isolation            *bool   `json:"isolation,omitempty"`
	LldpmedEnabled       *bool   `json:"lldpmed_enabled,omitempty"`
	FlowControlEnabled   *bool   `json:"flow_control_enabled,omitempty"`
	PTPEnabled           *bool   `json:"precision_time_protocol_enabled,omitempty"`
	StpPortMode          *bool   `json:"stp_port_mode,omitempty"`
	StpBpduGuardEnabled  *bool   `json:"stp_bpdu_guard_enabled,omitempty"`
	PortKeepaliveEnabled *bool   `json:"port_keepalive_enabled,omitempty"` // UI "Non-STP loop protection"
	EEEEnabled           *bool   `json:"eee_enabled,omitempty"`
	// Auto ignores the stored debounce; Off uses 0; Custom uses 100–5000 ms.
	LinkDebounceAuto *bool  `json:"link_debounce_auto,omitempty"`
	LinkDebounce     *int64 `json:"link_debounce,omitempty"`

	// Each enabled class uses a percentage level or packets-per-second rate.
	// Disabled classes may retain stale thresholds, which reads must ignore.
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

// TaggedVLANModeToAPI converts the public VLAN mode to the controller value.
func TaggedVLANModeToAPI(mode string) string {
	if mode == "allow_all" {
		return "auto"
	}
	return mode
}

// TaggedVLANModeFromAPI converts the controller value to the public VLAN mode.
func TaggedVLANModeFromAPI(mode string) string {
	if mode == "" || mode == "auto" {
		return "allow_all"
	}
	return mode
}

// StormControlTypeToAPI converts the public threshold unit to the wire value.
func StormControlTypeToAPI(unit string) string {
	switch unit {
	case "percentage":
		return "level"
	case "packets_per_second":
		return "rate"
	}
	return unit
}

// StormControlTypeFromAPI converts the wire value to the public threshold unit.
func StormControlTypeFromAPI(unit string) string {
	switch unit {
	case "", "level":
		return "percentage"
	case "rate":
		return "packets_per_second"
	}
	return unit
}
