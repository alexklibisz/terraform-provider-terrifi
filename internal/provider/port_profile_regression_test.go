package provider

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"
)

func TestSettleFreshTaggedNetworks_BypassesCachedList(t *testing.T) {
	fresh := freshNetworkID(time.Now())
	fake, r, m, lan := setupSettleTest(t, fresh)
	r.client.cache = newResponseCache()
	sent := mustModelToAPI(t, &m, lan)
	written, err := r.client.CreatePortProfile(t.Context(), "default", sent)
	require.NoError(t, err)
	sent.ID = written.ID
	// A concurrent resource reads the same list before the background job.
	_, err = r.client.GetPortProfile(t.Context(), "default", sent.ID)
	require.NoError(t, err)
	fake.mu.Lock()
	fake.excludeAfterWrite = []string{fresh}
	fake.runExcludeJob(sent.ID)
	fake.mu.Unlock()
	_, err = r.settleFreshTaggedNetworks(t.Context(), "default", &m, sent, written)
	require.NoError(t, err)
	r.client.cache.invalidateAll()
	actual, err := r.client.GetPortProfile(t.Context(), "default", sent.ID)
	require.NoError(t, err)
	require.NotContains(t, actual.ExcludedNetworkIDs, fresh, "settle must repair the controller state even when another read primed the cache")
}

// A successful POST must survive a failure in the asynchronous verification.
func TestPortProfileCreateRetainsStateOnVerificationError(t *testing.T) {
	fresh := freshNetworkID(time.Now())
	fake, r, plan, _ := setupSettleTest(t, fresh)
	var schemaResp resource.SchemaResponse
	r.Schema(t.Context(), resource.SchemaRequest{}, &schemaResp)
	plan.ID = types.StringUnknown()
	plan.Site = types.StringValue("default")
	state := tfsdk.State{Schema: schemaResp.Schema}
	require.False(t, state.Set(t.Context(), &plan).HasError())
	// An already canceled context cannot POST, so cancel immediately after POST
	// returns using a RoundTripper that cancels only when its body is closed.
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	transport := r.client.HTTP.HTTPClient.Transport
	r.client.HTTP.HTTPClient.Transport = cancelPostTransport{base: transport, cancel: cancel}
	var resp resource.CreateResponse
	resp.State = tfsdk.State{Schema: schemaResp.Schema}
	r.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: state.Raw}}, &resp)
	require.True(t, resp.Diagnostics.HasError())
	require.Len(t, fake.profiles, 1)
	var saved portProfileResourceModel
	require.False(t, resp.State.Get(t.Context(), &saved).HasError())
	require.NotEmpty(t, saved.ID.ValueString())
	require.Contains(t, fake.profiles, saved.ID.ValueString())
}

type cancelPostTransport struct {
	base   http.RoundTripper
	cancel context.CancelFunc
}

func (c cancelPostTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := c.base
	if base == nil {
		base = http.DefaultTransport
	}
	resp, err := base.RoundTrip(req)
	if err == nil && req.Method == http.MethodPost {
		resp.Body = cancelOnClose{ReadCloser: resp.Body, cancel: c.cancel}
	}
	return resp, err
}

type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c cancelOnClose) Close() error { err := c.ReadCloser.Close(); c.cancel(); return err }
