package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/ubiquiti-community/go-unifi/unifi"
)

// ---------------------------------------------------------------------------
// Unit tests
// ---------------------------------------------------------------------------

func TestFirewallPolicyModelToAPI(t *testing.T) {
	r := &firewallPolicyResource{}
	ctx := context.Background()

	t.Run("minimal block rule", func(t *testing.T) {
		srcObj := types.ObjectValueMust(endpointAttrTypes, map[string]attr.Value{
			"zone_id":            types.StringValue("zone-src"),
			"ips":                types.SetNull(types.StringType),
			"mac_addresses":      types.SetNull(types.StringType),
			"network_ids":        types.SetNull(types.StringType),
			"device_ids":         types.SetNull(types.StringType),
			"port_matching_type": types.StringValue("ANY"),
			"port":               types.Int64Null(),
			"port_group_id":      types.StringNull(),
		})
		dstObj := types.ObjectValueMust(endpointAttrTypes, map[string]attr.Value{
			"zone_id":            types.StringValue("zone-dst"),
			"ips":                types.SetNull(types.StringType),
			"mac_addresses":      types.SetNull(types.StringType),
			"network_ids":        types.SetNull(types.StringType),
			"device_ids":         types.SetNull(types.StringType),
			"port_matching_type": types.StringValue("ANY"),
			"port":               types.Int64Null(),
			"port_group_id":      types.StringNull(),
		})

		model := &firewallPolicyResourceModel{
			Name:                types.StringValue("Block IoT"),
			Action:              types.StringValue("BLOCK"),
			Enabled:             types.BoolValue(true),
			IPVersion:           types.StringValue("BOTH"),
			Protocol:            types.StringValue("all"),
			ConnectionStateType: types.StringValue("ALL"),
			ConnectionStates:    types.SetNull(types.StringType),
			Description:         types.StringNull(),
			MatchIPSec:          types.BoolNull(),
			Logging:             types.BoolNull(),
			CreateAllowRespond:  types.BoolNull(),
			Index:               types.Int64Null(),
			Source:              srcObj,
			Destination:         dstObj,
			Schedule:            types.ObjectNull(scheduleAttrTypes),
		}

		policy := r.modelToAPI(ctx, model)

		assert.Equal(t, "Block IoT", policy.Name)
		assert.Equal(t, "BLOCK", policy.Action)
		assert.True(t, policy.Enabled)
		assert.Equal(t, "BOTH", policy.IPVersion)
		assert.Equal(t, "all", policy.Protocol)
		assert.Equal(t, "ALL", policy.ConnectionStateType)
		assert.False(t, policy.Logging)
		assert.False(t, policy.MatchIPSec)
		assert.Nil(t, policy.Index)
		assert.Nil(t, policy.Schedule)
		assert.NotNil(t, policy.Source)
		assert.Equal(t, "zone-src", policy.Source.ZoneID)
		assert.NotNil(t, policy.Destination)
		assert.Equal(t, "zone-dst", policy.Destination.ZoneID)
	})

	t.Run("with source IPs and port", func(t *testing.T) {
		srcObj := types.ObjectValueMust(endpointAttrTypes, map[string]attr.Value{
			"zone_id": types.StringValue("zone-src"),
			"ips": types.SetValueMust(types.StringType, []attr.Value{
				types.StringValue("10.0.0.1"),
				types.StringValue("10.0.0.2"),
			}),
			"mac_addresses":      types.SetNull(types.StringType),
			"network_ids":        types.SetNull(types.StringType),
			"device_ids":         types.SetNull(types.StringType),
			"port_matching_type": types.StringValue("SPECIFIC"),
			"port":               types.Int64Value(443),
			"port_group_id":      types.StringNull(),
		})
		dstObj := types.ObjectValueMust(endpointAttrTypes, map[string]attr.Value{
			"zone_id":            types.StringValue("zone-dst"),
			"ips":                types.SetNull(types.StringType),
			"mac_addresses":      types.SetNull(types.StringType),
			"network_ids":        types.SetNull(types.StringType),
			"device_ids":         types.SetNull(types.StringType),
			"port_matching_type": types.StringValue("ANY"),
			"port":               types.Int64Null(),
			"port_group_id":      types.StringNull(),
		})

		model := &firewallPolicyResourceModel{
			Name:                types.StringValue("Allow HTTPS"),
			Action:              types.StringValue("ALLOW"),
			Enabled:             types.BoolValue(true),
			IPVersion:           types.StringValue("IPV4"),
			Protocol:            types.StringValue("tcp"),
			ConnectionStateType: types.StringValue("ALL"),
			ConnectionStates:    types.SetNull(types.StringType),
			Description:         types.StringValue("Allow HTTPS from specific IPs"),
			MatchIPSec:          types.BoolNull(),
			Logging:             types.BoolValue(true),
			CreateAllowRespond:  types.BoolNull(),
			Index:               types.Int64Null(),
			Source:              srcObj,
			Destination:         dstObj,
			Schedule:            types.ObjectNull(scheduleAttrTypes),
		}

		policy := r.modelToAPI(ctx, model)

		assert.Equal(t, "Allow HTTPS", policy.Name)
		assert.Equal(t, "ALLOW", policy.Action)
		assert.Equal(t, "IPV4", policy.IPVersion)
		assert.Equal(t, "tcp", policy.Protocol)
		assert.Equal(t, "Allow HTTPS from specific IPs", policy.Description)
		assert.True(t, policy.Logging)
		assert.Equal(t, "IP", policy.Source.MatchingTarget)
		assert.ElementsMatch(t, []string{"10.0.0.1", "10.0.0.2"}, policy.Source.IPs)
		assert.Equal(t, "SPECIFIC", policy.Source.PortMatchingType)
		assert.Equal(t, int64(443), *policy.Source.Port)
	})

	t.Run("with schedule", func(t *testing.T) {
		srcObj := types.ObjectValueMust(endpointAttrTypes, map[string]attr.Value{
			"zone_id":            types.StringValue("zone-src"),
			"ips":                types.SetNull(types.StringType),
			"mac_addresses":      types.SetNull(types.StringType),
			"network_ids":        types.SetNull(types.StringType),
			"device_ids":         types.SetNull(types.StringType),
			"port_matching_type": types.StringValue("ANY"),
			"port":               types.Int64Null(),
			"port_group_id":      types.StringNull(),
		})
		dstObj := types.ObjectValueMust(endpointAttrTypes, map[string]attr.Value{
			"zone_id":            types.StringValue("zone-dst"),
			"ips":                types.SetNull(types.StringType),
			"mac_addresses":      types.SetNull(types.StringType),
			"network_ids":        types.SetNull(types.StringType),
			"device_ids":         types.SetNull(types.StringType),
			"port_matching_type": types.StringValue("ANY"),
			"port":               types.Int64Null(),
			"port_group_id":      types.StringNull(),
		})
		schedObj := types.ObjectValueMust(scheduleAttrTypes, map[string]attr.Value{
			"mode":             types.StringValue("EVERY_WEEK"),
			"date":             types.StringNull(),
			"time_all_day":     types.BoolNull(),
			"time_range_start": types.StringValue("08:00"),
			"time_range_end":   types.StringValue("17:00"),
			"repeat_on_days": types.SetValueMust(types.StringType, []attr.Value{
				types.StringValue("mon"),
				types.StringValue("tue"),
				types.StringValue("wed"),
			}),
		})

		model := &firewallPolicyResourceModel{
			Name:                types.StringValue("Weekday Block"),
			Action:              types.StringValue("BLOCK"),
			Enabled:             types.BoolValue(true),
			IPVersion:           types.StringValue("BOTH"),
			Protocol:            types.StringValue("all"),
			ConnectionStateType: types.StringValue("ALL"),
			ConnectionStates:    types.SetNull(types.StringType),
			Description:         types.StringNull(),
			MatchIPSec:          types.BoolNull(),
			Logging:             types.BoolNull(),
			CreateAllowRespond:  types.BoolNull(),
			Index:               types.Int64Null(),
			Source:              srcObj,
			Destination:         dstObj,
			Schedule:            schedObj,
		}

		policy := r.modelToAPI(ctx, model)

		assert.NotNil(t, policy.Schedule)
		assert.Equal(t, "EVERY_WEEK", policy.Schedule.Mode)
		assert.Equal(t, "08:00", policy.Schedule.TimeRangeStart)
		assert.Equal(t, "17:00", policy.Schedule.TimeRangeEnd)
		assert.ElementsMatch(t, []string{"mon", "tue", "wed"}, policy.Schedule.RepeatOnDays)
	})

	t.Run("disabled rule", func(t *testing.T) {
		srcObj := types.ObjectValueMust(endpointAttrTypes, map[string]attr.Value{
			"zone_id":            types.StringValue("zone-src"),
			"ips":                types.SetNull(types.StringType),
			"mac_addresses":      types.SetNull(types.StringType),
			"network_ids":        types.SetNull(types.StringType),
			"device_ids":         types.SetNull(types.StringType),
			"port_matching_type": types.StringValue("ANY"),
			"port":               types.Int64Null(),
			"port_group_id":      types.StringNull(),
		})
		dstObj := types.ObjectValueMust(endpointAttrTypes, map[string]attr.Value{
			"zone_id":            types.StringValue("zone-dst"),
			"ips":                types.SetNull(types.StringType),
			"mac_addresses":      types.SetNull(types.StringType),
			"network_ids":        types.SetNull(types.StringType),
			"device_ids":         types.SetNull(types.StringType),
			"port_matching_type": types.StringValue("ANY"),
			"port":               types.Int64Null(),
			"port_group_id":      types.StringNull(),
		})

		model := &firewallPolicyResourceModel{
			Name:                types.StringValue("Disabled Rule"),
			Action:              types.StringValue("REJECT"),
			Enabled:             types.BoolValue(false),
			IPVersion:           types.StringValue("BOTH"),
			Protocol:            types.StringValue("all"),
			ConnectionStateType: types.StringValue("ALL"),
			ConnectionStates:    types.SetNull(types.StringType),
			Description:         types.StringNull(),
			MatchIPSec:          types.BoolNull(),
			Logging:             types.BoolNull(),
			CreateAllowRespond:  types.BoolNull(),
			Index:               types.Int64Null(),
			Source:              srcObj,
			Destination:         dstObj,
			Schedule:            types.ObjectNull(scheduleAttrTypes),
		}

		policy := r.modelToAPI(ctx, model)

		assert.Equal(t, "REJECT", policy.Action)
		assert.False(t, policy.Enabled)
	})

	t.Run("with connection states", func(t *testing.T) {
		srcObj := types.ObjectValueMust(endpointAttrTypes, map[string]attr.Value{
			"zone_id":            types.StringValue("zone-src"),
			"ips":                types.SetNull(types.StringType),
			"mac_addresses":      types.SetNull(types.StringType),
			"network_ids":        types.SetNull(types.StringType),
			"device_ids":         types.SetNull(types.StringType),
			"port_matching_type": types.StringValue("ANY"),
			"port":               types.Int64Null(),
			"port_group_id":      types.StringNull(),
		})
		dstObj := types.ObjectValueMust(endpointAttrTypes, map[string]attr.Value{
			"zone_id":            types.StringValue("zone-dst"),
			"ips":                types.SetNull(types.StringType),
			"mac_addresses":      types.SetNull(types.StringType),
			"network_ids":        types.SetNull(types.StringType),
			"device_ids":         types.SetNull(types.StringType),
			"port_matching_type": types.StringValue("ANY"),
			"port":               types.Int64Null(),
			"port_group_id":      types.StringNull(),
		})

		model := &firewallPolicyResourceModel{
			Name:                types.StringValue("Stateful Rule"),
			Action:              types.StringValue("ALLOW"),
			Enabled:             types.BoolValue(true),
			IPVersion:           types.StringValue("BOTH"),
			Protocol:            types.StringValue("tcp"),
			ConnectionStateType: types.StringValue("RESPOND_ONLY"),
			ConnectionStates: types.SetValueMust(types.StringType, []attr.Value{
				types.StringValue("NEW"),
				types.StringValue("ESTABLISHED"),
			}),
			Description:        types.StringNull(),
			MatchIPSec:         types.BoolNull(),
			Logging:            types.BoolNull(),
			CreateAllowRespond: types.BoolNull(),
			Index:              types.Int64Null(),
			Source:             srcObj,
			Destination:        dstObj,
			Schedule:           types.ObjectNull(scheduleAttrTypes),
		}

		policy := r.modelToAPI(ctx, model)

		assert.Equal(t, "RESPOND_ONLY", policy.ConnectionStateType)
		assert.ElementsMatch(t, []string{"NEW", "ESTABLISHED"}, policy.ConnectionStates)
	})

	t.Run("with MAC addresses", func(t *testing.T) {
		srcObj := types.ObjectValueMust(endpointAttrTypes, map[string]attr.Value{
			"zone_id": types.StringValue("zone-src"),
			"ips":     types.SetNull(types.StringType),
			"mac_addresses": types.SetValueMust(types.StringType, []attr.Value{
				types.StringValue("aa:bb:cc:dd:ee:ff"),
			}),
			"network_ids":        types.SetNull(types.StringType),
			"device_ids":         types.SetNull(types.StringType),
			"port_matching_type": types.StringValue("ANY"),
			"port":               types.Int64Null(),
			"port_group_id":      types.StringNull(),
		})
		dstObj := types.ObjectValueMust(endpointAttrTypes, map[string]attr.Value{
			"zone_id":            types.StringValue("zone-dst"),
			"ips":                types.SetNull(types.StringType),
			"mac_addresses":      types.SetNull(types.StringType),
			"network_ids":        types.SetNull(types.StringType),
			"device_ids":         types.SetNull(types.StringType),
			"port_matching_type": types.StringValue("ANY"),
			"port":               types.Int64Null(),
			"port_group_id":      types.StringNull(),
		})

		model := &firewallPolicyResourceModel{
			Name:                types.StringValue("MAC Rule"),
			Action:              types.StringValue("BLOCK"),
			Enabled:             types.BoolValue(true),
			IPVersion:           types.StringValue("BOTH"),
			Protocol:            types.StringValue("all"),
			ConnectionStateType: types.StringValue("ALL"),
			ConnectionStates:    types.SetNull(types.StringType),
			Description:         types.StringNull(),
			MatchIPSec:          types.BoolNull(),
			Logging:             types.BoolNull(),
			CreateAllowRespond:  types.BoolNull(),
			Index:               types.Int64Null(),
			Source:              srcObj,
			Destination:         dstObj,
			Schedule:            types.ObjectNull(scheduleAttrTypes),
		}

		policy := r.modelToAPI(ctx, model)

		assert.Equal(t, "MAC", policy.Source.MatchingTarget)
		assert.ElementsMatch(t, []string{"aa:bb:cc:dd:ee:ff"}, policy.Source.IPs)
		assert.Equal(t, "ANY", policy.Destination.MatchingTarget)
		assert.Nil(t, policy.Destination.IPs)
	})

	t.Run("with device IDs", func(t *testing.T) {
		srcObj := types.ObjectValueMust(endpointAttrTypes, map[string]attr.Value{
			"zone_id":       types.StringValue("zone-src"),
			"ips":           types.SetNull(types.StringType),
			"mac_addresses": types.SetNull(types.StringType),
			"network_ids":   types.SetNull(types.StringType),
			"device_ids": types.SetValueMust(types.StringType, []attr.Value{
				types.StringValue("aa:bb:cc:dd:ee:f1"),
				types.StringValue("aa:bb:cc:dd:ee:f2"),
			}),
			"port_matching_type": types.StringValue("ANY"),
			"port":               types.Int64Null(),
			"port_group_id":      types.StringNull(),
		})
		dstObj := types.ObjectValueMust(endpointAttrTypes, map[string]attr.Value{
			"zone_id":            types.StringValue("zone-dst"),
			"ips":                types.SetNull(types.StringType),
			"mac_addresses":      types.SetNull(types.StringType),
			"network_ids":        types.SetNull(types.StringType),
			"device_ids":         types.SetNull(types.StringType),
			"port_matching_type": types.StringValue("ANY"),
			"port":               types.Int64Null(),
			"port_group_id":      types.StringNull(),
		})

		model := &firewallPolicyResourceModel{
			Name:                types.StringValue("Device Rule"),
			Action:              types.StringValue("ALLOW"),
			Enabled:             types.BoolValue(true),
			IPVersion:           types.StringValue("BOTH"),
			Protocol:            types.StringValue("all"),
			ConnectionStateType: types.StringValue("ALL"),
			ConnectionStates:    types.SetNull(types.StringType),
			Description:         types.StringNull(),
			MatchIPSec:          types.BoolNull(),
			Logging:             types.BoolNull(),
			CreateAllowRespond:  types.BoolNull(),
			Index:               types.Int64Null(),
			Source:              srcObj,
			Destination:         dstObj,
			Schedule:            types.ObjectNull(scheduleAttrTypes),
		}

		policy := r.modelToAPI(ctx, model)

		assert.Equal(t, "CLIENT", policy.Source.MatchingTarget)
		assert.ElementsMatch(t, []string{"aa:bb:cc:dd:ee:f1", "aa:bb:cc:dd:ee:f2"}, policy.Source.IPs)
		assert.Equal(t, "ANY", policy.Destination.MatchingTarget)
		assert.Nil(t, policy.Destination.IPs)
	})

	t.Run("with network IDs", func(t *testing.T) {
		srcObj := types.ObjectValueMust(endpointAttrTypes, map[string]attr.Value{
			"zone_id":       types.StringValue("zone-src"),
			"ips":           types.SetNull(types.StringType),
			"mac_addresses": types.SetNull(types.StringType),
			"network_ids": types.SetValueMust(types.StringType, []attr.Value{
				types.StringValue("net-001"),
				types.StringValue("net-002"),
			}),
			"device_ids":         types.SetNull(types.StringType),
			"port_matching_type": types.StringValue("ANY"),
			"port":               types.Int64Null(),
			"port_group_id":      types.StringNull(),
		})
		dstObj := types.ObjectValueMust(endpointAttrTypes, map[string]attr.Value{
			"zone_id":            types.StringValue("zone-dst"),
			"ips":                types.SetNull(types.StringType),
			"mac_addresses":      types.SetNull(types.StringType),
			"network_ids":        types.SetNull(types.StringType),
			"device_ids":         types.SetNull(types.StringType),
			"port_matching_type": types.StringValue("ANY"),
			"port":               types.Int64Null(),
			"port_group_id":      types.StringNull(),
		})

		model := &firewallPolicyResourceModel{
			Name:                types.StringValue("Network Rule"),
			Action:              types.StringValue("ALLOW"),
			Enabled:             types.BoolValue(true),
			IPVersion:           types.StringValue("BOTH"),
			Protocol:            types.StringValue("all"),
			ConnectionStateType: types.StringValue("ALL"),
			ConnectionStates:    types.SetNull(types.StringType),
			Description:         types.StringNull(),
			MatchIPSec:          types.BoolNull(),
			Logging:             types.BoolNull(),
			CreateAllowRespond:  types.BoolNull(),
			Index:               types.Int64Null(),
			Source:              srcObj,
			Destination:         dstObj,
			Schedule:            types.ObjectNull(scheduleAttrTypes),
		}

		policy := r.modelToAPI(ctx, model)

		assert.Equal(t, "NETWORK", policy.Source.MatchingTarget)
		assert.ElementsMatch(t, []string{"net-001", "net-002"}, policy.Source.IPs)
	})
}

