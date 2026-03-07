package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ubiquiti-community/go-unifi/unifi"
)

// newTestClient creates a Client pointing at the given test server with
// optional response caching enabled.
func newTestClient(t *testing.T, serverURL string, enableCache bool) *Client {
	t.Helper()
	httpClient := retryablehttp.NewClient()
	httpClient.Logger = nil
	httpClient.RetryMax = 0

	var cache *responseCache
	if enableCache {
		cache = newResponseCache()
	}

	return &Client{
		BaseURL: serverURL,
		APIPath: "/proxy/network",
		HTTP:    httpClient,
		cache:   cache,
	}
}

// --- Firewall Zones ---

func TestResponseCaching_FirewallZone_CachesListAll(t *testing.T) {
	var hits atomic.Int64

	zones := []unifi.FirewallZone{
		{ID: "zone-1", Name: "LAN"},
		{ID: "zone-2", Name: "WAN"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(zones)
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL, true)
	ctx := context.Background()

	z1, err := client.GetFirewallZone(ctx, "default", "zone-1")
	require.NoError(t, err)
	assert.Equal(t, "LAN", z1.Name)

	z2, err := client.GetFirewallZone(ctx, "default", "zone-2")
	require.NoError(t, err)
	assert.Equal(t, "WAN", z2.Name)

	assert.Equal(t, int64(1), hits.Load(), "expected 1 server hit (second call should be cached)")
}

func TestResponseCaching_FirewallZone_InvalidatesOnWrite(t *testing.T) {
	var hits atomic.Int64

	zones := []unifi.FirewallZone{
		{ID: "zone-1", Name: "LAN"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(zones)
		case http.MethodPost:
			json.NewEncoder(w).Encode(unifi.FirewallZone{ID: "zone-new", Name: "DMZ"})
		}
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL, true)
	ctx := context.Background()

	// First GET — cache miss, hits server
	_, err := client.GetFirewallZone(ctx, "default", "zone-1")
	require.NoError(t, err)
	assert.Equal(t, int64(1), hits.Load())

	// POST — invalidates cache
	_, err = client.CreateFirewallZone(ctx, "default", &unifi.FirewallZone{Name: "DMZ"})
	require.NoError(t, err)
	assert.Equal(t, int64(2), hits.Load())

	// Second GET — cache was invalidated, hits server again
	_, err = client.GetFirewallZone(ctx, "default", "zone-1")
	require.NoError(t, err)
	assert.Equal(t, int64(3), hits.Load())
}

// --- Firewall Policies ---

func TestResponseCaching_FirewallPolicy_CachesListAll(t *testing.T) {
	var hits atomic.Int64

	policies := []firewallPolicyResponse{
		{ID: "pol-1", Name: "Allow DNS"},
		{ID: "pol-2", Name: "Block SSH"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(policies)
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL, true)
	ctx := context.Background()

	p1, err := client.GetFirewallPolicy(ctx, "default", "pol-1")
	require.NoError(t, err)
	assert.Equal(t, "Allow DNS", p1.Name)

	p2, err := client.GetFirewallPolicy(ctx, "default", "pol-2")
	require.NoError(t, err)
	assert.Equal(t, "Block SSH", p2.Name)

	assert.Equal(t, int64(1), hits.Load(), "expected 1 server hit (second call should be cached)")
}

func TestResponseCaching_FirewallPolicy_InvalidatesOnWrite(t *testing.T) {
	var hits atomic.Int64

	policies := []firewallPolicyResponse{
		{ID: "pol-1", Name: "Allow DNS"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(policies)
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL, true)
	ctx := context.Background()

	// First GET — cache miss
	_, err := client.GetFirewallPolicy(ctx, "default", "pol-1")
	require.NoError(t, err)
	assert.Equal(t, int64(1), hits.Load())

	// DELETE — invalidates cache
	err = client.DeleteFirewallPolicy(ctx, "default", "pol-1")
	require.NoError(t, err)
	assert.Equal(t, int64(2), hits.Load())

	// Second GET — cache invalidated, hits server
	_, err = client.GetFirewallPolicy(ctx, "default", "pol-1")
	require.NoError(t, err)
	assert.Equal(t, int64(3), hits.Load())
}

// --- Firewall Policy Ordering ---

func TestResponseCaching_FirewallPolicyOrdering_CachesListAll(t *testing.T) {
	var hits atomic.Int64

	idx0, idx1 := int64(0), int64(1)
	policies := []firewallPolicyResponse{
		{
			ID:   "pol-1",
			Name: "Allow DNS",
			Source: &firewallPolicyEndpointResponse{
				ZoneID: "zone-lan",
			},
			Destination: &firewallPolicyEndpointResponse{
				ZoneID: "zone-wan",
			},
			Index: &idx0,
		},
		{
			ID:   "pol-2",
			Name: "Block SSH",
			Source: &firewallPolicyEndpointResponse{
				ZoneID: "zone-lan",
			},
			Destination: &firewallPolicyEndpointResponse{
				ZoneID: "zone-wan",
			},
			Index: &idx1,
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(policies)
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL, true)
	ctx := context.Background()

	ids1, err := client.GetFirewallPolicyOrdering(ctx, "default", "zone-lan", "zone-wan")
	require.NoError(t, err)
	assert.Equal(t, []string{"pol-1", "pol-2"}, ids1)

	ids2, err := client.GetFirewallPolicyOrdering(ctx, "default", "zone-lan", "zone-wan")
	require.NoError(t, err)
	assert.Equal(t, []string{"pol-1", "pol-2"}, ids2)

	assert.Equal(t, int64(1), hits.Load(), "expected 1 server hit (second call should be cached)")
}

// --- Disabled by Default ---

func TestResponseCaching_DisabledByDefault(t *testing.T) {
	var hits atomic.Int64

	zones := []unifi.FirewallZone{
		{ID: "zone-1", Name: "LAN"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(zones)
	}))
	defer srv.Close()

	// Create client WITHOUT caching
	client := newTestClient(t, srv.URL, false)
	ctx := context.Background()

	_, err := client.GetFirewallZone(ctx, "default", "zone-1")
	require.NoError(t, err)

	_, err = client.GetFirewallZone(ctx, "default", "zone-1")
	require.NoError(t, err)

	assert.Equal(t, int64(2), hits.Load(), "expected 2 server hits (caching disabled)")
}

// --- Edge Cases ---

func TestResponseCaching_DifferentSitesNotCached(t *testing.T) {
	var hits atomic.Int64

	zones := []unifi.FirewallZone{
		{ID: "zone-1", Name: "LAN"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(zones)
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL, true)
	ctx := context.Background()

	_, err := client.GetFirewallZone(ctx, "site-a", "zone-1")
	require.NoError(t, err)

	_, err = client.GetFirewallZone(ctx, "site-b", "zone-1")
	require.NoError(t, err)

	assert.Equal(t, int64(2), hits.Load(), "different sites should produce different URLs and cache keys")
}

func TestResponseCaching_NotFoundStillCached(t *testing.T) {
	var hits atomic.Int64

	zones := []unifi.FirewallZone{
		{ID: "zone-1", Name: "LAN"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(zones)
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL, true)
	ctx := context.Background()

	// Look for a non-existent zone — list-all succeeds but filter finds nothing
	_, err := client.GetFirewallZone(ctx, "default", "nonexistent")
	require.Error(t, err)
	assert.IsType(t, &unifi.NotFoundError{}, err)

	// Second lookup for a different non-existent zone — should use cached list
	_, err = client.GetFirewallZone(ctx, "default", "also-nonexistent")
	require.Error(t, err)

	assert.Equal(t, int64(1), hits.Load(), "list-all response should be cached even when ID not found")
}

func TestResponseCaching_V1RequestsCachedThroughDoV2(t *testing.T) {
	var hits atomic.Int64

	clientResp := struct {
		Meta json.RawMessage `json:"meta"`
		Data []unifi.Client  `json:"data"`
	}{
		Data: []unifi.Client{
			{ID: "client-1", MAC: "aa:bb:cc:dd:ee:ff", Name: "Test Client"},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(clientResp)
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL, true)
	ctx := context.Background()

	// doV1Request calls doV2Request under the hood, so caching applies
	c1, err := client.GetClientDevice(ctx, "default", "client-1")
	require.NoError(t, err)
	assert.Equal(t, "Test Client", c1.Name)

	c2, err := client.GetClientDevice(ctx, "default", "client-1")
	require.NoError(t, err)
	assert.Equal(t, "Test Client", c2.Name)

	assert.Equal(t, int64(1), hits.Load(),
		fmt.Sprintf("expected 1 server hit for v1 GET-by-ID (doV1Request delegates to doV2Request), got %d", hits.Load()))
}
