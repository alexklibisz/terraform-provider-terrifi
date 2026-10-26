package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"
)

func TestDevicePortsValidateConfigUnknownValues(t *testing.T) {
	r := &devicePortsResource{}
	var schemaResp resource.SchemaResponse
	r.Schema(t.Context(), resource.SchemaRequest{}, &schemaResp)
	objectType := types.ObjectType{AttrTypes: map[string]attr.Type{"name": types.StringType, "port_profile_id": types.StringType, "native_network_id": types.StringType, "tagged_vlan_mode": types.StringType}}
	knownPort := func(native, mode types.String) types.Object {
		return types.ObjectValueMust(objectType.AttrTypes, map[string]attr.Value{
			"name": types.StringNull(), "port_profile_id": types.StringNull(), "native_network_id": native, "tagged_vlan_mode": mode,
		})
	}
	for _, tc := range []struct {
		name    string
		ports   types.Map
		invalid bool
	}{
		{"whole map unknown", types.MapUnknown(objectType), false},
		{"one port unknown", types.MapValueMust(objectType, map[string]attr.Value{"2": types.ObjectUnknown(objectType.AttrTypes)}), false},
		{"null port", types.MapValueMust(objectType, map[string]attr.Value{"2": types.ObjectNull(objectType.AttrTypes)}), true},
		{"unknown native network", types.MapValueMust(objectType, map[string]attr.Value{"2": knownPort(types.StringUnknown(), types.StringValue("block_all"))}), false},
		{"invalid known port beside unknown port", types.MapValueMust(objectType, map[string]attr.Value{
			"2": knownPort(types.StringValue("lan"), types.StringNull()), "3": types.ObjectUnknown(objectType.AttrTypes),
		}), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := tfsdk.State{Schema: schemaResp.Schema}
			require.False(t, state.Set(t.Context(), &devicePortsResourceModel{DeviceMAC: types.StringValue("aa:bb:cc:dd:ee:ff")}).HasError())
			require.False(t, state.SetAttribute(t.Context(), path.Root("ports"), tc.ports).HasError())
			var resp resource.ValidateConfigResponse
			r.ValidateConfig(t.Context(), resource.ValidateConfigRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: state.Raw}}, &resp)
			require.Equal(t, tc.invalid, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
		})
	}
}

func TestPortProfileValidatePartialUnknownSets(t *testing.T) {
	partial := func(id string) types.Set {
		return types.SetValueMust(types.StringType, []attr.Value{types.StringValue(id), types.StringUnknown()})
	}
	for _, tc := range []struct {
		name             string
		tagged, excluded types.Set
		native, voice    string
		invalid          bool
	}{
		{name: "voice explicitly present", tagged: partial(testNetB), voice: testNetB},
		{name: "voice may become present", tagged: partial(testNetC), voice: testNetB},
		{name: "voice definitely absent", tagged: stringSet(testNetC), voice: testNetB, invalid: true},
		{name: "native explicitly tagged", tagged: partial(testNetA), native: testNetA, invalid: true},
		{name: "native may become tagged", tagged: partial(testNetB), native: testNetA},
		{name: "voice explicitly excluded", excluded: partial(testNetB), voice: testNetB, invalid: true},
		{name: "voice may become excluded", excluded: partial(testNetC), voice: testNetB},
		{name: "native explicitly excluded", excluded: partial(testNetA), native: testNetA, invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := defaultPortProfileModel("partial")
			m.TaggedVLANMgmt = types.StringValue("custom")
			if !tc.tagged.IsNull() {
				m.TaggedNetworkIDs = tc.tagged
			}
			if !tc.excluded.IsNull() {
				m.ExcludedNetworkIDs = tc.excluded
			}
			if tc.native != "" {
				m.NativeNetworkID = types.StringValue(tc.native)
			}
			if tc.voice != "" {
				m.VoiceNetworkID = types.StringValue(tc.voice)
			}
			diags := validatePortProfileModel(t.Context(), &m)
			require.Equal(t, tc.invalid, diags.HasError(), "%v", diags)
		})
	}
}
