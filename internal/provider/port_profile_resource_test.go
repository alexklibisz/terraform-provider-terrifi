package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alexklibisz/terrifi/internal/unifi"
)

// ---------------------------------------------------------------------------
// Unit tests — no TF_ACC, no network, no env vars needed
// ---------------------------------------------------------------------------

const (
	testDefaultNetID = "000000000000000000000010" // the site's Default network
	testNetA         = "000000000000000000000011"
	testNetB         = "000000000000000000000012"
	testNetC         = "000000000000000000000013"
)

var testLANNetworks = []string{testDefaultNetID, testNetA, testNetB, testNetC}

// uiDefaultPortProfileJSON has the exact key set of a profile created in the
// UniFi UI with every setting left at its default (captured from UniFi OS 5.1 /
// Network 10.x; IDs and names are synthetic). Notable: forward "all" with the
// Default network as native, setting_preference "auto", poe_mode "auto",
// storm control rates pre-filled but disabled, and fields the provider doesn't
// manage (qos_profile, multicast_router_mode, dot1x_idle_timeout, ...).
const uiDefaultPortProfileJSON = `{
  "_id": "000000000000000000000100",
  "autoneg": true,
  "dot1x_ctrl": "force_authorized",
  "dot1x_idle_timeout": 300,
  "eee_enabled": false,
  "egress_rate_limit_kbps_enabled": false,
  "flow_control_enabled": true,
  "forward": "all",
  "isolation": false,
  "link_debounce": 300,
  "link_debounce_auto": true,
  "lldpmed_enabled": true,
  "lldpmed_notify_enabled": false,
  "multicast_router_mode": "NONE",
  "name": "Example",
  "native_networkconf_id": "000000000000000000000010",
  "op_mode": "switch",
  "poe_mode": "auto",
  "port_keepalive_enabled": false,
  "port_security_enabled": false,
  "port_security_mac_address": [],
  "precision_time_protocol_enabled": true,
  "qos_profile": {"qos_policies": [], "qos_profile_mode": "custom"},
  "setting_preference": "auto",
  "site_id": "000000000000000000000001",
  "stormctrl_bcast_enabled": false,
  "stormctrl_bcast_rate": 100,
  "stormctrl_mcast_enabled": false,
  "stormctrl_mcast_rate": 100,
  "stormctrl_ucast_enabled": false,
  "stormctrl_ucast_rate": 100,
  "stp_bpdu_guard_enabled": false,
  "stp_edge_state": "disabled",
  "stp_port_mode": true,
  "stp_uplink": false,
  "tagged_vlan_mgmt": "auto",
  "voice_networkconf_id": ""
}`

// defaultPortProfileModel returns a model as the framework would plan it for
// a config that only sets name: every defaulted attribute is filled in.
func defaultPortProfileModel(name string) portProfileResourceModel {
	return portProfileResourceModel{
		ID:                       types.StringUnknown(),
		Site:                     types.StringUnknown(),
		Name:                     types.StringValue(name),
		NativeNetworkID:          types.StringNull(),
		TaggedVLANMgmt:           types.StringValue("auto"),
		TaggedNetworkIDs:         types.SetNull(types.StringType),
		ExcludedNetworkIDs:       types.SetNull(types.StringType),
		VoiceNetworkID:           types.StringNull(),
		PoeMode:                  types.StringValue("auto"),
		Autoneg:                  types.BoolValue(true),
		Speed:                    types.Int64Null(),
		FullDuplex:               types.BoolValue(false),
		EgressRateLimitKbps:      types.Int64Null(),
		FlowControlEnabled:       types.BoolValue(true),
		PTPEnabled:               types.BoolValue(true),
		Isolation:                types.BoolValue(false),
		StpEnabled:               types.BoolValue(true),
		StpUplink:                types.BoolValue(false),
		BpduGuardEnabled:         types.BoolValue(false),
		LoopProtectionEnabled:    types.BoolValue(false),
		EEEEnabled:               types.BoolValue(false),
		LldpmedEnabled:           types.BoolValue(true),
		LinkDebounceMs:           types.Int64Null(),
		PortSecurityEnabled:      types.BoolValue(false),
		PortSecurityMACAddresses: types.SetValueMust(types.StringType, []attr.Value{}),
		Dot1XCtrl:                types.StringValue("force_authorized"),
	}
}

func stringSet(vs ...string) types.Set {
	return stringSetValue(vs)
}

// toWire marshals a PortProfile the way the API layer does and returns it as
// a generic map so tests can assert on exact keys and values.
func toWire(t *testing.T, p *unifi.PortProfile) map[string]any {
	t.Helper()
	b, err := json.Marshal(normalizePortProfile(p))
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(b, &m))
	return m
}

func mustModelToAPI(t *testing.T, m *portProfileResourceModel, lan []string) *unifi.PortProfile {
	t.Helper()
	p, diags := (&portProfileResource{}).modelToAPI(t.Context(), m, lan)
	require.False(t, diags.HasError(), "%v", diags)
	return p
}

// A UI profile left at its defaults must read back as exactly what a config
// with only `name` and `native_network_id` (the UI's default native network)
// plans, so importing it shows no diff.
func TestPortProfileAPIToModel_UIDefaults(t *testing.T) {
	var p unifi.PortProfile
	require.NoError(t, json.Unmarshal([]byte(uiDefaultPortProfileJSON), &p))

	var got portProfileResourceModel
	(&portProfileResource{}).apiToModel(&p, &got, "default", nil)

	want := defaultPortProfileModel("Example")
	want.ID = types.StringValue("000000000000000000000100")
	want.Site = types.StringValue("default")
	want.NativeNetworkID = types.StringValue(testDefaultNetID)
	assert.Equal(t, want, got)
}

func TestPortProfileAPIToModel_MissingFieldsFallBackToDefaults(t *testing.T) {
	var got portProfileResourceModel
	(&portProfileResource{}).apiToModel(&unifi.PortProfile{ID: "x", Name: "bare"}, &got, "default", nil)

	want := defaultPortProfileModel("bare")
	want.ID = types.StringValue("x")
	want.Site = types.StringValue("default")
	assert.Equal(t, want, got)
}

// Values the controller keeps but ignores must read as unset, or they'd show
// up as perpetual diffs (all observed in the UI: the controller leaves speed
// and duplex behind when autoneg is re-enabled, and so on).
func TestPortProfileAPIToModel_StaleValuesIgnored(t *testing.T) {
	read := func(p *unifi.PortProfile) portProfileResourceModel {
		var m portProfileResourceModel
		(&portProfileResource{}).apiToModel(p, &m, "default", nil)
		return m
	}

	t.Run("speed and duplex ignored while autoneg is on", func(t *testing.T) {
		m := read(&unifi.PortProfile{Autoneg: ptrTo(true), Speed: ptrTo(int64(100)), FullDuplex: ptrTo(true)})
		assert.True(t, m.Speed.IsNull())
		assert.False(t, m.FullDuplex.ValueBool())
	})

	t.Run("speed and duplex read while autoneg is off", func(t *testing.T) {
		m := read(&unifi.PortProfile{Autoneg: ptrTo(false), Speed: ptrTo(int64(100)), FullDuplex: ptrTo(true)})
		assert.Equal(t, int64(100), m.Speed.ValueInt64())
		assert.True(t, m.FullDuplex.ValueBool())
	})

	t.Run("voice network ignored while LLDP-MED is off", func(t *testing.T) {
		m := read(&unifi.PortProfile{LldpmedEnabled: ptrTo(false), VoiceNetworkID: ptrTo(testNetA)})
		assert.True(t, m.VoiceNetworkID.IsNull())
		assert.False(t, m.LldpmedEnabled.ValueBool())
	})

	t.Run("egress kbps ignored while disabled", func(t *testing.T) {
		m := read(&unifi.PortProfile{EgressRateLimitKbpsEnabled: ptrTo(false), EgressRateLimitKbps: ptrTo(int64(1000))})
		assert.True(t, m.EgressRateLimitKbps.IsNull())
	})

	t.Run("link debounce value ignored while auto", func(t *testing.T) {
		m := read(&unifi.PortProfile{LinkDebounceAuto: ptrTo(true), LinkDebounce: ptrTo(int64(1200))})
		assert.True(t, m.LinkDebounceMs.IsNull())
	})

	t.Run("storm control thresholds ignored while every class is disabled", func(t *testing.T) {
		m := read(&unifi.PortProfile{
			StormctrlType:         "rate",
			StormctrlBcastEnabled: ptrTo(false),
			StormctrlBcastRate:    ptrTo(int64(12345)),
			StormctrlBcastLevel:   ptrTo(int64(37)),
		})
		assert.Nil(t, m.StormControl)
	})

	t.Run("storm control reads the field matching its type", func(t *testing.T) {
		m := read(&unifi.PortProfile{
			StormctrlType:         "rate",
			StormctrlBcastEnabled: ptrTo(true),
			StormctrlBcastLevel:   ptrTo(int64(37)),
			StormctrlBcastRate:    ptrTo(int64(12345)),
		})
		require.NotNil(t, m.StormControl)
		assert.Equal(t, "rate", m.StormControl.Type.ValueString())
		assert.Equal(t, int64(12345), m.StormControl.Broadcast.ValueInt64())
		assert.True(t, m.StormControl.Multicast.IsNull())
	})
}

