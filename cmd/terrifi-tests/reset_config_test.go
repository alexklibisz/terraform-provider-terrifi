package main

import (
	"encoding/json"
	"testing"

	"github.com/alexklibisz/terrifi/internal/generate"
	"github.com/stretchr/testify/require"
)

func TestCaptureConfigLinksDependencies(t *testing.T) {
	blocks := []generate.ResourceBlock{
		{ResourceType: "terrifi_device", ImportID: "aa:bb:cc:dd:ee:ff"},
		{ResourceType: "terrifi_network", ImportID: "network", Attributes: []generate.Attr{{Key: "name", Value: `"Temporary"`}, {Key: "purpose", Value: `"corporate"`}}},
		{ResourceType: "terrifi_firewall_zone", ImportID: "zone", Attributes: []generate.Attr{{Key: "name", Value: `"Temporary"`}, {Key: "network_ids", Value: `["network"]`}}},
		{ResourceType: "terrifi_firewall_policy", ImportID: "policy", Blocks: []generate.NestedBlock{
			{Name: "source", Attributes: []generate.Attr{{Key: "zone_id", Value: `"zone"`}}},
			{Name: "destination", Attributes: []generate.Attr{{Key: "zone_id", Value: `"builtin"`}}},
		}},
	}
	config, count, err := captureConfig(blocks)
	require.NoError(t, err)
	require.Equal(t, 3, count)
	require.NotContains(t, string(config), "terrifi_device")
	require.Contains(t, string(config), "network_ids = [terrifi_network.hil_reset_")
	require.Contains(t, string(config), "zone_id = terrifi_firewall_zone.hil_reset_")
	require.Contains(t, string(config), `zone_id = "builtin"`)
	require.Contains(t, string(config), "ignore_changes = all")
	again, _, err := captureConfig(blocks)
	require.NoError(t, err)
	require.Equal(t, config, again)
	_, _, err = captureConfig([]generate.ResourceBlock{blocks[1], blocks[1]})
	require.Error(t, err)
}

func TestBaselineAddressesRejectsUnsafeConfigurations(t *testing.T) {
	for _, root := range []string{`{}`, `{"module_calls":{"lab":{}}}`, `{"resources":[{"mode":"managed","address":"terrifi_network.hil_reset_custom"}]}`} {
		var plan resetPlan
		require.NoError(t, json.Unmarshal([]byte(`{"configuration":{"root_module":`+root+`}}`), &plan))
		_, err := baselineAddresses(plan)
		require.Error(t, err)
	}
}

func TestCaptureConfigLinksPortDependencies(t *testing.T) {
	blocks := []generate.ResourceBlock{
		{ResourceType: "terrifi_network", ImportID: "network"},
		{ResourceType: "terrifi_port_profile", ImportID: "profile", Attributes: []generate.Attr{{Key: "native_network_id", Value: `"network"`}}},
		{ResourceType: "terrifi_device_ports", ImportID: "aa:bb:cc:dd:ee:ff", Attributes: []generate.Attr{{Key: "ports", Value: `{"2" = {name = "profile", port_profile_id = "profile"}, "3" = {native_network_id = "network", tagged_vlan_mode = "block_all"}}`}}},
	}
	config, count, err := captureConfig(blocks)
	require.NoError(t, err)
	require.Equal(t, 3, count)
	require.Contains(t, string(config), "port_profile_id = terrifi_port_profile.hil_reset_")
	require.Contains(t, string(config), "native_network_id = terrifi_network.hil_reset_")
	require.Contains(t, string(config), `name = "profile"`)
	require.Equal(t, `"network"`, blocks[1].Attributes[0].Value)
}
