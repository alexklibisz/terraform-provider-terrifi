package provider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"
	"github.com/ubiquiti-community/go-unifi/unifi"
)

func clientDeviceImportState(t *testing.T, r *clientDeviceResource, id string) resource.ImportStateResponse {
	t.Helper()
	var schema resource.SchemaResponse
	r.Schema(t.Context(), resource.SchemaRequest{}, &schema)
	state := tfsdk.State{Schema: schema.Schema}
	diags := state.Set(t.Context(), &clientDeviceResourceModel{ClientGroupIDs: types.SetNull(types.StringType)})
	require.False(t, diags.HasError(), diags)
	resp := resource.ImportStateResponse{State: state}
	r.ImportState(t.Context(), resource.ImportStateRequest{ID: id}, &resp)
	return resp
}

func TestClientDeviceImportAndRead(t *testing.T) {
	const mac = "aa:bb:cc:dd:ee:ff"
	for _, tc := range []struct {
		input, site string
		byMAC       bool
	}{
		{"client-id", "fallback", false}, {"office:client-id", "office", false},
		{strings.ToUpper(mac), "fallback", true}, {"office:" + mac, "office", true},
		{mac, "fallback", true}, {"office:" + strings.ToUpper(mac), "office", true},
	} {
		t.Run(tc.input, func(t *testing.T) {
			var paths []string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if strings.Contains(req.URL.Path, "/v2/") {
					fmt.Fprint(w, `{}`)
					return
				}
				paths = append(paths, req.URL.Path)
				fmt.Fprint(w, `{"meta":{"rc":"ok"},"data":[{"_id":"client-id","mac":"aa:bb:cc:dd:ee:ff","name":"Printer"}]}`)
			}))
			defer srv.Close()
			c := newTestClient(t, srv.URL, false)
			c.Site = "fallback"
			r := &clientDeviceResource{client: c}
			imported := clientDeviceImportState(t, r, tc.input)
			require.False(t, imported.Diagnostics.HasError(), imported.Diagnostics)
			state := imported.State
			for range 2 {
				resp := resource.ReadResponse{State: state}
				r.Read(t.Context(), resource.ReadRequest{State: state}, &resp)
				require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)
				var model clientDeviceResourceModel
				require.False(t, resp.State.Get(t.Context(), &model).HasError())
				require.Equal(t, "client-id", model.ID.ValueString())
				require.Equal(t, tc.site, model.Site.ValueString())
				require.Equal(t, mac, model.MAC.ValueString())
				state = resp.State
			}
			direct := "/proxy/network/api/s/" + tc.site + "/rest/user/client-id"
			first := direct
			if tc.byMAC {
				first = strings.TrimSuffix(direct, "/client-id")
			}
			require.Equal(t, []string{first, direct}, paths)
		})
	}
}

func TestClientDeviceImportInvalid(t *testing.T) {
	for _, id := range []string{"", "a:b:c", ":client-id", "office:", "gg:bb:cc:dd:ee:ff", "office:gg:bb:cc:dd:ee:ff", " aa:bb:cc:dd:ee:ff", "office:aa:bb:cc:dd:ee"} {
		t.Run(id, func(t *testing.T) {
			resp := clientDeviceImportState(t, &clientDeviceResource{}, id)
			require.True(t, resp.Diagnostics.HasError(), "invalid import %q", id)
			require.Contains(t, resp.Diagnostics.Errors()[0].Detail(), "site:MAC")
		})
	}
}

func TestClientDeviceMACLookup(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		wantID     string
		missing    bool
	}{
		{"multiple clients", `{"meta":{"rc":"ok"},"data":[{"_id":"other","mac":"11:22:33:44:55:66"},{"_id":"target","mac":"AA:BB:CC:DD:EE:FF"}]}`, 200, "target", false},
		{"single unrelated", `{"meta":{"rc":"ok"},"data":[{"_id":"other","mac":"11:22:33:44:55:66"}]}`, 200, "", true},
		{"empty", `{"meta":{"rc":"ok"},"data":[]}`, 200, "", true},
		{"metadata error", `{"meta":{"rc":"error","msg":"api.err.NoSiteContext"},"data":[]}`, 200, "", false},
		{"metadata error with data", `{"meta":{"rc":"error","msg":"api.err.NoSiteContext"},"data":[{"_id":"target","mac":"aa:bb:cc:dd:ee:ff"}]}`, 200, "", false},
		{"HTTP error", `{}`, 500, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				require.Equal(t, "/proxy/network/api/s/office/rest/user", req.URL.Path)
				// Deliberately return the same list regardless of any MAC query.
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer srv.Close()
			got, err := newTestClient(t, srv.URL, false).GetClientDeviceByMAC(t.Context(), "office", "aa:bb:cc:dd:ee:ff")
			if tc.wantID != "" {
				require.NoError(t, err)
				require.Equal(t, tc.wantID, got.ID)
				return
			}
			require.Error(t, err)
			var missing *unifi.NotFoundError
			if tc.missing {
				require.ErrorAs(t, err, &missing)
			} else {
				require.NotErrorAs(t, err, &missing)
			}
		})
	}
}

func TestClientDeviceReadLookupErrors(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		missing    bool
	}{
		{"metadata error", `{"meta":{"rc":"error","msg":"api.err.NoSiteContext"},"data":[]}`, 200, false},
		{"HTTP error", `{}`, 500, false},
		{"missing client", `{"meta":{"rc":"ok"},"data":[]}`, 200, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer srv.Close()
			r := &clientDeviceResource{client: newTestClient(t, srv.URL, false)}
			imported := clientDeviceImportState(t, r, "aa:bb:cc:dd:ee:ff")
			require.False(t, imported.Diagnostics.HasError())
			resp := resource.ReadResponse{State: imported.State}
			r.Read(t.Context(), resource.ReadRequest{State: imported.State}, &resp)
			if tc.missing {
				require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)
				require.True(t, resp.State.Raw.IsNull(), "a missing client must be removed from state")
			} else {
				require.True(t, resp.Diagnostics.HasError())
				require.True(t, imported.State.Raw.Equal(resp.State.Raw), "controller errors must not remove the resource")
				if tc.name == "metadata error" {
					require.Contains(t, resp.Diagnostics.Errors()[0].Detail(), "api.err.NoSiteContext")
				}
			}
		})
	}
}
