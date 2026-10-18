package main

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateResetPlan(t *testing.T) {
	baseline := map[string]bool{"terrifi_network.default": true}
	for _, test := range []struct {
		phase, address, actions string
		allowed                 bool
	}{
		{"baseline", "terrifi_network.default", `["update"]`, true},
		{"baseline", "terrifi_network.default", `["delete"]`, false},
		{"baseline", "terrifi_network.default", `["delete","create"]`, false},
		{"baseline", "terrifi_dns_record.hil_reset_retry", `["delete"]`, true},
		{"capture", "terrifi_dns_record.hil_reset_new", `["no-op"]`, true},
		{"capture", "terrifi_dns_record.hil_reset_new", `["update"]`, false},
		{"capture", "terrifi_dns_record.hil_reset_new", `["create"]`, false},
		{"delete", "terrifi_dns_record.hil_reset_new", `["delete"]`, true},
		{"delete", "terrifi_network.default", `["delete"]`, false},
		{"delete", "terrifi_firewall_zone.internal", `["delete"]`, false},
		{"delete", "terrifi_dns_record.hil_reset_new", `["delete","create"]`, false},
		{"verify", "terrifi_network.default", `["update"]`, false},
		{"verify", "terrifi_network.default", `["no-op"]`, true},
	} {
		t.Run(test.phase+"/"+test.address+"/"+test.actions, func(t *testing.T) {
			var plan resetPlan
			require.NoError(t, json.Unmarshal([]byte(`{"resource_changes":[{"address":"`+test.address+`","change":{"actions":`+test.actions+`}}]}`), &plan))
			err := validateResetPlan(plan, test.phase, baseline)
			if test.allowed {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
	var importing resetPlan
	require.NoError(t, json.Unmarshal([]byte(`{"resource_changes":[{"address":"terrifi_network.default","change":{"actions":["no-op"],"importing":{"id":"lan"}}}]}`), &importing))
	require.Error(t, validateResetPlan(importing, "verify", baseline))
	require.Error(t, validateResetPlan(importing, "delete", baseline))
}
