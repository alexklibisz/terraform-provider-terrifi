package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alexklibisz/terrifi/internal/unifi"
)

// Synthetic IDs. The entry shapes copy what a UniFi Network 10.x controller
// stores (see mergePortOverrides), the values are made up.
const (
	testProfileA = "000000000000000000000a01"
	testProfileB = "000000000000000000000a02"
	testNetwork  = "000000000000000000000b01"
)

func overridesJSON(t *testing.T, s string) []map[string]json.RawMessage {
	t.Helper()
	var out []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(s), &out))
	return out
}

func mergedJSON(t *testing.T, current string, want map[int64]*portWant) string {
	t.Helper()
	out, err := mergePortOverrides(overridesJSON(t, current), want)
	require.NoError(t, err)
	b, err := json.Marshal(out)
	require.NoError(t, err)
	return string(b)
}

func TestMergePortOverrides(t *testing.T) {
	// Port 5 carries inline per-port settings made in the UI; it must survive
	// every write that doesn't manage it.
	const inline5 = `{"port_idx":5,"name":"desk","forward":"customize","native_networkconf_id":"` + testNetwork + `","poe_mode":"off"}`

	tests := []struct {
		name    string
		current string
		want    map[int64]*portWant
		expect  string
	}{
		{
			name:    "attach to a port with no entry",
			current: `[` + inline5 + `]`,
			want:    map[int64]*portWant{3: {Name: ptrTo("lab"), ProfileID: ptrTo(testProfileA)}},
			expect:  `[{"name":"lab","port_idx":3,"portconf_id":"` + testProfileA + `"},` + inline5 + `]`,
		},
		{
			name:    "attach replaces inline settings and poe_mode",
			current: `[` + inline5 + `]`,
			want:    map[int64]*portWant{5: {ProfileID: ptrTo(testProfileA)}},
			expect:  `[{"port_idx":5,"portconf_id":"` + testProfileA + `"}]`,
		},
		{
			name:    "switch profile",
			current: `[{"port_idx":1,"name":"a","portconf_id":"` + testProfileA + `","setting_preference":"auto"}]`,
			want:    map[int64]*portWant{1: {Name: ptrTo("a"), ProfileID: ptrTo(testProfileB)}},
			expect:  `[{"name":"a","port_idx":1,"portconf_id":"` + testProfileB + `"}]`,
		},
		{
			name:    "remove profile keeps name",
			current: `[{"port_idx":1,"name":"a","portconf_id":"` + testProfileA + `"}]`,
			want:    map[int64]*portWant{1: {Name: ptrTo("a")}},
			expect:  `[{"name":"a","port_idx":1}]`,
		},
		{
			name:    "rename without profile keeps inline settings",
			current: `[` + inline5 + `]`,
			want:    map[int64]*portWant{5: {Name: ptrTo("desk2")}},
			expect:  `[{"forward":"customize","name":"desk2","native_networkconf_id":"` + testNetwork + `","poe_mode":"off","port_idx":5}]`,
		},
		{
			name:    "remove name",
			current: `[` + inline5 + `]`,
			want:    map[int64]*portWant{5: {}},
			expect:  `[{"forward":"customize","native_networkconf_id":"` + testNetwork + `","poe_mode":"off","port_idx":5}]`,
		},
		{
			name:    "unmanage drops an entry left with only port_idx",
			current: `[{"port_idx":1,"name":"a","portconf_id":"` + testProfileA + `"},` + inline5 + `]`,
			want:    map[int64]*portWant{1: nil},
			expect:  `[` + inline5 + `]`,
		},
		{
			name:    "unmanage keeps inline settings",
			current: `[` + inline5 + `]`,
			want:    map[int64]*portWant{5: nil},
			expect:  `[{"forward":"customize","native_networkconf_id":"` + testNetwork + `","poe_mode":"off","port_idx":5}]`,
		},
		{
			name:    "unmanage a port with no entry",
			current: `[]`,
			want:    map[int64]*portWant{2: nil},
			expect:  `[]`,
		},
		{
			name:    "sorted by port number",
			current: `[{"port_idx":10,"name":"j"},{"port_idx":2,"name":"b"}]`,
			want:    map[int64]*portWant{9: {Name: ptrTo("i")}},
			expect:  `[{"name":"b","port_idx":2},{"name":"i","port_idx":9},{"name":"j","port_idx":10}]`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.JSONEq(t, tt.expect, mergedJSON(t, tt.current, tt.want))
		})
	}
}

