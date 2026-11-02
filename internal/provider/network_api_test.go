package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
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

// TestUpdateNetworkManagedFreshRead exercises the HTTP update path with stale
// cached data, a disabled network, and UI changes between consecutive updates.
func TestUpdateNetworkManagedFreshRead(t *testing.T) {
	for _, emptyPutResponse := range []bool{false, true} {
		t.Run(fmt.Sprintf("empty PUT response=%t", emptyPutResponse), func(t *testing.T) {
			stored := map[string]any{
				"_id": "net-1", "purpose": "corporate", "name": "original", "enabled": false,
				"mdns_enabled": true, "domain_name": "ui.example",
				"ipv6_interface_type": "pd", "ipv6_pd_interface": "wan",
				"ipv6_ra_preferred_lifetime": "14400",
			}
			var mu sync.Mutex
			gets, puts := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				if req.URL.Path != "/proxy/network/api/s/default/rest/networkconf/net-1" {
					http.NotFound(w, req)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				switch req.Method {
				case http.MethodGet:
					gets++
				case http.MethodPut:
					puts++
					var body map[string]any
					if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
						http.Error(w, err.Error(), http.StatusBadRequest)
						return
					}
					for _, key := range []string{"enabled", "mdns_enabled", "domain_name", "ipv6_interface_type", "ipv6_pd_interface", "ipv6_ra_preferred_lifetime"} {
						assert.Equal(t, stored[key], body[key], "preserve %s", key)
					}
					stored = body
					if emptyPutResponse {
						_, _ = w.Write([]byte(`{"meta":{"rc":"ok"},"data":[]}`))
						return
					}
				default:
					http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"meta": map[string]string{"rc": "ok"}, "data": []any{stored}})
			}))
			defer server.Close()
			client := &Client{BaseURL: server.URL, APIPath: "/proxy/network", HTTP: newRetryableHTTPClient(false), cache: newResponseCache()}
			url := client.networkURL("default", "net-1")
			// A stale refresh result must not overwrite the latest controller settings.
			client.cache.set(url, []byte(`{"meta":{"rc":"ok"},"data":[{"_id":"net-1","purpose":"corporate","enabled":true,"domain_name":"stale.example"}]}`))
			for _, name := range []string{"renamed", "renamed-again"} {
				updated, err := client.UpdateNetworkManaged(t.Context(), "default", "net-1", map[string]any{"name": name})
				require.NoError(t, err)
				require.NotNil(t, updated.Name)
				assert.Equal(t, name, *updated.Name)
				assert.False(t, updated.Enabled)
				require.NotNil(t, updated.IPV6RaPreferredLifetime)
				assert.Equal(t, int64(14400), *updated.IPV6RaPreferredLifetime)
				// Re-prime the cache before a UI edit that does not invalidate it.
				_, err = client.GetNetwork(t.Context(), "default", "net-1")
				require.NoError(t, err)
				mu.Lock()
				stored["domain_name"] = "changed-in-ui.example"
				stored["mdns_enabled"] = false
				mu.Unlock()
			}
			mu.Lock()
			defer mu.Unlock()
			assert.Equal(t, 2, puts)
			assert.GreaterOrEqual(t, gets, 2, "each update must read the controller")
		})
	}
}
