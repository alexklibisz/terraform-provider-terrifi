package provider

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alexklibisz/terrifi/internal/unifi"
)

// Verifies that a networkconf payload carrying a string-encoded
// ipv6_ra_preferred_lifetime decodes cleanly into the local unifi.Network
// type. The local type intentionally omits that field, so the coercion the
// SDK needed (issue #154) is unnecessary here — json.Unmarshal silently
// skips the unknown field. Guard regression: if a future contributor adds
// IPV6RaPreferredLifetime as *int64, this test will fail.
func TestNetwork_decodesStringIpv6RaPreferredLifetime(t *testing.T) {
	const payload = `{
		"_id": "abc123",
		"name": "LAN",
		"purpose": "corporate",
		"ipv6_ra_preferred_lifetime": "14400"
	}`

	var n unifi.Network
	require.NoError(t, json.Unmarshal([]byte(payload), &n))
	assert.Equal(t, "abc123", n.ID)
	require.NotNil(t, n.Name)
	assert.Equal(t, "LAN", *n.Name)
	assert.Equal(t, "corporate", n.Purpose)
}

// Verifies the SDK's emptyBoolToTrue behavior for internet_access_enabled:
// a missing or null field decodes as true, matching the controller's
// implicit default.
func TestNetwork_internetAccessDefaultTrue(t *testing.T) {
	t.Run("missing field defaults to true", func(t *testing.T) {
		var n unifi.Network
		require.NoError(t, json.Unmarshal([]byte(`{"_id":"x","purpose":"corporate"}`), &n))
		assert.True(t, n.InternetAccessEnabled)
	})

	t.Run("explicit false stays false", func(t *testing.T) {
		var n unifi.Network
		require.NoError(t, json.Unmarshal([]byte(`{"_id":"x","internet_access_enabled":false}`), &n))
		assert.False(t, n.InternetAccessEnabled)
	})

	t.Run("explicit true stays true", func(t *testing.T) {
		var n unifi.Network
		require.NoError(t, json.Unmarshal([]byte(`{"_id":"x","internet_access_enabled":true}`), &n))
		assert.True(t, n.InternetAccessEnabled)
	})

	t.Run("null defaults to true", func(t *testing.T) {
		var n unifi.Network
		require.NoError(t, json.Unmarshal([]byte(`{"_id":"x","internet_access_enabled":null}`), &n))
		assert.True(t, n.InternetAccessEnabled)
	})
}