func TestPortProfileAPIToModel_LinkDebounce(t *testing.T) {
	cases := map[string]struct {
		auto *bool
		ms   *int64
		want types.Int64
	}{
		"auto (UI default)":   {ptrTo(true), ptrTo(int64(300)), types.Int64Null()},
		"missing keys = auto": {nil, nil, types.Int64Null()},
		"off":                 {ptrTo(false), ptrTo(int64(0)), types.Int64Value(0)},
		"off without value":   {ptrTo(false), nil, types.Int64Value(0)},
		"custom":              {ptrTo(false), ptrTo(int64(1200)), types.Int64Value(1200)},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var m portProfileResourceModel
			(&portProfileResource{}).apiToModel(&unifi.PortProfile{LinkDebounceAuto: tc.auto, LinkDebounce: tc.ms}, &m, "default", nil)
			assert.Equal(t, tc.want, m.LinkDebounceMs)
		})
	}
}

func TestPortProfileAPIToModel_TaggedNetworks(t *testing.T) {
	// Stored like the UI stores "native A, tagged C": every other LAN
	// network (Default, B) is excluded.
	p := &unifi.PortProfile{
		TaggedVLANMgmt:     "custom",
		NativeNetworkID:    ptrTo(testNetA),
		ExcludedNetworkIDs: []string{testDefaultNetID, testNetB},
	}
	read := func(prior portProfileResourceModel) portProfileResourceModel {
		(&portProfileResource{}).apiToModel(p, &prior, "default", testLANNetworks)
		return prior
	}

	t.Run("tagged list in state", func(t *testing.T) {
		prior := defaultPortProfileModel("x")
		prior.TaggedNetworkIDs = stringSet(testNetC)
		m := read(prior)
		assert.Equal(t, stringSet(testNetC), m.TaggedNetworkIDs)
		assert.True(t, m.ExcludedNetworkIDs.IsNull())
	})

	t.Run("excluded list in state", func(t *testing.T) {
		prior := defaultPortProfileModel("x")
		prior.ExcludedNetworkIDs = stringSet(testDefaultNetID, testNetB)
		m := read(prior)
		assert.Equal(t, stringSet(testDefaultNetID, testNetB), m.ExcludedNetworkIDs)
		assert.True(t, m.TaggedNetworkIDs.IsNull())
	})

	t.Run("import (neither in state) uses the tagged list", func(t *testing.T) {
		m := read(defaultPortProfileModel("x"))
		assert.Equal(t, stringSet(testNetC), m.TaggedNetworkIDs)
		assert.True(t, m.ExcludedNetworkIDs.IsNull())
	})

	t.Run("a network missing from the deny-list shows as tagged", func(t *testing.T) {
		// testNetB was created after the profile was written.
		p := &unifi.PortProfile{
			TaggedVLANMgmt:     "custom",
			NativeNetworkID:    ptrTo(testNetA),
			ExcludedNetworkIDs: []string{testDefaultNetID},
		}
		prior := defaultPortProfileModel("x")
		prior.TaggedNetworkIDs = stringSet(testNetC)
		(&portProfileResource{}).apiToModel(p, &prior, "default", testLANNetworks)
		assert.Equal(t, stringSet(testNetB, testNetC), prior.TaggedNetworkIDs)
	})

	t.Run("not custom clears both", func(t *testing.T) {
		prior := defaultPortProfileModel("x")
		prior.TaggedNetworkIDs = stringSet(testNetC)
		var m = prior
		(&portProfileResource{}).apiToModel(&unifi.PortProfile{TaggedVLANMgmt: "block_all", ExcludedNetworkIDs: []string{testNetB}}, &m, "default", nil)
		assert.True(t, m.TaggedNetworkIDs.IsNull())
		assert.True(t, m.ExcludedNetworkIDs.IsNull())
	})
}

func TestPortProfileModelToAPI_Defaults(t *testing.T) {
	m := defaultPortProfileModel("Defaults")
	wire := toWire(t, mustModelToAPI(t, &m, nil))

	assert.Equal(t, map[string]any{
		"name":                            "Defaults",
		"setting_preference":              "manual",
		"forward":                         "customize",
		"native_networkconf_id":           "",
		"tagged_vlan_mgmt":                "auto",
		"excluded_networkconf_ids":        []any{},
		"voice_networkconf_id":            "",
		"poe_mode":                        "auto",
		"autoneg":                         true,
		"full_duplex":                     false,
		"egress_rate_limit_kbps_enabled":  false,
		"flow_control_enabled":            true,
		"precision_time_protocol_enabled": true,
		"isolation":                       false,
		"stp_port_mode":                   true,
		"stp_uplink":                      false,
		"stp_bpdu_guard_enabled":          false,
		"port_keepalive_enabled":          false,
		"eee_enabled":                     false,
		"lldpmed_enabled":                 true,
		"link_debounce_auto":              true,
		"port_security_enabled":           false,
		"port_security_mac_address":       []any{},
		"dot1x_ctrl":                      "force_authorized",
		"stormctrl_bcast_enabled":         false,
		"stormctrl_mcast_enabled":         false,
		"stormctrl_ucast_enabled":         false,
	}, wire, "exact payload for a name-only config")
}

func TestPortProfileModelToAPI_AllSettings(t *testing.T) {
	m := defaultPortProfileModel("Everything")
	m.NativeNetworkID = types.StringValue(testNetA)
	m.TaggedVLANMgmt = types.StringValue("custom")
	m.TaggedNetworkIDs = stringSet(testNetC)
	m.VoiceNetworkID = types.StringValue(testNetC)
	m.PoeMode = types.StringValue("off")
	m.Autoneg = types.BoolValue(false)
	m.Speed = types.Int64Value(1000)
	m.FullDuplex = types.BoolValue(true)
	m.EgressRateLimitKbps = types.Int64Value(5000)
	m.FlowControlEnabled = types.BoolValue(false)
	m.PTPEnabled = types.BoolValue(false)
	m.Isolation = types.BoolValue(true)
	m.StpEnabled = types.BoolValue(false)
	m.StpUplink = types.BoolValue(true)
	m.BpduGuardEnabled = types.BoolValue(true)
	m.LoopProtectionEnabled = types.BoolValue(true)
	m.EEEEnabled = types.BoolValue(true)
	m.LinkDebounceMs = types.Int64Value(1200)
	m.PortSecurityEnabled = types.BoolValue(true)
	m.PortSecurityMACAddresses = stringSet("00:00:5e:00:53:01")
	m.Dot1XCtrl = types.StringValue("multi_host")
	m.StormControl = &portProfileStormControlModel{
		Type:      types.StringValue("rate"),
		Broadcast: types.Int64Value(100),
		Multicast: types.Int64Null(),
		Unicast:   types.Int64Value(300),
	}

	wire := toWire(t, mustModelToAPI(t, &m, testLANNetworks))

	assert.Equal(t, "customize", wire["forward"])
	assert.Equal(t, testNetA, wire["native_networkconf_id"])
	assert.Equal(t, "custom", wire["tagged_vlan_mgmt"])
	assert.ElementsMatch(t, []any{testDefaultNetID, testNetB}, wire["excluded_networkconf_ids"],
		"deny-list = LAN − tagged − native")
	assert.Equal(t, testNetC, wire["voice_networkconf_id"])
	assert.Equal(t, "off", wire["poe_mode"])
	assert.Equal(t, false, wire["autoneg"])
	assert.Equal(t, float64(1000), wire["speed"])
	assert.Equal(t, true, wire["full_duplex"])
	assert.Equal(t, true, wire["egress_rate_limit_kbps_enabled"])
	assert.Equal(t, float64(5000), wire["egress_rate_limit_kbps"])
	assert.Equal(t, false, wire["flow_control_enabled"])
	assert.Equal(t, false, wire["precision_time_protocol_enabled"])
	assert.Equal(t, true, wire["isolation"])
	assert.Equal(t, false, wire["stp_port_mode"])
	assert.Equal(t, true, wire["stp_uplink"])
	assert.Equal(t, true, wire["stp_bpdu_guard_enabled"])
	assert.Equal(t, true, wire["port_keepalive_enabled"])
	assert.Equal(t, true, wire["eee_enabled"])
	assert.Equal(t, false, wire["link_debounce_auto"])
	assert.Equal(t, float64(1200), wire["link_debounce"])
	assert.Equal(t, true, wire["port_security_enabled"])
	assert.Equal(t, []any{"00:00:5e:00:53:01"}, wire["port_security_mac_address"])
	assert.Equal(t, "multi_host", wire["dot1x_ctrl"])
	assert.Equal(t, "rate", wire["stormctrl_type"])
	assert.Equal(t, true, wire["stormctrl_bcast_enabled"])
	assert.Equal(t, float64(100), wire["stormctrl_bcast_rate"])
	assert.Equal(t, false, wire["stormctrl_mcast_enabled"])
	assert.Equal(t, true, wire["stormctrl_ucast_enabled"])
	assert.Equal(t, float64(300), wire["stormctrl_ucast_rate"])
	assert.NotContains(t, wire, "stormctrl_bcast_level")
}

func TestPortProfileModelToAPI_TaggedNetworks(t *testing.T) {
	t.Run("excluded list is sent literally", func(t *testing.T) {
		m := defaultPortProfileModel("x")
		m.TaggedVLANMgmt = types.StringValue("custom")
		m.ExcludedNetworkIDs = stringSet(testNetB)
		assert.Equal(t, []any{testNetB}, toWire(t, mustModelToAPI(t, &m, nil))["excluded_networkconf_ids"])
	})

	t.Run("tagged list with no native network", func(t *testing.T) {
		m := defaultPortProfileModel("x")
		m.TaggedVLANMgmt = types.StringValue("custom")
		m.TaggedNetworkIDs = stringSet(testNetC)
		assert.ElementsMatch(t, []any{testDefaultNetID, testNetA, testNetB},
			toWire(t, mustModelToAPI(t, &m, testLANNetworks))["excluded_networkconf_ids"])
	})

	t.Run("tagged network that isn't a LAN network is an error", func(t *testing.T) {
		m := defaultPortProfileModel("x")
		m.TaggedVLANMgmt = types.StringValue("custom")
		m.TaggedNetworkIDs = stringSet("not-a-lan-network")
		_, diags := (&portProfileResource{}).modelToAPI(t.Context(), &m, testLANNetworks)
		require.True(t, diags.HasError())
		assert.Contains(t, diags.Errors()[0].Detail(), "is not a LAN network")
	})

	t.Run("block_all sends an empty deny-list and forward native", func(t *testing.T) {
		m := defaultPortProfileModel("x")
		m.TaggedVLANMgmt = types.StringValue("block_all")
		wire := toWire(t, mustModelToAPI(t, &m, nil))
		assert.Equal(t, []any{}, wire["excluded_networkconf_ids"])
		assert.Equal(t, "native", wire["forward"])
	})
}

