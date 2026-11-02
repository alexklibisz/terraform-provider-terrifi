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
		name    string
		tagged  types.Set
		invalid bool
	}{
		{name: "native explicitly tagged", tagged: partial(testNetA), invalid: true},
		{name: "native may become tagged", tagged: partial(testNetB)},
		{name: "every element unknown", tagged: types.SetValueMust(types.StringType, []attr.Value{types.StringUnknown()})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := defaultPortProfileModel("partial")
			m.TaggedVLANMgmt = types.StringValue("custom")
			m.NativeNetworkID = types.StringValue(testNetA)
			m.TaggedNetworkIDs = tc.tagged
			diags := validatePortProfileModel(&m)
			require.Equal(t, tc.invalid, diags.HasError(), "%v", diags)
		})
	}
}

// ValidateConfig decodes the raw config, so unknown values (references to
// resources not yet created) must reach the model without conversion errors.
func TestPortProfileValidateConfigUnknownValues(t *testing.T) {
	r := &portProfileResource{}
	var schemaResp resource.SchemaResponse
	r.Schema(t.Context(), resource.SchemaRequest{}, &schemaResp)
	for _, tc := range []struct {
		name    string
		mutate  func(m *portProfileResourceModel)
		invalid bool
	}{
		{name: "unknown native network", mutate: func(m *portProfileResourceModel) {
			m.TaggedVLANMgmt = types.StringValue("custom")
			m.NativeNetworkID = types.StringUnknown()
			m.TaggedNetworkIDs = stringSet(testNetB)
		}},
		{name: "unknown tagged set", mutate: func(m *portProfileResourceModel) {
			m.TaggedVLANMgmt = types.StringValue("custom")
			m.TaggedNetworkIDs = types.SetUnknown(types.StringType)
		}},
		{name: "unknown mode", mutate: func(m *portProfileResourceModel) {
			m.TaggedVLANMgmt = types.StringUnknown()
			m.TaggedNetworkIDs = stringSet(testNetB)
		}},
		{name: "known invalid combination", mutate: func(m *portProfileResourceModel) {
			m.TaggedNetworkIDs = stringSet(testNetB)
		}, invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			model := defaultPortProfileModel("unknown")
			tc.mutate(&model)
			state := tfsdk.State{Schema: schemaResp.Schema}
			require.False(t, state.Set(t.Context(), &model).HasError())
			var resp resource.ValidateConfigResponse
			r.ValidateConfig(t.Context(), resource.ValidateConfigRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: state.Raw}}, &resp)
			require.Equal(t, tc.invalid, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
		})
	}
}
