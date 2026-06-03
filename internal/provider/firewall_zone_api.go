package provider

// Local CRUD methods for the v2 firewall/zone endpoint. These shadow the
// promoted go-unifi methods on *Client and use the local internal/unifi types
// instead, removing the SDK dependency for this resource — see issue #157.
//
// Notes about wire-format quirks the controller has (these are not SDK bugs
// — they are controller behavior that any client must handle):
//
//  1. The Create/Update endpoint rejects a body containing `default_zone:
//     false` with HTTP 400. The local unifi.FirewallZone type omits the
//     DefaultZone field entirely, so json.Marshal never emits it.
//
//  2. PUT requires `_id` in the request body (not just in the URL path) and
//     returns 500 ("The given id must not be null") without it. The local
//     unifi.FirewallZone declares ID with `json:"_id,omitempty"`, so a
//     non-empty ID is always serialized on update.
//
//  3. DELETE returns 204 No Content. doV2Request accepts any 2xx status,
//     so this is handled transparently.
//
//  4. The v2 endpoint does not expose a per-ID GET, so we list all zones and
//     filter by ID — this also ensures `network_ids` is consistently
//     populated (the v1 single-zone endpoint would have returned without it).

import (
	"context"
	"fmt"
	"net/http"

	"github.com/alexklibisz/terrifi/internal/unifi"
)

// GetFirewallZone reads a firewall zone by ID via the v2 list endpoint
// (the v2 API does not support per-ID GET).
func (c *Client) GetFirewallZone(ctx context.Context, site string, id string) (*unifi.FirewallZone, error) {
	zones, err := c.ListFirewallZone(ctx, site)
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

// ListFirewallZone returns all firewall zones for the site.
func (c *Client) ListFirewallZone(ctx context.Context, site string) ([]unifi.FirewallZone, error) {
	var zones []unifi.FirewallZone
	if err := c.doV2Request(ctx, http.MethodGet,
		fmt.Sprintf("%s%s/v2/api/site/%s/firewall/zone", c.BaseURL, c.APIPath, site),
		nil, &zones); err != nil {
		return nil, err
	}
	return zones, nil
}

// CreateFirewallZone creates a firewall zone via the v2 endpoint.
func (c *Client) CreateFirewallZone(ctx context.Context, site string, d *unifi.FirewallZone) (*unifi.FirewallZone, error) {
	payload := normalizeFirewallZone(d)
	var result unifi.FirewallZone
	if err := c.doV2Request(ctx, http.MethodPost,
		fmt.Sprintf("%s%s/v2/api/site/%s/firewall/zone", c.BaseURL, c.APIPath, site),
		payload, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateFirewallZone updates a firewall zone via the v2 endpoint. The local
// FirewallZone struct includes `_id` in the marshaled body, satisfying the
// controller's PUT requirement (see file-level note 2).
func (c *Client) UpdateFirewallZone(ctx context.Context, site string, d *unifi.FirewallZone) (*unifi.FirewallZone, error) {
	payload := normalizeFirewallZone(d)
	var result unifi.FirewallZone
	if err := c.doV2Request(ctx, http.MethodPut,
		fmt.Sprintf("%s%s/v2/api/site/%s/firewall/zone/%s", c.BaseURL, c.APIPath, site, d.ID),
		payload, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteFirewallZone deletes a firewall zone via the v2 endpoint.
func (c *Client) DeleteFirewallZone(ctx context.Context, site string, id string) error {
	return c.doV2Request(ctx, http.MethodDelete,
		fmt.Sprintf("%s%s/v2/api/site/%s/firewall/zone/%s", c.BaseURL, c.APIPath, site, id),
		struct{}{}, nil)
}

// normalizeFirewallZone ensures NetworkIDs is a non-nil slice so it serializes
// as [] rather than null. The controller treats a missing network_ids field
// as "leave unchanged"; we always want to send the authoritative set.
func normalizeFirewallZone(d *unifi.FirewallZone) unifi.FirewallZone {
	out := *d
	if out.NetworkIDs == nil {
		out.NetworkIDs = []string{}
	}
	return out
}