func TestPortProfileForward(t *testing.T) {
	assert.Equal(t, "customize", portProfileForward("auto"))
	assert.Equal(t, "customize", portProfileForward("custom"))
	assert.Equal(t, "native", portProfileForward("block_all"))
}

// Every model permutation must survive model → API → JSON → API → model
// unchanged; any asymmetry would show up as a perpetual diff.
func TestPortProfileRoundTrip(t *testing.T) {
	cases := map[string]func(m *portProfileResourceModel){
		"defaults": func(m *portProfileResourceModel) {},
		"native only": func(m *portProfileResourceModel) {
			m.NativeNetworkID = types.StringValue(testNetA)
		},
		"trunk, tagged list": func(m *portProfileResourceModel) {
			m.NativeNetworkID = types.StringValue(testNetA)
			m.TaggedVLANMgmt = types.StringValue("custom")
			m.TaggedNetworkIDs = stringSet(testNetB, testNetC)
		},
		"trunk, excluded list": func(m *portProfileResourceModel) {
			m.NativeNetworkID = types.StringValue(testNetA)
			m.TaggedVLANMgmt = types.StringValue("custom")
			m.ExcludedNetworkIDs = stringSet(testDefaultNetID)
		},
		"access port": func(m *portProfileResourceModel) {
			m.NativeNetworkID = types.StringValue(testNetA)
			m.TaggedVLANMgmt = types.StringValue("block_all")
		},
		"voice network": func(m *portProfileResourceModel) {
			m.VoiceNetworkID = types.StringValue(testNetB)
		},
		"poe off": func(m *portProfileResourceModel) { m.PoeMode = types.StringValue("off") },
		"fixed speed half duplex": func(m *portProfileResourceModel) {
			m.Autoneg = types.BoolValue(false)
			m.Speed = types.Int64Value(100)
		},
		"fixed speed full duplex": func(m *portProfileResourceModel) {
			m.Autoneg = types.BoolValue(false)
			m.Speed = types.Int64Value(1000)
			m.FullDuplex = types.BoolValue(true)
		},
		"advanced flags flipped": func(m *portProfileResourceModel) {
			m.FlowControlEnabled = types.BoolValue(false)
			m.PTPEnabled = types.BoolValue(false)
			m.Isolation = types.BoolValue(true)
			m.StpEnabled = types.BoolValue(false)
			m.StpUplink = types.BoolValue(true)
			m.BpduGuardEnabled = types.BoolValue(true)
			m.LoopProtectionEnabled = types.BoolValue(true)
			m.EEEEnabled = types.BoolValue(true)
			m.LldpmedEnabled = types.BoolValue(false)
		},
		"egress limit":         func(m *portProfileResourceModel) { m.EgressRateLimitKbps = types.Int64Value(64) },
		"link debounce off":    func(m *portProfileResourceModel) { m.LinkDebounceMs = types.Int64Value(0) },
		"link debounce custom": func(m *portProfileResourceModel) { m.LinkDebounceMs = types.Int64Value(5000) },
		"dot1x multi-auth":     func(m *portProfileResourceModel) { m.Dot1XCtrl = types.StringValue("multi_host") },
		"mac filter": func(m *portProfileResourceModel) {
			m.PortSecurityEnabled = types.BoolValue(true)
			m.PortSecurityMACAddresses = stringSet("00:00:5e:00:53:01", "00:00:5e:00:53:02")
		},
		"storm control level, one class": func(m *portProfileResourceModel) {
			m.StormControl = &portProfileStormControlModel{
				Type: types.StringValue("level"), Broadcast: types.Int64Value(37),
				Multicast: types.Int64Null(), Unicast: types.Int64Null(),
			}
		},
		"storm control rate, all classes": func(m *portProfileResourceModel) {
			m.StormControl = &portProfileStormControlModel{
				Type: types.StringValue("rate"), Broadcast: types.Int64Value(12345),
				Multicast: types.Int64Value(14880000), Unicast: types.Int64Value(0),
			}
		},
	}

	r := &portProfileResource{}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			want := defaultPortProfileModel("round-trip")
			mutate(&want)
			want.ID = types.StringValue("000000000000000000000100")
			want.Site = types.StringValue("default")

			p := mustModelToAPI(t, &want, testLANNetworks)
			p.ID = want.ID.ValueString()

			b, err := json.Marshal(normalizePortProfile(p))
			require.NoError(t, err)
			var decoded unifi.PortProfile
			require.NoError(t, json.Unmarshal(b, &decoded))

			got := want // apiToModel reads the tagged/excluded choice from prior state
			r.apiToModel(&decoded, &got, "default", testLANNetworks)
			assert.Equal(t, want, got)
		})
	}
}

func TestMergePortProfile(t *testing.T) {
	var existing map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(uiDefaultPortProfileJSON), &existing))

	m := defaultPortProfileModel("new name")
	p := mustModelToAPI(t, &m, nil)
	p.ID = "000000000000000000000100"

	merged, err := mergePortProfile(existing, p)
	require.NoError(t, err)
	b, err := json.Marshal(merged)
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(b, &got))

	// Identity kept from the controller.
	assert.Equal(t, "000000000000000000000100", got["_id"])
	assert.Equal(t, "000000000000000000000001", got["site_id"])

	// Unmanaged settings passed through untouched.
	assert.Equal(t, map[string]any{"qos_policies": []any{}, "qos_profile_mode": "custom"}, got["qos_profile"])
	assert.Equal(t, "NONE", got["multicast_router_mode"])
	assert.Equal(t, float64(300), got["dot1x_idle_timeout"])
	assert.Equal(t, false, got["lldpmed_notify_enabled"])
	assert.Equal(t, "disabled", got["stp_edge_state"])

	// Managed settings replaced: advanced mode forced to manual.
	assert.Equal(t, "new name", got["name"])
	assert.Equal(t, "manual", got["setting_preference"])
	assert.Equal(t, "", got["native_networkconf_id"])
}

func TestPortProfileApplyPlanToState(t *testing.T) {
	r := &portProfileResource{}

	state := defaultPortProfileModel("before")
	state.ID = types.StringValue("000000000000000000000100")
	state.Site = types.StringValue("default")
	state.EgressRateLimitKbps = types.Int64Value(1000)
	state.LinkDebounceMs = types.Int64Value(1200)
	state.StormControl = &portProfileStormControlModel{
		Type: types.StringValue("level"), Broadcast: types.Int64Value(10),
		Multicast: types.Int64Null(), Unicast: types.Int64Null(),
	}

	plan := defaultPortProfileModel("after")
	plan.Site = types.StringValue("default")

	r.applyPlanToState(&plan, &state)

	assert.Equal(t, "000000000000000000000100", state.ID.ValueString(), "ID comes from state")
	assert.Equal(t, "after", state.Name.ValueString())
	assert.True(t, state.EgressRateLimitKbps.IsNull(), "null in plan clears egress limit")
	assert.True(t, state.LinkDebounceMs.IsNull(), "null in plan returns link debounce to auto")
	assert.Nil(t, state.StormControl, "null in plan clears storm control")
}