func TestFirewallPolicyAPIToModel(t *testing.T) {
	r := &firewallPolicyResource{}

	t.Run("minimal policy", func(t *testing.T) {
		policy := &unifi.FirewallPolicy{
			ID:      "pol-001",
			Name:    "Block IoT",
			Action:  "BLOCK",
			Enabled: true,
			Source: &unifi.FirewallPolicySource{
				ZoneID:         "zone-src",
				MatchingTarget: "ANY",
			},
			Destination: &unifi.FirewallPolicyDestination{
				ZoneID:         "zone-dst",
				MatchingTarget: "ANY",
			},
		}

		var model firewallPolicyResourceModel
		r.apiToModel(policy, &model, "default")

		assert.Equal(t, "pol-001", model.ID.ValueString())
		assert.Equal(t, "default", model.Site.ValueString())
		assert.Equal(t, "Block IoT", model.Name.ValueString())
		assert.Equal(t, "BLOCK", model.Action.ValueString())
		assert.True(t, model.Enabled.ValueBool())
		assert.True(t, model.Description.IsNull())
		assert.True(t, model.Schedule.IsNull())
	})

	t.Run("full policy", func(t *testing.T) {
		idx := int64(5)
		port := int64(443)
		policy := &unifi.FirewallPolicy{
			ID:                  "pol-002",
			Name:                "Allow HTTPS",
			Description:         "HTTPS traffic",
			Action:              "ALLOW",
			Enabled:             true,
			IPVersion:           "IPV4",
			Protocol:            "tcp",
			ConnectionStateType: "RESPOND_ONLY",
			ConnectionStates:    []string{"NEW", "ESTABLISHED"},
			Logging:             true,
			Index:               &idx,
			Source: &unifi.FirewallPolicySource{
				ZoneID:           "zone-src",
				MatchingTarget:   "IP",
				IPs:              []string{"10.0.0.1"},
				PortMatchingType: "SPECIFIC",
				Port:             &port,
			},
			Destination: &unifi.FirewallPolicyDestination{
				ZoneID:         "zone-dst",
				MatchingTarget: "ANY",
			},
		}

		var model firewallPolicyResourceModel
		r.apiToModel(policy, &model, "mysite")

		assert.Equal(t, "pol-002", model.ID.ValueString())
		assert.Equal(t, "mysite", model.Site.ValueString())
		assert.Equal(t, "Allow HTTPS", model.Name.ValueString())
		assert.Equal(t, "HTTPS traffic", model.Description.ValueString())
		assert.Equal(t, "ALLOW", model.Action.ValueString())
		assert.Equal(t, "IPV4", model.IPVersion.ValueString())
		assert.Equal(t, "tcp", model.Protocol.ValueString())
		assert.Equal(t, "RESPOND_ONLY", model.ConnectionStateType.ValueString())
		assert.True(t, model.Logging.ValueBool())
		assert.Equal(t, int64(5), model.Index.ValueInt64())
		assert.False(t, model.Source.IsNull())
		assert.False(t, model.Destination.IsNull())

		// Verify the source IPs are in the "ips" typed field.
		var srcModel firewallPolicyEndpointModel
		model.Source.As(context.Background(), &srcModel, basetypes.ObjectAsOptions{})
		assert.False(t, srcModel.IPs.IsNull())
		assert.True(t, srcModel.MACAddresses.IsNull())
		assert.True(t, srcModel.NetworkIDs.IsNull())
		assert.True(t, srcModel.DeviceIDs.IsNull())
	})

	t.Run("zero-value booleans are null", func(t *testing.T) {
		policy := &unifi.FirewallPolicy{
			ID:      "pol-003",
			Name:    "Test",
			Action:  "BLOCK",
			Enabled: false,
			Source: &unifi.FirewallPolicySource{
				ZoneID: "zone-src",
			},
			Destination: &unifi.FirewallPolicyDestination{
				ZoneID: "zone-dst",
			},
		}

		var model firewallPolicyResourceModel
		r.apiToModel(policy, &model, "default")

		assert.False(t, model.Enabled.ValueBool())
		assert.True(t, model.Logging.IsNull())
		assert.True(t, model.MatchIPSec.IsNull())
		assert.True(t, model.CreateAllowRespond.IsNull())
	})

	t.Run("nil index", func(t *testing.T) {
		policy := &unifi.FirewallPolicy{
			ID:     "pol-004",
			Name:   "No Index",
			Action: "BLOCK",
			Source: &unifi.FirewallPolicySource{
				ZoneID: "zone-src",
			},
			Destination: &unifi.FirewallPolicyDestination{
				ZoneID: "zone-dst",
			},
		}

		var model firewallPolicyResourceModel
		r.apiToModel(policy, &model, "default")

		assert.True(t, model.Index.IsNull())
	})

	t.Run("empty description is null", func(t *testing.T) {
		policy := &unifi.FirewallPolicy{
			ID:          "pol-005",
			Name:        "No Desc",
			Description: "",
			Action:      "BLOCK",
			Source: &unifi.FirewallPolicySource{
				ZoneID: "zone-src",
			},
			Destination: &unifi.FirewallPolicyDestination{
				ZoneID: "zone-dst",
			},
		}

		var model firewallPolicyResourceModel
		r.apiToModel(policy, &model, "default")

		assert.True(t, model.Description.IsNull())
	})

	t.Run("defaults for empty enum fields", func(t *testing.T) {
		policy := &unifi.FirewallPolicy{
			ID:     "pol-006",
			Name:   "Defaults",
			Action: "BLOCK",
			Source: &unifi.FirewallPolicySource{
				ZoneID: "zone-src",
			},
			Destination: &unifi.FirewallPolicyDestination{
				ZoneID: "zone-dst",
			},
		}

		var model firewallPolicyResourceModel
		r.apiToModel(policy, &model, "default")

		assert.Equal(t, "BOTH", model.IPVersion.ValueString())
		assert.Equal(t, "all", model.Protocol.ValueString())
		assert.Equal(t, "ALL", model.ConnectionStateType.ValueString())
	})

	t.Run("with schedule", func(t *testing.T) {
		policy := &unifi.FirewallPolicy{
			ID:     "pol-007",
			Name:   "Scheduled",
			Action: "BLOCK",
			Source: &unifi.FirewallPolicySource{
				ZoneID: "zone-src",
			},
			Destination: &unifi.FirewallPolicyDestination{
				ZoneID: "zone-dst",
			},
			Schedule: &unifi.FirewallPolicySchedule{
				Mode:           "EVERY_WEEK",
				TimeRangeStart: "08:00",
				TimeRangeEnd:   "17:00",
				RepeatOnDays:   []string{"mon", "fri"},
			},
		}

		var model firewallPolicyResourceModel
		r.apiToModel(policy, &model, "default")

		assert.False(t, model.Schedule.IsNull())
	})

	t.Run("MAC matching target populates mac_addresses", func(t *testing.T) {
		policy := &unifi.FirewallPolicy{
			ID:     "pol-008",
			Name:   "MAC Rule",
			Action: "BLOCK",
			Source: &unifi.FirewallPolicySource{
				ZoneID:         "zone-src",
				MatchingTarget: "MAC",
				IPs:            []string{"aa:bb:cc:dd:ee:ff"},
			},
			Destination: &unifi.FirewallPolicyDestination{
				ZoneID:         "zone-dst",
				MatchingTarget: "ANY",
			},
		}

		var model firewallPolicyResourceModel
		r.apiToModel(policy, &model, "default")

		var srcModel firewallPolicyEndpointModel
		model.Source.As(context.Background(), &srcModel, basetypes.ObjectAsOptions{})
		assert.True(t, srcModel.IPs.IsNull())
		assert.False(t, srcModel.MACAddresses.IsNull())
		assert.True(t, srcModel.NetworkIDs.IsNull())
		assert.True(t, srcModel.DeviceIDs.IsNull())

		var macs []string
		srcModel.MACAddresses.ElementsAs(context.Background(), &macs, false)
		assert.ElementsMatch(t, []string{"aa:bb:cc:dd:ee:ff"}, macs)
	})

	t.Run("CLIENT matching target populates device_ids", func(t *testing.T) {
		policy := &unifi.FirewallPolicy{
			ID:     "pol-010",
			Name:   "Device Rule",
			Action: "ALLOW",
			Source: &unifi.FirewallPolicySource{
				ZoneID:         "zone-src",
				MatchingTarget: "CLIENT",
				IPs:            []string{"aa:bb:cc:dd:ee:f1", "aa:bb:cc:dd:ee:f2"},
			},
			Destination: &unifi.FirewallPolicyDestination{
				ZoneID:         "zone-dst",
				MatchingTarget: "ANY",
			},
		}

		var model firewallPolicyResourceModel
		r.apiToModel(policy, &model, "default")

		var srcModel firewallPolicyEndpointModel
		model.Source.As(context.Background(), &srcModel, basetypes.ObjectAsOptions{})
		assert.True(t, srcModel.IPs.IsNull())
		assert.True(t, srcModel.MACAddresses.IsNull())
		assert.True(t, srcModel.NetworkIDs.IsNull())
		assert.False(t, srcModel.DeviceIDs.IsNull())

		var devices []string
		srcModel.DeviceIDs.ElementsAs(context.Background(), &devices, false)
		assert.ElementsMatch(t, []string{"aa:bb:cc:dd:ee:f1", "aa:bb:cc:dd:ee:f2"}, devices)
	})

	t.Run("NETWORK matching target populates network_ids", func(t *testing.T) {
		policy := &unifi.FirewallPolicy{
			ID:     "pol-009",
			Name:   "Network Rule",
			Action: "ALLOW",
			Source: &unifi.FirewallPolicySource{
				ZoneID:         "zone-src",
				MatchingTarget: "NETWORK",
				IPs:            []string{"net-001"},
			},
			Destination: &unifi.FirewallPolicyDestination{
				ZoneID:         "zone-dst",
				MatchingTarget: "ANY",
			},
		}

		var model firewallPolicyResourceModel
		r.apiToModel(policy, &model, "default")

		var srcModel firewallPolicyEndpointModel
		model.Source.As(context.Background(), &srcModel, basetypes.ObjectAsOptions{})
		assert.True(t, srcModel.IPs.IsNull())
		assert.True(t, srcModel.MACAddresses.IsNull())
		assert.False(t, srcModel.NetworkIDs.IsNull())
		assert.True(t, srcModel.DeviceIDs.IsNull())

		var networks []string
		srcModel.NetworkIDs.ElementsAs(context.Background(), &networks, false)
		assert.ElementsMatch(t, []string{"net-001"}, networks)
	})
}