func TestMergePortOverrides_BadPortIdx(t *testing.T) {
	_, err := mergePortOverrides(overridesJSON(t, `[{"name":"x"}]`), nil)
	assert.Error(t, err)
}

func testGatewayPorts() *unifi.DevicePorts {
	return &unifi.DevicePorts{
		ID:  "000000000000000000000c01",
		MAC: "00:00:5e:00:53:01",
		PortTable: []unifi.DevicePort{
			{PortIdx: 1, Ifname: "eth0"}, {PortIdx: 2, Ifname: "eth1"}, {PortIdx: 5, Ifname: "eth4"},
		},
		EthernetOverrides: []unifi.EthernetOverride{{Ifname: "eth0", Networkgroup: "LAN"}, {Ifname: "eth4", Networkgroup: "WAN"}},
	}
}

func TestCheckPortsAssignable(t *testing.T) {
	d := testGatewayPorts()
	profile := &portWant{ProfileID: ptrTo(testProfileA)}

	assert.NoError(t, checkPortsAssignable(d, map[int64]*portWant{1: profile, 2: profile}))
	assert.NoError(t, checkPortsAssignable(d, map[int64]*portWant{5: {Name: ptrTo("wan")}}), "naming a WAN port is fine")
	assert.NoError(t, checkPortsAssignable(d, map[int64]*portWant{9: nil}), "releasing a missing port is fine")
	assert.ErrorContains(t, checkPortsAssignable(d, map[int64]*portWant{5: profile}), "is a WAN port")
	assert.ErrorContains(t, checkPortsAssignable(d, map[int64]*portWant{9: {Name: ptrTo("x")}}), "has no port 9")
}

func TestPortWants(t *testing.T) {
	prior := map[string]devicePortModel{
		"1": {Name: types.StringValue("a"), PortProfileID: types.StringNull()},
		"2": {Name: types.StringNull(), PortProfileID: types.StringValue(testProfileA)},
	}
	planned := map[string]devicePortModel{
		"2": {Name: types.StringValue("b"), PortProfileID: types.StringNull()},
	}
	assert.Equal(t, map[int64]*portWant{1: nil, 2: {Name: ptrTo("b")}}, portWants(planned, prior))
}

func TestPortsToModel(t *testing.T) {
	got := map[int64]portWant{
		1: {Name: ptrTo("a"), ProfileID: ptrTo(testProfileA)},
		2: {Name: ptrTo("b")},
		3: {},
	}
	str := types.StringValue
	null := types.StringNull()

	// Managed ports only; a managed port with no entry reads as null.
	prior := map[string]devicePortModel{"1": {}, "4": {}}
	assert.Equal(t, map[string]devicePortModel{
		"1": {Name: str("a"), PortProfileID: str(testProfileA)},
		"4": {Name: null, PortProfileID: null},
	}, portsToModel(got, prior))

	// After import (nil prior): every port with a name or profile.
	assert.Equal(t, map[string]devicePortModel{
		"1": {Name: str("a"), PortProfileID: str(testProfileA)},
		"2": {Name: str("b"), PortProfileID: null},
	}, portsToModel(got, nil))
}

func TestDevicePortsSchemaValid(t *testing.T) {
	var resp fwresource.SchemaResponse
	(&devicePortsResource{}).Schema(t.Context(), fwresource.SchemaRequest{}, &resp)
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
	diags := resp.Schema.ValidateImplementation(t.Context())
	require.False(t, diags.HasError(), "%v", diags)
}

