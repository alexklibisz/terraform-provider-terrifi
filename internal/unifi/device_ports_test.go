package unifi

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPortVLANSettingsImportability(t *testing.T) {
	for _, tc := range []struct {
		name, data, native, mode string
		unsupported              bool
	}{
		{name: "name only", data: `{"name":"desk"}`},
		{name: "default forward without native", data: `{"forward":"all"}`},
		{name: "explicit access", data: `{"native_networkconf_id":"lan","tagged_vlan_mgmt":"block_all"}`, native: "lan", mode: "block_all"},
		{name: "explicit trunk", data: `{"native_networkconf_id":"lan","tagged_vlan_mgmt":"auto"}`, native: "lan", mode: "allow_all"},
		{name: "legacy access", data: `{"native_networkconf_id":"lan","forward":"native"}`, native: "lan", mode: "block_all"},
		{name: "legacy trunk", data: `{"native_networkconf_id":"lan","forward":"all"}`, native: "lan", mode: "allow_all"},
		{name: "disabled with trunk fields", data: `{"forward":"disabled","native_networkconf_id":"lan","tagged_vlan_mgmt":"auto"}`, unsupported: true},
		{name: "disabled with access fields", data: `{"forward":"disabled","native_networkconf_id":"lan","tagged_vlan_mgmt":"block_all"}`, unsupported: true},
		{name: "disabled with profile", data: `{"forward":"disabled","portconf_id":"profile"}`, unsupported: true},
		{name: "future forwarding mode", data: `{"forward":"future","native_networkconf_id":"lan","tagged_vlan_mgmt":"auto"}`, unsupported: true},
		{name: "custom", data: `{"name":"server","native_networkconf_id":"lan","tagged_vlan_mgmt":"custom"}`, unsupported: true},
		{name: "legacy custom", data: `{"native_networkconf_id":"lan","forward":"customize"}`, unsupported: true},
		{name: "unknown mode", data: `{"native_networkconf_id":"lan","tagged_vlan_mgmt":"future"}`, unsupported: true},
		{name: "missing mode", data: `{"native_networkconf_id":"lan"}`, unsupported: true},
		{name: "missing native", data: `{"tagged_vlan_mgmt":"block_all"}`, unsupported: true},
		{name: "custom without native", data: `{"forward":"customize"}`, unsupported: true},
		{name: "profile with stale inline fields", data: `{"portconf_id":"profile","tagged_vlan_mgmt":"custom","native_networkconf_id":"stale"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var entry map[string]json.RawMessage
			require.NoError(t, json.Unmarshal([]byte(tc.data), &entry))
			before, _ := json.Marshal(entry)
			native, mode := PortVLANSettings(entry)
			assert.Equal(t, tc.native, native)
			assert.Equal(t, tc.mode, mode)
			assert.Equal(t, tc.unsupported, UnsupportedPortVLANReason(entry) != "")
			after, _ := json.Marshal(entry)
			assert.Equal(t, string(before), string(after), "classification must not mutate controller settings")
		})
	}
}