func TestFirewallPolicyApplyPlanToState(t *testing.T) {
	r := &firewallPolicyResource{}

	t.Run("updates all non-null fields", func(t *testing.T) {
		state := &firewallPolicyResourceModel{
			Name:   types.StringValue("Old Name"),
			Action: types.StringValue("BLOCK"),
		}

		plan := &firewallPolicyResourceModel{
			Name:                types.StringValue("New Name"),
			Action:              types.StringValue("ALLOW"),
			Description:         types.StringValue("Updated desc"),
			Enabled:             types.BoolValue(false),
			IPVersion:           types.StringValue("IPV4"),
			Protocol:            types.StringValue("tcp"),
			ConnectionStateType: types.StringValue("RESPOND_ONLY"),
			ConnectionStates:    types.SetNull(types.StringType),
			MatchIPSec:          types.BoolNull(),
			Logging:             types.BoolNull(),
			CreateAllowRespond:  types.BoolNull(),
			Index:               types.Int64Null(),
			Source:              types.ObjectNull(endpointAttrTypes),
			Destination:         types.ObjectNull(endpointAttrTypes),
			Schedule:            types.ObjectNull(scheduleAttrTypes),
		}

		r.applyPlanToState(plan, state)

		assert.Equal(t, "New Name", state.Name.ValueString())
		assert.Equal(t, "ALLOW", state.Action.ValueString())
		assert.Equal(t, "Updated desc", state.Description.ValueString())
		assert.False(t, state.Enabled.ValueBool())
		assert.Equal(t, "IPV4", state.IPVersion.ValueString())
		assert.Equal(t, "tcp", state.Protocol.ValueString())
	})
}

