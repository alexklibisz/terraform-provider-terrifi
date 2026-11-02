package unifi

import "encoding/json"

// DeviceRadioTable is the local controller API contract.
type DeviceRadioTable struct {
	raw            map[string]json.RawMessage
	Channel        string `json:"channel,omitempty"`
	Ht             *int64 `json:"ht,omitempty"`
	MinRssi        *int64 `json:"min_rssi,omitempty"`
	MinRssiEnabled bool   `json:"min_rssi_enabled,omitempty"`
	Name           string `json:"name,omitempty"`
	Radio          string `json:"radio,omitempty"`
	TxPower        string `json:"tx_power,omitempty"`
	TxPowerMode    string `json:"tx_power_mode,omitempty"`
}
