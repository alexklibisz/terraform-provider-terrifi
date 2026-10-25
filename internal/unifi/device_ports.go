package unifi

import (
	"encoding/json"
	"fmt"
)

// DevicePorts is the part of a device (/api/s/{site}/stat/device/{mac}) that
// port assignment needs. port_overrides entries stay raw so that fields the
// provider doesn't manage (inline per-port settings) survive a
// read-modify-write untouched.
type DevicePorts struct {
	ID            string                       `json:"_id"`
	MAC           string                       `json:"mac"`
	Name          string                       `json:"name"`
	Model         string                       `json:"model"`
	PortOverrides []map[string]json.RawMessage `json:"port_overrides"`
	PortTable     []DevicePort                 `json:"port_table"`
	// Gateways only: the LAN/WAN role of each interface. "WAN", "WAN2", ...
	// mark WAN ports; assigning one strips portconf_id from its override.
	EthernetOverrides []EthernetOverride `json:"ethernet_overrides"`
}

// DevicePort is a port_table entry: a physical port and its interface name
// (eth0 for port 1 on a gateway).
type DevicePort struct {
	PortIdx  int64  `json:"port_idx"`
	Ifname   string `json:"ifname"`
	IsUplink bool   `json:"is_uplink"`
}

// EthernetOverride is a gateway interface's LAN/WAN role.
type EthernetOverride struct {
	Ifname       string `json:"ifname"`
	Networkgroup string `json:"networkgroup"`
}

// ParsePortOverride reads the fields the provider manages from a raw
// port_overrides entry. name and portconfID are "" when absent.
func ParsePortOverride(e map[string]json.RawMessage) (idx int64, name, portconfID string, err error) {
	if err := json.Unmarshal(e["port_idx"], &idx); err != nil {
		return 0, "", "", fmt.Errorf("port_overrides entry has a bad port_idx: %s", e["port_idx"])
	}
	_ = json.Unmarshal(e["name"], &name)
	_ = json.Unmarshal(e["portconf_id"], &portconfID)
	return idx, name, portconfID, nil
}

// PortVLANSettings reads inline VLAN settings. Profiles own their VLANs, so
// cached inline keys on profiled entries are intentionally ignored.
func PortVLANSettings(e map[string]json.RawMessage) (native, mode string) {
	var profile string
	_ = json.Unmarshal(e["portconf_id"], &profile)
	if profile != "" {
		return "", ""
	}
	_ = json.Unmarshal(e["native_networkconf_id"], &native)
	_ = json.Unmarshal(e["tagged_vlan_mgmt"], &mode)
	if mode == "auto" {
		mode = "allow_all"
	}
	if mode == "" && native != "" {
		var forward string
		_ = json.Unmarshal(e["forward"], &forward)
		switch forward {
		case "native":
			mode = "block_all"
		case "all":
			mode = "allow_all"
		}
	}
	return
}

// PortNetwork is the network metadata needed for validating native VLANs.
type PortNetwork struct {
	ID          string `json:"_id"`
	Purpose     string `json:"purpose"`
	VLANEnabled bool   `json:"vlan_enabled"`
}
