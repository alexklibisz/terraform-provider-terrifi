package provider

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ubiquiti-community/go-unifi/unifi"
)

// Verifies the workaround that wraps numeric tx_power values in quotes so the
// SDK's DeviceRadioTable.UnmarshalJSON does not fail on payloads from
// controllers that emit tx_power as a JSON number.
func TestFixTxPowerBytes(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "numeric tx_power gets quoted",
			in:   `{"tx_power":23}`,
			want: `{"tx_power":"23"}`,
		},
		{
			name: "negative numeric tx_power gets quoted",
			in:   `{"tx_power":-5}`,
			want: `{"tx_power":"-5"}`,
		},
		{
			name: "fractional tx_power gets quoted",
			in:   `{"tx_power": 1.5}`,
			want: `{"tx_power":"1.5"}`,
		},
		{
			name: "string tx_power is left alone",
			in:   `{"tx_power":"23"}`,
			want: `{"tx_power":"23"}`,
		},
		{
			name: "auto string is left alone",
			in:   `{"tx_power":"auto"}`,
			want: `{"tx_power":"auto"}`,
		},
		{
			name: "tx_power_mode is not affected",
			in:   `{"tx_power_mode":"auto","tx_power":18}`,
			want: `{"tx_power_mode":"auto","tx_power":"18"}`,
		},
		{
			name: "multiple radio entries all coerced",
			in:   `[{"tx_power":18},{"tx_power":23}]`,
			want: `[{"tx_power":"18"},{"tx_power":"23"}]`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := string(fixTxPowerBytes([]byte(tc.in)))
			assert.Equal(t, tc.want, got)
		})
	}
}

// Verifies that a stat/device payload with a numeric tx_power decodes cleanly
// after the workaround is applied. Without the workaround, the SDK's
// DeviceRadioTable.UnmarshalJSON returns "cannot unmarshal number into Go
// struct field .Alias.tx_power of type string".
func TestFixTxPowerBytes_decodesIntoUnifiDevice(t *testing.T) {
	const payload = `{
		"meta": {"rc": "ok"},
		"data": [{
			"_id": "abc123",
			"mac": "aa:bb:cc:dd:ee:ff",
			"name": "AP",
			"radio_table": [
				{"radio": "ng", "channel": "auto", "ht": 40, "tx_power": 18, "tx_power_mode": "auto"},
				{"radio": "na", "channel": "auto", "ht": 80, "tx_power": "23", "tx_power_mode": "high"}
			]
		}]
	}`

	// Sanity: raw payload fails to decode without the workaround.
	var rawEnvelope struct {
		Data []unifi.Device `json:"data"`
	}
	rawErr := json.Unmarshal([]byte(payload), &rawEnvelope)
	require.Error(t, rawErr, "expected SDK to reject numeric tx_power without workaround")

	patched := fixTxPowerBytes([]byte(payload))
	var envelope struct {
		Data []unifi.Device `json:"data"`
	}
	require.NoError(t, json.Unmarshal(patched, &envelope))
	require.Len(t, envelope.Data, 1)

	d := envelope.Data[0]
	require.Len(t, d.RadioTable, 2)
	assert.Equal(t, "18", d.RadioTable[0].TxPower)
	assert.Equal(t, "auto", d.RadioTable[0].TxPowerMode)
	assert.Equal(t, "23", d.RadioTable[1].TxPower)
	assert.Equal(t, "high", d.RadioTable[1].TxPowerMode)
}
