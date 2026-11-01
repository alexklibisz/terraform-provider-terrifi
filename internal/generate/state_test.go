package generate

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExcludeStateMatchesIdentityAcrossModulesAndSites(t *testing.T) {
	index, err := ReadStateIndex(strings.NewReader(`{
 "format_version":"1.0", "values":{"root_module":{
 "resources":[
  {"mode":"managed","type":"terrifi_network","values":{"id":"lan","site":"default"}},
  {"mode":"data","type":"terrifi_network","values":{"id":"observed","site":"default"}},
  {"mode":"managed","type":"terrifi_firewall_zone","values":{"id":"other-site","site":"other"}},
  {"mode":"managed","type":"terrifi_firewall_policy_order","values":{"id":"src:dst","site":"default"}}
 ],"child_modules":[{"address":"module.lab","resources":[
  {"address":"module.lab.terrifi_device.ap[0]","mode":"managed","type":"terrifi_device","values":{"id":"controller-device-id","mac":"AA:BB:CC:DD:EE:FF"}},
  {"address":"module.lab.terrifi_client_device.vm[\"server\"]","mode":"managed","type":"terrifi_client_device","values":{"id":"old-client-id","mac":"11:22:33:44:55:66"}}
 ]}]}}}`))
	require.NoError(t, err)
	blocks := []ResourceBlock{
		{ResourceType: "terrifi_network", ResourceName: "renamed", ImportID: "lan"},
		{ResourceType: "terrifi_device", ImportID: "aa:bb:cc:dd:ee:ff", Attributes: []Attr{{Key: "mac", Value: HCLString("aa:bb:cc:dd:ee:ff")}}},
		{ResourceType: "terrifi_client_device", ImportID: "new-client-id", Attributes: []Attr{{Key: "mac", Value: HCLString("11:22:33:44:55:66")}}},
		{ResourceType: "terrifi_firewall_policy_order", ImportID: "src:dst"},
		{ResourceType: "terrifi_network", ImportID: "observed"},
		{ResourceType: "terrifi_firewall_zone", ImportID: "other-site"},
		// An ID match in a different resource type must not exclude this object.
		{ResourceType: "terrifi_dns_record", ImportID: "lan"},
	}
	require.Equal(t, blocks[4:], index.ExcludeManaged(blocks, "default"))
	require.Empty(t, index.ExcludeManaged(blocks[5:6], "other"))
	require.Len(t, index.ExcludeManaged(blocks[:1], "other"), 1)
	require.Equal(t, blocks[4:], index.ExcludeManaged(blocks, ""))
}

func TestReadStateIndexFailsClosed(t *testing.T) {
	for _, input := range []string{
		`{`, `{}`, `{"version":4,"resources":[]}`, `{"format_version":"2.0"}`,
		`{"format_version":"1.0","planned_values":{}}`,
		`{"format_version":"1.0"} {}`,
		`{"format_version":"1.0","values":{"root_module":{"resources":[{"mode":"managed","type":"terrifi_network","values":{}}]}}}`,
		`{"format_version":"1.0","values":{"root_module":{"resources":[{"mode":"managed","type":"terrifi_device","values":{"id":"not-a-mac"}}]}}}`,
	} {
		t.Run(input, func(t *testing.T) { _, err := ReadStateIndex(strings.NewReader(input)); require.Error(t, err) })
	}
	index, err := ReadStateIndex(strings.NewReader(`{"format_version":"1.0"}`))
	require.NoError(t, err)
	require.Empty(t, index)
}

func TestExcludeStateProtectsDevicePortsByMAC(t *testing.T) {
	index, err := ReadStateIndex(strings.NewReader(`{"format_version":"1.0","values":{"root_module":{"resources":[{"mode":"managed","type":"terrifi_device_ports","values":{"id":"controller-id","device_mac":"AA:BB:CC:DD:EE:FF","site":"default"}}]}}}`))
	require.NoError(t, err)
	blocks := []ResourceBlock{{ResourceType: "terrifi_device_ports", ImportID: "aa:bb:cc:dd:ee:ff", Attributes: []Attr{{Key: "device_mac", Value: HCLString("aa:bb:cc:dd:ee:ff")}}}}
	require.Empty(t, index.ExcludeManaged(blocks, "default"))
	require.Len(t, index.ExcludeManaged(blocks, "other"), 1)
}
