package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/require"
)

// The first Read after ImportState must report every custom policy of the zone
// pair in controller order, so the first plan after import shows a reorder
// rather than adding the whole list (#265). Later Reads keep only the configured
// policies. Exercise the real ImportState and Read methods against a fake API.
func TestFirewallPolicyOrderImportReadsFullOrder(t *testing.T) {
	const src, dst = "zone-src", "zone-dst"
	pair := func(id string, index int, predefined bool, from string) map[string]any {
		return map[string]any{
			"_id": id, "index": index, "predefined": predefined, "action": "ALLOW",
			"source":      map[string]string{"zone_id": from},
			"destination": map[string]string{"zone_id": dst},
		}
	}
	// Listed out of index order on purpose: the controller's order is the index.
	policies := []map[string]any{
		pair("p3", 10003, false, src),
		pair("p1", 10001, false, src),
		pair("other-pair", 10000, false, "zone-other"),
		pair("predefined", 30000, true, src),
		pair("p2", 10002, false, src),
	}
	writes := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			writes++
			http.Error(w, "unexpected write", http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(policies)
	}))
	defer srv.Close()

	r := &firewallPolicyOrderResource{client: newTestClient(t, srv.URL, false)}
	var schemaResp resource.SchemaResponse
	r.Schema(t.Context(), resource.SchemaRequest{}, &schemaResp)

	stateWith := func(t *testing.T, policyIDs types.List) tfsdk.State {
		t.Helper()
		state := tfsdk.State{Schema: schemaResp.Schema}
		m := firewallPolicyOrderResourceModel{
			ID:                types.StringValue(src + ":" + dst),
			Site:              types.StringNull(),
			SourceZoneID:      types.StringValue(src),
			DestinationZoneID: types.StringValue(dst),
			PolicyIDs:         policyIDs,
		}
		require.False(t, state.Set(t.Context(), &m).HasError())
		return state
	}
	// ImportState receives a null object of the schema type, as from the framework.
	nullState := func() tfsdk.State {
		return tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(schemaResp.Schema.Type().TerraformType(t.Context()), nil)}
	}
	read := func(t *testing.T, state tfsdk.State) []string {
		t.Helper()
		resp := resource.ReadResponse{State: state}
		r.Read(t.Context(), resource.ReadRequest{State: state}, &resp)
		require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)
		var m firewallPolicyOrderResourceModel
		require.False(t, resp.State.Get(t.Context(), &m).HasError())
		require.False(t, m.PolicyIDs.IsNull(), "policy_ids must be known after Read")
		var ids []string
		require.False(t, m.PolicyIDs.ElementsAs(t.Context(), &ids, false).HasError())
		return ids
	}
	list := func(ids ...string) types.List {
		vals := make([]attr.Value, len(ids))
		for i, id := range ids {
			vals[i] = types.StringValue(id)
		}
		return types.ListValueMust(types.StringType, vals)
	}

	t.Run("first read after import reports the full order", func(t *testing.T) {
		importResp := resource.ImportStateResponse{State: nullState()}
		r.ImportState(t.Context(), resource.ImportStateRequest{ID: src + ":" + dst}, &importResp)
		require.False(t, importResp.Diagnostics.HasError(), importResp.Diagnostics)
		require.Equal(t, []string{"p1", "p2", "p3"}, read(t, importResp.State))
	})

	t.Run("site-qualified import reports the full order", func(t *testing.T) {
		importResp := resource.ImportStateResponse{State: nullState()}
		r.ImportState(t.Context(), resource.ImportStateRequest{ID: "default:" + src + ":" + dst}, &importResp)
		require.False(t, importResp.Diagnostics.HasError(), importResp.Diagnostics)
		require.Equal(t, []string{"p1", "p2", "p3"}, read(t, importResp.State))
	})

	t.Run("later reads keep only configured policies, in controller order", func(t *testing.T) {
		// A configured order that differs from the controller reads back in
		// controller order, so the plan shows the reorder; p2 stays unmanaged.
		require.Equal(t, []string{"p1", "p3"}, read(t, stateWith(t, list("p3", "p1"))))
	})

	t.Run("an empty configured list stays empty", func(t *testing.T) {
		require.Empty(t, read(t, stateWith(t, list())))
	})

	require.Zero(t, writes, "import and read must not write to the controller")
}
