package unifi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

type DeviceUpdate struct {
	Name                       string                     `json:"name"`
	LedOverride                string                     `json:"led_override,omitempty"`
	LedOverrideColor           string                     `json:"led_override_color,omitempty"`
	LedOverrideColorBrightness *int64                     `json:"led_override_color_brightness,omitempty"`
	OutdoorModeOverride        string                     `json:"outdoor_mode_override,omitempty"`
	Locked                     bool                       `json:"locked"`
	Disabled                   bool                       `json:"disabled"`
	SnmpContact                string                     `json:"snmp_contact,omitempty"`
	SnmpLocation               string                     `json:"snmp_location,omitempty"`
	Volume                     *int64                     `json:"volume,omitempty"`
	ConfigNetwork              *DeviceConfigNetworkUpdate `json:"config_network,omitempty"`
	RadioTable                 []DeviceRadioTable         `json:"radio_table,omitempty"`
}

type DeviceConfigNetworkUpdate struct {
	Type    string `json:"type"`
	IP      string `json:"ip,omitempty"`
	Netmask string `json:"netmask,omitempty"`
	Gateway string `json:"gateway,omitempty"`
	DNS1    string `json:"dns1,omitempty"`
	DNS2    string `json:"dns2,omitempty"`
}

func (c *APIClient) UpdateDevice(ctx context.Context, site, id string, payload *DeviceUpdate) error {
	// Use the v1 REST API endpoint for device updates.
	var resp struct {
		Meta struct {
			RC  string `json:"rc"`
			Msg string `json:"msg,omitempty"`
		} `json:"meta"`
		Data []Device `json:"data"`
	}

	url := fmt.Sprintf("%s%s/api/s/%s/rest/device/%s", c.BaseURL, c.APIPath, site, id)
	err := c.Do(ctx, http.MethodPut, url, payload, &resp)
	if err != nil {
		return err
	}

	if resp.Meta.RC != "ok" {
		return fmt.Errorf("controller returned rc=%s msg=%s", resp.Meta.RC, resp.Meta.Msg)
	}

	return nil
}

var txPowerNumberRE = regexp.MustCompile(`"tx_power"\s*:\s*(-?\d+(?:\.\d+)?)`)

var channelNumberRE = regexp.MustCompile(`"channel"\s*:\s*(-?\d+(?:\.\d+)?)`)

func fixRadioTableBytes(b []byte) []byte {
	b = txPowerNumberRE.ReplaceAll(b, []byte(`"tx_power":"$1"`))
	b = channelNumberRE.ReplaceAll(b, []byte(`"channel":"$1"`))
	return b
}

// deviceListResponse is the v1 stat/device envelope. We unmarshal into
// json.RawMessage first so we can patch the bytes before decoding into
// Device. See fixRadioTableBytes.
type deviceListResponse struct {
	Meta struct {
		RC  string `json:"rc"`
		Msg string `json:"msg,omitempty"`
	} `json:"meta"`
	Data []json.RawMessage `json:"data"`
}

// fetchDeviceList performs a GET against the v1 stat/device endpoint, applies
// the radio_table coercions, and decodes each entry into Device.
func (c *APIClient) fetchDeviceList(ctx context.Context, site, suffix string) ([]Device, error) {
	url := fmt.Sprintf("%s%s/api/s/%s/stat/device", c.BaseURL, c.APIPath, site)
	if suffix != "" {
		url += "/" + suffix
	}

	var raw json.RawMessage
	if err := c.Do(ctx, http.MethodGet, url, nil, &raw); err != nil {
		return nil, err
	}

	var envelope deviceListResponse
	if err := json.Unmarshal(fixRadioTableBytes(raw), &envelope); err != nil {
		return nil, fmt.Errorf("decoding device envelope: %w", err)
	}

	if envelope.Meta.RC != "" && envelope.Meta.RC != "ok" {
		return nil, fmt.Errorf("controller returned rc=%s msg=%s", envelope.Meta.RC, envelope.Meta.Msg)
	}

	devices := make([]Device, 0, len(envelope.Data))
	for i, item := range envelope.Data {
		var d Device
		if err := json.Unmarshal(item, &d); err != nil {
			return nil, fmt.Errorf("decoding device[%d]: %w", i, err)
		}
		devices = append(devices, d)
	}
	return devices, nil
}

func (c *APIClient) ListDevice(ctx context.Context, site string) ([]Device, error) {
	return c.fetchDeviceList(ctx, site, "")
}

func (c *APIClient) GetDeviceByMAC(ctx context.Context, site, mac string) (*Device, error) {
	devices, err := c.fetchDeviceList(ctx, site, strings.ToLower(mac))
	if err != nil {
		return nil, err
	}
	if len(devices) == 0 {
		return nil, &NotFoundError{}
	}
	d := devices[0]
	return &d, nil
}

func (c *APIClient) GetDevice(ctx context.Context, site, id string) (*Device, error) {
	devices, err := c.ListDevice(ctx, site)
	if err != nil {
		return nil, err
	}
	for i := range devices {
		if devices[i].ID == id {
			return &devices[i], nil
		}
	}
	return nil, &NotFoundError{}
}
