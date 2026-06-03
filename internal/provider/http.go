package provider

// Shared HTTP helper used by every *_api.go file to make authenticated
// requests against the UniFi controller. Previously lived in
// firewall_zone_api.go; moved here when firewall_zone was migrated off the
// go-unifi SDK to keep the helper independent of any single resource file.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/go-retryablehttp"
)

// doV2Request makes an authenticated HTTP request to the UniFi controller.
// Despite the v2 in the name, the helper is endpoint-agnostic and is used for
// both v1 (/api/s/{site}/rest/...) and v2 (/v2/api/site/{site}/...) paths.
//
// When response caching is enabled (c.cache != nil), GET responses are cached
// by URL and subsequent GETs return cached bytes without hitting the
// controller. Any non-GET request (POST, PUT, DELETE) invalidates the entire
// cache to ensure subsequent reads see fresh data.
func (c *Client) doV2Request(ctx context.Context, method, url string, body any, result any) error {
	if method == http.MethodGet && c.cache != nil {
		if cached, ok := c.cache.get(url); ok {
			if result != nil && len(cached) > 0 {
				if err := json.Unmarshal(cached, result); err != nil {
					return fmt.Errorf("unmarshaling cached response: %w", err)
				}
			}
			return nil
		}
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshaling request body: %w", err)
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
		return fmt.Errorf("(%d) for %s %s\npayload: %s\nresponse: %s", resp.StatusCode, method, url, string(bodyBytes), string(respBytes))
	}

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