func TestBuildEndpointRequest(t *testing.T) {
	t.Run("MAC matching sends values in macs field", func(t *testing.T) {
		ep := buildEndpointRequest("zone1", "MAC", []string{"aa:bb:cc:dd:ee:ff"}, "ANY", nil, "")
		assert.Equal(t, "MAC", ep.MatchingTarget)
		assert.Equal(t, []string{"aa:bb:cc:dd:ee:ff"}, ep.MACs)
		assert.Nil(t, ep.IPs)
	})

	t.Run("CLIENT matching sends values in macs field", func(t *testing.T) {
		ep := buildEndpointRequest("zone1", "CLIENT", []string{"aa:bb:cc:dd:ee:f1", "aa:bb:cc:dd:ee:f2"}, "ANY", nil, "")
		assert.Equal(t, "CLIENT", ep.MatchingTarget)
		assert.Equal(t, []string{"aa:bb:cc:dd:ee:f1", "aa:bb:cc:dd:ee:f2"}, ep.MACs)
		assert.Nil(t, ep.IPs)
	})

	t.Run("IP matching sends values in ips field", func(t *testing.T) {
		ep := buildEndpointRequest("zone1", "IP", []string{"10.0.0.1"}, "ANY", nil, "")
		assert.Equal(t, "IP", ep.MatchingTarget)
		assert.Equal(t, []string{"10.0.0.1"}, ep.IPs)
		assert.Nil(t, ep.MACs)
	})

	t.Run("NETWORK matching sends values in ips field", func(t *testing.T) {
		ep := buildEndpointRequest("zone1", "NETWORK", []string{"net-001"}, "ANY", nil, "")
		assert.Equal(t, "NETWORK", ep.MatchingTarget)
		assert.Equal(t, []string{"net-001"}, ep.IPs)
		assert.Nil(t, ep.MACs)
	})
}