// A fake stat/device + rest/device pair: write() must GET the device, then
// PUT only port_overrides with the unmanaged entries intact.
func TestDevicePortsWrite(t *testing.T) {
	const mac = "00:00:5e:00:53:02"
	stored := `[{"port_idx":5,"name":"desk","forward":"customize"}]`
	var putBody map[string]json.RawMessage

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/proxy/network/api/s/default/stat/device/"+mac:
			_, _ = io.WriteString(w, `{"meta":{"rc":"ok"},"data":[{"_id":"000000000000000000000c02","mac":"`+mac+
				`","port_overrides":`+stored+`,"port_table":[{"port_idx":3},{"port_idx":5}]}]}`)
		case r.Method == http.MethodPut && r.URL.Path == "/proxy/network/api/s/default/rest/device/000000000000000000000c02":
			b, _ := io.ReadAll(r.Body)
			require.NoError(t, json.Unmarshal(b, &putBody))
			_, _ = io.WriteString(w, `{"meta":{"rc":"ok"},"data":[]}`)
		default:
			http.Error(w, "unexpected request", http.StatusBadRequest)
		}
	}))
	t.Cleanup(srv.Close)

	r := &devicePortsResource{client: newTestClient(t, srv.URL, false)}
	id, err := r.write(t.Context(), "default", mac, map[int64]*portWant{3: {ProfileID: ptrTo(testProfileA)}})
	require.NoError(t, err)
	assert.Equal(t, "000000000000000000000c02", id)
	assert.Equal(t, []string{"port_overrides"}, slices.Collect(maps.Keys(putBody)))
	assert.JSONEq(t, `[{"port_idx":3,"portconf_id":"`+testProfileA+`"},{"port_idx":5,"name":"desk","forward":"customize"}]`,
		string(putBody["port_overrides"]))

	_, err = r.write(t.Context(), "default", mac, map[int64]*portWant{7: {Name: ptrTo("x")}})
	assert.ErrorContains(t, err, "has no port 7")
}

// ---------------------------------------------------------------------------
// Acceptance tests
// ---------------------------------------------------------------------------
//
// These reconfigure real ports, so on hardware they only run against ports
// named explicitly:
//
//	TERRIFI_ACC_PORTS_MAC       an adopted switch or gateway
//	TERRIFI_ACC_PORTS           two ports on it that are safe to change, e.g. "3,4"
//	TERRIFI_ACC_PORTS_WAN_MAC   (optional) a gateway, for the WAN port check
//	TERRIFI_ACC_PORTS_WAN_PORT  (optional) its WAN port; nothing is written to it
//
// Against the Docker controller they fall back to the first simulated device
// with at least two ports. Every test restores the two ports' entries to what
// they were before it ran, and checks that no other port changed.

type devicePortsTarget struct {
	mac      string
	p1, p2   int64
	original []map[string]json.RawMessage // the device's port_overrides before the test
}

func (tg devicePortsTarget) k1() string { return strconv.FormatInt(tg.p1, 10) }
func (tg devicePortsTarget) k2() string { return strconv.FormatInt(tg.p2, 10) }

