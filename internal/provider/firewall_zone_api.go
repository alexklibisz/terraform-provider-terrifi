package provider

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

// firewallZoneRequest is the payload for creating/updating firewall zones.
// We define our own struct instead of using unifi.FirewallZone because the
// SDK struct includes "default_zone" (without omitempty) which the UniFi v2
// API rejects as an unrecognized field.
type firewallZoneRequest struct {
	Name       string   `json:"name,omitempty"`
	NetworkIDs []string `json:"network_ids"`
}

// CreateFirewallZone creates a firewall zone using the v2 API. This bypasses
// the go-unifi SDK's CreateFirewallZone because the SDK serializes
// "default_zone":false which the API rejects (400 Bad Request).
func (c *Client) CreateFirewallZone(ctx context.Context, site string, d *unifi.FirewallZone) (*unifi.FirewallZone, error) {
	payload := firewallZoneRequest{
		Name:       d.Name,
		NetworkIDs: d.NetworkIDs,
	}
	if payload.NetworkIDs == nil {
		payload.NetworkIDs = []string{}
	}

	var result unifi.FirewallZone
	err := c.doFirewallZoneRequest(ctx, http.MethodPost,
		fmt.Sprintf("%s%s/v2/api/site/%s/firewall/zone", c.BaseURL, c.APIPath, site),
		payload, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateFirewallZone updates a firewall zone using the v2 API. Same
// serialization workaround as CreateFirewallZone.
func (c *Client) UpdateFirewallZone(ctx context.Context, site string, d *unifi.FirewallZone) (*unifi.FirewallZone, error) {
	payload := firewallZoneRequest{
		Name:       d.Name,
		NetworkIDs: d.NetworkIDs,
	}
	if payload.NetworkIDs == nil {
		payload.NetworkIDs = []string{}
	}

	var result unifi.FirewallZone
	err := c.doFirewallZoneRequest(ctx, http.MethodPut,
		fmt.Sprintf("%s%s/v2/api/site/%s/firewall/zone/%s", c.BaseURL, c.APIPath, site, d.ID),
		payload, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteFirewallZone deletes a firewall zone using the v2 API. The SDK's
// DeleteFirewallZone fails because the API returns 204 No Content, which the
// SDK misinterprets as an error (it only treats 200 as success).
func (c *Client) DeleteFirewallZone(ctx context.Context, site string, id string) error {
	return c.doFirewallZoneRequest(ctx, http.MethodDelete,
		fmt.Sprintf("%s%s/v2/api/site/%s/firewall/zone/%s", c.BaseURL, c.APIPath, site, id),
		struct{}{}, nil)
}

// doFirewallZoneRequest makes an authenticated HTTP request to the UniFi v2 API.
func (c *Client) doFirewallZoneRequest(ctx context.Context, method, url string, body any, result any) error {
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
	} else if csrf := c.ApiClient.CSRFToken(); csrf != "" {
		req.Header.Set("X-Csrf-Token", csrf)
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
