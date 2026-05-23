package provider

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ubiquiti-community/go-unifi/unifi"
)

// Verifies the workaround that strips quotes from string-encoded
// ipv6_ra_preferred_lifetime values so the SDK's Network.UnmarshalJSON does not
// fail on payloads from controllers that emit the field as a JSON string.
func TestFixNetworkBytes(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "string ipv6_ra_preferred_lifetime gets unquoted",
			in:   `{"ipv6_ra_preferred_lifetime":"14400"}`,
			want: `{"ipv6_ra_preferred_lifetime":14400}`,
		},
		{
			name: "string with spaces around colon gets unquoted",
			in:   `{"ipv6_ra_preferred_lifetime" : "14400"}`,
			want: `{"ipv6_ra_preferred_lifetime":14400}`,
		},
		{
			name: "numeric ipv6_ra_preferred_lifetime is left alone",
			in:   `{"ipv6_ra_preferred_lifetime":14400}`,
			want: `{"ipv6_ra_preferred_lifetime":14400}`,
		},
		{
			name: "empty string becomes null",
			in:   `{"ipv6_ra_preferred_lifetime":""}`,
			want: `{"ipv6_ra_preferred_lifetime":null}`,
		},
		{
			name: "null is left alone",
			in:   `{"ipv6_ra_preferred_lifetime":null}`,
			want: `{"ipv6_ra_preferred_lifetime":null}`,
		},
		{
			name: "other string fields are not affected",
			in:   `{"name":"LAN","ipv6_ra_preferred_lifetime":"14400"}`,
			want: `{"name":"LAN","ipv6_ra_preferred_lifetime":14400}`,
		},
		{
			name: "multiple networks all coerced",
			in:   `[{"ipv6_ra_preferred_lifetime":"14400"},{"ipv6_ra_preferred_lifetime":""}]`,
			want: `[{"ipv6_ra_preferred_lifetime":14400},{"ipv6_ra_preferred_lifetime":null}]`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := string(fixNetworkBytes([]byte(tc.in)))
			assert.Equal(t, tc.want, got)
		})
	}
}

// Verifies that a networkconf payload with a string-encoded
// ipv6_ra_preferred_lifetime decodes cleanly after the workaround. Without it
// the SDK's Network.UnmarshalJSON returns "cannot unmarshal string into Go
// struct field .Alias.ipv6_ra_preferred_lifetime of type int64".
func TestFixNetworkBytes_decodesIntoUnifiNetwork(t *testing.T) {
	const payload = `{
		"meta": {"rc": "ok"},
		"data": [{
			"_id": "abc123",
			"name": "LAN",
			"ipv6_ra_preferred_lifetime": "14400"
		}]
	}`

	// Sanity: raw payload fails to decode without the workaround.
	var rawEnvelope struct {
		Data []unifi.Network `json:"data"`
	}
	rawErr := json.Unmarshal([]byte(payload), &rawEnvelope)
	require.Error(t, rawErr, "expected SDK to reject string ipv6_ra_preferred_lifetime without workaround")

	patched := fixNetworkBytes([]byte(payload))
	var envelope struct {
		Data []unifi.Network `json:"data"`
	}
	require.NoError(t, json.Unmarshal(patched, &envelope))
	require.Len(t, envelope.Data, 1)

	n := envelope.Data[0]
	require.NotNil(t, n.IPV6RaPreferredLifetime)
	assert.Equal(t, int64(14400), *n.IPV6RaPreferredLifetime)
}