func testAccDevicePortsTarget(t *testing.T) devicePortsTarget {
	t.Helper()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC not set")
	}
	preCheck(t)
	client := testAccGetClient(t)

	var tg devicePortsTarget
	if mac, ports := os.Getenv("TERRIFI_ACC_PORTS_MAC"), os.Getenv("TERRIFI_ACC_PORTS"); mac != "" && ports != "" {
		var err1, err2 error
		p1, p2, ok := strings.Cut(ports, ",")
		tg.mac = mac
		tg.p1, err1 = strconv.ParseInt(strings.TrimSpace(p1), 10, 64)
		tg.p2, err2 = strconv.ParseInt(strings.TrimSpace(p2), 10, 64)
		if !ok || err1 != nil || err2 != nil || tg.p1 == tg.p2 {
			t.Fatalf("TERRIFI_ACC_PORTS must be two different port numbers like \"3,4\", got %q", ports)
		}
	} else if os.Getenv("TERRIFI_ACC_TARGET") == "hardware" {
		t.Skip("set TERRIFI_ACC_PORTS_MAC and TERRIFI_ACC_PORTS to run device port tests on hardware")
	} else {
		devices, err := client.ListDevicePorts(t.Context(), "default", "")
		require.NoError(t, err)
		for _, d := range devices {
			var usable []int64 // ports a profile can go on (not a gateway WAN port)
			for _, p := range d.PortTable {
				if checkPortsAssignable(&d, map[int64]*portWant{p.PortIdx: {ProfileID: ptrTo("x")}}) == nil {
					usable = append(usable, p.PortIdx)
				}
			}
			if len(usable) >= 2 {
				tg.mac, tg.p1, tg.p2 = d.MAC, usable[0], usable[1]
				break
			}
		}
		if tg.mac == "" {
			t.Skip("no simulated device with at least two ports")
		}
	}

	d, err := client.GetDevicePorts(t.Context(), "default", tg.mac)
	require.NoError(t, err)
	tg.original = d.PortOverrides
	t.Cleanup(func() {
		// Put the two test ports back exactly as they were.
		keep := map[int64]bool{tg.p1: true, tg.p2: true}
		d, err := client.GetDevicePorts(context.Background(), "default", tg.mac)
		if err != nil {
			t.Errorf("cleanup: reading device: %s", err)
			return
		}
		var restored []map[string]json.RawMessage
		for _, e := range d.PortOverrides {
			if idx, _ := portIdx(e); !keep[idx] {
				restored = append(restored, e)
			}
		}
		for _, e := range tg.original {
			if idx, _ := portIdx(e); keep[idx] {
				restored = append(restored, e)
			}
		}
		if restored == nil {
			restored = []map[string]json.RawMessage{}
		}
		if err := client.PutPortOverrides(context.Background(), "default", d.ID, restored); err != nil {
			t.Errorf("cleanup: restoring ports %d and %d: %s", tg.p1, tg.p2, err)
		}
	})
	return tg
}

// portEntry returns the stored port_overrides entry for port (nil if none).
func (tg devicePortsTarget) portEntry() func(port int64) (map[string]any, []map[string]json.RawMessage, error) {
	return func(port int64) (map[string]any, []map[string]json.RawMessage, error) {
		d, err := testAccGetClientNoT().GetDevicePorts(context.Background(), "default", tg.mac)
		if err != nil {
			return nil, nil, err
		}
		for _, e := range d.PortOverrides {
			if idx, _ := portIdx(e); idx == port {
				b, _ := json.Marshal(e)
				var out map[string]any
				return out, d.PortOverrides, json.Unmarshal(b, &out)
			}
		}
		return nil, d.PortOverrides, nil
	}
}

// setPortEntry writes port's entry behind Terraform's back, as the UI would.
// A nil entry removes it.
func (tg devicePortsTarget) setPortEntry(port int64, entry map[string]any) error {
	client := testAccGetClientNoT()
	d, err := client.GetDevicePorts(context.Background(), "default", tg.mac)
	if err != nil {
		return err
	}
	out := []map[string]json.RawMessage{}
	for _, e := range d.PortOverrides {
		if idx, _ := portIdx(e); idx != port {
			out = append(out, e)
		}
	}
	if entry != nil {
		entry["port_idx"] = port
		raw := map[string]json.RawMessage{}
		for k, v := range entry {
			raw[k], _ = json.Marshal(v)
		}
		out = append(out, raw)
	}
	return client.PutPortOverrides(context.Background(), "default", d.ID, out)
}

