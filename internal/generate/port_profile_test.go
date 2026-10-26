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
		"stp_port_mode": false, "dot1x_ctrl": "multi_host", "port_security_enabled": true,
		"link_debounce_auto": false, "link_debounce": 1200,
		"stormctrl_bcast_enabled": true, "stormctrl_bcast_level": 20
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
