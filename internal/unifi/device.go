package unifi

// Device is the local controller API contract.
const DeviceStateConnected = 1

type Device struct {
	State                      int64                `json:"state"`
	ID                         string               `json:"_id,omitempty"`
	SiteID                     string               `json:"site_id,omitempty"`
	Hidden                     bool                 `json:"attr_hidden,omitempty"`
	MAC                        string               `json:"mac,omitempty"`
	Adopted                    bool                 `json:"adopted"`
	ConfigNetwork              *DeviceConfigNetwork `json:"config_network,omitempty"`
	Disabled                   bool                 `json:"disabled,omitempty"`
	IP                         string               `json:"ip,omitempty"`
	LedOverride                string               `json:"led_override,omitempty"`
	LedOverrideColor           string               `json:"led_override_color,omitempty"`
	LedOverrideColorBrightness *int64               `json:"led_override_color_brightness,omitempty"`
	Locked                     bool                 `json:"locked,omitempty"`
	Model                      string               `json:"model,omitempty"`
	Name                       string               `json:"name,omitempty"`
	OutdoorModeOverride        string               `json:"outdoor_mode_override,omitempty"`
	PoeMode                    string               `json:"poe_mode,omitempty"`
	RadioTable                 []DeviceRadioTable   `json:"radio_table,omitempty"`
	SnmpContact                string               `json:"snmp_contact,omitempty"`
	SnmpLocation               string               `json:"snmp_location,omitempty"`
	Type                       string               `json:"type,omitempty"`
	Volume                     *int64               `json:"volume,omitempty"`
}
