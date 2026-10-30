package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"
	"github.com/ubiquiti-community/go-unifi/unifi"
)

type policySettleAPI struct {
	mu           sync.Mutex
	stored       firewallPolicyResponse
	methods      []string
	ignoreRepair bool
	failRead     bool
}

func (api *policySettleAPI) serveHTTP(w http.ResponseWriter, req *http.Request) {
	api.mu.Lock()
	defer api.mu.Unlock()
	api.methods = append(api.methods, req.Method)
	w.Header().Set("Content-Type", "application/json")
	if req.Method == http.MethodGet {
		if api.failRead {
			http.Error(w, "verification failed", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode([]firewallPolicyResponse{api.stored})
		return
	}
	var body firewallPolicyResponse
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Method == http.MethodPost {
		body.ID = "policy-1"
		index := int64(10000)
		body.Index = &index
	}
	if !api.ignoreRepair {
		api.stored = body
	}
	_ = json.NewEncoder(w).Encode(body)
}

func setupPolicySettleTest(t *testing.T) (*policySettleAPI, *firewallPolicyResource, *unifi.FirewallPolicy, *firewallPolicyScheduleRequest, *firewallPolicyFull) {
	t.Helper()
	previousDelay := firewallPolicySettleDelay
	firewallPolicySettleDelay = 0
	t.Cleanup(func() { firewallPolicySettleDelay = previousDelay })
	api := &policySettleAPI{}
	server := httptest.NewServer(http.HandlerFunc(api.serveHTTP))
	t.Cleanup(server.Close)
	r := &firewallPolicyResource{client: &Client{BaseURL: server.URL, HTTP: newRetryableHTTPClient(false)}}
	sent := &unifi.FirewallPolicy{
		Name: "Network match", Action: "ALLOW", Enabled: true,
		Source: &unifi.FirewallPolicySource{
			ZoneID: "zone-src", MatchingTarget: "NETWORK", PortMatchingType: "ANY",
			IPs: []string{freshNetworkID(time.Now())}, MatchOppositeIPs: true,
		},
		Destination: &unifi.FirewallPolicyDestination{
			ZoneID: "zone-dst", MatchingTarget: "NETWORK", PortMatchingType: "ANY",
			IPs: []string{freshNetworkID(time.Now().Add(-time.Second))},
		},
	}
	schedule := &firewallPolicyScheduleRequest{Mode: "CUSTOM", DateStart: "2026-01-01", DateEnd: "2026-12-31"}
	written, err := r.client.CreateFirewallPolicy(t.Context(), "default", sent, schedule)
	require.NoError(t, err)
	sent.ID = written.ID
	return api, r, sent, schedule, written
}

func TestSettleFreshNetworkPolicyRepairsControllerChanges(t *testing.T) {
	for _, endpoint := range []string{"source", "destination", "both", "enabled only", "source inversion", "destination inversion"} {
		t.Run(endpoint, func(t *testing.T) {
			api, r, sent, schedule, written := setupPolicySettleTest(t)
			// A concurrent read may have cached the successful POST state before
			// the controller's background job strips the actual stored match.
			r.client.cache = newResponseCache()
			_, err := r.client.GetFirewallPolicy(t.Context(), "default", written.ID)
			require.NoError(t, err)
			api.mu.Lock()
			if endpoint == "source inversion" {
				api.stored.Source.MatchOppositeNetworks = false
			} else if endpoint == "destination inversion" {
				api.stored.Destination.MatchOppositeNetworks = true
			} else {
				api.stored.Enabled = false
			}
			if endpoint == "source" || endpoint == "both" {
				api.stored.Source.NetworkIDs = nil
			}
			if endpoint == "destination" || endpoint == "both" {
				api.stored.Destination.NetworkIDs = nil
			}
			api.mu.Unlock()

			got, err := r.settleFreshNetworkPolicy(t.Context(), "default", sent, schedule, written)
			require.NoError(t, err)
			require.True(t, networkPolicyMatches(sent, got.FirewallPolicy))
			api.mu.Lock()
			defer api.mu.Unlock()
			require.True(t, networkPolicyMatches(sent, api.stored.toSDK()), "verify the persisted controller state, not just the PUT response")
			require.Equal(t, schedule, api.stored.Schedule, "repair must preserve fields missing from the SDK schedule")
			require.Equal(t, written.Index, api.stored.Index, "repair must preserve the controller-assigned policy index")
			require.Equal(t, []string{"POST", "GET", "GET", "PUT", "GET"}, api.methods)
		})
	}
}

func TestSettleFreshNetworkPolicyPreservesDisabledPolicy(t *testing.T) {
	api, r, sent, schedule, written := setupPolicySettleTest(t)
	sent.Enabled = false
	written.Enabled = false
	api.mu.Lock()
	api.stored.Enabled = false
	api.stored.Source.NetworkIDs = nil
	api.mu.Unlock()
	got, err := r.settleFreshNetworkPolicy(t.Context(), "default", sent, schedule, written)
	require.NoError(t, err)
	require.False(t, got.Enabled)
	require.True(t, networkPolicyMatches(sent, got.FirewallPolicy))
}

func TestSettleFreshNetworkPolicyNoRepairWhenUnchanged(t *testing.T) {
	api, r, sent, schedule, written := setupPolicySettleTest(t)
	got, err := r.settleFreshNetworkPolicy(t.Context(), "default", sent, schedule, written)
	require.NoError(t, err)
	require.True(t, networkPolicyMatches(sent, got.FirewallPolicy))
	api.mu.Lock()
	defer api.mu.Unlock()
	require.Equal(t, []string{"POST", "GET"}, api.methods)
}

func TestSettleFreshNetworkPolicySkipsEstablishedAndOtherTargets(t *testing.T) {
	for _, target := range []string{"NETWORK", "IP", "MAC", "CLIENT", "ANY"} {
		t.Run(target, func(t *testing.T) {
			api, r, sent, schedule, written := setupPolicySettleTest(t)
			sent.Source.MatchingTarget = target
			sent.Destination.MatchingTarget = target
			if target == "NETWORK" {
				sent.Source.IPs = []string{freshNetworkID(time.Now().Add(-2 * time.Minute))}
				sent.Destination.IPs = sent.Source.IPs
			}
			got, err := r.settleFreshNetworkPolicy(t.Context(), "default", sent, schedule, written)
			require.NoError(t, err)
			require.Same(t, written, got)
			api.mu.Lock()
			defer api.mu.Unlock()
			require.Equal(t, []string{"POST"}, api.methods)
		})
	}
}

func TestSettleFreshNetworkPolicyReportsUnrepairedState(t *testing.T) {
	api, r, sent, schedule, written := setupPolicySettleTest(t)
	api.mu.Lock()
	api.stored.Destination.NetworkIDs = nil
	api.ignoreRepair = true
	api.mu.Unlock()
	_, err := r.settleFreshNetworkPolicy(t.Context(), "default", sent, schedule, written)
	require.ErrorContains(t, err, "did not retain network matches")
	api.mu.Lock()
	defer api.mu.Unlock()
	require.Equal(t, []string{"POST", "GET", "PUT", "GET"}, api.methods, "repair is bounded even when the controller keeps dropping the match")
}

func TestSettleFreshNetworkPolicyCancellationAndReadError(t *testing.T) {
	t.Run("cancellation", func(t *testing.T) {
		_, r, sent, schedule, written := setupPolicySettleTest(t)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err := r.settleFreshNetworkPolicy(ctx, "default", sent, schedule, written)
		require.ErrorIs(t, err, context.Canceled)
	})
	t.Run("read error", func(t *testing.T) {
		api, r, sent, schedule, written := setupPolicySettleTest(t)
		api.mu.Lock()
		api.failRead = true
		api.mu.Unlock()
		_, err := r.settleFreshNetworkPolicy(t.Context(), "default", sent, schedule, written)
		require.ErrorContains(t, err, "verification failed")
	})
}

func TestFirewallPolicyCreateRetainsIDOnVerificationError(t *testing.T) {
	api, r, _, _, written := setupPolicySettleTest(t)
	var schemaResp resource.SchemaResponse
	r.Schema(t.Context(), resource.SchemaRequest{}, &schemaResp)
	var plan firewallPolicyResourceModel
	r.apiToModel(written, &plan, "default")
	plan.ID = types.StringUnknown()
	state := tfsdk.State{Schema: schemaResp.Schema}
	require.False(t, state.Set(t.Context(), &plan).HasError())
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	r.client.HTTP.HTTPClient.Transport = cancelPostTransport{cancel: cancel}
	var resp resource.CreateResponse
	resp.State = tfsdk.State{Schema: schemaResp.Schema}
	r.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: state.Raw}}, &resp)
	require.True(t, resp.Diagnostics.HasError())
	var saved firewallPolicyResourceModel
	require.False(t, resp.State.Get(t.Context(), &saved).HasError())
	require.Equal(t, "policy-1", saved.ID.ValueString())
	api.mu.Lock()
	defer api.mu.Unlock()
	require.Equal(t, saved.ID.ValueString(), api.stored.ID)
}