func TestValidatePortProfileModel(t *testing.T) {
	custom := func(m *portProfileResourceModel) {
		m.TaggedVLANMgmt = types.StringValue("custom")
		m.NativeNetworkID = types.StringValue(testNetA)
	}
	storm := func(scType types.String, b, mc, u types.Int64) *portProfileStormControlModel {
		return &portProfileStormControlModel{Type: scType, Broadcast: b, Multicast: mc, Unicast: u}
	}

	cases := []struct {
		name    string
		mutate  func(m *portProfileResourceModel)
		wantErr string
	}{
		{name: "defaults", mutate: func(m *portProfileResourceModel) {}},

		// Link speed.
		{
			name: "speed with autoneg off",
			mutate: func(m *portProfileResourceModel) {
				m.Autoneg = types.BoolValue(false)
				m.Speed = types.Int64Value(1000)
				m.FullDuplex = types.BoolValue(true)
			},
		},
		{
			name:    "speed with autoneg on",
			mutate:  func(m *portProfileResourceModel) { m.Speed = types.Int64Value(1000) },
			wantErr: "`speed` can only be set when `autoneg` is `false`",
		},
		{
			name: "speed with autoneg null (defaults to on)",
			mutate: func(m *portProfileResourceModel) {
				m.Autoneg = types.BoolNull()
				m.Speed = types.Int64Value(1000)
			},
			wantErr: "`speed` can only be set when `autoneg` is `false`",
		},
		{
			name:    "autoneg off without speed",
			mutate:  func(m *portProfileResourceModel) { m.Autoneg = types.BoolValue(false) },
			wantErr: "`speed` is required when `autoneg` is `false`",
		},
		{
			name:    "full duplex with autoneg on",
			mutate:  func(m *portProfileResourceModel) { m.FullDuplex = types.BoolValue(true) },
			wantErr: "`full_duplex` can only be `true` when `autoneg` is `false`",
		},
		{
			name: "speed unknown is not validated yet",
			mutate: func(m *portProfileResourceModel) {
				m.Autoneg = types.BoolValue(false)
				m.Speed = types.Int64Unknown()
			},
		},

		// Tagged networks.
		{
			name:   "custom with tagged list",
			mutate: func(m *portProfileResourceModel) { custom(m); m.TaggedNetworkIDs = stringSet(testNetB) },
		},
		{
			name:   "custom with excluded list",
			mutate: func(m *portProfileResourceModel) { custom(m); m.ExcludedNetworkIDs = stringSet(testNetB) },
		},
		{
			name:    "custom with neither",
			mutate:  custom,
			wantErr: "set exactly one of `tagged_network_ids` or `excluded_network_ids`",
		},
		{
			name:    "custom with empty tagged list",
			mutate:  func(m *portProfileResourceModel) { custom(m); m.TaggedNetworkIDs = stringSet() },
			wantErr: "`tagged_network_ids` can't be empty: use `tagged_vlan_mgmt = \"block_all\"`",
		},
		{
			name: "custom with both",
			mutate: func(m *portProfileResourceModel) {
				custom(m)
				m.TaggedNetworkIDs = stringSet(testNetB)
				m.ExcludedNetworkIDs = stringSet(testNetC)
			},
			wantErr: "set exactly one of",
		},
		{
			name:    "tagged list without custom",
			mutate:  func(m *portProfileResourceModel) { m.TaggedNetworkIDs = stringSet(testNetB) },
			wantErr: "can only be set when `tagged_vlan_mgmt` is `custom`",
		},
		{
			name:    "excluded list without custom",
			mutate:  func(m *portProfileResourceModel) { m.ExcludedNetworkIDs = stringSet(testNetB) },
			wantErr: "can only be set when `tagged_vlan_mgmt` is `custom`",
		},
		{
			name:    "native also tagged",
			mutate:  func(m *portProfileResourceModel) { custom(m); m.TaggedNetworkIDs = stringSet(testNetA) },
			wantErr: "The native network can't also be tagged",
		},
		{
			name:    "native excluded",
			mutate:  func(m *portProfileResourceModel) { custom(m); m.ExcludedNetworkIDs = stringSet(testNetA) },
			wantErr: "The native network can't be excluded",
		},
		{
			name:   "tagged list unknown is not validated yet",
			mutate: func(m *portProfileResourceModel) { custom(m); m.TaggedNetworkIDs = types.SetUnknown(types.StringType) },
		},

		// Voice VLAN.
		{
			name:   "voice with allow all",
			mutate: func(m *portProfileResourceModel) { m.VoiceNetworkID = types.StringValue(testNetB) },
		},
		{
			name: "voice in tagged list",
			mutate: func(m *portProfileResourceModel) {
				custom(m)
				m.TaggedNetworkIDs = stringSet(testNetB)
				m.VoiceNetworkID = types.StringValue(testNetB)
			},
		},
		{
			name: "voice not in tagged list",
			mutate: func(m *portProfileResourceModel) {
				custom(m)
				m.TaggedNetworkIDs = stringSet(testNetB)
				m.VoiceNetworkID = types.StringValue(testNetC)
			},
			wantErr: "The voice network must be one of `tagged_network_ids`",
		},
		{
			name: "voice in excluded list",
			mutate: func(m *portProfileResourceModel) {
				custom(m)
				m.ExcludedNetworkIDs = stringSet(testNetB)
				m.VoiceNetworkID = types.StringValue(testNetB)
			},
			wantErr: "The voice network can't be in `excluded_network_ids`",
		},
		{
			name: "voice with block all",
			mutate: func(m *portProfileResourceModel) {
				m.TaggedVLANMgmt = types.StringValue("block_all")
				m.VoiceNetworkID = types.StringValue(testNetB)
			},
			wantErr: "can't be set when `tagged_vlan_mgmt` is `block_all`",
		},
		{
			name: "voice without LLDP-MED",
			mutate: func(m *portProfileResourceModel) {
				m.LldpmedEnabled = types.BoolValue(false)
				m.VoiceNetworkID = types.StringValue(testNetB)
			},
			wantErr: "A voice network requires `lldpmed_enabled`",
		},

		// Link debounce.
		{name: "debounce off", mutate: func(m *portProfileResourceModel) { m.LinkDebounceMs = types.Int64Value(0) }},
		{name: "debounce min", mutate: func(m *portProfileResourceModel) { m.LinkDebounceMs = types.Int64Value(100) }},
		{name: "debounce max", mutate: func(m *portProfileResourceModel) { m.LinkDebounceMs = types.Int64Value(5000) }},
		{
			name:    "debounce not a multiple of 100",
			mutate:  func(m *portProfileResourceModel) { m.LinkDebounceMs = types.Int64Value(1234) },
			wantErr: "must be 0 (off) or 100–5000 in steps of 100, got 1234",
		},
		{
			name:    "debounce too high",
			mutate:  func(m *portProfileResourceModel) { m.LinkDebounceMs = types.Int64Value(5100) },
			wantErr: "got 5100",
		},
		{
			name:    "debounce between off and min",
			mutate:  func(m *portProfileResourceModel) { m.LinkDebounceMs = types.Int64Value(50) },
			wantErr: "got 50",
		},

		// Storm control.
		{
			name: "storm control with no classes",
			mutate: func(m *portProfileResourceModel) {
				m.StormControl = storm(types.StringNull(), types.Int64Null(), types.Int64Null(), types.Int64Null())
			},
			wantErr: "must set at least one of",
		},
		{
			name: "storm control level above 100",
			mutate: func(m *portProfileResourceModel) {
				m.StormControl = storm(types.StringNull(), types.Int64Value(101), types.Int64Null(), types.Int64Null())
			},
			wantErr: "`broadcast` must be between 0 and 100",
		},
		{
			name: "storm control rate above 100 is fine",
			mutate: func(m *portProfileResourceModel) {
				m.StormControl = storm(types.StringValue("rate"), types.Int64Value(12345), types.Int64Null(), types.Int64Null())
			},
		},
		{
			name: "storm control rate above max",
			mutate: func(m *portProfileResourceModel) {
				m.StormControl = storm(types.StringValue("rate"), types.Int64Null(), types.Int64Value(14880001), types.Int64Null())
			},
			wantErr: "`multicast` must be between 0 and 14880000",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := defaultPortProfileModel("validate")
			// Config values: defaulted attributes are null unless set.
			tc.mutate(&m)
			diags := validatePortProfileModel(t.Context(), &m)
			if tc.wantErr == "" {
				assert.False(t, diags.HasError(), "unexpected error: %v", diags)
				return
			}
			require.True(t, diags.HasError())
			var details []string
			for _, d := range diags.Errors() {
				details = append(details, d.Detail())
			}
			assert.Contains(t, strings.Join(details, "\n"), tc.wantErr)
		})
	}
}

// The framework only validates schemas when serving them, which unit tests
// never do. Validate explicitly so schema mistakes (e.g. a Default on a
// non-computed attribute) fail here instead of in acceptance tests.
func TestPortProfileSchemaValid(t *testing.T) {
	var resp fwresource.SchemaResponse
	(&portProfileResource{}).Schema(t.Context(), fwresource.SchemaRequest{}, &resp)
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
	diags := resp.Schema.ValidateImplementation(t.Context())
	require.False(t, diags.HasError(), "%v", diags)
}

// ---------------------------------------------------------------------------
// API tests against a fake controller
// ---------------------------------------------------------------------------

// fakePortconfController is a minimal in-memory /rest/portconf endpoint (plus
// a read-only /rest/networkconf) that records every request body it receives.
// Like the real controller, PUT merges into the stored object.
type fakePortconfController struct {
	mu       sync.Mutex
	profiles map[string]map[string]any
	networks []map[string]any
	nextID   int
	requests []fakeRequest
	// excludeAfterWrite mimics the controller's network-create background job
	// running just after the next POST or PUT: those network IDs are added to
	// the written profile's exclude list, once.
	excludeAfterWrite []string
}

type fakeRequest struct {
	Method string
	Path   string
	Body   map[string]any
}

func newFakePortconfController(t *testing.T) (*fakePortconfController, *Client) {
	t.Helper()
	f := &fakePortconfController{profiles: map[string]map[string]any{}, nextID: 100}
	for _, id := range testLANNetworks {
		f.networks = append(f.networks, map[string]any{"_id": id, "purpose": "corporate"})
	}
	f.networks = append(f.networks, map[string]any{"_id": "000000000000000000000099", "purpose": "wan"})
	srv := httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(srv.Close)
	return f, newTestClient(t, srv.URL, false)
}

