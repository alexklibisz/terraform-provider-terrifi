package unifi

// Device mirrors entries at /api/s/{site}/stat/device. Only fields the
// provider reads or writes are typed; everything else (port_table,
// uplink, link_speed, stat counters, etc.) is dropped on decode.
type Device struct {
	ID     string `json:"_id,omitempty"`
	SiteID string `json:"site_id,omitempty"`

	MAC                        string               `json:"mac,omitempty"`
	Name                       string               `json:"name,omitempty"`
	IP                         string               `json:"ip,omitempty"`
	Model                      string               `json:"model,omitempty"`
	Type                       string               `json:"type,omitempty"`
	State                      DeviceState          `json:"state"`
	Adopted                    bool                 `json:"adopted"`
	Disabled                   bool                 `json:"disabled,omitempty"`
	Locked                     bool                 `json:"locked,omitempty"`
	LedOverride                string               `json:"led_override,omitempty"`
	LedOverrideColor           string               `json:"led_override_color,omitempty"`
	LedOverrideColorBrightness *int64               `json:"led_override_color_brightness,omitempty"`
	OutdoorModeOverride        string               `json:"outdoor_mode_override,omitempty"`
	SnmpContact                string               `json:"snmp_contact,omitempty"`
	SnmpLocation               string               `json:"snmp_location,omitempty"`
	Volume                     *int64               `json:"volume,omitempty"`
	ConfigNetwork              *DeviceConfigNetwork `json:"config_network,omitempty"`
	RadioTable                 []DeviceRadioTable   `json:"radio_table,omitempty"`
}

// DeviceConfigNetwork is the network configuration nested in a Device.
type DeviceConfigNetwork struct {
	DNS1    string `json:"dns1,omitempty"`
	DNS2    string `json:"dns2,omitempty"`
	Gateway string `json:"gateway,omitempty"`
	IP      string `json:"ip,omitempty"`
	Netmask string `json:"netmask,omitempty"`
	Type    string `json:"type,omitempty"`
}

// DeviceRadioTable is one radio entry in a Device.RadioTable. The controller
// emits Channel and TxPower as numbers on some hardware and as strings on
// others; the wire bytes are coerced to strings in device_api.go's
// fixRadioTableBytes before unmarshal.
type DeviceRadioTable struct {
	Channel        string `json:"channel,omitempty"`
	Ht             *int64 `json:"ht,omitempty"`
	MinRssi        *int64 `json:"min_rssi,omitempty"`
	MinRssiEnabled bool   `json:"min_rssi_enabled,omitempty"`
	Name           string `json:"name,omitempty"`
	Radio          string `json:"radio,omitempty"`
	TxPower        string `json:"tx_power,omitempty"`
	TxPowerMode    string `json:"tx_power_mode,omitempty"`
}

// DeviceState is the controller's adoption/connection state enum.
type DeviceState int64

const (
	DeviceStateUnknown          DeviceState = 0
	DeviceStateConnected        DeviceState = 1
	DeviceStatePending          DeviceState = 2
	DeviceStateFirmwareMismatch DeviceState = 3
	DeviceStateUpgrading        DeviceState = 4
	DeviceStateProvisioning     DeviceState = 5
	DeviceStateHeartbeatMissed  DeviceState = 6
	DeviceStateAdopting         DeviceState = 7
	DeviceStateDeleting         DeviceState = 8
	DeviceStateInformError      DeviceState = 9
	DeviceStateAdoptFailed      DeviceState = 10
	DeviceStateIsolated         DeviceState = 11
)