func TestResolveIPs(t *testing.T) {
	t.Run("MAC matching returns macs", func(t *testing.T) {
		ep := &firewallPolicyEndpointResponse{
			MatchingTarget: "MAC",
			MACs:           []string{"aa:bb:cc:dd:ee:ff"},
		}
		assert.Equal(t, []string{"aa:bb:cc:dd:ee:ff"}, ep.resolveIPs())
	})

	t.Run("CLIENT matching returns macs", func(t *testing.T) {
		ep := &firewallPolicyEndpointResponse{
			MatchingTarget: "CLIENT",
			MACs:           []string{"aa:bb:cc:dd:ee:f1", "aa:bb:cc:dd:ee:f2"},
		}
		assert.Equal(t, []string{"aa:bb:cc:dd:ee:f1", "aa:bb:cc:dd:ee:f2"}, ep.resolveIPs())
	})

	t.Run("IP matching returns ips", func(t *testing.T) {
		ep := &firewallPolicyEndpointResponse{
			MatchingTarget: "IP",
			IPs:            []string{"10.0.0.1"},
		}
		assert.Equal(t, []string{"10.0.0.1"}, ep.resolveIPs())
	})

	t.Run("NETWORK matching returns ips", func(t *testing.T) {
		ep := &firewallPolicyEndpointResponse{
			MatchingTarget: "NETWORK",
			IPs:            []string{"net-001"},
		}
		assert.Equal(t, []string{"net-001"}, ep.resolveIPs())
	})

	t.Run("ANY matching returns ips", func(t *testing.T) {
		ep := &firewallPolicyEndpointResponse{
			MatchingTarget: "ANY",
		}
		assert.Nil(t, ep.resolveIPs())
	})

	t.Run("CLIENT matching falls back to ips when macs empty", func(t *testing.T) {
		ep := &firewallPolicyEndpointResponse{
			MatchingTarget: "CLIENT",
			IPs:            []string{"aa:bb:cc:dd:ee:f1"},
		}
		assert.Equal(t, []string{"aa:bb:cc:dd:ee:f1"}, ep.resolveIPs())
	})
}