func (f *fakePortconfController) handle(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	reply := func(data ...map[string]any) {
		if data == nil {
			data = []map[string]any{}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"meta": map[string]any{"rc": "ok"}, "data": data})
	}

	if r.Method == http.MethodGet && r.URL.Path == "/proxy/network/api/s/default/rest/networkconf" {
		reply(f.networks...)
		return
	}

	const prefix = "/proxy/network/api/s/default/rest/portconf"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		http.NotFound(w, r)
		return
	}
	id := strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, prefix), "/")

	// Decode twice: one copy is recorded as sent, the other may be mutated
	// into the stored object.
	var body, recorded map[string]any
	if b, _ := io.ReadAll(r.Body); len(b) > 0 {
		_ = json.Unmarshal(b, &body)
		_ = json.Unmarshal(b, &recorded)
	}
	f.requests = append(f.requests, fakeRequest{Method: r.Method, Path: r.URL.Path, Body: recorded})

	switch {
	case r.Method == http.MethodGet && id == "":
		all := make([]map[string]any, 0, len(f.profiles))
		for _, p := range f.profiles {
			all = append(all, p)
		}
		reply(all...)
	case r.Method == http.MethodPost && id == "":
		f.nextID++
		newID := fmt.Sprintf("%024d", f.nextID)
		body["_id"] = newID
		body["site_id"] = "000000000000000000000001"
		// Mimic the controller filling in fields the provider doesn't manage.
		body["qos_profile"] = map[string]any{"qos_policies": []any{}, "qos_profile_mode": "custom"}
		body["dot1x_idle_timeout"] = float64(300)
		f.profiles[newID] = body
		reply(body)
		f.runExcludeJob(newID)
	case r.Method == http.MethodPut && id != "":
		if _, ok := f.profiles[id]; !ok {
			http.NotFound(w, r)
			return
		}
		for k, v := range body {
			f.profiles[id][k] = v
		}
		reply(f.profiles[id])
		f.runExcludeJob(id)
	case r.Method == http.MethodDelete && id != "":
		if _, ok := f.profiles[id]; !ok {
			http.NotFound(w, r)
			return
		}
		delete(f.profiles, id)
		reply()
	default:
		http.Error(w, "unexpected request", http.StatusBadRequest)
	}
}

func TestPortProfileAPI_Lifecycle(t *testing.T) {
	fake, client := newFakePortconfController(t)
	ctx := t.Context()
	r := &portProfileResource{}

	// Create a trunk using the tagged allow-list.
	lan, err := client.ListLANNetworkIDs(ctx, "default")
	require.NoError(t, err)
	assert.Equal(t, testLANNetworks, lan, "WAN networks are not LAN networks")

	m := defaultPortProfileModel("lifecycle")
	m.NativeNetworkID = types.StringValue(testNetA)
	m.TaggedVLANMgmt = types.StringValue("custom")
	m.TaggedNetworkIDs = stringSet(testNetC)
	created, err := client.CreatePortProfile(ctx, "default", mustModelToAPI(t, &m, lan))
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)

	post := fake.requests[len(fake.requests)-1]
	assert.Equal(t, http.MethodPost, post.Method)
	assert.NotContains(t, post.Body, "_id")
	assert.ElementsMatch(t, []any{testDefaultNetID, testNetB}, post.Body["excluded_networkconf_ids"])

	// Get, and read back as the same model.
	got, err := client.GetPortProfile(ctx, "default", created.ID)
	require.NoError(t, err)
	readBack := m
	r.apiToModel(got, &readBack, "default", lan)
	assert.Equal(t, stringSet(testNetC), readBack.TaggedNetworkIDs)

	// Update: rename and switch to an access port. The PUT carries the
	// controller-filled unmanaged fields through and clears the deny-list.
	m.Name = types.StringValue("lifecycle-renamed")
	m.TaggedVLANMgmt = types.StringValue("block_all")
	m.TaggedNetworkIDs = types.SetNull(types.StringType)
	p := mustModelToAPI(t, &m, nil)
	p.ID = created.ID

	updated, err := client.UpdatePortProfile(ctx, "default", p)
	require.NoError(t, err)
	assert.Equal(t, "lifecycle-renamed", updated.Name)
	assert.Empty(t, updated.ExcludedNetworkIDs)

	put := fake.requests[len(fake.requests)-1]
	assert.Equal(t, http.MethodPut, put.Method)
	assert.True(t, strings.HasSuffix(put.Path, "/"+created.ID))
	assert.Equal(t, created.ID, put.Body["_id"])
	assert.Equal(t, float64(300), put.Body["dot1x_idle_timeout"])
	assert.Equal(t, "native", put.Body["forward"])

	// Delete, then reads report not found and a second delete is a no-op.
	require.NoError(t, client.DeletePortProfile(ctx, "default", created.ID))
	_, err = client.GetPortProfile(ctx, "default", created.ID)
	assert.IsType(t, &unifi.NotFoundError{}, err)
	require.NoError(t, client.DeletePortProfile(ctx, "default", created.ID))
}

func TestPortProfileAPI_UpdateMissingProfile(t *testing.T) {
	_, client := newFakePortconfController(t)
	_, err := client.UpdatePortProfile(t.Context(), "default", &unifi.PortProfile{ID: "does-not-exist", Name: "x"})
	assert.IsType(t, &unifi.NotFoundError{}, err)
}

// runExcludeJob applies excludeAfterWrite to the profile with the given ID.
// The caller holds f.mu.
func (f *fakePortconfController) runExcludeJob(id string) {
	if len(f.excludeAfterWrite) == 0 {
		return
	}
	excluded, _ := f.profiles[id]["excluded_networkconf_ids"].([]any)
	for _, n := range f.excludeAfterWrite {
		excluded = append(excluded, n)
	}
	f.profiles[id]["excluded_networkconf_ids"] = excluded
	f.excludeAfterWrite = nil
}

// freshNetworkID returns a network ID created at t, as the controller mints
// them (the first four bytes are the creation time).
func freshNetworkID(t time.Time) string {
	return fmt.Sprintf("%08x0000000000000042", t.Unix())
}

func TestAnyCreatedWithin(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	for _, tc := range []struct {
		name string
		ids  []string
		want bool
	}{
		{"none", nil, false},
		{"old", []string{testNetA, freshNetworkID(now.Add(-2 * time.Minute))}, false},
		{"fresh", []string{testNetA, freshNetworkID(now.Add(-5 * time.Second))}, true},
		{"controller clock ahead", []string{freshNetworkID(now.Add(20 * time.Second))}, true},
		{"far future", []string{freshNetworkID(now.Add(2 * time.Minute))}, false},
		{"not an ID", []string{"net-a"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, anyCreatedWithin(tc.ids, now, time.Minute))
		})
	}
}

// setupSettleTest returns a fake controller whose LAN networks include one
// created just now, and a trunk model tagging the given networks.
func setupSettleTest(t *testing.T, tagged ...string) (*fakePortconfController, *portProfileResource, portProfileResourceModel, []string) {
	t.Helper()
	delay := portProfileSettleDelay
	portProfileSettleDelay = 0
	t.Cleanup(func() { portProfileSettleDelay = delay })

	fake, client := newFakePortconfController(t)
	fake.networks = append(fake.networks, map[string]any{"_id": freshNetworkID(time.Now()), "purpose": "corporate"})
	lan, err := client.ListLANNetworkIDs(t.Context(), "default")
	require.NoError(t, err)

	m := defaultPortProfileModel("settle")
	m.NativeNetworkID = types.StringValue(testNetA)
	m.TaggedVLANMgmt = types.StringValue("custom")
	m.TaggedNetworkIDs = stringSet(tagged...)
	return fake, &portProfileResource{client: client}, m, lan
}

func requestMethods(f *fakePortconfController) []string {
	var ms []string
	for _, r := range f.requests {
		ms = append(ms, r.Method)
	}
	return ms
}

func TestSettleFreshTaggedNetworks_RewritesWhenControllerExcludes(t *testing.T) {
	fresh := freshNetworkID(time.Now())
	fake, r, m, lan := setupSettleTest(t, testNetC, fresh)
	ctx := t.Context()

	sent := mustModelToAPI(t, &m, lan)
	fake.excludeAfterWrite = []string{fresh}
	written, err := r.client.CreatePortProfile(ctx, "default", sent)
	require.NoError(t, err)
	assert.NotContains(t, written.ExcludedNetworkIDs, fresh, "the POST response predates the job")
	sent.ID = written.ID

	got, err := r.settleFreshTaggedNetworks(ctx, "default", &m, sent, written)
	require.NoError(t, err)
	assert.NotContains(t, got.ExcludedNetworkIDs, fresh)
	assert.ElementsMatch(t, []string{testDefaultNetID, testNetB}, got.ExcludedNetworkIDs)
	assert.Equal(t, []string{"POST", "GET", "GET", "PUT"}, requestMethods(fake),
		"created, re-read, then written again (UpdatePortProfile reads before its PUT)")

	stored, err := r.client.GetPortProfile(ctx, "default", written.ID)
	require.NoError(t, err)
	assert.NotContains(t, stored.ExcludedNetworkIDs, fresh)
}

func TestSettleFreshTaggedNetworks_ChecksOnlyWhenNothingExcluded(t *testing.T) {
	fresh := freshNetworkID(time.Now())
	fake, r, m, lan := setupSettleTest(t, fresh)

	sent := mustModelToAPI(t, &m, lan)
	written, err := r.client.CreatePortProfile(t.Context(), "default", sent)
	require.NoError(t, err)
	sent.ID = written.ID

	_, err = r.settleFreshTaggedNetworks(t.Context(), "default", &m, sent, written)
	require.NoError(t, err)
	assert.Equal(t, []string{"POST", "GET"}, requestMethods(fake), "re-read, no second write")
}

func TestSettleFreshTaggedNetworks_NoFreshNetworkNoRequests(t *testing.T) {
	fake, r, m, lan := setupSettleTest(t, testNetC)
	sent := mustModelToAPI(t, &m, lan)
	written, err := r.client.CreatePortProfile(t.Context(), "default", sent)
	require.NoError(t, err)
	sent.ID = written.ID

	// Even with the job pending, a profile not tagging a fresh network isn't
	// re-checked: the fresh network belongs in its exclude list anyway.
	got, err := r.settleFreshTaggedNetworks(t.Context(), "default", &m, sent, written)
	require.NoError(t, err)
	assert.Same(t, written, got)
	assert.Equal(t, []string{"POST"}, requestMethods(fake))
}