// checkPort checks port's stored entry: its name ("" = none) and profile
// (the address of a terrifi_port_profile in state, "" = none). With exact,
// the entry must hold nothing else (a profile clears per-port settings).
func (tg devicePortsTarget) checkPort(port int64, name, profileAddr string, exact bool) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		e, _, err := tg.portEntry()(port)
		if err != nil {
			return err
		}
		if e == nil {
			e = map[string]any{}
		}
		wantProfile := ""
		if profileAddr != "" {
			wantProfile = s.RootModule().Resources[profileAddr].Primary.ID
		}
		gotName, _ := e["name"].(string)
		gotProfile, _ := e["portconf_id"].(string)
		if gotName != name || gotProfile != wantProfile {
			return fmt.Errorf("port %d: got name %q profile %q, want name %q profile %q (entry %v)", port, gotName, gotProfile, name, wantProfile, e)
		}
		if exact {
			want := map[string]bool{"port_idx": true, "name": name != "", "portconf_id": wantProfile != ""}
			for k := range e {
				if !want[k] {
					return fmt.Errorf("port %d: unexpected %q in entry %v", port, k, e)
				}
			}
		}
		return nil
	}
}

// checkOtherPorts checks that every port except the two test ports is
// byte-for-byte as it was before the test.
func (tg devicePortsTarget) checkOtherPorts(*terraform.State) error {
	_, now, err := tg.portEntry()(0)
	if err != nil {
		return err
	}
	others := func(all []map[string]json.RawMessage) string {
		var out []map[string]json.RawMessage
		for _, e := range all {
			if idx, _ := portIdx(e); idx != tg.p1 && idx != tg.p2 {
				out = append(out, e)
			}
		}
		b, _ := json.Marshal(out)
		return string(b)
	}
	if before, after := others(tg.original), others(now); before != after {
		return fmt.Errorf("ports outside the test changed:\nbefore %s\nafter  %s", before, after)
	}
	return nil
}

// checkDestroy: the profiles are gone (so the assignments were removed first),
// the test ports have no name or profile, and no other port changed.
func (tg devicePortsTarget) checkDestroy(s *terraform.State) error {
	if err := testAccCheckPortProfileDestroyed(s); err != nil {
		return err
	}
	for _, p := range []int64{tg.p1, tg.p2} {
		e, _, err := tg.portEntry()(p)
		if err != nil {
			return err
		}
		if e["portconf_id"] != nil {
			return fmt.Errorf("port %d still has a profile after destroy: %v", p, e)
		}
	}
	return tg.checkOtherPorts(s)
}

const testAccDevicePortsAddr = "terrifi_device_ports.test"

// testAccDevicePortsConfig declares two profiles (a, b) and the device ports
// resource with the given ports map body. An empty body leaves the resource
// out.
func testAccDevicePortsConfig(tg devicePortsTarget, suffix, ports string) string {
	cfg := fmt.Sprintf(`
resource "terrifi_port_profile" "a" {
  name = "tfacc-dp-a-%[1]s"
}

resource "terrifi_port_profile" "b" {
  name = "tfacc-dp-b-%[1]s"
}
`, suffix)
	if ports != "" {
		cfg += fmt.Sprintf(`
resource "terrifi_device_ports" "test" {
  device_mac = %q
  ports = {
%s
  }
}
`, tg.mac, ports)
	}
	return cfg
}

func TestAccDevicePorts_basic(t *testing.T) {
	tg := testAccDevicePortsTarget(t)
	suffix := randomSuffix()
	name := "tfacc-dp-" + suffix
	cfg := testAccDevicePortsConfig(tg, suffix, fmt.Sprintf(`
    %q = { name = %q, port_profile_id = terrifi_port_profile.a.id }`, tg.k1(), name))

	importCheck := func(states []*terraform.InstanceState) error {
		for _, st := range states {
			if st.Attributes["ports."+tg.k1()+".name"] != name {
				return fmt.Errorf("imported ports: %v", st.Attributes)
			}
		}
		return nil
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             tg.checkDestroy,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(testAccDevicePortsAddr, "id"),
					resource.TestCheckResourceAttr(testAccDevicePortsAddr, "site", "default"),
					resource.TestCheckResourceAttr(testAccDevicePortsAddr, "ports."+tg.k1()+".name", name),
					resource.TestCheckResourceAttrPair(testAccDevicePortsAddr, "ports."+tg.k1()+".port_profile_id", "terrifi_port_profile.a", "id"),
					tg.checkPort(tg.p1, name, "terrifi_port_profile.a", true),
					tg.checkOtherPorts,
				),
			},
			// Import takes every named or profiled port on the device, so only
			// the test port is checked.
			{ResourceName: testAccDevicePortsAddr, ImportState: true, ImportStateId: tg.mac, ImportStateCheck: importCheck},
			{ResourceName: testAccDevicePortsAddr, ImportState: true, ImportStateId: "default:" + tg.mac, ImportStateCheck: importCheck},
		},
	})
}

