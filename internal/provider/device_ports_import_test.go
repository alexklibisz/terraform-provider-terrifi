package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"
)

// Imported unsupported ports must remain outside the owned port map, including
// during an update followed by destroy. Check the actual resource methods and
// whole-array writes, not just the parser's return value.
func TestDevicePortsImportExcludesUnsupportedPorts(t *testing.T) {
	for _, mixed := range []bool{false, true} {
		t.Run(fmt.Sprint("mixed=", mixed), func(t *testing.T) {
			const mac = "00:00:5e:00:53:02"
			const unsupported = `{"port_idx":3,"name":"Server","native_networkconf_id":"lan","tagged_vlan_mgmt":"custom","excluded_networkconf_ids":["private"],"poe_mode":"off"}`
			stored := overridesJSON(t, `[`+unsupported+`]`)
			if mixed {
				stored = append(stored, overridesJSON(t, `[{"port_idx":2,"name":"Office"}]`)[0])
			}
			writes := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				switch req.Method {
				case http.MethodGet:
					_ = json.NewEncoder(w).Encode(map[string]any{"meta": map[string]string{"rc": "ok"}, "data": []any{map[string]any{"_id": "device", "mac": mac, "port_overrides": stored, "port_table": []any{map[string]int{"port_idx": 2}, map[string]int{"port_idx": 3}}}}})
				case http.MethodPut:
					var body struct {
						Overrides []map[string]json.RawMessage `json:"port_overrides"`
					}
					if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
						http.Error(w, err.Error(), 400)
						return
					}
					stored = body.Overrides
					writes++
					_, _ = w.Write([]byte(`{"meta":{"rc":"ok"},"data":[]}`))
				}
			}))
			defer srv.Close()
			r := &devicePortsResource{client: newTestClient(t, srv.URL, false)}
			var schemaResp resource.SchemaResponse
			r.Schema(t.Context(), resource.SchemaRequest{}, &schemaResp)
			state := tfsdk.State{Schema: schemaResp.Schema}
			initial := devicePortsResourceModel{}
			require.False(t, state.Set(t.Context(), &initial).HasError())
			importResp := resource.ImportStateResponse{State: state}
			r.ImportState(t.Context(), resource.ImportStateRequest{ID: "default:" + mac}, &importResp)
			require.False(t, importResp.Diagnostics.HasError(), importResp.Diagnostics)
			state = importResp.State
			var read resource.ReadResponse
			read.State = state
			r.Read(t.Context(), resource.ReadRequest{State: state}, &read)
			require.False(t, read.Diagnostics.HasError(), read.Diagnostics)
			require.Len(t, read.Diagnostics.Warnings(), 1)
			require.Contains(t, read.Diagnostics.Warnings()[0].Detail(), "Port 3 was excluded")
			var imported devicePortsResourceModel
			require.False(t, read.State.Get(t.Context(), &imported).HasError())
			require.NotContains(t, imported.Ports, "3")
			require.Zero(t, writes, "import must be read-only")
			planned := imported
			if mixed {
				port := planned.Ports["2"]
				port.Name = types.StringValue("Renamed")
				planned.Ports["2"] = port
			}
			planState := tfsdk.State{Schema: schemaResp.Schema}
			require.False(t, planState.Set(t.Context(), &planned).HasError())
			update := resource.UpdateResponse{State: read.State}
			r.Update(t.Context(), resource.UpdateRequest{State: read.State, Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: planState.Raw}}, &update)
			require.False(t, update.Diagnostics.HasError(), update.Diagnostics)
			var refresh resource.ReadResponse
			refresh.State = update.State
			r.Read(t.Context(), resource.ReadRequest{State: update.State}, &refresh)
			require.False(t, refresh.Diagnostics.HasError(), refresh.Diagnostics)
			require.Empty(t, refresh.Diagnostics.Warnings(), "normal refresh should not repeat import warnings")
			var refreshed devicePortsResourceModel
			require.False(t, refresh.State.Get(t.Context(), &refreshed).HasError())
			require.NotContains(t, refreshed.Ports, "3")
			var deleted resource.DeleteResponse
			r.Delete(t.Context(), resource.DeleteRequest{State: refresh.State}, &deleted)
			require.False(t, deleted.Diagnostics.HasError(), deleted.Diagnostics)
			require.Len(t, stored, 1)
			actual, _ := json.Marshal(stored[0])
			require.JSONEq(t, unsupported, string(actual), "unsupported port must survive update and destroy unchanged")
			if mixed {
				require.Equal(t, 2, writes)
			} else {
				require.Zero(t, writes, "empty ownership must never PUT the override array")
			}
		})
	}
}
