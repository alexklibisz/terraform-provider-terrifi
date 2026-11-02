package unifi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/go-retryablehttp"
)

// Do makes an authenticated HTTP request to the UniFi v2 API.
// It is shared by firewall zone and firewall policy operations.
//
// When response caching is enabled (c.cache != nil), GET responses are cached
// by URL and subsequent GETs return cached bytes without hitting the controller.
// Any non-GET request (POST, PUT, DELETE) invalidates the entire cache to ensure
// subsequent reads see fresh data.
func (c *APIClient) Do(ctx context.Context, method, url string, body any, result any) error {
	return c.doRequest(ctx, method, url, body, result, true)
}

// doRequest shares transport and authentication for local API contracts.
// useCached=false forces a controller read when verifying asynchronous changes.
// Fresh responses still replace the cached value for subsequent readers.
func (c *APIClient) doRequest(ctx context.Context, method, url string, body, result any, useCached bool) error {
	// Cache hit path: return cached bytes for GET requests without making an HTTP call.
	if useCached && method == http.MethodGet && c.cache != nil {
		if cached, ok := c.cache.get(url); ok {
			if result != nil && len(cached) > 0 {
				if err := json.Unmarshal(cached, result); err != nil {
					return fmt.Errorf("unmarshaling cached response: %w", err)
				}
			}
			return nil
		}
	}

	var bodyBytes []byte
	var err error
	if body != nil {
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshaling request body: %w", err)
		}
	}

	req, err := retryablehttp.NewRequestWithContext(ctx, method, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if c.APIKey != "" {
		req.Header.Set("X-API-Key", c.APIKey)
	} else if c.csrf != "" {
		req.Header.Set("X-Csrf-Token", c.csrf)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("performing request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &HTTPError{StatusCode: resp.StatusCode, Method: method, URL: url, Response: string(respBytes)}
	}

	// Cache management: store GET responses, invalidate on writes.
	if c.cache != nil {
		if method == http.MethodGet {
			c.cache.set(url, respBytes)
		} else {
			c.cache.invalidateAll()
		}
	}

	if result != nil && len(respBytes) > 0 {
		if err := json.Unmarshal(respBytes, result); err != nil {
			return fmt.Errorf("unmarshaling response: %w", err)
		}
	}

	return nil
}
