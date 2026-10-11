package provider

// Local CRUD methods for the v1 REST firewall group endpoint. These shadow
// the promoted github.com/ubiquiti-community/go-unifi methods on *Client and
// use the local internal/unifi types instead, removing the SDK dependency for
// this resource — see issue #157.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/alexklibisz/terrifi/internal/unifi"
)

type firewallGroupEnvelope struct {
	Meta json.RawMessage       `json:"meta"`
	Data []unifi.FirewallGroup `json:"data"`
}

// CreateFirewallGroup posts a new firewall group to /api/s/{site}/rest/firewallgroup.
func (c *Client) CreateFirewallGroup(ctx context.Context, site string, d *unifi.FirewallGroup) (*unifi.FirewallGroup, error) {
	payload := normalizeFirewallGroup(d)
	var resp firewallGroupEnvelope
	if err := c.doFirewallGroupRequest(ctx, http.MethodPost,
		fmt.Sprintf("%s%s/api/s/%s/rest/firewallgroup", c.BaseURL, c.APIPath, site),
		payload, &resp); err != nil {
		return nil, err
	}
	if err := checkV1Meta(resp.Meta); err != nil {
		return nil, err
	}
	if len(resp.Data) != 1 {
		return nil, &unifi.NotFoundError{}
	}
	return &resp.Data[0], nil
}

// GetFirewallGroup reads a firewall group by ID.
func (c *Client) GetFirewallGroup(ctx context.Context, site, id string) (*unifi.FirewallGroup, error) {
	var resp firewallGroupEnvelope
	if err := c.doFirewallGroupRequest(ctx, http.MethodGet,
		fmt.Sprintf("%s%s/api/s/%s/rest/firewallgroup/%s", c.BaseURL, c.APIPath, site, id),
		nil, &resp); err != nil {
		return nil, err
	}
	if err := checkV1Meta(resp.Meta); err != nil {
		return nil, err
	}
	if len(resp.Data) != 1 {
		return nil, &unifi.NotFoundError{}
	}
	return &resp.Data[0], nil
}

// UpdateFirewallGroup writes the full firewall group at its ID.
func (c *Client) UpdateFirewallGroup(ctx context.Context, site string, d *unifi.FirewallGroup) (*unifi.FirewallGroup, error) {
	payload := normalizeFirewallGroup(d)
	var resp firewallGroupEnvelope
	if err := c.doFirewallGroupRequest(ctx, http.MethodPut,
		fmt.Sprintf("%s%s/api/s/%s/rest/firewallgroup/%s", c.BaseURL, c.APIPath, site, d.ID),
		payload, &resp); err != nil {
		return nil, err
	}
	if err := checkV1Meta(resp.Meta); err != nil {
		return nil, err
	}
	if len(resp.Data) == 1 {
		return &resp.Data[0], nil
	}
	// No-op updates can return an empty data array; fall back to GET.
	return c.GetFirewallGroup(ctx, site, d.ID)
}

// DeleteFirewallGroup removes the firewall group by ID.
func (c *Client) DeleteFirewallGroup(ctx context.Context, site, id string) error {
	var resp firewallGroupEnvelope
	if err := c.doFirewallGroupRequest(ctx, http.MethodDelete,
		fmt.Sprintf("%s%s/api/s/%s/rest/firewallgroup/%s", c.BaseURL, c.APIPath, site, id),
		nil, &resp); err != nil {
		return err
	}
	return checkV1Meta(resp.Meta)
}

// ListFirewallGroup returns all firewall groups for the site.
func (c *Client) ListFirewallGroup(ctx context.Context, site string) ([]unifi.FirewallGroup, error) {
	var resp firewallGroupEnvelope
	if err := c.doFirewallGroupRequest(ctx, http.MethodGet,
		fmt.Sprintf("%s%s/api/s/%s/rest/firewallgroup", c.BaseURL, c.APIPath, site),
		nil, &resp); err != nil {
		return nil, err
	}
	if err := checkV1Meta(resp.Meta); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// normalizeFirewallGroup copies d and ensures GroupMembers is a non-nil slice
// so it serializes as [] rather than null. The controller treats a missing
// group_members value as "leave unchanged"; we always want to send the
// authoritative set from the plan.
func normalizeFirewallGroup(d *unifi.FirewallGroup) unifi.FirewallGroup {
	out := *d
	if out.GroupMembers == nil {
		out.GroupMembers = []string{}
	}
	return out
}

// doFirewallGroupRequest reuses the shared HTTP layer but translates 404
// responses into the local *unifi.NotFoundError. The shared doV1Request
// returns the SDK's NotFoundError type, which we are migrating away from.
func (c *Client) doFirewallGroupRequest(ctx context.Context, method, url string, body, result any) error {
	err := c.doV2Request(ctx, method, url, body, result)
	if err != nil && strings.Contains(err.Error(), "(404)") {
		return &unifi.NotFoundError{}
	}
	return err
}
