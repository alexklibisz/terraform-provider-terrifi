package unifi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// LAN networks A–E; A plays the role of the Default network.
var testLAN = []string{"net-a", "net-b", "net-c", "net-d", "net-e"}

func TestExcludedFromTagged(t *testing.T) {
	t.Run("excludes everything but tagged and native", func(t *testing.T) {
		// Mirrors a UI change: native C, tagged E → A, B, D excluded.
		got, err := ExcludedFromTagged(testLAN, []string{"net-e"}, "net-c")
		require.NoError(t, err)
		assert.Equal(t, []string{"net-a", "net-b", "net-d"}, got)
	})

	t.Run("no native network", func(t *testing.T) {
		got, err := ExcludedFromTagged(testLAN, []string{"net-e"}, "")
		require.NoError(t, err)
		assert.Equal(t, []string{"net-a", "net-b", "net-c", "net-d"}, got)
	})

	t.Run("nothing tagged", func(t *testing.T) {
		got, err := ExcludedFromTagged(testLAN, nil, "net-a")
		require.NoError(t, err)
		assert.Equal(t, []string{"net-b", "net-c", "net-d", "net-e"}, got)
	})

	t.Run("everything tagged gives an empty, non-nil list", func(t *testing.T) {
		got, err := ExcludedFromTagged(testLAN, []string{"net-b", "net-c", "net-d", "net-e"}, "net-a")
		require.NoError(t, err)
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})

	t.Run("unknown tagged network is an error", func(t *testing.T) {
		_, err := ExcludedFromTagged(testLAN, []string{"net-wan"}, "net-a")
		assert.ErrorContains(t, err, `"net-wan" is not a LAN network`)
	})
}

func TestTaggedFromExcluded(t *testing.T) {
	t.Run("inverse of ExcludedFromTagged", func(t *testing.T) {
		assert.Equal(t, []string{"net-e"}, TaggedFromExcluded(testLAN, []string{"net-a", "net-b", "net-d"}, "net-c"))
	})

	t.Run("a network missing from the deny-list reads as tagged", func(t *testing.T) {
		// net-e was created after the profile was written and isn't excluded.
		assert.Equal(t, []string{"net-d", "net-e"}, TaggedFromExcluded(testLAN, []string{"net-a", "net-b"}, "net-c"))
	})

	t.Run("excluded IDs that aren't LAN networks are ignored", func(t *testing.T) {
		assert.Equal(t, []string{"net-e"}, TaggedFromExcluded(testLAN, []string{"net-a", "net-b", "net-d", "net-gone"}, "net-c"))
	})

	t.Run("nothing tagged gives an empty, non-nil list", func(t *testing.T) {
		got := TaggedFromExcluded(testLAN, []string{"net-b", "net-c", "net-d", "net-e"}, "net-a")
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})

	t.Run("round trip for every subset", func(t *testing.T) {
		for mask := 0; mask < 1<<4; mask++ {
			var tagged []string
			for i, id := range testLAN[1:] {
				if mask&(1<<i) != 0 {
					tagged = append(tagged, id)
				}
			}
			excluded, err := ExcludedFromTagged(testLAN, tagged, "net-a")
			require.NoError(t, err)
			got := TaggedFromExcluded(testLAN, excluded, "net-a")
			if tagged == nil {
				tagged = []string{}
			}
			assert.Equal(t, tagged, got, "mask %04b", mask)
		}
	})
}