// ---------------------------------------------------------------------------
// Acceptance tests
// ---------------------------------------------------------------------------

func TestAccFirewallPolicy_basic(t *testing.T) {
	zone1Name := fmt.Sprintf("tfacc-pol-z1-%s", randomSuffix())
	zone2Name := fmt.Sprintf("tfacc-pol-z2-%s", randomSuffix())
	policyName := fmt.Sprintf("tfacc-pol-%s", randomSuffix())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t); requireHardware(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFirewallPolicyZonesConfig(zone1Name, zone2Name) + fmt.Sprintf(`
resource "terrifi_firewall_policy" "test" {
  name   = %q
  action = "BLOCK"

  source {
    zone_id = terrifi_firewall_zone.zone1.id
  }

  destination {
    zone_id = terrifi_firewall_zone.zone2.id
  }
}
`, policyName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_policy.test", "name", policyName),
					resource.TestCheckResourceAttr("terrifi_firewall_policy.test", "action", "BLOCK"),
					resource.TestCheckResourceAttr("terrifi_firewall_policy.test", "enabled", "true"),
					resource.TestCheckResourceAttr("terrifi_firewall_policy.test", "ip_version", "BOTH"),
					resource.TestCheckResourceAttr("terrifi_firewall_policy.test", "protocol", "all"),
					resource.TestCheckResourceAttr("terrifi_firewall_policy.test", "connection_state_type", "ALL"),
					resource.TestCheckResourceAttr("terrifi_firewall_policy.test", "site", "default"),
					resource.TestCheckResourceAttrSet("terrifi_firewall_policy.test", "id"),
					resource.TestCheckResourceAttrSet("terrifi_firewall_policy.test", "source.zone_id"),
					resource.TestCheckResourceAttrSet("terrifi_firewall_policy.test", "destination.zone_id"),
				),
			},
		},
	})
}

func TestAccFirewallPolicy_allow(t *testing.T) {
	zone1Name := fmt.Sprintf("tfacc-pol-a-z1-%s", randomSuffix())
	zone2Name := fmt.Sprintf("tfacc-pol-a-z2-%s", randomSuffix())
	policyName := fmt.Sprintf("tfacc-pol-allow-%s", randomSuffix())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t); requireHardware(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFirewallPolicyZonesConfig(zone1Name, zone2Name) + fmt.Sprintf(`
resource "terrifi_firewall_policy" "test" {
  name   = %q
  action = "ALLOW"

  source {
    zone_id = terrifi_firewall_zone.zone1.id
    ips     = ["10.0.0.0/24"]
  }

  destination {
    zone_id = terrifi_firewall_zone.zone2.id
  }
}
`, policyName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_policy.test", "action", "ALLOW"),
					resource.TestCheckResourceAttr("terrifi_firewall_policy.test", "source.ips.#", "1"),
				),
			},
		},
	})
}

func TestAccFirewallPolicy_macAddress(t *testing.T) {
	zone1Name := fmt.Sprintf("tfacc-pol-mac-z1-%s", randomSuffix())
	zone2Name := fmt.Sprintf("tfacc-pol-mac-z2-%s", randomSuffix())
	policyName := fmt.Sprintf("tfacc-pol-mac-%s", randomSuffix())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t); requireHardware(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFirewallPolicyZonesConfig(zone1Name, zone2Name) + fmt.Sprintf(`
resource "terrifi_firewall_policy" "test" {
  name   = %q
  action = "BLOCK"

  source {
    zone_id       = terrifi_firewall_zone.zone1.id
    mac_addresses = ["aa:bb:cc:dd:ee:ff"]
  }

  destination {
    zone_id = terrifi_firewall_zone.zone2.id
  }
}
`, policyName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_policy.test", "action", "BLOCK"),
					resource.TestCheckResourceAttr("terrifi_firewall_policy.test", "source.mac_addresses.#", "1"),
				),
			},
		},
	})
}

func TestAccFirewallPolicy_deviceIDs(t *testing.T) {
	zone1Name := fmt.Sprintf("tfacc-pol-dev-z1-%s", randomSuffix())
	zone2Name := fmt.Sprintf("tfacc-pol-dev-z2-%s", randomSuffix())
	policyName := fmt.Sprintf("tfacc-pol-dev-%s", randomSuffix())
	deviceName := fmt.Sprintf("tfacc-pol-dev-cd-%s", randomSuffix())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t); requireHardware(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFirewallPolicyZonesConfig(zone1Name, zone2Name) + fmt.Sprintf(`
resource "terrifi_client_device" "dev" {
  mac  = "aa:bb:cc:dd:ee:01"
  name = %q
}

resource "terrifi_firewall_policy" "test" {
  name   = %q
  action = "ALLOW"

  source {
    zone_id    = terrifi_firewall_zone.zone1.id
    device_ids = [terrifi_client_device.dev.mac]
  }

  destination {
    zone_id = terrifi_firewall_zone.zone2.id
  }
}
`, deviceName, policyName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_policy.test", "action", "ALLOW"),
					resource.TestCheckResourceAttr("terrifi_firewall_policy.test", "source.device_ids.#", "1"),
				),
			},
			// Verify import round-trip preserves device_ids.
			{
				ResourceName:      "terrifi_firewall_policy.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccFirewallPolicy_updateAction(t *testing.T) {
	zone1Name := fmt.Sprintf("tfacc-pol-ua-z1-%s", randomSuffix())
	zone2Name := fmt.Sprintf("tfacc-pol-ua-z2-%s", randomSuffix())
	policyName := fmt.Sprintf("tfacc-pol-upact-%s", randomSuffix())

	zonesConfig := testAccFirewallPolicyZonesConfig(zone1Name, zone2Name)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t); requireHardware(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: zonesConfig + fmt.Sprintf(`
resource "terrifi_firewall_policy" "test" {
  name   = %q
  action = "BLOCK"

  source {
    zone_id = terrifi_firewall_zone.zone1.id
  }

  destination {
    zone_id = terrifi_firewall_zone.zone2.id
  }
}
`, policyName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_policy.test", "action", "BLOCK"),
				),
			},
			{
				Config: zonesConfig + fmt.Sprintf(`
resource "terrifi_firewall_policy" "test" {
  name   = %q
  action = "ALLOW"

  source {
    zone_id = terrifi_firewall_zone.zone1.id
  }

  destination {
    zone_id = terrifi_firewall_zone.zone2.id
  }
}
`, policyName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_policy.test", "action", "ALLOW"),
				),
			},
		},
	})
}