func TestSettleFreshTaggedNetworks_SkippedOutsideTaggedList(t *testing.T) {
	fresh := freshNetworkID(time.Now())
	for _, tc := range []struct {
		name  string
		setup func(m *portProfileResourceModel)
	}{
		{"allow all", func(m *portProfileResourceModel) {
			m.TaggedVLANMgmt = types.StringValue("auto")
			m.TaggedNetworkIDs = types.SetNull(types.StringType)
		}},
		{"excluded list", func(m *portProfileResourceModel) {
			m.TaggedNetworkIDs = types.SetNull(types.StringType)
			m.ExcludedNetworkIDs = stringSet(testNetB)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake, r, m, lan := setupSettleTest(t, fresh)
			tc.setup(&m)
			sent := mustModelToAPI(t, &m, lan)
			written := &unifi.PortProfile{ID: "p"}
			got, err := r.settleFreshTaggedNetworks(t.Context(), "default", &m, sent, written)
			require.NoError(t, err)
			assert.Same(t, written, got)
			assert.Empty(t, fake.requests)
		})
	}
}

// ---------------------------------------------------------------------------
// Acceptance tests
//
// These create, update and delete tfacc-* port profiles, and (in the tests
// marked below) tfacc-* networks. No profile is ever assigned to a device
// port, so they don't change how any physical port behaves. Creating a
// network makes the controller add it to existing custom profiles'
// exclude lists.
// ---------------------------------------------------------------------------

const testAccPortProfileAddr = "terrifi_port_profile.test"

// testAccPortProfileNetworks returns HCL for n throwaway corporate networks,
// terrifi_network.pp0 .. pp<n-1>, one block each. VLANs are random, so a test
// that adds networks across steps uses prefixes of one call's result (see
// networksHCL) to keep the earlier networks unchanged.
//
// The networks are independent, so Terraform deletes them concurrently, which
// testAccCheckNoStaleExcludedIDs relies on (see networkDeleteMu).
func testAccPortProfileNetworks(suffix string, n int) []string {
	blocks := make([]string, n)
	for i := range blocks {
		vlan := randomVLAN()
		blocks[i] = fmt.Sprintf(`
resource "terrifi_network" "pp%d" {
  name          = "tfacc-pp-net%d-%s"
  purpose       = "corporate"
  vlan_id       = %d
  subnet        = "10.%d.%d.1/24"
  network_group = "LAN"
  dhcp_enabled  = false
}
`, i, i, suffix, vlan, vlan/256, vlan%256)
	}
	return blocks
}

// testAccCheckNoStaleExcludedIDs verifies that no port profile on the site
// excludes a network that no longer exists. Deleting networks concurrently
// used to leave such IDs behind (see networkDeleteMu).
func testAccCheckNoStaleExcludedIDs(s *terraform.State) error {
	client := testAccGetClientNoT()
	site := client.SiteOrDefault(types.StringNull())
	ctx := context.Background()

	var nets struct {
		Data []struct {
			ID string `json:"_id"`
		} `json:"data"`
	}
	if err := client.doPortProfileRequest(ctx, http.MethodGet,
		fmt.Sprintf("%s%s/api/s/%s/rest/networkconf", client.BaseURL, client.APIPath, site), nil, &nets); err != nil {
		return err
	}
	exists := map[string]bool{}
	for _, n := range nets.Data {
		exists[n.ID] = true
	}
	if len(exists) == 0 {
		return fmt.Errorf("no networks listed")
	}

	profiles, err := client.ListPortProfiles(ctx, site)
	if err != nil {
		return err
	}
	for _, p := range profiles {
		for _, id := range p.ExcludedNetworkIDs {
			if !exists[id] {
				return fmt.Errorf("port profile %s excludes deleted network %s", p.ID, id)
			}
		}
	}
	return nil
}

// networksHCL joins the first n network blocks.
func networksHCL(blocks []string, n int) string {
	return strings.Join(blocks[:n], "")
}

// testAccCheckPortProfileDestroyed verifies that no port profile from the
// state still exists on the controller.
func testAccCheckPortProfileDestroyed(s *terraform.State) error {
	client := testAccGetClientNoT()
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "terrifi_port_profile" {
			continue
		}
		site := rs.Primary.Attributes["site"]
		_, err := client.GetPortProfile(context.Background(), site, rs.Primary.ID)
		var nf *unifi.NotFoundError
		if errors.As(err, &nf) {
			continue
		}
		if err != nil {
			return fmt.Errorf("checking port profile %s: %w", rs.Primary.ID, err)
		}
		return fmt.Errorf("port profile %s still exists", rs.Primary.ID)
	}
	return nil
}

// testAccGetClientNoT builds a client outside a *testing.T context (CheckDestroy
// and PreConfig callbacks don't receive one).
func testAccGetClientNoT() *Client {
	client, err := NewClient(context.Background(), ClientConfigFromEnv())
	if err != nil {
		panic(fmt.Sprintf("failed to create test client: %s", err))
	}
	return client
}

// testAccRawPortProfile reads the stored profile as raw JSON, to check fields
// the resource doesn't expose.
func testAccRawPortProfile(site, id string) (map[string]any, error) {
	client := testAccGetClientNoT()
	var resp struct {
		Data []map[string]any `json:"data"`
	}
	if err := client.doPortProfileRequest(context.Background(), http.MethodGet, client.portProfileURL(site, ""), nil, &resp); err != nil {
		return nil, err
	}
	for _, p := range resp.Data {
		if p["_id"] == id {
			return p, nil
		}
	}
	return nil, fmt.Errorf("port profile %s not found", id)
}

// testAccPutRawPortProfile changes stored fields behind Terraform's back.
func testAccPutRawPortProfile(site, id string, fields map[string]any) error {
	client := testAccGetClientNoT()
	fields["_id"] = id
	return client.doPortProfileRequest(context.Background(), http.MethodPut, client.portProfileURL(site, id), fields, nil)
}

func testAccPortProfileIDs(s *terraform.State) (site, id string, err error) {
	rs := s.RootModule().Resources[testAccPortProfileAddr]
	if rs == nil {
		return "", "", fmt.Errorf("resource not found in state")
	}
	return rs.Primary.Attributes["site"], rs.Primary.ID, nil
}

func portProfileDefaultsChecks() resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckNoResourceAttr(testAccPortProfileAddr, "native_network_id"),
		resource.TestCheckResourceAttr(testAccPortProfileAddr, "tagged_vlan_mgmt", "auto"),
		resource.TestCheckNoResourceAttr(testAccPortProfileAddr, "tagged_network_ids"),
		resource.TestCheckNoResourceAttr(testAccPortProfileAddr, "excluded_network_ids"),
		resource.TestCheckNoResourceAttr(testAccPortProfileAddr, "voice_network_id"),
		resource.TestCheckResourceAttr(testAccPortProfileAddr, "poe_mode", "auto"),
		resource.TestCheckResourceAttr(testAccPortProfileAddr, "autoneg", "true"),
		resource.TestCheckNoResourceAttr(testAccPortProfileAddr, "speed"),
		resource.TestCheckResourceAttr(testAccPortProfileAddr, "full_duplex", "false"),
		resource.TestCheckNoResourceAttr(testAccPortProfileAddr, "egress_rate_limit_kbps"),
		resource.TestCheckResourceAttr(testAccPortProfileAddr, "flow_control_enabled", "true"),
		resource.TestCheckResourceAttr(testAccPortProfileAddr, "ptp_enabled", "true"),
		resource.TestCheckResourceAttr(testAccPortProfileAddr, "isolation", "false"),
		resource.TestCheckResourceAttr(testAccPortProfileAddr, "stp_enabled", "true"),
		resource.TestCheckResourceAttr(testAccPortProfileAddr, "stp_uplink", "false"),
		resource.TestCheckResourceAttr(testAccPortProfileAddr, "bpdu_guard_enabled", "false"),
		resource.TestCheckResourceAttr(testAccPortProfileAddr, "loop_protection_enabled", "false"),
		resource.TestCheckResourceAttr(testAccPortProfileAddr, "eee_enabled", "false"),
		resource.TestCheckResourceAttr(testAccPortProfileAddr, "lldpmed_enabled", "true"),
		resource.TestCheckNoResourceAttr(testAccPortProfileAddr, "link_debounce_ms"),
		resource.TestCheckResourceAttr(testAccPortProfileAddr, "port_security_enabled", "false"),
		resource.TestCheckResourceAttr(testAccPortProfileAddr, "port_security_mac_addresses.#", "0"),
		resource.TestCheckResourceAttr(testAccPortProfileAddr, "dot1x_ctrl", "force_authorized"),
		resource.TestCheckNoResourceAttr(testAccPortProfileAddr, "storm_control"),
	)
}

func testAccPortProfileConfig(name, body string) string {
	return fmt.Sprintf(`
resource "terrifi_port_profile" "test" {
  name = %q
%s
}
`, name, body)
}

