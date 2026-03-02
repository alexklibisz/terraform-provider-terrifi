package generate

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ubiquiti-community/go-unifi/unifi"
)

// ---------------------------------------------------------------------------
// ToTerraformName
// ---------------------------------------------------------------------------

func TestToTerraformName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"My Device", "my_device"},
		{"my-device", "my_device"},
		{"My-Device_123", "my_device_123"},
		{"", "SET_NAME"},
		{"  spaces  ", "spaces"},
		{"UPPER", "upper"},
		{"hello world", "hello_world"},
		{"123abc", "_123abc"},
		{"---", "SET_NAME"},
		{"café", "caf"},
		{"a", "a"},
		{"My.Device (v2)", "my_device_v2"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ToTerraformName(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ---------------------------------------------------------------------------
// DeduplicateNames
// ---------------------------------------------------------------------------

func TestDeduplicateNames(t *testing.T) {
	blocks := []ResourceBlock{
		{ResourceName: "foo"},
		{ResourceName: "bar"},
		{ResourceName: "foo"},
		{ResourceName: "foo"},
	}
	DeduplicateNames(blocks)

	assert.Equal(t, "foo", blocks[0].ResourceName)
	assert.Equal(t, "bar", blocks[1].ResourceName)
	assert.Equal(t, "foo_2", blocks[2].ResourceName)
	assert.Equal(t, "foo_3", blocks[3].ResourceName)
}

// ---------------------------------------------------------------------------
// HCL helpers
// ---------------------------------------------------------------------------

func TestHCLFormatters(t *testing.T) {
	assert.Equal(t, `"hello"`, HCLString("hello"))
	assert.Equal(t, `"say \"hi\""`, HCLString(`say "hi"`))
	assert.Equal(t, "true", HCLBool(true))
	assert.Equal(t, "false", HCLBool(false))
	assert.Equal(t, "42", HCLInt64(42))
	assert.Equal(t, `["a", "b"]`, HCLStringList([]string{"a", "b"}))
	assert.Equal(t, "[]", HCLStringList(nil))
}

// ---------------------------------------------------------------------------
// WriteBlocks
// ---------------------------------------------------------------------------

func TestWriteBlocks(t *testing.T) {
	blocks := []ResourceBlock{
		{
			ResourceType: "terrifi_dns_record",
			ResourceName: "example",
			ImportID:     "abc123",
			Attributes: []Attr{
				{Key: "name", Value: `"example.com"`},
				{Key: "value", Value: `"1.2.3.4"`},
				{Key: "record_type", Value: `"A"`},
			},
		},
	}

	var buf bytes.Buffer
	err := WriteBlocks(&buf, blocks)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, `to = terrifi_dns_record.example`)
	assert.Contains(t, output, `id = "abc123"`)
	assert.Contains(t, output, `resource "terrifi_dns_record" "example"`)
	assert.Contains(t, output, `name = "example.com"`)
	assert.Contains(t, output, `value = "1.2.3.4"`)
	assert.Contains(t, output, `record_type = "A"`)
}

func TestWriteBlocksWithNestedBlocks(t *testing.T) {
	blocks := []ResourceBlock{
		{
			ResourceType: "terrifi_firewall_policy",
			ResourceName: "allow_dns",
			ImportID:     "pol123",
			Attributes: []Attr{
				{Key: "name", Value: `"Allow DNS"`},
				{Key: "action", Value: `"ALLOW"`},
			},
			Blocks: []NestedBlock{
				{
					Name: "source",
					Attributes: []Attr{
						{Key: "zone_id", Value: `"zone1"`, Comment: "TODO: find and reference corresponding terrifi_firewall_zone resource"},
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	err := WriteBlocks(&buf, blocks)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "source {")
	assert.Contains(t, output, `zone_id = "zone1" # TODO:`)
}

func TestWriteBlocksWithComment(t *testing.T) {
	blocks := []ResourceBlock{
		{
			Comment:      "A test block",
			ResourceType: "terrifi_dns_record",
			ResourceName: "test",
			ImportID:     "id1",
			Attributes: []Attr{
				{Key: "name", Value: `"test"`, Comment: "inline comment"},
			},
		},
	}

	var buf bytes.Buffer
	err := WriteBlocks(&buf, blocks)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "# A test block")
	assert.Contains(t, output, `name = "test" # inline comment`)
}

// ---------------------------------------------------------------------------
// ClientDeviceBlocks
// ---------------------------------------------------------------------------

func TestClientDeviceBlocks(t *testing.T) {
	blocked := true
	overrideEnabled := true
	clients := []unifi.Client{
		{
			ID:                            "id1",
			MAC:                           "aa:bb:cc:dd:ee:ff",
			Name:                          "My Device",
			Note:                          "A note",
			UseFixedIP:                    true,
			FixedIP:                       "192.168.1.100",
			NetworkID:                     "net1",
			LocalDNSRecordEnabled:         true,
			LocalDNSRecord:                "mydevice.local",
			VirtualNetworkOverrideEnabled: &overrideEnabled,
			VirtualNetworkOverrideID:      "net2",
			Blocked:                       &blocked,
		},
		{
			ID:  "id2",
			MAC: "11:22:33:44:55:66",
		},
	}

	blocks := ClientDeviceBlocks(clients)
	require.Len(t, blocks, 2)

	// First block: all fields, network_id omitted because network_override_id is set
	b := blocks[0]
	assert.Equal(t, "terrifi_client_device", b.ResourceType)
	assert.Equal(t, "my_device", b.ResourceName)
	assert.Equal(t, "id1", b.ImportID)

	attrMap := attrMapFromBlock(b)
	assert.Equal(t, `"aa:bb:cc:dd:ee:ff"`, attrMap["mac"])
	assert.Equal(t, `"My Device"`, attrMap["name"])
	assert.Equal(t, `"A note"`, attrMap["note"])
	assert.Equal(t, `"192.168.1.100"`, attrMap["fixed_ip"])
	_, hasNetworkID := attrMap["network_id"]
	assert.False(t, hasNetworkID, "network_id should be omitted when network_override_id is set")
	assert.Equal(t, `"mydevice.local"`, attrMap["local_dns_record"])
	assert.Equal(t, `"net2"`, attrMap["network_override_id"])
	assert.Equal(t, "true", attrMap["blocked"])

	// Second block: MAC only
	b2 := blocks[1]
	assert.Equal(t, "_11_22_33_44_55_66", b2.ResourceName)
	attrs2 := attrMapFromBlock(b2)
	assert.Equal(t, `"11:22:33:44:55:66"`, attrs2["mac"])
	_, hasName := attrs2["name"]
	assert.False(t, hasName)
}

func TestClientDeviceBlocks_fixedIPWithNetworkID(t *testing.T) {
	clients := []unifi.Client{
		{
			ID:         "id1",
			MAC:        "aa:bb:cc:dd:ee:ff",
			UseFixedIP: true,
			FixedIP:    "192.168.1.100",
			NetworkID:  "net1",
		},
	}

	blocks := ClientDeviceBlocks(clients)
	require.Len(t, blocks, 1)

	attrMap := attrMapFromBlock(blocks[0])
	assert.Equal(t, `"192.168.1.100"`, attrMap["fixed_ip"])
	assert.Equal(t, `"net1"`, attrMap["network_id"])
	_, hasOverride := attrMap["network_override_id"]
	assert.False(t, hasOverride)
}

func TestClientDeviceBlocks_deduplication(t *testing.T) {
	clients := []unifi.Client{
		{ID: "id1", MAC: "aa:bb:cc:dd:ee:ff", Name: "device"},
		{ID: "id2", MAC: "11:22:33:44:55:66", Name: "device"},
	}

	blocks := ClientDeviceBlocks(clients)
	require.Len(t, blocks, 2)
	assert.Equal(t, "device", blocks[0].ResourceName)
	assert.Equal(t, "device_2", blocks[1].ResourceName)
}

// ---------------------------------------------------------------------------
// DNSRecordBlocks
// ---------------------------------------------------------------------------

func TestDNSRecordBlocks(t *testing.T) {
	port := int64(443)
	records := []unifi.DNSRecord{
		{
			ID:         "dns1",
			Key:        "example.com",
			Value:      "1.2.3.4",
			RecordType: "A",
			Enabled:    true,
		},
		{
			ID:         "dns2",
			Key:        "mail.example.com",
			Value:      "mx.example.com",
			RecordType: "MX",
			Enabled:    false,
			Port:       &port,
			Priority:   10,
			Ttl:        3600,
			Weight:     5,
		},
	}

	blocks := DNSRecordBlocks(records)
	require.Len(t, blocks, 2)

	// Simple A record
	b := blocks[0]
	assert.Equal(t, "terrifi_dns_record", b.ResourceType)
	assert.Equal(t, "example_com", b.ResourceName)
	attrs := attrMapFromBlock(b)
	assert.Equal(t, `"example.com"`, attrs["name"])
	assert.Equal(t, `"1.2.3.4"`, attrs["value"])
	assert.Equal(t, `"A"`, attrs["record_type"])
	_, hasEnabled := attrs["enabled"]
	assert.False(t, hasEnabled) // not set when true (default)

	// MX with all optional fields
	b2 := blocks[1]
	attrs2 := attrMapFromBlock(b2)
	assert.Equal(t, "false", attrs2["enabled"])
	assert.Equal(t, "443", attrs2["port"])
	assert.Equal(t, "10", attrs2["priority"])
	assert.Equal(t, "3600", attrs2["ttl"])
	assert.Equal(t, "5", attrs2["weight"])
}

// ---------------------------------------------------------------------------
// FirewallZoneBlocks
// ---------------------------------------------------------------------------

func TestFirewallZoneBlocks(t *testing.T) {
	zones := []unifi.FirewallZone{
		{
			ID:         "zone1",
			Name:       "Internal",
			NetworkIDs: []string{"net1", "net2"},
		},
		{
			ID:   "zone2",
			Name: "External",
		},
	}

	blocks := FirewallZoneBlocks(zones)
	require.Len(t, blocks, 2)

	b := blocks[0]
	assert.Equal(t, "terrifi_firewall_zone", b.ResourceType)
	assert.Equal(t, "internal", b.ResourceName)
	attrs := attrMapFromBlock(b)
	assert.Equal(t, `"Internal"`, attrs["name"])
	assert.Equal(t, `["net1", "net2"]`, attrs["network_ids"])

	// External has no network_ids
	b2 := blocks[1]
	attrs2 := attrMapFromBlock(b2)
	_, hasNets := attrs2["network_ids"]
	assert.False(t, hasNets)
}

// ---------------------------------------------------------------------------
// FirewallPolicyBlocks
// ---------------------------------------------------------------------------

func TestFirewallPolicyBlocks(t *testing.T) {
	index := int64(5)
	port := int64(443)
	policies := []*unifi.FirewallPolicy{
		{
			ID:                  "pol1",
			Name:                "Allow DNS",
			Enabled:             true,
			Action:              "ALLOW",
			IPVersion:           "BOTH",
			Protocol:            "tcp",
			ConnectionStateType: "ALL",
			Logging:             true,
			Index:               &index,
			Source: &unifi.FirewallPolicySource{
				ZoneID:           "zone1",
				MatchingTarget:   "IP",
				IPs:              []string{"10.0.0.0/8"},
				PortMatchingType: "SPECIFIC",
				Port:             &port,
			},
			Destination: &unifi.FirewallPolicyDestination{
				ZoneID:         "zone2",
				MatchingTarget: "ANY",
			},
		},
	}

	blocks := FirewallPolicyBlocks(policies)
	require.Len(t, blocks, 1)

	b := blocks[0]
	assert.Equal(t, "terrifi_firewall_policy", b.ResourceType)
	assert.Equal(t, "allow_dns", b.ResourceName)

	attrs := attrMapFromBlock(b)
	assert.Equal(t, `"Allow DNS"`, attrs["name"])
	assert.Equal(t, `"ALLOW"`, attrs["action"])
	assert.Equal(t, `"tcp"`, attrs["protocol"])
	assert.Equal(t, "true", attrs["logging"])
	// index is computed-only and should not appear in generated HCL
	_, hasIndex := attrs["index"]
	assert.False(t, hasIndex)
	// ip_version should be omitted when BOTH
	_, hasIPVersion := attrs["ip_version"]
	assert.False(t, hasIPVersion)
	// enabled should be omitted when true
	_, hasEnabled := attrs["enabled"]
	assert.False(t, hasEnabled)

	// Check nested blocks
	require.Len(t, b.Blocks, 2)
	srcAttrs := nestedAttrMap(b.Blocks[0])
	assert.Equal(t, `"zone1"`, srcAttrs["zone_id"])
	assert.Equal(t, `["10.0.0.0/8"]`, srcAttrs["ips"])
	assert.Equal(t, `"SPECIFIC"`, srcAttrs["port_matching_type"])
	assert.Equal(t, "443", srcAttrs["port"])

	dstAttrs := nestedAttrMap(b.Blocks[1])
	assert.Equal(t, `"zone2"`, dstAttrs["zone_id"])
}

func TestFirewallPolicyBlocks_deviceIDs(t *testing.T) {
	policies := []*unifi.FirewallPolicy{
		{
			ID:      "pol1",
			Name:    "Allow Devices",
			Enabled: true,
			Action:  "ALLOW",
			Source: &unifi.FirewallPolicySource{
				ZoneID:         "zone1",
				MatchingTarget: "CLIENT",
				IPs:            []string{"aa:bb:cc:dd:ee:f1", "aa:bb:cc:dd:ee:f2"},
			},
			Destination: &unifi.FirewallPolicyDestination{
				ZoneID:         "zone2",
				MatchingTarget: "ANY",
			},
		},
	}

	blocks := FirewallPolicyBlocks(policies)
	require.Len(t, blocks, 1)

	b := blocks[0]
	require.Len(t, b.Blocks, 2)

	srcAttrs := nestedAttrMap(b.Blocks[0])
	assert.Equal(t, `"zone1"`, srcAttrs["zone_id"])
	assert.Equal(t, `["aa:bb:cc:dd:ee:f1", "aa:bb:cc:dd:ee:f2"]`, srcAttrs["device_ids"])

	// Destination should only have zone_id (matching target is ANY).
	dstAttrs := nestedAttrMap(b.Blocks[1])
	assert.Equal(t, `"zone2"`, dstAttrs["zone_id"])
	_, hasDeviceIDs := dstAttrs["device_ids"]
	assert.False(t, hasDeviceIDs)
}

func TestFirewallPolicyBlocks_macAddresses(t *testing.T) {
	policies := []*unifi.FirewallPolicy{
		{
			ID:      "pol1",
			Name:    "Block MACs",
			Enabled: true,
			Action:  "BLOCK",
			Source: &unifi.FirewallPolicySource{
				ZoneID:         "zone1",
				MatchingTarget: "MAC",
				IPs:            []string{"aa:bb:cc:dd:ee:ff"},
			},
			Destination: &unifi.FirewallPolicyDestination{
				ZoneID:         "zone2",
				MatchingTarget: "ANY",
			},
		},
	}

	blocks := FirewallPolicyBlocks(policies)
	require.Len(t, blocks, 1)

	srcAttrs := nestedAttrMap(blocks[0].Blocks[0])
	assert.Equal(t, `["aa:bb:cc:dd:ee:ff"]`, srcAttrs["mac_addresses"])
	_, hasIPs := srcAttrs["ips"]
	assert.False(t, hasIPs)
	_, hasDeviceIDs := srcAttrs["device_ids"]
	assert.False(t, hasDeviceIDs)
}

func TestFirewallPolicyBlocks_networkIDs(t *testing.T) {
	policies := []*unifi.FirewallPolicy{
		{
			ID:      "pol1",
			Name:    "Allow Networks",
			Enabled: true,
			Action:  "ALLOW",
			Source: &unifi.FirewallPolicySource{
				ZoneID:         "zone1",
				MatchingTarget: "NETWORK",
				IPs:            []string{"net-001", "net-002"},
			},
			Destination: &unifi.FirewallPolicyDestination{
				ZoneID:         "zone2",
				MatchingTarget: "ANY",
			},
		},
	}

	blocks := FirewallPolicyBlocks(policies)
	require.Len(t, blocks, 1)

	srcAttrs := nestedAttrMap(blocks[0].Blocks[0])
	assert.Equal(t, `["net-001", "net-002"]`, srcAttrs["network_ids"])
	_, hasIPs := srcAttrs["ips"]
	assert.False(t, hasIPs)
}

func TestFirewallPolicyBlocks_schedule(t *testing.T) {
	policies := []*unifi.FirewallPolicy{
		{
			ID:      "pol1",
			Name:    "Timed",
			Enabled: true,
			Action:  "BLOCK",
			Schedule: &unifi.FirewallPolicySchedule{
				Mode:           "EVERY_DAY",
				TimeRangeStart: "08:00",
				TimeRangeEnd:   "17:00",
			},
		},
	}

	blocks := FirewallPolicyBlocks(policies)
	require.Len(t, blocks, 1)
	require.Len(t, blocks[0].Blocks, 1)

	sched := blocks[0].Blocks[0]
	assert.Equal(t, "schedule", sched.Name)
	schedAttrs := nestedAttrMap(sched)
	assert.Equal(t, `"EVERY_DAY"`, schedAttrs["mode"])
	assert.Equal(t, `"08:00"`, schedAttrs["time_range_start"])
	assert.Equal(t, `"17:00"`, schedAttrs["time_range_end"])
}

func TestFirewallPolicyBlocks_alwaysScheduleOmitted(t *testing.T) {
	policies := []*unifi.FirewallPolicy{
		{
			ID:      "pol1",
			Name:    "Always",
			Enabled: true,
			Action:  "ALLOW",
			Schedule: &unifi.FirewallPolicySchedule{
				Mode: "ALWAYS",
			},
		},
	}

	blocks := FirewallPolicyBlocks(policies)
	require.Len(t, blocks, 1)
	assert.Empty(t, blocks[0].Blocks)
}

// ---------------------------------------------------------------------------
// NetworkBlocks
// ---------------------------------------------------------------------------

func TestNetworkBlocks(t *testing.T) {
	name := "My Network"
	vlan := int64(100)
	subnet := "192.168.33.0/24"
	group := "WAN"
	dhcpStart := "192.168.33.100"
	dhcpStop := "192.168.33.200"
	lease := int64(3600)

	networks := []unifi.Network{
		{
			ID:                    "net1",
			Purpose:               "corporate",
			Name:                  &name,
			VLAN:                  &vlan,
			IPSubnet:              &subnet,
			NetworkGroup:          &group,
			DHCPDEnabled:          true,
			DHCPDStart:            &dhcpStart,
			DHCPDStop:             &dhcpStop,
			DHCPDLeaseTime:        &lease,
			DHCPDDNS1:             "1.1.1.1",
			DHCPDDNS2:             "8.8.8.8",
			InternetAccessEnabled: false,
		},
		{
			ID:      "net2",
			Purpose: "wan",
		},
	}

	blocks := NetworkBlocks(networks)
	require.Len(t, blocks, 1) // WAN filtered out

	b := blocks[0]
	assert.Equal(t, "terrifi_network", b.ResourceType)
	assert.Equal(t, "my_network", b.ResourceName)

	attrs := attrMapFromBlock(b)
	assert.Equal(t, `"My Network"`, attrs["name"])
	assert.Equal(t, `"corporate"`, attrs["purpose"])
	assert.Equal(t, "100", attrs["vlan_id"])
	assert.Equal(t, `"192.168.33.0/24"`, attrs["subnet"])
	assert.Equal(t, `"WAN"`, attrs["network_group"])
	assert.Equal(t, "true", attrs["dhcp_enabled"])
	assert.Equal(t, `"192.168.33.100"`, attrs["dhcp_start"])
	assert.Equal(t, `"192.168.33.200"`, attrs["dhcp_stop"])
	assert.Equal(t, "3600", attrs["dhcp_lease"])
	assert.Equal(t, `["1.1.1.1", "8.8.8.8"]`, attrs["dhcp_dns"])
	assert.Equal(t, "false", attrs["internet_access_enabled"])
}

func TestNetworkBlocks_defaults(t *testing.T) {
	name := "Simple"
	networks := []unifi.Network{
		{
			ID:                    "net1",
			Purpose:               "corporate",
			Name:                  &name,
			InternetAccessEnabled: true,
		},
	}

	blocks := NetworkBlocks(networks)
	require.Len(t, blocks, 1)

	attrs := attrMapFromBlock(blocks[0])
	// Defaults should not appear
	_, hasGroup := attrs["network_group"]
	assert.False(t, hasGroup)
	_, hasDHCP := attrs["dhcp_enabled"]
	assert.False(t, hasDHCP)
	_, hasInternet := attrs["internet_access_enabled"]
	assert.False(t, hasInternet)
}

// ---------------------------------------------------------------------------
// WLANBlocks
// ---------------------------------------------------------------------------

func TestWLANBlocks(t *testing.T) {
	wlans := []unifi.WLAN{
		{
			ID:             "wlan1",
			Name:           "MyWiFi",
			NetworkID:      "net1",
			WLANBand:       "5g",
			Security:       "wpapsk",
			HideSSID:       true,
			WPAMode:        "wpa2",
			WPA3Support:    true,
			WPA3Transition: true,
		},
		{
			ID:        "wlan2",
			Name:      "Guest",
			NetworkID: "net2",
			WLANBand:  "both",
			Security:  "open",
		},
	}

	blocks := WLANBlocks(wlans)
	require.Len(t, blocks, 2)

	// First WLAN
	b := blocks[0]
	assert.Equal(t, "terrifi_wlan", b.ResourceType)
	assert.Equal(t, "mywifi", b.ResourceName)

	attrs := attrMapFromBlock(b)
	assert.Equal(t, `"MyWiFi"`, attrs["name"])
	assert.Equal(t, `"REPLACE_ME"`, attrs["passphrase"])
	assert.Equal(t, `"net1"`, attrs["network_id"])
	assert.Equal(t, `"5g"`, attrs["wifi_band"])
	assert.Equal(t, "true", attrs["hide_ssid"])
	assert.Equal(t, "true", attrs["wpa3_support"])
	assert.Equal(t, "true", attrs["wpa3_transition"])
	// wpa_mode "wpa2" is default, should not appear
	_, hasWPAMode := attrs["wpa_mode"]
	assert.False(t, hasWPAMode)
	// security "wpapsk" is default, should not appear
	_, hasSecurity := attrs["security"]
	assert.False(t, hasSecurity)

	// Guest WLAN
	b2 := blocks[1]
	attrs2 := attrMapFromBlock(b2)
	assert.Equal(t, `"open"`, attrs2["security"])
	// wifi_band "both" is default, should not appear
	_, hasBand := attrs2["wifi_band"]
	assert.False(t, hasBand)
}

// ---------------------------------------------------------------------------
// ClientGroupBlocks
// ---------------------------------------------------------------------------

func TestClientGroupBlocks(t *testing.T) {
	groups := []unifi.ClientGroup{
		{
			ID:   "grp1",
			Name: "WiFi Smart Plugs",
		},
		{
			ID:   "grp2",
			Name: "IoT Devices",
		},
	}

	blocks := ClientGroupBlocks(groups)
	require.Len(t, blocks, 2)

	b := blocks[0]
	assert.Equal(t, "terrifi_client_group", b.ResourceType)
	assert.Equal(t, "wifi_smart_plugs", b.ResourceName)
	assert.Equal(t, "grp1", b.ImportID)

	attrs := attrMapFromBlock(b)
	assert.Equal(t, `"WiFi Smart Plugs"`, attrs["name"])

	b2 := blocks[1]
	assert.Equal(t, "iot_devices", b2.ResourceName)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func attrMapFromBlock(b ResourceBlock) map[string]string {
	m := make(map[string]string)
	for _, a := range b.Attributes {
		m[a.Key] = a.Value
	}
	return m
}

func nestedAttrMap(nb NestedBlock) map[string]string {
	m := make(map[string]string)
	for _, a := range nb.Attributes {
		m[a.Key] = a.Value
	}
	return m
}