func TestAccFirewallPolicy_updateZones(t *testing.T) {
	zone1Name := fmt.Sprintf("tfacc-pol-uz-z1-%s", randomSuffix())
	zone2Name := fmt.Sprintf("tfacc-pol-uz-z2-%s", randomSuffix())
	policyName := fmt.Sprintf("tfacc-pol-upz-%s", randomSuffix())

	zonesConfig := testAccFirewallPolicyZonesConfig(zone1Name, zone2Name)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t); requireHardware(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: zonesConfig + fmt.Sprintf(`
resource "terrifi_firewall_policy" "test" {
  name   = %q
  action = "BLOCK"

  source {
    zone_id = terrifi_firewall_zone.zone1.id
  }

  destination {
    zone_id = terrifi_firewall_zone.zone2.id
  }
}
`, policyName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("terrifi_firewall_policy.test", "source.zone_id", "terrifi_firewall_zone.zone1", "id"),
					resource.TestCheckResourceAttrPair("terrifi_firewall_policy.test", "destination.zone_id", "terrifi_firewall_zone.zone2", "id"),
				),
			},
			{
				Config: zonesConfig + fmt.Sprintf(`
resource "terrifi_firewall_policy" "test" {
  name   = %q
  action = "BLOCK"

  source {
    zone_id = terrifi_firewall_zone.zone2.id
  }

  destination {
    zone_id = terrifi_firewall_zone.zone1.id
  }
}
`, policyName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("terrifi_firewall_policy.test", "source.zone_id", "terrifi_firewall_zone.zone2", "id"),
					resource.TestCheckResourceAttrPair("terrifi_firewall_policy.test", "destination.zone_id", "terrifi_firewall_zone.zone1", "id"),
				),
			},
		},
	})
}

func TestAccFirewallPolicy_protocol(t *testing.T) {
	zone1Name := fmt.Sprintf("tfacc-pol-pr-z1-%s", randomSuffix())
	zone2Name := fmt.Sprintf("tfacc-pol-pr-z2-%s", randomSuffix())
	policyName := fmt.Sprintf("tfacc-pol-proto-%s", randomSuffix())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t); requireHardware(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFirewallPolicyZonesConfig(zone1Name, zone2Name) + fmt.Sprintf(`
resource "terrifi_firewall_policy" "test" {
  name     = %q
  action   = "ALLOW"
  protocol = "tcp"

  source {
    zone_id = terrifi_firewall_zone.zone1.id
  }

  destination {
    zone_id          = terrifi_firewall_zone.zone2.id
    port_matching_type = "SPECIFIC"
    port             = 80
  }
}
`, policyName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_policy.test", "protocol", "tcp"),
					resource.TestCheckResourceAttr("terrifi_firewall_policy.test", "destination.port_matching_type", "SPECIFIC"),
					resource.TestCheckResourceAttr("terrifi_firewall_policy.test", "destination.port", "80"),
				),
			},
		},
	})
}

func TestAccFirewallPolicy_schedule(t *testing.T) {
	zone1Name := fmt.Sprintf("tfacc-pol-sc-z1-%s", randomSuffix())
	zone2Name := fmt.Sprintf("tfacc-pol-sc-z2-%s", randomSuffix())
	policyName := fmt.Sprintf("tfacc-pol-sched-%s", randomSuffix())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t); requireHardware(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFirewallPolicyZonesConfig(zone1Name, zone2Name) + fmt.Sprintf(`
resource "terrifi_firewall_policy" "test" {
  name   = %q
  action = "BLOCK"

  source {
    zone_id = terrifi_firewall_zone.zone1.id
  }

  destination {
    zone_id = terrifi_firewall_zone.zone2.id
  }

  schedule {
    mode             = "EVERY_WEEK"
    time_range_start = "08:00"
    time_range_end   = "17:00"
    repeat_on_days   = ["mon", "tue", "wed", "thu", "fri"]
  }
}
`, policyName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_policy.test", "schedule.mode", "EVERY_WEEK"),
					resource.TestCheckResourceAttr("terrifi_firewall_policy.test", "schedule.time_range_start", "08:00"),
					resource.TestCheckResourceAttr("terrifi_firewall_policy.test", "schedule.time_range_end", "17:00"),
					resource.TestCheckResourceAttr("terrifi_firewall_policy.test", "schedule.repeat_on_days.#", "5"),
				),
			},
		},
	})
}

func TestAccFirewallPolicy_disabled(t *testing.T) {
	zone1Name := fmt.Sprintf("tfacc-pol-dis-z1-%s", randomSuffix())
	zone2Name := fmt.Sprintf("tfacc-pol-dis-z2-%s", randomSuffix())
	policyName := fmt.Sprintf("tfacc-pol-disabled-%s", randomSuffix())

	zonesConfig := testAccFirewallPolicyZonesConfig(zone1Name, zone2Name)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t); requireHardware(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: zonesConfig + fmt.Sprintf(`
resource "terrifi_firewall_policy" "test" {
  name    = %q
  action  = "BLOCK"
  enabled = false

  source {
    zone_id = terrifi_firewall_zone.zone1.id
  }

  destination {
    zone_id = terrifi_firewall_zone.zone2.id
  }
}
`, policyName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_policy.test", "enabled", "false"),
				),
			},
			{
				Config: zonesConfig + fmt.Sprintf(`
resource "terrifi_firewall_policy" "test" {
  name    = %q
  action  = "BLOCK"
  enabled = true

  source {
    zone_id = terrifi_firewall_zone.zone1.id
  }

  destination {
    zone_id = terrifi_firewall_zone.zone2.id
  }
}
`, policyName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_policy.test", "enabled", "true"),
				),
			},
		},
	})
}

