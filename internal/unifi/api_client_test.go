package unifi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthentication(t *testing.T) {
	for _, tc := range []struct{ name, prefix, loginPath, key string }{
		{"legacy session", "", "/api/login", ""},
		{"UniFi OS session", "/proxy/network", "/api/auth/login", ""},
		{"API key takes precedence", "/proxy/network", "", "test-key"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var logins atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/":
					if tc.prefix == "" {
						w.Header().Set("Location", "/manage")
						w.WriteHeader(http.StatusFound)
					}
				case "/api/login", "/api/auth/login":
					logins.Add(1)
					assert.Equal(t, tc.loginPath, r.URL.Path)
					var credentials map[string]string
					require.NoError(t, json.NewDecoder(r.Body).Decode(&credentials))
					assert.Equal(t, "admin", credentials["username"])
					assert.Equal(t, "password", credentials["password"])
					http.SetCookie(w, &http.Cookie{Name: "session", Value: "authenticated", Path: "/"})
					w.Header().Set("X-Updated-Csrf-Token", "csrf-token")
					_, _ = io.WriteString(w, `{"meta":{"rc":"ok"}}`)
				case tc.prefix + "/api/self/sites":
					if tc.key != "" {
						assert.Equal(t, tc.key, r.Header.Get("X-API-Key"))
					} else {
						cookie, err := r.Cookie("session")
						require.NoError(t, err)
						assert.Equal(t, "authenticated", cookie.Value)
						assert.Equal(t, "csrf-token", r.Header.Get("X-Csrf-Token"))
					}
					_, _ = io.WriteString(w, `{"meta":{"rc":"ok"},"data":[{"_id":"site-id","name":"default"}]}`)
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			client, err := New(t.Context(), Config{APIURL: server.URL + "/", Username: "admin", Password: "password", APIKey: tc.key})
			require.NoError(t, err)
			assert.Equal(t, tc.prefix, client.APIPath)
			assert.Equal(t, "default", client.Site)
			sites, err := client.ListSites(t.Context())
			require.NoError(t, err)
			require.Len(t, sites, 1)
			wantLogins := int64(1)
			if tc.key != "" {
				wantLogins = 0
			}
			assert.Equal(t, wantLogins, logins.Load(), "one shared session for every endpoint")
		})
	}
}

func TestLoginRejectsControllerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusFound)
			return
		}
		_, _ = io.WriteString(w, `{"meta":{"rc":"error","msg":"api.err.Invalid"}}`)
	}))
	defer server.Close()
	_, err := New(t.Context(), Config{APIURL: server.URL, Username: "admin", Password: "invalid"})
	require.ErrorContains(t, err, "api.err.Invalid")
}

func TestTransportStatusesAndCancellation(t *testing.T) {
	for _, status := range []int{http.StatusCreated, http.StatusNoContent, http.StatusBadRequest} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) }))
			defer server.Close()
			client := &APIClient{HTTP: newRetryableHTTPClient(false)}
			err := client.Do(t.Context(), http.MethodPost, server.URL, map[string]string{"x_passphrase": "private-passphrase"}, nil)
			if status < 300 {
				require.NoError(t, err)
			} else {
				var httpErr *HTTPError
				require.ErrorAs(t, err, &httpErr)
				assert.Equal(t, status, httpErr.StatusCode)
				assert.NotContains(t, err.Error(), "private-passphrase")
			}
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			require.ErrorIs(t, client.Do(ctx, http.MethodGet, server.URL, nil, nil), context.Canceled)
		})
	}
}

func TestRadioWritePreservesUnknownSettings(t *testing.T) {
	var radio DeviceRadioTable
	require.NoError(t, json.Unmarshal([]byte(`{"radio":"na","channel":149,"tx_power":18,"ht":"80","min_rssi_enabled":true,"antenna_gain":4,"controller_future_setting":{"enabled":true}}`), &radio))
	radio.Channel = "36"
	radio.MinRssiEnabled = false
	b, err := json.Marshal(radio)
	require.NoError(t, err)
	var actual map[string]any
	require.NoError(t, json.Unmarshal(b, &actual))
	assert.Equal(t, "36", actual["channel"])
	assert.Equal(t, false, actual["min_rssi_enabled"])
	assert.Equal(t, float64(4), actual["antenna_gain"])
	assert.Equal(t, map[string]any{"enabled": true}, actual["controller_future_setting"])
}

func TestWLANUpdatePreservesUnmanagedSettings(t *testing.T) {
	for _, emptyPut := range []bool{false, true} {
		t.Run(map[bool]string{false: "object response", true: "empty PUT response"}[emptyPut], func(t *testing.T) {
			var updated map[string]json.RawMessage
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/proxy/network/api/s/default/rest/wlanconf/wlan-1", r.URL.Path)
				if r.Method == http.MethodGet {
					_, _ = io.WriteString(w, `{"meta":{"rc":"ok"},"data":[{"_id":"wlan-1","name":"before","enabled":true,"vlan_enabled":true,"schedule_with_duration":[{"start":60,"end":120}],"controller_future_setting":{"nested":true}}]}`)
					return
				}
				require.Equal(t, http.MethodPut, r.Method)
				require.NoError(t, json.NewDecoder(r.Body).Decode(&updated))
				if emptyPut {
					_, _ = io.WriteString(w, `{"meta":{"rc":"ok"},"data":[]}`)
					return
				}
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"meta": map[string]string{"rc": "ok"}, "data": []any{updated}}))
			}))
			defer server.Close()
			client := newTestClient(t, server.URL, false)
			_, err := client.UpdateWLAN(t.Context(), "default", &WLAN{ID: "wlan-1", Name: "after", Enabled: false})
			require.NoError(t, err)
			assert.JSONEq(t, `"after"`, string(updated["name"]))
			assert.JSONEq(t, `false`, string(updated["enabled"]))
			assert.JSONEq(t, `true`, string(updated["vlan_enabled"]))
			assert.JSONEq(t, `[{"start":60,"end":120}]`, string(updated["schedule_with_duration"]))
			assert.JSONEq(t, `{"nested":true}`, string(updated["controller_future_setting"]))
		})
	}
}
