package provider

// TODO(go-unifi): This file is a workaround for the SDK's UpdateDevice method.
// The SDK's UpdateDevice (1) re-fetches the device by MAC internally, (2) diffs
// against the target using JSON marshal/unmarshal, and (3) sends only changed
// fields. When the re-fetched device differs from the target in unexpected ways
// (e.g., stat counters that changed between reads, or structural differences
// between list vs. single-device endpoints), the diff includes noise that
// confuses the controller, causing it to return empty results (our "not found:
// type=" error).
//
// This workaround sends a minimal PUT payload containing only the fields we
// actually manage, avoiding the fragile diff mechanism entirely.
//
// Fix needed in SDK: UpdateDevice's diff approach should be more robust, or
// provide a way to do a simple field-level PUT without the diff.

import (
	"context"
	"fmt"
	"net/http"

	"github.com/ubiquiti-community/go-unifi/unifi"
)

// deviceUpdatePayload contains only the fields the device resource manages.
// By sending only these fields, we avoid the SDK's diff mechanism which
// produces spurious changes from stat counters and other runtime fields.
type deviceUpdatePayload struct {
	Name                       string `json:"name"`
	LedOverride                string `json:"led_override,omitempty"`
	LedOverrideColor           string `json:"led_override_color,omitempty"`
	LedOverrideColorBrightness *int64 `json:"led_override_color_brightness,omitempty"`
	OutdoorModeOverride        string `json:"outdoor_mode_override,omitempty"`
	Locked                     bool   `json:"locked"`
	Disabled                   bool   `json:"disabled"`
	SnmpContact                string `json:"snmp_contact,omitempty"`
	SnmpLocation               string `json:"snmp_location,omitempty"`
	Volume                     *int64 `json:"volume,omitempty"`
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