func TestAccFirewallPolicy_logging(t *testing.T) {
	zone1Name := fmt.Sprintf("tfacc-pol-log-z1-%s", randomSuffix())
	zone2Name := fmt.Sprintf("tfacc-pol-log-z2-%s", randomSuffix())
	policyName := fmt.Sprintf("tfacc-pol-logging-%s", randomSuffix())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t); requireHardware(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFirewallPolicyZonesConfig(zone1Name, zone2Name) + fmt.Sprintf(`
resource "terrifi_firewall_policy" "test" {
  name    = %q
  action  = "BLOCK"
  logging = true

  source {
    zone_id = terrifi_firewall_zone.zone1.id
  }

  destination {
    zone_id = terrifi_firewall_zone.zone2.id
  }
}
`, policyName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_policy.test", "logging", "true"),
				),
			},
		},
	})
}

func TestAccFirewallPolicy_import(t *testing.T) {
	zone1Name := fmt.Sprintf("tfacc-pol-imp-z1-%s", randomSuffix())
	zone2Name := fmt.Sprintf("tfacc-pol-imp-z2-%s", randomSuffix())
	policyName := fmt.Sprintf("tfacc-pol-import-%s", randomSuffix())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t); requireHardware(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFirewallPolicyZonesConfig(zone1Name, zone2Name) + fmt.Sprintf(`
resource "terrifi_firewall_policy" "test" {
  name   = %q
  action = "BLOCK"

  source {
    zone_id = terrifi_firewall_zone.zone1.id
  }

  destination {
    zone_id = terrifi_firewall_zone.zone2.id
  }
}
`, policyName),
			},
			{
				ResourceName:      "terrifi_firewall_policy.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccFirewallPolicy_importSiteID(t *testing.T) {
	zone1Name := fmt.Sprintf("tfacc-pol-imps-z1-%s", randomSuffix())
	zone2Name := fmt.Sprintf("tfacc-pol-imps-z2-%s", randomSuffix())
	policyName := fmt.Sprintf("tfacc-pol-impsid-%s", randomSuffix())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t); requireHardware(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFirewallPolicyZonesConfig(zone1Name, zone2Name) + fmt.Sprintf(`
resource "terrifi_firewall_policy" "test" {
  name   = %q
  action = "BLOCK"

  source {
    zone_id = terrifi_firewall_zone.zone1.id
  }

  destination {
    zone_id = terrifi_firewall_zone.zone2.id
  }
}
`, policyName),
			},
			{
				ResourceName:      "terrifi_firewall_policy.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs := s.RootModule().Resources["terrifi_firewall_policy.test"]
					if rs == nil {
						return "", fmt.Errorf("resource not found in state")
					}
					return fmt.Sprintf("%s:%s", rs.Primary.Attributes["site"], rs.Primary.Attributes["id"]), nil
				},
			},
		},
	})
}

func TestAccFirewallPolicy_description(t *testing.T) {
	zone1Name := fmt.Sprintf("tfacc-pol-desc-z1-%s", randomSuffix())
	zone2Name := fmt.Sprintf("tfacc-pol-desc-z2-%s", randomSuffix())
	policyName := fmt.Sprintf("tfacc-pol-desc-%s", randomSuffix())

	zonesConfig := testAccFirewallPolicyZonesConfig(zone1Name, zone2Name)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t); requireHardware(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: zonesConfig + fmt.Sprintf(`
resource "terrifi_firewall_policy" "test" {
  name        = %q
  action      = "BLOCK"
  description = "Initial description"

  source {
    zone_id = terrifi_firewall_zone.zone1.id
  }

  destination {
    zone_id = terrifi_firewall_zone.zone2.id
  }
}
`, policyName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_policy.test", "description", "Initial description"),
				),
			},
			{
				Config: zonesConfig + fmt.Sprintf(`
resource "terrifi_firewall_policy" "test" {
  name        = %q
  action      = "BLOCK"
  description = "Updated description"

  source {
    zone_id = terrifi_firewall_zone.zone1.id
  }

  destination {
    zone_id = terrifi_firewall_zone.zone2.id
  }
}
`, policyName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_policy.test", "description", "Updated description"),
				),
			},
		},
	})
}

func TestAccFirewallPolicy_multiple(t *testing.T) {
	zone1Name := fmt.Sprintf("tfacc-pol-mul-z1-%s", randomSuffix())
	zone2Name := fmt.Sprintf("tfacc-pol-mul-z2-%s", randomSuffix())
	pol1Name := fmt.Sprintf("tfacc-pol-multi1-%s", randomSuffix())
	pol2Name := fmt.Sprintf("tfacc-pol-multi2-%s", randomSuffix())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t); requireHardware(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFirewallPolicyZonesConfig(zone1Name, zone2Name) + fmt.Sprintf(`
resource "terrifi_firewall_policy" "pol1" {
  name   = %q
  action = "BLOCK"

  source {
    zone_id = terrifi_firewall_zone.zone1.id
  }

  destination {
    zone_id = terrifi_firewall_zone.zone2.id
  }
}

resource "terrifi_firewall_policy" "pol2" {
  name   = %q
  action = "ALLOW"

  source {
    zone_id = terrifi_firewall_zone.zone1.id
  }

  destination {
    zone_id = terrifi_firewall_zone.zone2.id
  }
}
`, pol1Name, pol2Name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_policy.pol1", "name", pol1Name),
					resource.TestCheckResourceAttr("terrifi_firewall_policy.pol1", "action", "BLOCK"),
					resource.TestCheckResourceAttr("terrifi_firewall_policy.pol2", "name", pol2Name),
					resource.TestCheckResourceAttr("terrifi_firewall_policy.pol2", "action", "ALLOW"),
				),
			},
		},
	})
}

func TestAccFirewallPolicy_idempotent(t *testing.T) {
	zone1Name := fmt.Sprintf("tfacc-pol-id-z1-%s", randomSuffix())
	zone2Name := fmt.Sprintf("tfacc-pol-id-z2-%s", randomSuffix())
	policyName := fmt.Sprintf("tfacc-pol-idem-%s", randomSuffix())

	config := testAccFirewallPolicyZonesConfig(zone1Name, zone2Name) + fmt.Sprintf(`
resource "terrifi_firewall_policy" "test" {
  name   = %q
  action = "BLOCK"

  source {
    zone_id = terrifi_firewall_zone.zone1.id
  }

  destination {
    zone_id = terrifi_firewall_zone.zone2.id
  }
}
`, policyName)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t); requireHardware(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_firewall_policy.test", "name", policyName),
				),
			},
			{
				Config:   config,
				PlanOnly: true,
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func testAccFirewallPolicyZonesConfig(zone1Name, zone2Name string) string {
	return fmt.Sprintf(`
resource "terrifi_firewall_zone" "zone1" {
  name = %q
}

resource "terrifi_firewall_zone" "zone2" {
  name = %q
}
`, zone1Name, zone2Name)
}
