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
// cached inline VLAN keys on profiled entries are intentionally ignored; an
// unrepresentable forward mode still makes any entry unsupported. Unsupported
// configurations return empty settings; callers must exclude the whole port
// from import using UnsupportedPortVLANReason, even when it has a name.
func PortVLANSettings(e map[string]json.RawMessage) (native, mode string) {
	if UnsupportedPortVLANReason(e) != "" {
		return "", ""
	}
	native, mode, _ = portVLANSettings(e)
	return
}

// UnsupportedPortVLANReason identifies inline VLAN configurations that cannot
// round-trip through the resource schema. Such ports must remain unowned when
// importing, rather than becoming name-only ports that reset on destroy.
func UnsupportedPortVLANReason(e map[string]json.RawMessage) string {
	// Forwarding modes such as disabled cannot be represented by the schema,
	// even when the VLAN fields themselves look supported. Inspect the raw
	// field before profile handling can hide cached inline settings. customize
	// can still represent a profile or explicit VLAN mode; inspect those below.
	var forwardMode string
	_ = json.Unmarshal(e["forward"], &forwardMode)
	if forwardMode != "" && forwardMode != "all" && forwardMode != "native" && forwardMode != "customize" {
		return fmt.Sprintf("forward mode %q cannot be represented", forwardMode)
	}
	// A voice VLAN on an inline port has no schema attribute. Importing the
	// port would hide it from state while Terraform owns the override.
	var profile, voice string
	_ = json.Unmarshal(e["portconf_id"], &profile)
	_ = json.Unmarshal(e["voice_networkconf_id"], &voice)
	if profile == "" && voice != "" {
		return "inline voice VLAN settings cannot be represented"
	}
	native, mode, forward := portVLANSettings(e)
	if native == "" && mode == "" && (forward == "" || forward == "all") {
		return ""
	}
	if mode != "allow_all" && mode != "block_all" {
		if mode != "" {
			return fmt.Sprintf("inline VLAN mode %q is unsupported", mode)
		}
		return fmt.Sprintf("inline VLAN settings with forward mode %q cannot be represented", forward)
	}
	if native == "" {
		return "inline VLAN settings without an explicit native network cannot be represented"
	}
	return ""
}

func portVLANSettings(e map[string]json.RawMessage) (native, mode, forward string) {
	var profile string
	_ = json.Unmarshal(e["portconf_id"], &profile)
	if profile != "" {
		return "", "", ""
	}
	_ = json.Unmarshal(e["native_networkconf_id"], &native)
	_ = json.Unmarshal(e["tagged_vlan_mgmt"], &mode)
	_ = json.Unmarshal(e["forward"], &forward)
	if mode == "auto" {
		mode = "allow_all"
	}
	if mode == "" && native != "" {
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