// TestAccDevicePorts_lifecycle walks one port through every state: profile +
// name, another profile + rename, name only, nothing, unmanaged.
func TestAccDevicePorts_lifecycle(t *testing.T) {
	tg := testAccDevicePortsTarget(t)
	suffix := randomSuffix()
	port := func(body string) string { return fmt.Sprintf("    %q = { %s }", tg.k1(), body) }
	step := func(ports string, check ...resource.TestCheckFunc) resource.TestStep {
		return resource.TestStep{
			Config: testAccDevicePortsConfig(tg, suffix, ports),
			Check:  resource.ComposeAggregateTestCheckFunc(append(check, tg.checkOtherPorts)...),
		}
	}
	// ports = {} still declares the resource; testAccDevicePortsConfig only
	// drops it for an empty body.
	const noPorts = " "

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             tg.checkDestroy,
		Steps: []resource.TestStep{
			step(port(`name = "tfacc-dp-1", port_profile_id = terrifi_port_profile.a.id`),
				tg.checkPort(tg.p1, "tfacc-dp-1", "terrifi_port_profile.a", true)),
			step(port(`name = "tfacc-dp-2", port_profile_id = terrifi_port_profile.b.id`),
				tg.checkPort(tg.p1, "tfacc-dp-2", "terrifi_port_profile.b", true)),
			step(port(`name = "tfacc-dp-2"`),
				tg.checkPort(tg.p1, "tfacc-dp-2", "", false),
				resource.TestCheckNoResourceAttr(testAccDevicePortsAddr, "ports."+tg.k1()+".port_profile_id")),
			step(port(``),
				tg.checkPort(tg.p1, "", "", false),
				resource.TestCheckNoResourceAttr(testAccDevicePortsAddr, "ports."+tg.k1()+".name")),
			step(port(`port_profile_id = terrifi_port_profile.a.id`),
				tg.checkPort(tg.p1, "", "terrifi_port_profile.a", true)),
			step(noPorts,
				tg.checkPort(tg.p1, "", "", false),
				resource.TestCheckResourceAttr(testAccDevicePortsAddr, "ports.%", "0")),
		},
	})
}

func TestAccDevicePorts_multiplePorts(t *testing.T) {
	tg := testAccDevicePortsTarget(t)
	suffix := randomSuffix()
	both := fmt.Sprintf(`
    %q = { name = "tfacc-dp-p1", port_profile_id = terrifi_port_profile.a.id }
    %q = { name = "tfacc-dp-p2", port_profile_id = terrifi_port_profile.b.id }`, tg.k1(), tg.k2())
	first := fmt.Sprintf(`
    %q = { name = "tfacc-dp-p1", port_profile_id = terrifi_port_profile.a.id }`, tg.k1())

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             tg.checkDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccDevicePortsConfig(tg, suffix, both),
				Check: resource.ComposeAggregateTestCheckFunc(
					tg.checkPort(tg.p1, "tfacc-dp-p1", "terrifi_port_profile.a", true),
					tg.checkPort(tg.p2, "tfacc-dp-p2", "terrifi_port_profile.b", true),
					tg.checkOtherPorts,
				),
			},
			// Dropping a port releases it and leaves the other alone.
			{
				Config: testAccDevicePortsConfig(tg, suffix, first),
				Check: resource.ComposeAggregateTestCheckFunc(
					tg.checkPort(tg.p1, "tfacc-dp-p1", "terrifi_port_profile.a", true),
					tg.checkPort(tg.p2, "", "", false),
					resource.TestCheckNoResourceAttr(testAccDevicePortsAddr, "ports."+tg.k2()+".name"),
					tg.checkOtherPorts,
				),
			},
		},
	})
}

