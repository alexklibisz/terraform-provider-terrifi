package unifi

// DeviceConfigNetwork is the local controller API contract.
type DeviceConfigNetwork struct {
	DNS1    string `json:"dns1,omitempty"`
	DNS2    string `json:"dns2,omitempty"`
	Gateway string `json:"gateway,omitempty"`
	IP      string `json:"ip,omitempty"`
	Netmask string `json:"netmask,omitempty"`
	Type    string `json:"type,omitempty"`
}
