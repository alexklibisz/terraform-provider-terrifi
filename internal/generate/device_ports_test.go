package generate

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alexklibisz/terrifi/internal/unifi"
)

func TestDevicePortsBlocks(t *testing.T) {
	var overrides []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(`[
		{"port_idx":10,"portconf_id":"pp-a","forward":"customize"},
		{"port_idx":2,"name":"uplink","portconf_id":"pp-b","poe_mode":"off"},
		{"port_idx":5,"name":"desk","forward":"customize"},
		{"port_idx":6,"forward":"all"},
		{"port_idx":8,"name":"inline","forward":"customize","native_networkconf_id":"lan","tagged_vlan_mgmt":"auto"},
		{"port_idx":7,"name":"disabled","forward":"disabled","native_networkconf_id":"lan","tagged_vlan_mgmt":"auto"}
	]`), &overrides))
	devices := []unifi.DevicePorts{
		{MAC: "00:00:5e:00:53:01", Name: "Core Switch", PortOverrides: overrides},
		{MAC: "00:00:5e:00:53:02", Name: "No overrides"},
	}

	var buf bytes.Buffer
	require.NoError(t, WriteBlocks(&buf, DevicePortsBlocks(devices)))
	assert.Equal(t, `import {
  to = terrifi_device_ports.core_switch
  id = "00:00:5e:00:53:01"
}

resource "terrifi_device_ports" "core_switch" {
  device_mac = "00:00:5e:00:53:01"
  ports = {
    "2" = { name = "uplink", port_profile_id = "pp-b" }
    # Port 5 excluded from import: inline VLAN settings with forward mode "customize" cannot be represented.
    # "5" = { name = "desk" }
    # Port 7 excluded from import: forward mode "disabled" cannot be represented.
    # "7" = { name = "disabled" }
    "8" = { name = "inline", native_network_id = "lan", tagged_vlan_mode = "allow_all" }
    "10" = { port_profile_id = "pp-a" }
  } # TODO: reference the corresponding terrifi_network and terrifi_port_profile resources
}

`, buf.String())
}