// Per-port settings made in the UI, as the UI stores them.
func testAccInlinePortSettings() map[string]any {
	return map[string]any{
		"name": "tfacc-dp-inline", "forward": "all",
		"autoneg": false, "speed": 100, "full_duplex": true, "poe_mode": "off",
	}
}

// TestAccDevicePorts_takeOverInline assigns a profile to a port that has
// per-port settings: they're cleared, poe_mode included.
func TestAccDevicePorts_takeOverInline(t *testing.T) {
	tg := testAccDevicePortsTarget(t)
	suffix := randomSuffix()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             tg.checkDestroy,
		Steps: []resource.TestStep{
			{
				PreConfig: func() { require.NoError(t, tg.setPortEntry(tg.p1, testAccInlinePortSettings())) },
				Config: testAccDevicePortsConfig(tg, suffix, fmt.Sprintf(`
    %q = { port_profile_id = terrifi_port_profile.a.id }`, tg.k1())),
				Check: resource.ComposeAggregateTestCheckFunc(
					tg.checkPort(tg.p1, "", "terrifi_port_profile.a", true),
					tg.checkOtherPorts,
				),
			},
		},
	})
}

// TestAccDevicePorts_releaseKeepsInline manages only the name of a port with
// per-port settings; renaming and then releasing it keeps those settings.
func TestAccDevicePorts_releaseKeepsInline(t *testing.T) {
	tg := testAccDevicePortsTarget(t)
	suffix := randomSuffix()
	speedKept := func(*terraform.State) error {
		e, _, err := tg.portEntry()(tg.p2)
		if err != nil {
			return err
		}
		if e["speed"] != float64(100) || e["autoneg"] != false {
			return fmt.Errorf("per-port settings lost: %v", e)
		}
		return nil
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             tg.checkDestroy,
		Steps: []resource.TestStep{
			{
				PreConfig: func() { require.NoError(t, tg.setPortEntry(tg.p2, testAccInlinePortSettings())) },
				Config: testAccDevicePortsConfig(tg, suffix, fmt.Sprintf(`
    %q = { name = "tfacc-dp-renamed" }`, tg.k2())),
				Check: resource.ComposeAggregateTestCheckFunc(
					tg.checkPort(tg.p2, "tfacc-dp-renamed", "", false),
					speedKept,
					tg.checkOtherPorts,
				),
			},
			{
				Config: testAccDevicePortsConfig(tg, suffix, " "),
				Check: resource.ComposeAggregateTestCheckFunc(
					tg.checkPort(tg.p2, "", "", false),
					speedKept,
					tg.checkOtherPorts,
				),
			},
		},
	})
}

// TestAccDevicePorts_drift detaches the profile behind Terraform's back (as
// the UI does, writing default per-port settings); the next plan restores it.
func TestAccDevicePorts_drift(t *testing.T) {
	tg := testAccDevicePortsTarget(t)
	suffix := randomSuffix()
	cfg := testAccDevicePortsConfig(tg, suffix, fmt.Sprintf(`
    %q = { name = "tfacc-dp-drift", port_profile_id = terrifi_port_profile.a.id }`, tg.k1()))
	uiDetach := func() {
		require.NoError(t, tg.setPortEntry(tg.p1, map[string]any{
			"name": "tfacc-dp-drift", "forward": "all", "stp_port_mode": true, "setting_preference": "auto",
		}))
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             tg.checkDestroy,
		Steps: []resource.TestStep{
			{Config: cfg, Check: tg.checkPort(tg.p1, "tfacc-dp-drift", "terrifi_port_profile.a", true)},
			{
				PreConfig:          uiDetach,
				Config:             cfg,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccDevicePortsAddr, plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					tg.checkPort(tg.p1, "tfacc-dp-drift", "terrifi_port_profile.a", true),
					tg.checkOtherPorts,
				),
			},
		},
	})
}

