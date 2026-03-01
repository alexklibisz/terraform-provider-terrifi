package provider

// TODO(go-unifi): This entire file is a workaround for bugs in the go-unifi
// SDK (github.com/ubiquiti-community/go-unifi). When the upstream SDK fixes
// these issues, this file can be deleted and the firewall zone resource can
// use the SDK's built-in methods directly (c.ApiClient.Get/Create/Update/
// DeleteFirewallZone). The upstream bugs are:
//
//  1. unifi.FirewallZone serializes `"default_zone": false` (no omitempty),
//     which the UniFi v2 API rejects with 400 Bad Request.
//     Fix needed in SDK: add `omitempty` to the DefaultZone field tag in
//     FirewallZone, or remove the field if it's not a real v2 API concept.
//
//  2. SDK's UpdateFirewallZone does not include `"_id"` in the PUT request
//     body (only in the URL path). The v2 API requires it in both places and
//     returns 500 ("The given id must not be null") without it.
//     Fix needed in SDK: include ID in the request body for PUT calls on the
//     v2 firewall zone endpoint.
//
//  3. SDK's DeleteFirewallZone only treats HTTP 200 as success. The v2
//     firewall zone DELETE endpoint returns 204 No Content on success, which
//     the SDK misinterprets as an error.
//     Fix needed in SDK: accept any 2xx status code as success, or
//     specifically handle 204 for delete operations.
//
//  4. SDK's GetFirewallZone uses the v1 REST endpoint, which does not
//     consistently return the `network_ids` field. Since Create and Update
//     use the v2 endpoint (which does return network_ids), this mismatch
//     causes Terraform to see empty network_ids on refresh, producing a
//     non-empty plan diff and flaky acceptance tests.
//     Fix needed in SDK: use the v2 GET endpoint for firewall zones.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/ubiquiti-community/go-unifi/unifi"
)

// firewallZoneCreateRequest is the minimal payload for POST /v2/api/site/{site}/firewall/zone.
// Uses a bespoke struct (rather than unifi.FirewallZone) to avoid SDK bug #1 above.
type firewallZoneCreateRequest struct {
	Name       string   `json:"name,omitempty"`
	NetworkIDs []string `json:"network_ids"`
}

// firewallZoneUpdateRequest is the minimal payload for PUT /v2/api/site/{site}/firewall/zone/{id}.
// Includes _id to work around SDK bug #2 above.
type firewallZoneUpdateRequest struct {
	ID         string   `json:"_id"`
	Name       string   `json:"name,omitempty"`
	NetworkIDs []string `json:"network_ids"`
}

// GetFirewallZone reads a firewall zone via the v2 API, bypassing the SDK
// to avoid bug #4 (v1 endpoint doesn't return network_ids consistently).
// The v2 API does not support GET on individual zones, so we list all zones
// and filter by ID (same pattern as GetFirewallPolicy).
// This method shadows the SDK's promoted GetFirewallZone on ApiClient.
func (c *Client) GetFirewallZone(ctx context.Context, site string, id string) (*unifi.FirewallZone, error) {
	var zones []unifi.FirewallZone
	err := c.doV2Request(ctx, http.MethodGet,
		fmt.Sprintf("%s%s/v2/api/site/%s/firewall/zone", c.BaseURL, c.APIPath, site),
		struct{}{}, &zones)
	if err != nil {
		return nil, err
	}

	for i := range zones {
		if zones[i].ID == id {
			return &zones[i], nil
		}
	}
	return nil, &unifi.NotFoundError{}
}

// CreateFirewallZone creates a firewall zone via the v2 API, bypassing the
// SDK to avoid bug #1 (default_zone serialization).
func (c *Client) CreateFirewallZone(ctx context.Context, site string, d *unifi.FirewallZone) (*unifi.FirewallZone, error) {
	payload := firewallZoneCreateRequest{
		Name:       d.Name,
		NetworkIDs: d.NetworkIDs,
	}
	if payload.NetworkIDs == nil {
		payload.NetworkIDs = []string{}
	}

	var result unifi.FirewallZone
	err := c.doV2Request(ctx, http.MethodPost,
		fmt.Sprintf("%s%s/v2/api/site/%s/firewall/zone", c.BaseURL, c.APIPath, site),
		payload, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateFirewallZone updates a firewall zone via the v2 API, bypassing the
// SDK to avoid bugs #1 (default_zone) and #2 (_id in PUT body).
func (c *Client) UpdateFirewallZone(ctx context.Context, site string, d *unifi.FirewallZone) (*unifi.FirewallZone, error) {
	payload := firewallZoneUpdateRequest{
		ID:         d.ID,
		Name:       d.Name,
		NetworkIDs: d.NetworkIDs,
	}
	if payload.NetworkIDs == nil {
		payload.NetworkIDs = []string{}
	}

	var result unifi.FirewallZone
	err := c.doV2Request(ctx, http.MethodPut,
		fmt.Sprintf("%s%s/v2/api/site/%s/firewall/zone/%s", c.BaseURL, c.APIPath, site, d.ID),
		payload, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteFirewallZone deletes a firewall zone via the v2 API, bypassing the
// SDK to avoid bug #3 (204 No Content treated as error).
func (c *Client) DeleteFirewallZone(ctx context.Context, site string, id string) error {
	return c.doV2Request(ctx, http.MethodDelete,
		fmt.Sprintf("%s%s/v2/api/site/%s/firewall/zone/%s", c.BaseURL, c.APIPath, site, id),
		struct{}{}, nil)
}

// doV2Request makes an authenticated HTTP request to the UniFi v2 API.
// It is shared by firewall zone and firewall policy operations.
func (c *Client) doV2Request(ctx context.Context, method, url string, body any, result any) error {
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshaling request body: %w", err)
	}

	req, err := retryablehttp.NewRequestWithContext(ctx, method, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Replicate the SDK's auth logic: API key takes precedence over CSRF token.
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

	if result != nil && len(respBytes) > 0 {
		if err := json.Unmarshal(respBytes, result); err != nil {
			return fmt.Errorf("unmarshaling response: %w", err)
		}
	}

	return nil
}