func TestAccPortProfile_basic(t *testing.T) {
	name := fmt.Sprintf("tfacc-pp-basic-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPortProfileDestroyed,
		Steps: []resource.TestStep{
			{
				Config: testAccPortProfileConfig(name, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(testAccPortProfileAddr, "id"),
					resource.TestCheckResourceAttrSet(testAccPortProfileAddr, "site"),
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "name", name),
					portProfileDefaultsChecks(),
				),
			},
			// Rename in place.
			{
				Config: testAccPortProfileConfig(name+"-renamed", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccPortProfileAddr, plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.TestCheckResourceAttr(testAccPortProfileAddr, "name", name+"-renamed"),
			},
			{
				ResourceName:      testAccPortProfileAddr,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccPortProfile_allSettingsThenDefaults sets every non-VLAN attribute to
// a non-default value, then removes them all. Removing proves that clearing
// an attribute in config clears it on the controller, given that the
// controller merges PUTs. Settings the Docker controller doesn't store are
// covered by TestAccPortProfile_newerSettings.
func TestAccPortProfile_allSettingsThenDefaults(t *testing.T) {
	name := fmt.Sprintf("tfacc-pp-all-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPortProfileDestroyed,
		Steps: []resource.TestStep{
			{
				Config: testAccPortProfileConfig(name, `
  poe_mode                    = "off"
  autoneg                     = false
  speed                       = 100
  full_duplex                 = true
  egress_rate_limit_kbps      = 10000
  isolation                   = true
  stp_enabled                 = false
  stp_uplink                  = true
  bpdu_guard_enabled          = true
  loop_protection_enabled     = true
  lldpmed_enabled             = false
  port_security_enabled       = true
  port_security_mac_addresses = ["00:00:5e:00:53:01", "00:00:5e:00:53:02"]
  dot1x_ctrl                  = "multi_host"
  storm_control = {
    type      = "level"
    broadcast = 20
    multicast = 30
    unicast   = 40
  }
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "poe_mode", "off"),
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "autoneg", "false"),
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "speed", "100"),
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "full_duplex", "true"),
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "egress_rate_limit_kbps", "10000"),
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "isolation", "true"),
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "stp_enabled", "false"),
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "stp_uplink", "true"),
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "bpdu_guard_enabled", "true"),
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "loop_protection_enabled", "true"),
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "lldpmed_enabled", "false"),
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "port_security_enabled", "true"),
					resource.TestCheckTypeSetElemAttr(testAccPortProfileAddr, "port_security_mac_addresses.*", "00:00:5e:00:53:01"),
					resource.TestCheckTypeSetElemAttr(testAccPortProfileAddr, "port_security_mac_addresses.*", "00:00:5e:00:53:02"),
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "dot1x_ctrl", "multi_host"),
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "storm_control.type", "level"),
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "storm_control.broadcast", "20"),
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "storm_control.multicast", "30"),
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "storm_control.unicast", "40"),
				),
			},
			{
				ResourceName:      testAccPortProfileAddr,
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Remove everything: all attributes revert to defaults or unset.
			{
				Config: testAccPortProfileConfig(name, ""),
				Check:  portProfileDefaultsChecks(),
			},
			// And set some again from defaults, to catch create-vs-update differences.
			{
				Config: testAccPortProfileConfig(name, `
  poe_mode               = "off"
  egress_rate_limit_kbps = 64
  storm_control          = { unicast = 100 }
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "poe_mode", "off"),
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "egress_rate_limit_kbps", "64"),
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "storm_control.type", "level"),
					resource.TestCheckNoResourceAttr(testAccPortProfileAddr, "storm_control.broadcast"),
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "storm_control.unicast", "100"),
				),
			},
		},
	})
}

// TestAccPortProfile_linkSettings toggles between auto-negotiation and fixed
// speed/duplex. The controller leaves speed and duplex behind when autoneg is
// re-enabled; that must not surface as a diff.
func TestAccPortProfile_linkSettings(t *testing.T) {
	name := fmt.Sprintf("tfacc-pp-link-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPortProfileDestroyed,
		Steps: []resource.TestStep{
			{
				Config: testAccPortProfileConfig(name, `
  autoneg = false
  speed   = 100
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "autoneg", "false"),
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "speed", "100"),
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "full_duplex", "false"),
				),
			},
			{
				Config: testAccPortProfileConfig(name, `
  autoneg     = false
  speed       = 1000
  full_duplex = true
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "speed", "1000"),
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "full_duplex", "true"),
				),
			},
			{
				Config: testAccPortProfileConfig(name, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "autoneg", "true"),
					resource.TestCheckNoResourceAttr(testAccPortProfileAddr, "speed"),
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "full_duplex", "false"),
				),
			},
			// Fixed again, to a value different from the stale one, half duplex.
			{
				Config: testAccPortProfileConfig(name, `
  autoneg = false
  speed   = 10
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "speed", "10"),
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "full_duplex", "false"),
				),
			},
		},
	})
}

// TestAccPortProfile_newerSettings covers settings the Docker test controller
// (Network Application 10.3.58) doesn't store: it drops eee_enabled,
// flow_control_enabled, precision_time_protocol_enabled and link debounce on
// write. UniFi OS 5.1 / Network 10.x stores them, so this runs on hardware only.
func TestAccPortProfile_newerSettings(t *testing.T) {
	if os.Getenv("TERRIFI_ACC_TARGET") != "hardware" {
		t.Skip("the Docker controller doesn't store these settings (TERRIFI_ACC_TARGET=hardware)")
	}
	name := fmt.Sprintf("tfacc-pp-newer-%s", randomSuffix())
	step := func(body string, checks ...resource.TestCheckFunc) resource.TestStep {
		return resource.TestStep{Config: testAccPortProfileConfig(name, body), Check: resource.ComposeAggregateTestCheckFunc(checks...)}
	}
	attr := func(k, v string) resource.TestCheckFunc {
		return resource.TestCheckResourceAttr(testAccPortProfileAddr, k, v)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPortProfileDestroyed,
		Steps: []resource.TestStep{
			step(`
  link_debounce_ms     = 0
  eee_enabled          = true
  flow_control_enabled = false
  ptp_enabled          = false
`, attr("link_debounce_ms", "0"), attr("eee_enabled", "true"), attr("flow_control_enabled", "false"), attr("ptp_enabled", "false")),
			{ResourceName: testAccPortProfileAddr, ImportState: true, ImportStateVerify: true},
			step(`  link_debounce_ms = 1200`, attr("link_debounce_ms", "1200"),
				attr("eee_enabled", "false"), attr("flow_control_enabled", "true"), attr("ptp_enabled", "true")),
			step("", resource.TestCheckNoResourceAttr(testAccPortProfileAddr, "link_debounce_ms")),
			step(`  link_debounce_ms = 5000`, attr("link_debounce_ms", "5000")),
			step(`  link_debounce_ms = 0`, attr("link_debounce_ms", "0")),
		},
	})
}

func TestAccPortProfile_stormControl(t *testing.T) {
	name := fmt.Sprintf("tfacc-pp-storm-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPortProfileDestroyed,
		Steps: []resource.TestStep{
			{
				Config: testAccPortProfileConfig(name, `
  storm_control = { broadcast = 37 }
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "storm_control.type", "level"),
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "storm_control.broadcast", "37"),
					resource.TestCheckNoResourceAttr(testAccPortProfileAddr, "storm_control.multicast"),
					resource.TestCheckNoResourceAttr(testAccPortProfileAddr, "storm_control.unicast"),
				),
			},
			// Switch to rate: the old level value must not leak into the rate.
			{
				Config: testAccPortProfileConfig(name, `
  storm_control = {
    type      = "rate"
    broadcast = 12345
    multicast = 6000
  }
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "storm_control.type", "rate"),
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "storm_control.broadcast", "12345"),
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "storm_control.multicast", "6000"),
				),
			},
			// Disable one class.
			{
				Config: testAccPortProfileConfig(name, `
  storm_control = {
    type      = "rate"
    multicast = 6000
  }
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr(testAccPortProfileAddr, "storm_control.broadcast"),
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "storm_control.multicast", "6000"),
				),
			},
			{
				ResourceName:      testAccPortProfileAddr,
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Remove the block entirely.
			{
				Config: testAccPortProfileConfig(name, ""),
				Check:  resource.TestCheckNoResourceAttr(testAccPortProfileAddr, "storm_control"),
			},
		},
	})
}

// TestAccPortProfile_unmanagedFieldsPreserved changes fields the resource
// doesn't manage behind Terraform's back, including setting Advanced back to
// "auto" (as a UI-created profile would be), then updates the profile. The
// unmanaged fields must survive and Advanced must be forced back to "manual".
func TestAccPortProfile_unmanagedFieldsPreserved(t *testing.T) {
	name := fmt.Sprintf("tfacc-pp-unmanaged-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPortProfileDestroyed,
		Steps: []resource.TestStep{
			{
				Config: testAccPortProfileConfig(name, ""),
				Check: func(s *terraform.State) error {
					site, id, err := testAccPortProfileIDs(s)
					if err != nil {
						return err
					}
					raw, err := testAccRawPortProfile(site, id)
					if err != nil {
						return err
					}
					if raw["setting_preference"] != "manual" {
						return fmt.Errorf("setting_preference = %v after create, want manual", raw["setting_preference"])
					}
					if err := testAccPutRawPortProfile(site, id, map[string]any{
						"setting_preference":     "auto",
						"dot1x_idle_timeout":     123,
						"lldpmed_notify_enabled": true,
					}); err != nil {
						return err
					}
					// Only fields that actually stuck can prove preservation.
					raw, err = testAccRawPortProfile(site, id)
					if err != nil {
						return err
					}
					t.Logf("after out-of-band PUT: setting_preference=%v dot1x_idle_timeout=%v lldpmed_notify_enabled=%v",
						raw["setting_preference"], raw["dot1x_idle_timeout"], raw["lldpmed_notify_enabled"])
					if raw["setting_preference"] != "auto" || raw["dot1x_idle_timeout"] != float64(123) {
						return fmt.Errorf("out-of-band PUT didn't stick: setting_preference=%v dot1x_idle_timeout=%v",
							raw["setting_preference"], raw["dot1x_idle_timeout"])
					}
					return nil
				},
			},
			{
				Config: testAccPortProfileConfig(name+"-renamed", ""),
				Check: func(s *terraform.State) error {
					site, id, err := testAccPortProfileIDs(s)
					if err != nil {
						return err
					}
					raw, err := testAccRawPortProfile(site, id)
					if err != nil {
						return err
					}
					if raw["setting_preference"] != "manual" {
						return fmt.Errorf("setting_preference = %v after update, want manual", raw["setting_preference"])
					}
					t.Logf("after terraform update: setting_preference=%v dot1x_idle_timeout=%v lldpmed_notify_enabled=%v",
						raw["setting_preference"], raw["dot1x_idle_timeout"], raw["lldpmed_notify_enabled"])
					if raw["dot1x_idle_timeout"] != float64(123) {
						return fmt.Errorf("unmanaged field not preserved: dot1x_idle_timeout=%v", raw["dot1x_idle_timeout"])
					}
					return nil
				},
			},
		},
	})
}

