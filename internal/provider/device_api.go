package provider

// Local CRUD methods for the v1 device endpoint. These shadow the promoted
// go-unifi methods on *Client and use the local internal/unifi.Device type
// instead, removing the SDK dependency for this resource — see issue #157.
//
// Two non-obvious controller behaviors are encoded here:
//
//  1. UpdateDevice sends a minimal payload of just the fields the provider
//     manages. Round-tripping the full Device struct on every update would
//     pull in stat counters and runtime fields that differ between successive
//     reads, and the controller rejects such payloads with "not found: type=".
//     deviceUpdatePayload is the authoritative shape we send.
//
//  2. DeviceRadioTable.TxPower and .Channel arrive as JSON numbers on some
//     hardware (Dream Router 7, USW Flex XG, certain APs reporting dBm
//     directly) and as JSON strings on others. The local DeviceRadioTable
//     type declares both as Go strings; fixRadioTableBytes wraps any numeric
//     occurrences in quotes before unmarshal so the decode succeeds across
//     all controller versions.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/alexklibisz/terrifi/internal/unifi"
)

// deviceUpdatePayload contains only the fields the device resource manages.
// By sending only these fields, we avoid the SDK's diff mechanism which
// produces spurious changes from stat counters and other runtime fields.
type deviceUpdatePayload struct {
	Name                       string                      `json:"name"`
	LedOverride                string                      `json:"led_override,omitempty"`
	LedOverrideColor           string                      `json:"led_override_color,omitempty"`
	LedOverrideColorBrightness *int64                      `json:"led_override_color_brightness,omitempty"`
	OutdoorModeOverride        string                      `json:"outdoor_mode_override,omitempty"`
	Locked                     bool                        `json:"locked"`
	Disabled                   bool                        `json:"disabled"`
	SnmpContact                string                      `json:"snmp_contact,omitempty"`
	SnmpLocation               string                      `json:"snmp_location,omitempty"`
	Volume                     *int64                      `json:"volume,omitempty"`
	ConfigNetwork              *deviceConfigNetworkPayload `json:"config_network,omitempty"`
	RadioTable                 []unifi.DeviceRadioTable    `json:"radio_table,omitempty"`
}

type deviceConfigNetworkPayload struct {
	Type    string `json:"type"`
	IP      string `json:"ip,omitempty"`
	Netmask string `json:"netmask,omitempty"`
	Gateway string `json:"gateway,omitempty"`
	DNS1    string `json:"dns1,omitempty"`
	DNS2    string `json:"dns2,omitempty"`
}

// applyPlannedToRadioEntry updates only the user-configured fields in an
// existing DeviceRadioTable entry, preserving all other controller-managed fields.
func applyPlannedToRadioEntry(rt *unifi.DeviceRadioTable, planned deviceRadioSettingsModel) {
	if !planned.Channel.IsNull() && !planned.Channel.IsUnknown() {
		rt.Channel = planned.Channel.ValueString()
	}
	if !planned.ChannelWidth.IsNull() && !planned.ChannelWidth.IsUnknown() {
		v := planned.ChannelWidth.ValueInt64()
		rt.Ht = &v
	}
	if !planned.TransmitPower.IsNull() && !planned.TransmitPower.IsUnknown() {
		rt.TxPower = planned.TransmitPower.ValueString()
	}
	if !planned.TransmitPowerMode.IsNull() && !planned.TransmitPowerMode.IsUnknown() {
		rt.TxPowerMode = planned.TransmitPowerMode.ValueString()
	}
	if !planned.MinRssiEnabled.IsNull() && !planned.MinRssiEnabled.IsUnknown() {
		rt.MinRssiEnabled = planned.MinRssiEnabled.ValueBool()
	}
	if !planned.MinRssi.IsNull() && !planned.MinRssi.IsUnknown() {
		v := planned.MinRssi.ValueInt64()
		rt.MinRssi = &v
	}
}

