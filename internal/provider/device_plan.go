package provider

import (
	"context"
	"fmt"

	"github.com/alexklibisz/terrifi/internal/unifi"
)

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

func (c *Client) UpdateDevice(ctx context.Context, site string, id string, m *deviceResourceModel) error {
	payload := unifi.DeviceUpdate{}

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
		payload.ConfigNetwork = &unifi.DeviceConfigNetworkUpdate{
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

	return c.APIClient.UpdateDevice(ctx, site, id, &payload)
}
