package provider

import "sync"

// responseCache stores raw response bytes from GET requests, keyed by URL.
// It prevents duplicate list-all API calls during Terraform's concurrent
// refresh phase — firewall zones and policies use v2 endpoints that only
// support list-all (no GET-by-ID), so N resources produce N identical calls.
//
// The cache is opt-in via the provider's response_caching attribute. When the
// Client.cache field is nil, all cache operations are no-ops (zero overhead).
type responseCache struct {
	mu      sync.RWMutex
	entries map[string][]byte
}

func newResponseCache() *responseCache {
	return &responseCache{entries: make(map[string][]byte)}
}

// get returns cached response bytes for the given URL. Returns nil, false on
// cache miss. Uses RLock for concurrent-read safety.
func (rc *responseCache) get(url string) ([]byte, bool) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	data, ok := rc.entries[url]
	return data, ok
}

// set stores response bytes for the given URL.
func (rc *responseCache) set(url string, data []byte) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.entries[url] = data
}

// invalidateAll clears all cached entries. Called on any write operation
// (POST, PUT, DELETE) to ensure subsequent reads see fresh data.
func (rc *responseCache) invalidateAll() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	clear(rc.entries)
}