// UpdateDevice sends a PUT to the UniFi REST API with only the managed fields.
// This bypasses the SDK's UpdateDevice which uses a fragile diff mechanism.
func (c *Client) UpdateDevice(ctx context.Context, site string, id string, m *deviceResourceModel) error {
	payload := deviceUpdatePayload{}

	if !m.Name.IsNull() && !m.Name.IsUnknown() {
		payload.Name = m.Name.ValueString()
	}

	if !m.LedEnabled.IsNull() && !m.LedEnabled.IsUnknown() {
		if m.LedEnabled.ValueBool() {
			payload.LedOverride = "on"
		} else {
			payload.LedOverride = "off"
		}
	}

	if !m.LedColor.IsNull() && !m.LedColor.IsUnknown() {
		payload.LedOverrideColor = m.LedColor.ValueString()
	}

	if !m.LedBrightness.IsNull() && !m.LedBrightness.IsUnknown() {
		v := m.LedBrightness.ValueInt64()
		payload.LedOverrideColorBrightness = &v
	}

	if !m.OutdoorModeOverride.IsNull() && !m.OutdoorModeOverride.IsUnknown() {
		payload.OutdoorModeOverride = m.OutdoorModeOverride.ValueString()
	}

	if !m.Locked.IsNull() && !m.Locked.IsUnknown() {
		payload.Locked = m.Locked.ValueBool()
	}

	if !m.Disabled.IsNull() && !m.Disabled.IsUnknown() {
		payload.Disabled = m.Disabled.ValueBool()
	}

	if !m.SnmpContact.IsNull() && !m.SnmpContact.IsUnknown() {
		payload.SnmpContact = m.SnmpContact.ValueString()
	}

	if !m.SnmpLocation.IsNull() && !m.SnmpLocation.IsUnknown() {
		payload.SnmpLocation = m.SnmpLocation.ValueString()
	}

	if !m.Volume.IsNull() && !m.Volume.IsUnknown() {
		v := m.Volume.ValueInt64()
		payload.Volume = &v
	}

	if m.ConfigNetwork != nil {
		payload.ConfigNetwork = &deviceConfigNetworkPayload{
			Type:    m.ConfigNetwork.Type.ValueString(),
			IP:      m.ConfigNetwork.IP.ValueString(),
			Netmask: m.ConfigNetwork.Netmask.ValueString(),
			Gateway: m.ConfigNetwork.Gateway.ValueString(),
			DNS1:    m.ConfigNetwork.DNS1.ValueString(),
			DNS2:    m.ConfigNetwork.DNS2.ValueString(),
		}
	}

	plannedByRadio := map[string]*deviceRadioSettingsModel{}
	if m.Radio24 != nil {
		plannedByRadio["ng"] = m.Radio24
	}
	if m.Radio5 != nil {
		plannedByRadio["na"] = m.Radio5
	}
	if m.Radio6 != nil {
		plannedByRadio["6e"] = m.Radio6
	}

	if len(plannedByRadio) > 0 {
		// Read-modify-write: fetch current device to preserve non-managed radio
		// fields and entries for radios not mentioned in the plan.
		existing, err := c.GetDevice(ctx, site, id)
		if err != nil {
			return fmt.Errorf("reading device for radio settings merge: %w", err)
		}

		radioTable := make([]unifi.DeviceRadioTable, len(existing.RadioTable))
		copy(radioTable, existing.RadioTable)
		for i := range radioTable {
			if planned, ok := plannedByRadio[radioTable[i].Radio]; ok {
				applyPlannedToRadioEntry(&radioTable[i], *planned)
			}
		}
		payload.RadioTable = radioTable
	}

	// Use the v1 REST API endpoint for device updates.
	var resp struct {
		Meta struct {
			RC  string `json:"rc"`
			Msg string `json:"msg,omitempty"`
		} `json:"meta"`
		Data []unifi.Device `json:"data"`
	}

	url := fmt.Sprintf("%s%s/api/s/%s/rest/device/%s", c.BaseURL, c.APIPath, site, id)
	err := c.doV2Request(ctx, http.MethodPut, url, payload, &resp)
	if err != nil {
		return err
	}

	if resp.Meta.RC != "ok" {
		return fmt.Errorf("controller returned rc=%s msg=%s", resp.Meta.RC, resp.Meta.Msg)
	}

	return nil
}