// TestAccDevicePorts_withDeviceResource manages the same device with
// terrifi_device in the same apply. Both PUT to the device; each sends only
// its own fields, so neither undoes the other. The device keeps its current
// name.
func TestAccDevicePorts_withDeviceResource(t *testing.T) {
	tg := testAccDevicePortsTarget(t)
	suffix := randomSuffix()
	d, err := testAccGetClient(t).GetDevicePorts(t.Context(), "default", tg.mac)
	require.NoError(t, err)
	if d.Name == "" {
		t.Skip("device has no name to keep")
	}
	cfg := testAccDevicePortsConfig(tg, suffix, fmt.Sprintf(`
    %q = { name = "tfacc-dp-both", port_profile_id = terrifi_port_profile.a.id }`, tg.k1())) +
		fmt.Sprintf(`
resource "terrifi_device" "test" {
  mac  = %q
  name = %q
}
`, tg.mac, d.Name)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             tg.checkDestroy,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					tg.checkPort(tg.p1, "tfacc-dp-both", "terrifi_port_profile.a", true),
					resource.TestCheckResourceAttr("terrifi_device.test", "name", d.Name),
					tg.checkOtherPorts,
				),
			},
		},
	})
}

// TestAccDevicePorts_deleteAssignedProfile assigns a profile outside
// Terraform, then removes it from the config: the controller refuses, and
// the error says why.
func TestAccDevicePorts_deleteAssignedProfile(t *testing.T) {
	tg := testAccDevicePortsTarget(t)
	suffix := randomSuffix()
	var profileB string
	onlyA := strings.Replace(testAccDevicePortsConfig(tg, suffix, ""), fmt.Sprintf(`
resource "terrifi_port_profile" "b" {
  name = "tfacc-dp-b-%s"
}
`, suffix), "", 1)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             tg.checkDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccDevicePortsConfig(tg, suffix, ""),
				Check: func(s *terraform.State) error {
					profileB = s.RootModule().Resources["terrifi_port_profile.b"].Primary.ID
					return nil
				},
			},
			{
				PreConfig:   func() { require.NoError(t, tg.setPortEntry(tg.p2, map[string]any{"portconf_id": profileB})) },
				Config:      onlyA,
				ExpectError: regexp.MustCompile(`still assigned to a device port`),
			},
			{
				PreConfig: func() { require.NoError(t, tg.setPortEntry(tg.p2, nil)) },
				Config:    onlyA,
			},
		},
	})
}

func TestAccDevicePorts_missingPort(t *testing.T) {
	tg := testAccDevicePortsTarget(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             tg.checkDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccDevicePortsConfig(tg, randomSuffix(), `
    "999" = { name = "tfacc-dp-missing" }`),
				ExpectError: regexp.MustCompile(`has no port 999`),
			},
		},
	})
}

// TestAccDevicePorts_wanPort tries to assign a profile to a gateway's WAN
// port. The check runs before any write, so the gateway isn't changed.
func TestAccDevicePorts_wanPort(t *testing.T) {
	mac, port := os.Getenv("TERRIFI_ACC_PORTS_WAN_MAC"), os.Getenv("TERRIFI_ACC_PORTS_WAN_PORT")
	if os.Getenv("TF_ACC") == "" || mac == "" || port == "" {
		t.Skip("set TERRIFI_ACC_PORTS_WAN_MAC and TERRIFI_ACC_PORTS_WAN_PORT to run")
	}
	preCheck(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPortProfileDestroyed,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_port_profile" "a" {
  name = "tfacc-dp-wan-%s"
}

resource "terrifi_device_ports" "test" {
  device_mac = %q
  ports = {
    %q = { port_profile_id = terrifi_port_profile.a.id }
  }
}
`, randomSuffix(), mac, port),
				ExpectError: regexp.MustCompile(`is a WAN\d* port`),
			},
		},
	})
}
