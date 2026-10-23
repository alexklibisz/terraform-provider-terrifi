package unifi

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIDTime(t *testing.T) {
	got, ok := IDTime("65000000000000000000abcd")
	assert.True(t, ok)
	assert.Equal(t, time.Unix(0x65000000, 0), got)

	for _, bad := range []string{"", "not-an-id", "65000000", "zz000000000000000000abcd"} {
		_, ok := IDTime(bad)
		assert.False(t, ok, bad)
	}
}
