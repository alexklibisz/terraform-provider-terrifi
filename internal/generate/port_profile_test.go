package generate

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alexklibisz/terrifi/internal/unifi"
)

func ptr[T any](v T) *T { return &v }

var testLAN = []string{"net-default", "net-a", "net-b", "net-c"}

// uiDefaults returns a profile as the UI stores one with every setting left
// at its default (UniFi OS 5.1 / Network 10.x).
func uiDefaults(id, name string) unifi.PortProfile {
	return unifi.PortProfile{
		ID:                id,
		Name:              name,
		SettingPreference: "auto",
		Forward:           "all",
		NativeNetworkID:   ptr("net-default"),
		TaggedVLANMgmt:    "auto",
		PoeMode:           ptr("auto"),
		Isolation:         ptr(false),
		LldpmedEnabled:    ptr(true),
	}
}

func TestPortProfileBlocks_UIDefaults(t *testing.T) {
	blocks := PortProfileBlocks([]unifi.PortProfile{uiDefaults("prof1", "Example")}, testLAN)
	require.Len(t, blocks, 1)
	b := blocks[0]
	assert.Equal(t, "terrifi_port_profile", b.ResourceType)
	assert.Equal(t, "example", b.ResourceName)
	assert.Equal(t, "prof1", b.ImportID)

	// Only name and the UI's default native network: everything else is a
	// schema default.
	assert.Equal(t, map[string]string{
		"name":              `"Example"`,
		"native_network_id": `"net-default"`,
	}, attrMapFromBlock(b))
}

func TestPortProfileBlocks_Trunk(t *testing.T) {
	// Stored as the UI stores "native A, tagged C": Default and B excluded.
	p := uiDefaults("prof2", "Trunk - Example")
	p.NativeNetworkID = ptr("net-a")
	p.TaggedVLANMgmt = "custom"
	p.ExcludedNetworkIDs = []string{"net-default", "net-b"}
	p.PoeMode = ptr("off")

	attrs := attrMapFromBlock(PortProfileBlocks([]unifi.PortProfile{p}, testLAN)[0])
	assert.Equal(t, map[string]string{
		"name":               `"Trunk - Example"`,
		"native_network_id":  `"net-a"`,
		"tagged_vlan_mode":   `"custom"`,
		"tagged_network_ids": `["net-c"]`,
		"poe_mode":           `"off"`,
	}, attrs)
}

func TestPortProfileBlocks_NonDefaults(t *testing.T) {
	p := uiDefaults("prof3", "Everything")
	p.NativeNetworkID = ptr("")
	p.TaggedVLANMgmt = "block_all"
	p.PoeMode = ptr("off")
	p.Isolation = ptr(true)
	p.LldpmedEnabled = ptr(false)

	attrs := attrMapFromBlock(PortProfileBlocks([]unifi.PortProfile{p}, testLAN)[0])
	assert.Equal(t, map[string]string{
		"name":                   `"Everything"`,
		"tagged_vlan_mode":       `"block_all"`,
		"poe_mode":               `"off"`,
		"port_isolation_enabled": "true",
		"lldpmed_enabled":        "false",
	}, attrs)
}

// Settings the resource doesn't manage must not be emitted, whatever their
// value: the generated config would not be valid.
func TestPortProfileBlocks_UnmanagedSettingsOmitted(t *testing.T) {
	var p unifi.PortProfile
	require.NoError(t, json.Unmarshal([]byte(`{
		"_id": "prof4", "name": "Unmanaged", "native_networkconf_id": "",
		"autoneg": false, "speed": 100, "full_duplex": true, "voice_networkconf_id": "net-b",
		"stp_uplink": true, "dot1x_ctrl": "multi_host", "port_security_enabled": true
	}`), &p))
	attrs := attrMapFromBlock(PortProfileBlocks([]unifi.PortProfile{p}, testLAN)[0])
	assert.Equal(t, map[string]string{"name": `"Unmanaged"`}, attrs)
}

func TestPortProfileBlocks_RendersValidHCL(t *testing.T) {
	p := uiDefaults("prof6", "Isolated")
	p.Isolation = ptr(true)

	var buf bytes.Buffer
	require.NoError(t, WriteBlocks(&buf, PortProfileBlocks([]unifi.PortProfile{p}, testLAN)))
	out := buf.String()
	assert.Contains(t, out, `to = terrifi_port_profile.isolated`)
	assert.Contains(t, out, `native_network_id = "net-default" # TODO: reference the corresponding terrifi_network resource`)
	assert.Contains(t, out, `port_isolation_enabled = true`)
}

func TestPortProfileBlocks_RestoredSettings(t *testing.T) {
	for _, unit := range []string{"level", "rate"} {
		t.Run(unit, func(t *testing.T) {
			p := uiDefaults("restored", "Restored")
			p.NativeNetworkID = nil
			p.FlowControlEnabled = ptr(false)
			p.PTPEnabled = ptr(false)
			p.StpPortMode = ptr(false)
			p.StpBpduGuardEnabled = ptr(true)
			p.PortKeepaliveEnabled = ptr(true)
			p.EEEEnabled = ptr(true)
			p.LinkDebounceAuto = ptr(false)
			p.LinkDebounce = ptr(int64(0))
			p.StormctrlType = unit
			p.StormctrlBcastEnabled = ptr(true)
			p.StormctrlBcastLevel = ptr(int64(0))
			p.StormctrlBcastRate = ptr(int64(12345))
			p.StormctrlMcastEnabled = ptr(false)
			p.StormctrlMcastLevel = ptr(int64(99)) // stale, disabled
			p.StormctrlUcastEnabled = ptr(true)
			p.StormctrlUcastLevel = ptr(int64(37))
			p.StormctrlUcastRate = ptr(int64(14880000))
			want := map[string]string{
				"name": `"Restored"`, "flow_control_enabled": "false", "ptp_enabled": "false",
				"stp_enabled": "false", "bpdu_guard_enabled": "true", "loop_protection_enabled": "true", "eee_enabled": "true",
				"link_debounce_ms": "0", "storm_control_broadcast": "0", "storm_control_unknown_unicast": "37",
			}
			if unit == "rate" {
				want["storm_control_type"] = `"packets_per_second"`
				want["storm_control_broadcast"] = "12345"
				want["storm_control_unknown_unicast"] = "14880000"
			}
			blocks := PortProfileBlocks([]unifi.PortProfile{p}, nil)
			assert.Equal(t, want, attrMapFromBlock(blocks[0]))
			var buf bytes.Buffer
			require.NoError(t, WriteBlocks(&buf, blocks))
			assert.Contains(t, buf.String(), "storm_control_unknown_unicast")
		})
	}
}

func TestPortProfileBlocks_RestoredDefaultsOmitted(t *testing.T) {
	p := uiDefaults("defaults", "Defaults")
	p.FlowControlEnabled, p.PTPEnabled, p.StpPortMode = ptr(true), ptr(true), ptr(true)
	p.StpBpduGuardEnabled, p.PortKeepaliveEnabled, p.EEEEnabled = ptr(false), ptr(false), ptr(false)
	p.LinkDebounceAuto, p.LinkDebounce = ptr(true), ptr(int64(1200))
	p.StormctrlType, p.StormctrlBcastEnabled, p.StormctrlBcastLevel = "level", ptr(false), ptr(int64(37))
	assert.Equal(t, map[string]string{"name": `"Defaults"`, "native_network_id": `"net-default"`}, attrMapFromBlock(PortProfileBlocks([]unifi.PortProfile{p}, nil)[0]))
}
