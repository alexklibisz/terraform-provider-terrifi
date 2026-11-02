package provider

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

// These lifecycle tests check controller persistence on both acceptance targets,
// including Network 10.6.106 in Docker. They do not check switch enforcement.
func TestAccPortProfile_linkTuning(t *testing.T) {
	name := fmt.Sprintf("tfacc-pp-link-%s", randomSuffix())
	step := func(body string, checks ...resource.TestCheckFunc) resource.TestStep {
		return resource.TestStep{Config: testAccPortProfileConfig(name, body), Check: resource.ComposeAggregateTestCheckFunc(checks...)}
	}
	attr := func(k, v string) resource.TestCheckFunc {
		return resource.TestCheckResourceAttr(testAccPortProfileAddr, k, v)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheckPortProfile(t) },
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

func TestPortProfileAdvancedSettingsRoundTrip(t *testing.T) {
	for _, unit := range []string{"percentage", "packets_per_second"} {
		for _, threshold := range []int64{0, 37, 100} {
			for _, debounce := range []types.Int64{types.Int64Null(), types.Int64Value(0), types.Int64Value(100), types.Int64Value(5000)} {
				t.Run(fmt.Sprintf("%s/%d/%s", unit, threshold, debounce.String()), func(t *testing.T) {
					want := defaultPortProfileModel("advanced")
					want.ID = types.StringValue("id")
					want.Site = types.StringValue("default")
					want.FlowControlEnabled = types.BoolValue(false)
					want.PTPEnabled = types.BoolValue(false)
					want.StpEnabled = types.BoolValue(false)
					want.BpduGuardEnabled = types.BoolValue(true)
					want.LoopProtectionEnabled = types.BoolValue(true)
					want.EEEEnabled = types.BoolValue(true)
					want.LinkDebounceMs = debounce
					want.StormControlType = types.StringValue(unit)
					want.StormControlBroadcast = types.Int64Value(threshold)
					want.StormControlMulticast = types.Int64Value(threshold)
					want.StormControlUnknownUnicast = types.Int64Value(threshold)
					p := mustModelToAPI(t, &want, nil)
					wire := toWire(t, p)
					for key, expected := range map[string]any{
						"flow_control_enabled": false, "precision_time_protocol_enabled": false,
						"stp_port_mode": false, "stp_bpdu_guard_enabled": true,
						"port_keepalive_enabled": true, "eee_enabled": true,
						"link_debounce_auto": debounce.IsNull(),
					} {
						assert.Equal(t, expected, wire[key], key)
					}
					suffix := "level"
					if unit == "packets_per_second" {
						suffix = "rate"
					}
					for _, class := range []string{"bcast", "mcast", "ucast"} {
						assert.Equal(t, true, wire["stormctrl_"+class+"_enabled"])
						assert.Equal(t, float64(threshold), wire["stormctrl_"+class+"_"+suffix])
					}
					p.ID = "id"
					var got portProfileResourceModel
					(&portProfileResource{}).apiToModel(p, &got, "default", nil)
					assert.Equal(t, want, got)
				})
			}
		}
	}
}

func TestPortProfileAdvancedSettingsValidation(t *testing.T) {
	for _, tc := range []struct {
		unit  string
		value int64
		valid bool
	}{
		{"percentage", -1, false}, {"percentage", 0, true}, {"percentage", 100, true}, {"percentage", 101, false},
		{"packets_per_second", -1, false}, {"packets_per_second", 0, true}, {"packets_per_second", 14880000, true}, {"packets_per_second", 14880001, false},
	} {
		for _, class := range []string{"broadcast", "multicast", "unknown_unicast"} {
			t.Run(fmt.Sprintf("%s/%s/%d", tc.unit, class, tc.value), func(t *testing.T) {
				m := defaultPortProfileModel("validation")
				m.TaggedVLANMgmt = types.StringUnknown() // independent validation still runs
				m.StormControlType = types.StringValue(tc.unit)
				field := map[string]*types.Int64{"broadcast": &m.StormControlBroadcast, "multicast": &m.StormControlMulticast, "unknown_unicast": &m.StormControlUnknownUnicast}[class]
				*field = types.Int64Value(tc.value)
				assert.Equal(t, !tc.valid, validatePortProfileModel(&m).HasError())
			})
		}
	}
	for _, v := range []int64{-100, 1, 99, 101, 5100, 0, 100, 1200, 5000} {
		m := defaultPortProfileModel("debounce")
		m.LinkDebounceMs = types.Int64Value(v)
		valid := v == 0 || (v >= 100 && v <= 5000 && v%100 == 0)
		assert.Equal(t, !valid, validatePortProfileModel(&m).HasError(), "debounce %d", v)
	}
	m := defaultPortProfileModel("unknown")
	m.LinkDebounceMs = types.Int64Unknown()
	m.StormControlType = types.StringUnknown()
	m.StormControlBroadcast = types.Int64Value(5000)
	assert.False(t, validatePortProfileModel(&m).HasError(), "defer unit-dependent checks until known")
	m.StormControlType = types.StringNull()
	assert.True(t, validatePortProfileModel(&m).HasError(), "omitted unit defaults to percentage")
	m.StormControlBroadcast = types.Int64Unknown()
	assert.False(t, validatePortProfileModel(&m).HasError())
}

func TestPortProfileAdvancedSettingsClearStaleValues(t *testing.T) {
	fake, client := newFakePortconfController(t)
	m := defaultPortProfileModel("clear")
	m.LinkDebounceMs = types.Int64Value(1200)
	m.StormControlType = types.StringValue("packets_per_second")
	m.StormControlBroadcast = types.Int64Value(12345)
	created, err := client.CreatePortProfile(t.Context(), "default", mustModelToAPI(t, &m, nil))
	require.NoError(t, err)
	m = defaultPortProfileModel("clear")
	p := mustModelToAPI(t, &m, nil)
	p.ID = created.ID
	updated, err := client.UpdatePortProfile(t.Context(), "default", p)
	require.NoError(t, err)
	var got portProfileResourceModel
	(&portProfileResource{}).apiToModel(updated, &got, "default", nil)
	assert.True(t, got.LinkDebounceMs.IsNull())
	assert.True(t, got.StormControlBroadcast.IsNull())
	assert.Equal(t, "percentage", got.StormControlType.ValueString())
	// The raw API preserves inactive thresholds; enabled/auto flags are authoritative.
	wire := fake.requests[len(fake.requests)-1].Body
	assert.Equal(t, float64(12345), wire["stormctrl_bcast_rate"])
	assert.Equal(t, float64(1200), wire["link_debounce"])
	assert.Equal(t, false, wire["stormctrl_bcast_enabled"])
	assert.Equal(t, true, wire["link_debounce_auto"])
}

func TestAccPortProfile_protection(t *testing.T) {
	for _, tc := range []struct{ name, value, def string }{
		{"stp_enabled", "false", "true"}, {"bpdu_guard_enabled", "true", "false"}, {"loop_protection_enabled", "true", "false"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			name := fmt.Sprintf("tfacc-pp-protect-%s", randomSuffix())
			cfg := testAccPortProfileConfig(name, fmt.Sprintf("%s = %s", tc.name, tc.value))
			resource.Test(t, resource.TestCase{
				PreCheck: func() { preCheckPortProfile(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, CheckDestroy: testAccCheckPortProfileDestroyed,
				Steps: []resource.TestStep{
					{Config: cfg, Check: resource.TestCheckResourceAttr(testAccPortProfileAddr, tc.name, tc.value)},
					{ResourceName: testAccPortProfileAddr, ImportState: true, ImportStateVerify: true},
					{Config: testAccPortProfileConfig(name, ""), Check: portProfileDefaultsChecks()},
					{Config: cfg, Check: resource.TestCheckResourceAttr(testAccPortProfileAddr, tc.name, tc.value)},
				},
			})
		})
	}
}

func TestAccPortProfile_stormControl(t *testing.T) {
	name := fmt.Sprintf("tfacc-pp-storm-%s", randomSuffix())
	step := func(unit string, b, m, u *int64) resource.TestStep {
		body := fmt.Sprintf("storm_control_type = %q\n", unit)
		checks := []resource.TestCheckFunc{resource.TestCheckResourceAttr(testAccPortProfileAddr, "storm_control_type", unit)}
		for i, class := range []string{"broadcast", "multicast", "unknown_unicast"} {
			key := "storm_control_" + class
			v := []*int64{b, m, u}[i]
			if v == nil {
				checks = append(checks, resource.TestCheckNoResourceAttr(testAccPortProfileAddr, key))
			} else {
				body += fmt.Sprintf("%s = %d\n", key, *v)
				checks = append(checks, resource.TestCheckResourceAttr(testAccPortProfileAddr, key, fmt.Sprint(*v)))
			}
		}
		return resource.TestStep{Config: testAccPortProfileConfig(name, body), Check: resource.ComposeAggregateTestCheckFunc(checks...)}
	}
	resource.Test(t, resource.TestCase{
		PreCheck: func() { preCheckPortProfile(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, CheckDestroy: testAccCheckPortProfileDestroyed,
		Steps: []resource.TestStep{
			// Default unit, including explicitly enabled zero and the percentage limit.
			{Config: testAccPortProfileConfig(name, "storm_control_broadcast = 37"), Check: resource.TestCheckResourceAttr(testAccPortProfileAddr, "storm_control_type", "percentage")},
			{ResourceName: testAccPortProfileAddr, ImportState: true, ImportStateVerify: true},
			step("percentage", ptrTo(int64(0)), ptrTo(int64(100)), ptrTo(int64(37))),
			step("packets_per_second", ptrTo(int64(12345)), ptrTo(int64(6000)), ptrTo(int64(14880000))),
			{ResourceName: testAccPortProfileAddr, ImportState: true, ImportStateVerify: true},
			step("packets_per_second", nil, ptrTo(int64(6000)), nil),
			step("percentage", ptrTo(int64(20)), nil, nil),
			{Config: testAccPortProfileConfig(name, ""), Check: portProfileDefaultsChecks()},
			// Re-enable after clearing; stale rate values must not leak into percentages.
			step("percentage", nil, nil, ptrTo(int64(10))),
			step("packets_per_second", nil, nil, nil),
			{ResourceName: testAccPortProfileAddr, ImportState: true, ImportStateVerify: true},
		},
	})
}

func TestAccPortProfile_advancedSettingsDrift(t *testing.T) {
	name := fmt.Sprintf("tfacc-pp-drift-%s", randomSuffix())
	var site, id string
	config := testAccPortProfileConfig(name, `
 stp_enabled = true
 bpdu_guard_enabled = true
 loop_protection_enabled = true
 eee_enabled = true
 flow_control_enabled = false
 ptp_enabled = false
 link_debounce_ms = 1200
 storm_control_broadcast = 37
 `)
	resource.Test(t, resource.TestCase{
		PreCheck: func() { preCheckPortProfile(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, CheckDestroy: testAccCheckPortProfileDestroyed,
		Steps: []resource.TestStep{
			{Config: config, Check: func(state *terraform.State) error {
				var err error
				site, id, err = testAccPortProfileIDs(state)
				return err
			}},
			{PreConfig: func() {
				require.NoError(t, testAccPutRawPortProfile(site, id, map[string]any{
					"stp_port_mode": false, "stp_bpdu_guard_enabled": false, "port_keepalive_enabled": false,
					"eee_enabled": false, "flow_control_enabled": true, "precision_time_protocol_enabled": true,
					"link_debounce_auto": true, "stormctrl_bcast_enabled": false,
				}))
			}, Config: config, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
				plancheck.ExpectResourceAction(testAccPortProfileAddr, plancheck.ResourceActionUpdate),
			}}, Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(testAccPortProfileAddr, "stp_enabled", "true"),
				resource.TestCheckResourceAttr(testAccPortProfileAddr, "bpdu_guard_enabled", "true"),
				resource.TestCheckResourceAttr(testAccPortProfileAddr, "loop_protection_enabled", "true"),
				resource.TestCheckResourceAttr(testAccPortProfileAddr, "eee_enabled", "true"),
				resource.TestCheckResourceAttr(testAccPortProfileAddr, "flow_control_enabled", "false"),
				resource.TestCheckResourceAttr(testAccPortProfileAddr, "ptp_enabled", "false"),
				resource.TestCheckResourceAttr(testAccPortProfileAddr, "link_debounce_ms", "1200"),
				resource.TestCheckResourceAttr(testAccPortProfileAddr, "storm_control_broadcast", "37"),
			)},
		},
	})
}

// Flat attributes must accept values that are unknown until another resource
// is created, including an unknown unit controlling threshold validation.
func TestAccPortProfile_advancedSettingsUnknown(t *testing.T) {
	name := fmt.Sprintf("tfacc-pp-unknown-%s", randomSuffix())
	config := `
resource "terraform_data" "settings" {
 input = {
  unit = "packets_per_second"
  broadcast = 12345
  debounce = 1200
  guard = true
 }
}
` + testAccPortProfileConfig(name, `
 storm_control_type = terraform_data.settings.output.unit
 storm_control_broadcast = terraform_data.settings.output.broadcast
 link_debounce_ms = terraform_data.settings.output.debounce
 bpdu_guard_enabled = terraform_data.settings.output.guard
 `)
	resource.Test(t, resource.TestCase{
		PreCheck: func() { preCheckPortProfile(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, CheckDestroy: testAccCheckPortProfileDestroyed,
		Steps: []resource.TestStep{
			{Config: config, Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(testAccPortProfileAddr, "storm_control_type", "packets_per_second"),
				resource.TestCheckResourceAttr(testAccPortProfileAddr, "storm_control_broadcast", "12345"),
				resource.TestCheckResourceAttr(testAccPortProfileAddr, "link_debounce_ms", "1200"),
				resource.TestCheckResourceAttr(testAccPortProfileAddr, "bpdu_guard_enabled", "true"),
			)},
			{ResourceName: testAccPortProfileAddr, ImportState: true, ImportStateVerify: true},
		},
	})
}
