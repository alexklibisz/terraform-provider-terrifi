package generate

import (
	"bytes"
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
		ID:                         id,
		Name:                       name,
		SettingPreference:          "auto",
		Forward:                    "all",
		NativeNetworkID:            ptr("net-default"),
		TaggedVLANMgmt:             "auto",
		VoiceNetworkID:             ptr(""),
		PoeMode:                    ptr("auto"),
		Autoneg:                    ptr(true),
		EgressRateLimitKbpsEnabled: ptr(false),
		FlowControlEnabled:         ptr(true),
		PTPEnabled:                 ptr(true),
		Isolation:                  ptr(false),
		StpPortMode:                ptr(true),
		StpUplink:                  ptr(false),
		StpBpduGuardEnabled:        ptr(false),
		PortKeepaliveEnabled:       ptr(false),
		EEEEnabled:                 ptr(false),
		LldpmedEnabled:             ptr(true),
		LinkDebounceAuto:           ptr(true),
		LinkDebounce:               ptr(int64(300)),
		PortSecurityEnabled:        ptr(false),
		PortSecurityMACAddress:     []string{},
		Dot1XCtrl:                  "force_authorized",
		StormctrlBcastEnabled:      ptr(false),
		StormctrlBcastRate:         ptr(int64(100)),
		StormctrlMcastEnabled:      ptr(false),
		StormctrlUcastEnabled:      ptr(false),
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
		"tagged_vlan_mgmt":   `"custom"`,
		"tagged_network_ids": `["net-c"]`,
		"poe_mode":           `"off"`,
	}, attrs)
}

func TestPortProfileBlocks_NonDefaults(t *testing.T) {
	p := uiDefaults("prof3", "Everything")
	p.NativeNetworkID = ptr("")
	p.VoiceNetworkID = ptr("net-b")
	p.Autoneg = ptr(false)
	p.Speed = ptr(int64(100))
	p.FullDuplex = ptr(true)
	p.EgressRateLimitKbpsEnabled = ptr(true)
	p.EgressRateLimitKbps = ptr(int64(5000))
	p.FlowControlEnabled = ptr(false)
	p.PTPEnabled = ptr(false)
	p.Isolation = ptr(true)
	p.StpPortMode = ptr(false)
	p.StpUplink = ptr(true)
	p.StpBpduGuardEnabled = ptr(true)
	p.PortKeepaliveEnabled = ptr(true)
	p.EEEEnabled = ptr(true)
	p.LinkDebounceAuto = ptr(false)
	p.LinkDebounce = ptr(int64(1200))
	p.PortSecurityEnabled = ptr(true)
	p.PortSecurityMACAddress = []string{"00:00:5e:00:53:01"}
	p.Dot1XCtrl = "multi_host"
	p.StormctrlType = "rate"
	p.StormctrlBcastEnabled = ptr(true)
	p.StormctrlBcastRate = ptr(int64(12345))
	p.StormctrlUcastEnabled = ptr(true)
	p.StormctrlUcastRate = ptr(int64(2000))

	attrs := attrMapFromBlock(PortProfileBlocks([]unifi.PortProfile{p}, testLAN)[0])
	assert.Equal(t, map[string]string{
		"name":                        `"Everything"`,
		"voice_network_id":            `"net-b"`,
		"autoneg":                     "false",
		"speed":                       "100",
		"full_duplex":                 "true",
		"egress_rate_limit_kbps":      "5000",
		"flow_control_enabled":        "false",
		"ptp_enabled":                 "false",
		"isolation":                   "true",
		"stp_enabled":                 "false",
		"stp_uplink":                  "true",
		"bpdu_guard_enabled":          "true",
		"loop_protection_enabled":     "true",
		"eee_enabled":                 "true",
		"link_debounce_ms":            "1200",
		"port_security_enabled":       "true",
		"port_security_mac_addresses": `["00:00:5e:00:53:01"]`,
		"dot1x_ctrl":                  `"multi_host"`,
		"storm_control":               `{ type = "rate", broadcast = 12345, unicast = 2000 }`,
	}, attrs)
}

// Values the controller keeps but ignores must not be emitted: the resource
// reads them as unset, so emitting them would produce an immediate diff.
func TestPortProfileBlocks_IgnoredStaleValues(t *testing.T) {
	p := uiDefaults("prof4", "Stale")
	p.NativeNetworkID = ptr("")
	p.Speed = ptr(int64(100)) // autoneg is on
	p.FullDuplex = ptr(true)  // autoneg is on
	p.LldpmedEnabled = ptr(false)
	p.VoiceNetworkID = ptr("net-b") // LLDP-MED is off
	p.EgressRateLimitKbps = ptr(int64(5000))
	p.LinkDebounce = ptr(int64(1200)) // debounce is auto
	p.StormctrlType = "level"
	p.StormctrlBcastLevel = ptr(int64(37)) // broadcast storm control is off

	attrs := attrMapFromBlock(PortProfileBlocks([]unifi.PortProfile{p}, testLAN)[0])
	assert.Equal(t, map[string]string{
		"name":            `"Stale"`,
		"lldpmed_enabled": "false",
	}, attrs)
}

func TestPortProfileBlocks_LinkDebounceOff(t *testing.T) {
	p := uiDefaults("prof5", "Debounce Off")
	p.LinkDebounceAuto = ptr(false)
	p.LinkDebounce = ptr(int64(0))
	assert.Equal(t, "0", attrMapFromBlock(PortProfileBlocks([]unifi.PortProfile{p}, testLAN)[0])["link_debounce_ms"])
}

func TestPortProfileBlocks_RendersValidHCL(t *testing.T) {
	p := uiDefaults("prof6", "Storm")
	p.StormctrlType = "level"
	p.StormctrlBcastEnabled = ptr(true)
	p.StormctrlBcastLevel = ptr(int64(20))

	var buf bytes.Buffer
	require.NoError(t, WriteBlocks(&buf, PortProfileBlocks([]unifi.PortProfile{p}, testLAN)))
	out := buf.String()
	assert.Contains(t, out, `to = terrifi_port_profile.storm`)
	assert.Contains(t, out, `native_network_id = "net-default" # TODO: reference the corresponding terrifi_network resource`)
	assert.Contains(t, out, `storm_control = { broadcast = 20 }`)
}
