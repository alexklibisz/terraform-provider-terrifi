package provider

import (
	"fmt"
	"testing"

	"github.com/alexklibisz/terrifi/internal/unifi"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDevicePortInlineValidation(t *testing.T) {
	s := types.StringValue
	for _, tc := range []struct {
		name    string
		port    devicePortModel
		invalid bool
	}{
		{"name only", devicePortModel{Name: s("desk")}, false},
		{"profile", devicePortModel{PortProfileID: s("profile")}, false},
		{"inline", devicePortModel{NativeNetworkID: s("network"), TaggedVLANMode: s("block_all")}, false},
		{"missing mode", devicePortModel{NativeNetworkID: s("network")}, true},
		{"missing native", devicePortModel{TaggedVLANMode: s("block_all")}, true},
		{"profile conflict", devicePortModel{PortProfileID: s("profile"), NativeNetworkID: s("network"), TaggedVLANMode: s("block_all")}, true},
		{"unknown reference", devicePortModel{NativeNetworkID: types.StringUnknown(), TaggedVLANMode: s("block_all")}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.invalid, validateDevicePortModels(map[string]devicePortModel{"2": tc.port}).HasError())
		})
	}
}

func TestMergePortOverrides_InlineTransitions(t *testing.T) {
	// Switching profile -> inline preserves unrelated controller fields while
	// removing the old profile. Subsequent profile assignment owns the full port.
	start := `[{"port_idx":2,"portconf_id":"profile","poe_mode":"off"},{"port_idx":4,"name":"untouched"}]`
	want := map[int64]*portWant{2: {NativeNetworkID: ptrTo("network"), TaggedVLANMode: ptrTo("block_all")}}
	got := mergedJSON(t, start, want)
	assert.JSONEq(t, `[{"port_idx":2,"poe_mode":"off","native_networkconf_id":"network","tagged_vlan_mgmt":"block_all","forward":"native","excluded_networkconf_ids":[],"setting_preference":"manual"},{"port_idx":4,"name":"untouched"}]`, got)
	got = mergedJSON(t, got, map[int64]*portWant{2: {ProfileID: ptrTo("next-profile")}})
	assert.JSONEq(t, `[{"port_idx":2,"portconf_id":"next-profile"},{"port_idx":4,"name":"untouched"}]`, got)
	got = mergedJSON(t, got, map[int64]*portWant{2: nil})
	assert.JSONEq(t, `[{"port_idx":4,"name":"untouched"}]`, got)
	// Removing inline settings while retaining the port name resets VLANs.
	got = mergedJSON(t, `[{"port_idx":2,"native_networkconf_id":"network","forward":"native","tagged_vlan_mgmt":"block_all"}]`, map[int64]*portWant{2: {Name: ptrTo("desk"), ResetVLAN: true}})
	assert.JSONEq(t, `[{"port_idx":2,"name":"desk"}]`, got)
}

func TestDevicePorts_FlexMiniCapabilities(t *testing.T) {
	d := &unifi.DevicePorts{Model: "USMINI", PortTable: []unifi.DevicePort{{PortIdx: 2}}}
	assert.ErrorContains(t, checkPortsAssignable(d, map[int64]*portWant{2: {ProfileID: ptrTo("profile")}}), "does not support custom port profiles")
	networks := []unifi.PortNetwork{{ID: "default", Purpose: "corporate"}, {ID: "vlan", Purpose: "corporate", VLANEnabled: true}, {ID: "wan", Purpose: "wan"}}
	for _, tc := range []struct {
		native, mode string
		invalid      bool
	}{
		{"default", "allow_all", false}, {"vlan", "block_all", false}, {"vlan", "allow_all", true}, {"missing", "block_all", true}, {"wan", "block_all", true},
	} {
		assert.Equal(t, tc.invalid, validatePortNetworks(d, map[int64]*portWant{2: {NativeNetworkID: &tc.native, TaggedVLANMode: &tc.mode}}, networks) != nil)
	}
	gateway := testGatewayPorts()
	assert.ErrorContains(t, checkPortsAssignable(gateway, map[int64]*portWant{5: {NativeNetworkID: ptrTo("vlan"), TaggedVLANMode: ptrTo("block_all")}}), "is a WAN port")
}

func TestDevicePorts_InlineImportAndRefresh(t *testing.T) {
	got := portsFromOverrides(overridesJSON(t, `[{"port_idx":2,"native_networkconf_id":"network","forward":"native"},{"port_idx":4,"portconf_id":"profile","native_networkconf_id":"stale"}]`))
	imported := portsToModel(got, nil)
	require.Len(t, imported, 2)
	assert.Equal(t, types.StringValue("network"), imported["2"].NativeNetworkID)
	assert.Equal(t, types.StringValue("block_all"), imported["2"].TaggedVLANMode)
	assert.True(t, imported["4"].NativeNetworkID.IsNull())
	assert.Equal(t, imported, portsToModel(got, imported))
	want := portWants(map[string]devicePortModel{"2": {Name: types.StringValue("desk")}}, imported)
	assert.True(t, want[2].ResetVLAN)
	assert.Nil(t, want[4])
}

