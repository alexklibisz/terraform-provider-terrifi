package generate

import (
	"encoding/json"
	"testing"

	"github.com/alexklibisz/terrifi/internal/unifi"
	"github.com/stretchr/testify/require"
)

func TestDevicePortsBlocks_DuplicateNames(t *testing.T) {
	overrides := []map[string]json.RawMessage{{"port_idx": json.RawMessage(`2`), "name": json.RawMessage(`"desk"`)}}
	blocks := DevicePortsBlocks([]unifi.DevicePorts{
		{Name: "Flex Mini", MAC: "00:00:00:00:00:01", PortOverrides: overrides},
		{Name: "Flex Mini", MAC: "00:00:00:00:00:02", PortOverrides: overrides},
	})
	require.Len(t, blocks, 2)
	require.NotEqual(t, blocks[0].ResourceName, blocks[1].ResourceName, "devices need unique Terraform addresses")
}

func TestDevicePortsBlocks_InlineVLANs(t *testing.T) {
	overrides := []map[string]json.RawMessage{{"port_idx": json.RawMessage(`2`), "native_networkconf_id": json.RawMessage(`"network"`), "forward": json.RawMessage(`"native"`)}}
	blocks := DevicePortsBlocks([]unifi.DevicePorts{{Name: "Mini", MAC: "00:00:00:00:00:01", PortOverrides: overrides}})
	require.Len(t, blocks, 1)
	require.Contains(t, blocks[0].Attributes[1].Value, `native_network_id = "network"`)
	require.Contains(t, blocks[0].Attributes[1].Value, `tagged_vlan_mode = "block_all"`)
}
