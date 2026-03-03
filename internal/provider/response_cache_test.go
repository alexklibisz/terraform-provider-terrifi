package provider

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponseCache_Miss(t *testing.T) {
	rc := newResponseCache()
	data, ok := rc.get("https://example.com/api/zones")
	assert.False(t, ok)
	assert.Nil(t, data)
}

func TestResponseCache_Hit(t *testing.T) {
	rc := newResponseCache()
	url := "https://example.com/api/zones"
	payload := []byte(`[{"_id":"z1","name":"LAN"}]`)

	rc.set(url, payload)

	data, ok := rc.get(url)
	require.True(t, ok)
	assert.Equal(t, payload, data)
}

func TestResponseCache_InvalidateAll(t *testing.T) {
	rc := newResponseCache()
	rc.set("https://example.com/api/zones", []byte(`[]`))
	rc.set("https://example.com/api/policies", []byte(`[]`))

	rc.invalidateAll()

	_, ok := rc.get("https://example.com/api/zones")
	assert.False(t, ok)
	_, ok = rc.get("https://example.com/api/policies")
	assert.False(t, ok)
}

func TestResponseCache_Overwrite(t *testing.T) {
	rc := newResponseCache()
	url := "https://example.com/api/zones"

	rc.set(url, []byte(`[{"_id":"z1"}]`))
	rc.set(url, []byte(`[{"_id":"z1"},{"_id":"z2"}]`))

	data, ok := rc.get(url)
	require.True(t, ok)
	assert.Equal(t, []byte(`[{"_id":"z1"},{"_id":"z2"}]`), data)
}

func TestResponseCache_ConcurrentReads(t *testing.T) {
	rc := newResponseCache()
	url := "https://example.com/api/zones"
	payload := []byte(`[{"_id":"z1"}]`)
	rc.set(url, payload)

	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data, ok := rc.get(url)
			assert.True(t, ok)
			assert.Equal(t, payload, data)
		}()
	}
	wg.Wait()
}

func TestResponseCache_ConcurrentSetAndInvalidate(t *testing.T) {
	// This test exercises the race detector — it should not deadlock or panic.
	rc := newResponseCache()
	url := "https://example.com/api/zones"

	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			rc.set(url, []byte(`data`))
		}(i)
		go func(n int) {
			defer wg.Done()
			rc.invalidateAll()
		}(i)
	}
	wg.Wait()
}