func TestAccPortProfile_importSiteID(t *testing.T) {
	name := fmt.Sprintf("tfacc-pp-impsid-%s", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPortProfileDestroyed,
		Steps: []resource.TestStep{
			{
				Config: testAccPortProfileConfig(name, `
  poe_mode  = "off"
  isolation = true
`),
			},
			{
				ResourceName:      testAccPortProfileAddr,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					site, id, err := testAccPortProfileIDs(s)
					return fmt.Sprintf("%s:%s", site, id), err
				},
			},
		},
	})
}

// TestAccPortProfile_disappears deletes the profile behind Terraform's back
// and expects the next plan to recreate it.
func TestAccPortProfile_disappears(t *testing.T) {
	name := fmt.Sprintf("tfacc-pp-gone-%s", randomSuffix())
	var id, site string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPortProfileDestroyed,
		Steps: []resource.TestStep{
			{
				Config: testAccPortProfileConfig(name, ""),
				Check: func(s *terraform.State) (err error) {
					site, id, err = testAccPortProfileIDs(s)
					return err
				},
			},
			{
				PreConfig: func() {
					if err := testAccGetClientNoT().DeletePortProfile(context.Background(), site, id); err != nil {
						t.Fatalf("deleting port profile out of band: %s", err)
					}
				},
				Config: testAccPortProfileConfig(name, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccPortProfileAddr, plancheck.ResourceActionCreate),
					},
				},
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Acceptance tests that create networks
// ---------------------------------------------------------------------------

// TestAccPortProfile_vlanTrunk walks a profile through the VLAN
// configurations used for hypervisor trunks, switch uplinks and access ports,
// using both the tagged allow-list and the literal excluded list.
func TestAccPortProfile_vlanTrunk(t *testing.T) {
	suffix := randomSuffix()
	name := fmt.Sprintf("tfacc-pp-trunk-%s", suffix)
	nets := networksHCL(testAccPortProfileNetworks(suffix, 3), 3)
	// The profile waits for all networks: one created after it isn't excluded
	// from it until about a second later, and the post-apply refresh would
	// read it as tagged in the meantime.
	cfg := func(body string) string {
		return nets + testAccPortProfileConfig(name, body+"  depends_on = [terrifi_network.pp2]\n")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             resource.ComposeAggregateTestCheckFunc(testAccCheckPortProfileDestroyed, testAccCheckNoStaleExcludedIDs),
		Steps: []resource.TestStep{
			// Native pp0, tagged pp1 only.
			{
				Config: cfg(`
  native_network_id  = terrifi_network.pp0.id
  tagged_vlan_mgmt   = "custom"
  tagged_network_ids = [terrifi_network.pp1.id]
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(testAccPortProfileAddr, "native_network_id", "terrifi_network.pp0", "id"),
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "tagged_vlan_mgmt", "custom"),
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "tagged_network_ids.#", "1"),
					resource.TestCheckTypeSetElemAttrPair(testAccPortProfileAddr, "tagged_network_ids.*", "terrifi_network.pp1", "id"),
					resource.TestCheckNoResourceAttr(testAccPortProfileAddr, "excluded_network_ids"),
				),
			},
			// Tag pp2 too, with pp2 as the voice VLAN.
			{
				Config: cfg(`
  native_network_id  = terrifi_network.pp0.id
  tagged_vlan_mgmt   = "custom"
  tagged_network_ids = [terrifi_network.pp1.id, terrifi_network.pp2.id]
  voice_network_id   = terrifi_network.pp2.id
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "tagged_network_ids.#", "2"),
					resource.TestCheckResourceAttrPair(testAccPortProfileAddr, "voice_network_id", "terrifi_network.pp2", "id"),
				),
			},
			{
				ResourceName:      testAccPortProfileAddr,
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Swap the native network and the tagged one; voice VLAN removed.
			{
				Config: cfg(`
  native_network_id  = terrifi_network.pp2.id
  tagged_vlan_mgmt   = "custom"
  tagged_network_ids = [terrifi_network.pp0.id]
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(testAccPortProfileAddr, "native_network_id", "terrifi_network.pp2", "id"),
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "tagged_network_ids.#", "1"),
					resource.TestCheckTypeSetElemAttrPair(testAccPortProfileAddr, "tagged_network_ids.*", "terrifi_network.pp0", "id"),
					resource.TestCheckNoResourceAttr(testAccPortProfileAddr, "voice_network_id"),
				),
			},
			// The literal deny-list form.
			{
				Config: cfg(`
  native_network_id    = terrifi_network.pp2.id
  tagged_vlan_mgmt     = "custom"
  excluded_network_ids = [terrifi_network.pp0.id]
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr(testAccPortProfileAddr, "tagged_network_ids"),
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "excluded_network_ids.#", "1"),
					resource.TestCheckTypeSetElemAttrPair(testAccPortProfileAddr, "excluded_network_ids.*", "terrifi_network.pp0", "id"),
				),
			},
			// Access port: native only, block all tagged.
			{
				Config: cfg(`
  native_network_id = terrifi_network.pp1.id
  tagged_vlan_mgmt  = "block_all"
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "tagged_vlan_mgmt", "block_all"),
					resource.TestCheckNoResourceAttr(testAccPortProfileAddr, "tagged_network_ids"),
					resource.TestCheckNoResourceAttr(testAccPortProfileAddr, "excluded_network_ids"),
				),
			},
			// Allow all with a voice VLAN.
			{
				Config: cfg(`
  native_network_id = terrifi_network.pp1.id
  voice_network_id  = terrifi_network.pp0.id
`),
				Check: resource.TestCheckResourceAttrPair(testAccPortProfileAddr, "voice_network_id", "terrifi_network.pp0", "id"),
			},
			// Back to defaults: native and voice network cleared.
			{
				Config: cfg(""),
				Check:  portProfileDefaultsChecks(),
			},
		},
	})
}

// TestAccPortProfile_newNetworks covers the controller excluding every newly
// created network from all existing custom profiles, in a background job about
// a second after the network is created:
//
//   - A network created in the same apply as a profile that tags it stays
//     tagged (settleFreshTaggedNetworks re-applies the tag on Create).
//   - A network created later and not tagged is excluded by the controller,
//     which is what tagged_network_ids means, so the plan stays clean.
//   - A network created in the same apply as an update that tags it on an
//     existing profile stays tagged (the same, on Update).
func TestAccPortProfile_newNetworks(t *testing.T) {
	suffix := randomSuffix()
	name := fmt.Sprintf("tfacc-pp-newnet-%s", suffix)
	nets := testAccPortProfileNetworks(suffix, 4)
	profile := func(tagged string) string {
		return testAccPortProfileConfig(name, `
  native_network_id  = terrifi_network.pp0.id
  tagged_vlan_mgmt   = "custom"
  tagged_network_ids = [`+tagged+`]
`)
	}

	// checkExcluded verifies the stored exclude list: tagged networks are
	// absent and excluded ones present. It waits out the background job
	// first, so the post-apply plan sees the settled profile.
	checkExcluded := func(tagged, excluded []string) resource.TestCheckFunc {
		return func(s *terraform.State) error {
			time.Sleep(3 * time.Second)
			site, id, err := testAccPortProfileIDs(s)
			if err != nil {
				return err
			}
			raw, err := testAccRawPortProfile(site, id)
			if err != nil {
				return err
			}
			stored, _ := raw["excluded_networkconf_ids"].([]any)
			has := func(addr string) bool {
				netID := s.RootModule().Resources[addr].Primary.ID
				return slices.Contains(stored, any(netID))
			}
			for _, addr := range tagged {
				if has(addr) {
					return fmt.Errorf("%s is excluded, want tagged", addr)
				}
			}
			for _, addr := range excluded {
				if !has(addr) {
					return fmt.Errorf("%s is tagged, want excluded", addr)
				}
			}
			return nil
		}
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             resource.ComposeAggregateTestCheckFunc(testAccCheckPortProfileDestroyed, testAccCheckNoStaleExcludedIDs),
		Steps: []resource.TestStep{
			// Networks and profile created together; pp1 tagged.
			{
				Config: networksHCL(nets, 2) + profile("terrifi_network.pp1.id"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "tagged_network_ids.#", "1"),
					resource.TestCheckTypeSetElemAttrPair(testAccPortProfileAddr, "tagged_network_ids.*", "terrifi_network.pp1", "id"),
					checkExcluded([]string{"terrifi_network.pp1"}, nil),
				),
			},
			// pp2 created, profile unchanged: the controller excludes pp2.
			{
				Config: networksHCL(nets, 3) + profile("terrifi_network.pp1.id"),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkExcluded([]string{"terrifi_network.pp1"}, []string{"terrifi_network.pp2"}),
				),
			},
			// pp3 created and tagged, together with pp2, on the existing
			// profile in the same apply.
			{
				Config: networksHCL(nets, 4) +
					profile("terrifi_network.pp1.id, terrifi_network.pp2.id, terrifi_network.pp3.id"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(testAccPortProfileAddr, "tagged_network_ids.#", "3"),
					checkExcluded([]string{"terrifi_network.pp1", "terrifi_network.pp2", "terrifi_network.pp3"}, nil),
				),
			},
		},
	})
}
