package unifi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

// normalizeNumbers accepts controller versions that encode numeric settings as
// JSON strings. Empty strings represent an unset optional setting.
func normalizeNumbers(data []byte, fields ...string) ([]byte, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return nil, err
	}
	for _, field := range fields {
		value := bytes.TrimSpace(object[field])
		if len(value) == 0 || value[0] != '"' {
			continue
		}
		var text string
		if err := json.Unmarshal(value, &text); err != nil {
			return nil, err
		}
		if text == "" {
			object[field] = json.RawMessage("null")
			continue
		}
		if _, err := strconv.ParseInt(text, 10, 64); err != nil {
			return nil, fmt.Errorf("%s: %w", field, err)
		}
		object[field] = json.RawMessage(text)
	}
	return json.Marshal(object)
}

func (dst *DNSRecord) UnmarshalJSON(data []byte) error {
	type wire DNSRecord
	normalized, err := normalizeNumbers(data, "port", "priority", "ttl", "weight")
	if err != nil {
		return err
	}
	if err := json.Unmarshal(normalized, (*wire)(dst)); err != nil {
		return err
	}
	return nil
}

func (dst *Network) UnmarshalJSON(data []byte) error {
	type wire Network
	normalized, err := normalizeNumbers(data, "dhcpd_leasetime", "ipv6_ra_preferred_lifetime", "priority", "vlan")
	if err != nil {
		return err
	}
	if err := json.Unmarshal(normalized, (*wire)(dst)); err != nil {
		return err
	}
	return nil
}

func (dst *Device) UnmarshalJSON(data []byte) error {
	type wire Device
	normalized, err := normalizeNumbers(data, "state", "led_override_color_brightness", "volume")
	if err != nil {
		return err
	}
	if err := json.Unmarshal(normalized, (*wire)(dst)); err != nil {
		return err
	}
	return nil
}

func (dst *DeviceRadioTable) UnmarshalJSON(data []byte) error {
	type wire DeviceRadioTable
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return err
	}
	for _, field := range []string{"tx_power", "channel"} {
		value := bytes.TrimSpace(object[field])
		if len(value) > 0 && value[0] != '"' && string(value) != "null" {
			object[field], _ = json.Marshal(string(value))
		}
	}
	var err error
	data, err = json.Marshal(object)
	if err != nil {
		return err
	}
	normalized, err := normalizeNumbers(data, "ht", "min_rssi")
	if err != nil {
		return err
	}
	if err := json.Unmarshal(normalized, (*wire)(dst)); err != nil {
		return err
	}
	dst.raw = object
	return nil
}

// MarshalJSON retains radio settings outside the provider's write contract.
func (src DeviceRadioTable) MarshalJSON() ([]byte, error) {
	type wire DeviceRadioTable
	b, err := json.Marshal(wire(src))
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(b, &fields); err != nil {
		return nil, err
	}
	result := make(map[string]json.RawMessage, len(src.raw)+len(fields))
	for k, v := range src.raw {
		result[k] = v
	}
	for k, v := range fields {
		result[k] = v
	}
	// false must override a previously enabled minimum RSSI setting.
	result["min_rssi_enabled"], _ = json.Marshal(src.MinRssiEnabled)
	return json.Marshal(result)
}

func (dst *FirewallPolicy) UnmarshalJSON(data []byte) error {
	type wire FirewallPolicy
	normalized, err := normalizeNumbers(data, "index")
	if err != nil {
		return err
	}
	if err := json.Unmarshal(normalized, (*wire)(dst)); err != nil {
		return err
	}
	return nil
}

func (dst *FirewallPolicySource) UnmarshalJSON(data []byte) error {
	type wire FirewallPolicySource
	normalized, err := normalizeNumbers(data, "port")
	if err != nil {
		return err
	}
	if err := json.Unmarshal(normalized, (*wire)(dst)); err != nil {
		return err
	}
	return nil
}

func (dst *FirewallPolicyDestination) UnmarshalJSON(data []byte) error {
	type wire FirewallPolicyDestination
	normalized, err := normalizeNumbers(data, "port")
	if err != nil {
		return err
	}
	if err := json.Unmarshal(normalized, (*wire)(dst)); err != nil {
		return err
	}
	return nil
}