// Runs only on explicitly designated spare ports on hardware. Exercises access
// VLAN changes, import, drift correction, reset of one port, and full destroy.
func TestAccDevicePorts_inlineLifecycle(t *testing.T) {
	tg := testAccDevicePortsTarget(t)
	nets := networksHCL(testAccPortProfileNetworks(randomSuffix(), 2), 2)
	var firstNetworkID string
	cfg := func(network string, second bool) string {
		ports := fmt.Sprintf(`%q = { name = "tfacc-inline", native_network_id = terrifi_network.%s.id, tagged_vlan_mode = "block_all" }`, tg.k1(), network)
		if second {
			ports += fmt.Sprintf("\n%q = { name = \"tfacc-inline-second\", native_network_id = terrifi_network.pp1.id, tagged_vlan_mode = \"block_all\" }", tg.k2())
		}
		return nets + fmt.Sprintf("\nresource \"terrifi_device_ports\" \"test\" {\n device_mac = %q\n ports = terrifi_network.pp0.id != \"\" ? {\n%s\n} : {}\n}\n", tg.mac, ports)
	}
	check := func(network string, second bool) resource.TestCheckFunc {
		return resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttrPair(testAccDevicePortsAddr, "ports."+tg.k1()+".native_network_id", "terrifi_network."+network, "id"),
			func(s *terraform.State) error {
				entry, _, err := tg.portEntry()(tg.p1)
				if err != nil {
					return err
				}
				id := s.RootModule().Resources["terrifi_network."+network].Primary.ID
				firstNetworkID = s.RootModule().Resources["terrifi_network.pp0"].Primary.ID
				if entry["native_networkconf_id"] != id || entry["tagged_vlan_mgmt"] != "block_all" || entry["portconf_id"] != nil {
					return fmt.Errorf("incorrect inline port: %v", entry)
				}
				if !second {
					entry, _, err = tg.portEntry()(tg.p2)
					if err != nil {
						return err
					}
					if entry != nil {
						return fmt.Errorf("removed port not reset: %v", entry)
					}
				}
				return nil
			}, tg.checkOtherPorts)
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			for _, port := range []int64{tg.p1, tg.p2} {
				e, _, err := tg.portEntry()(port)
				if err != nil {
					return err
				}
				if e != nil {
					return fmt.Errorf("destroy left port %d configured: %v", port, e)
				}
			}
			return tg.checkOtherPorts(s)
		},
		Steps: []resource.TestStep{
			{Config: cfg("pp0", true), Check: check("pp0", true)},
			{ResourceName: testAccDevicePortsAddr, ImportState: true, ImportStateId: tg.mac, ImportStateVerify: true},
			{Config: cfg("pp1", true), Check: check("pp1", true)},
			{PreConfig: func() {
				require.NoError(t, tg.setPortEntry(tg.p1, map[string]any{"name": "drift", "native_networkconf_id": firstNetworkID, "tagged_vlan_mgmt": "block_all", "forward": "native"}))
			}, Config: cfg("pp1", true), Check: check("pp1", true)},
			{Config: cfg("pp1", false), Check: check("pp1", false)},
		},
	})
}

func TestAccDevicePorts_inlineDefaultTrunk(t *testing.T) {
	tg := testAccDevicePortsTarget(t)
	networks, err := testAccGetClient(t).listPortNetworks(t.Context(), "default")
	require.NoError(t, err)
	var native string
	for _, n := range networks {
		if n.Purpose == "corporate" && !n.VLANEnabled {
			native = n.ID
			break
		}
	}
	require.NotEmpty(t, native, "site needs a default LAN network")
	cfg := func(mode string) string {
		vlan := ""
		if mode != "" {
			vlan = fmt.Sprintf("native_network_id = %q, tagged_vlan_mode = %q,", native, mode)
		}
		return fmt.Sprintf(`resource "terrifi_device_ports" "test" {
   device_mac = %q
   ports = { %q = { %s name = "tfacc-default-trunk" } }
  }`, tg.mac, tg.k1(), vlan)
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			e, _, err := tg.portEntry()(tg.p1)
			if err != nil {
				return err
			}
			if e != nil {
				return fmt.Errorf("destroy left default trunk overrides: %v", e)
			}
			return tg.checkOtherPorts(s)
		},
		Steps: []resource.TestStep{
			{Config: cfg("allow_all"), Check: resource.TestCheckResourceAttr(testAccDevicePortsAddr, "ports."+tg.k1()+".tagged_vlan_mode", "allow_all")},
			{ResourceName: testAccDevicePortsAddr, ImportState: true, ImportStateId: tg.mac, ImportStateVerify: true},
			{Config: cfg("block_all"), Check: resource.TestCheckResourceAttr(testAccDevicePortsAddr, "ports."+tg.k1()+".tagged_vlan_mode", "block_all")},
			{Config: cfg("allow_all")},
			{Config: cfg(""), Check: resource.TestCheckNoResourceAttr(testAccDevicePortsAddr, "ports."+tg.k1()+".native_network_id")},
		},
	})
}
