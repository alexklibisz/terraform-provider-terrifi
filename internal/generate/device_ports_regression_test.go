package generate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"

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

func TestDevicePortsBlocks_UnsupportedPortsAreCommentedOut(t *testing.T) {
	for _, mixed := range []bool{false, true} {
		t.Run(fmt.Sprint("mixed=", mixed), func(t *testing.T) {
			var overrides []map[string]json.RawMessage
			require.NoError(t, json.Unmarshal([]byte(`[
    {"port_idx":3,"name":"Server","native_networkconf_id":"lan","tagged_vlan_mgmt":"custom"},
    {"port_idx":4,"native_networkconf_id":"lan","forward":"customize"}
   ]`), &overrides))
			if mixed {
				overrides = append(overrides, map[string]json.RawMessage{"port_idx": json.RawMessage(`2`), "name": json.RawMessage(`"Office"`), "native_networkconf_id": json.RawMessage(`"lan"`), "tagged_vlan_mgmt": json.RawMessage(`"block_all"`)})
			}
			blocks := DevicePortsBlocks([]unifi.DevicePorts{{MAC: "00:00:00:00:00:01", Name: "Switch", PortOverrides: overrides}})
			require.Len(t, blocks, 1, "devices with only unsupported ports still need explanatory output")
			source := blocks[0].Attributes[1].Value
			require.Contains(t, source, `# Port 3 excluded from import: inline VLAN mode "custom" is unsupported.`)
			require.Contains(t, source, `# "3" = { name = "Server" }`)
			require.Contains(t, source, `# "4" = {  }`)
			expr, diags := hclsyntax.ParseExpression([]byte(source), "ports", hcl.InitialPos)
			require.False(t, diags.HasErrors(), diags.Error())
			value, diags := expr.Value(nil)
			require.False(t, diags.HasErrors(), diags.Error())
			ports := value.AsValueMap()
			require.NotContains(t, ports, "3")
			require.NotContains(t, ports, "4")
			if mixed {
				require.Len(t, ports, 1)
				require.Contains(t, ports, "2")
			} else {
				require.Empty(t, ports)
			}
			var out bytes.Buffer
			require.NoError(t, WriteBlocks(&out, blocks))
			_, diags = hclsyntax.ParseConfig(out.Bytes(), "generated.tf", hcl.InitialPos)
			require.False(t, diags.HasErrors(), diags.Error())
		})
	}
}