// txPowerNumberRE matches a JSON `"tx_power": <number>` pair so we can wrap the
// number in quotes before handing the bytes to the SDK's UnmarshalJSON, which
// only accepts a string. The controller emits the field as a number for some
// devices (notably APs reporting dBm as an integer).
var txPowerNumberRE = regexp.MustCompile(`"tx_power"\s*:\s*(-?\d+(?:\.\d+)?)`)

// channelNumberRE matches a JSON `"channel": <number>` pair so we can wrap the
// number in quotes before handing the bytes to the SDK's UnmarshalJSON, which
// only accepts a string. Some controllers (e.g. Dream Router 7, USW Flex XG)
// emit the field as a number.
var channelNumberRE = regexp.MustCompile(`"channel"\s*:\s*(-?\d+(?:\.\d+)?)`)

// fixRadioTableBytes coerces JSON numeric tx_power and channel values into
// strings so the SDK's DeviceRadioTable.UnmarshalJSON does not fail. See the
// file-level TODO for details.
func fixRadioTableBytes(b []byte) []byte {
	b = txPowerNumberRE.ReplaceAll(b, []byte(`"tx_power":"$1"`))
	b = channelNumberRE.ReplaceAll(b, []byte(`"channel":"$1"`))
	return b
}

// deviceListResponse is the v1 stat/device envelope. We unmarshal into
// json.RawMessage first so we can patch the bytes before decoding into
// unifi.Device. See fixRadioTableBytes.
type deviceListResponse struct {
	Meta struct {
		RC  string `json:"rc"`
		Msg string `json:"msg,omitempty"`
	} `json:"meta"`
	Data []json.RawMessage `json:"data"`
}

// fetchDeviceList performs a GET against the v1 stat/device endpoint, applies
// the radio_table coercions, and decodes each entry into unifi.Device.
func (c *Client) fetchDeviceList(ctx context.Context, site, suffix string) ([]unifi.Device, error) {
	url := fmt.Sprintf("%s%s/api/s/%s/stat/device", c.BaseURL, c.APIPath, site)
	if suffix != "" {
		url += "/" + suffix
	}

	var raw json.RawMessage
	if err := c.doV2Request(ctx, http.MethodGet, url, nil, &raw); err != nil {
		return nil, err
	}

	var envelope deviceListResponse
	if err := json.Unmarshal(fixRadioTableBytes(raw), &envelope); err != nil {
		return nil, fmt.Errorf("decoding device envelope: %w", err)
	}

	if envelope.Meta.RC != "" && envelope.Meta.RC != "ok" {
		return nil, fmt.Errorf("controller returned rc=%s msg=%s", envelope.Meta.RC, envelope.Meta.Msg)
	}

	devices := make([]unifi.Device, 0, len(envelope.Data))
	for i, item := range envelope.Data {
		var d unifi.Device
		if err := json.Unmarshal(item, &d); err != nil {
			return nil, fmt.Errorf("decoding device[%d]: %w", i, err)
		}
		devices = append(devices, d)
	}
	return devices, nil
}

// ListDevice returns all devices for a site, working around the SDK's
// tx_power and channel unmarshal bugs.
func (c *Client) ListDevice(ctx context.Context, site string) ([]unifi.Device, error) {
	return c.fetchDeviceList(ctx, site, "")
}

// GetDeviceByMAC fetches a device by MAC, working around the SDK's tx_power
// and channel unmarshal bugs. The controller's stat/device/<mac> endpoint
// returns a single device when a MAC is supplied.
func (c *Client) GetDeviceByMAC(ctx context.Context, site, mac string) (*unifi.Device, error) {
	devices, err := c.fetchDeviceList(ctx, site, strings.ToLower(mac))
	if err != nil {
		return nil, err
	}
	if len(devices) == 0 {
		return nil, &unifi.NotFoundError{}
	}
	d := devices[0]
	return &d, nil
}

// GetDevice fetches a device by its controller ID. The v1 stat/device endpoint
// only supports lookup by MAC, so we list and filter, matching the SDK's
// behavior while applying our tx_power and channel workarounds.
func (c *Client) GetDevice(ctx context.Context, site, id string) (*unifi.Device, error) {
	devices, err := c.ListDevice(ctx, site)
	if err != nil {
		return nil, err
	}
	for i := range devices {
		if devices[i].ID == id {
			return &devices[i], nil
		}
	}
	return nil, &unifi.NotFoundError{}
}
