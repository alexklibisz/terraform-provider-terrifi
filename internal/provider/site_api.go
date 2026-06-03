package provider

// Local list helpers for sites, AP groups, and user groups. These shadow the
// promoted go-unifi methods on *Client and use the local internal/unifi types
// instead, removing the SDK dependency for the provider's small set of
// "look up the default group" call sites (wlan_resource.go) and the CLI's
// check-connection command — see issue #157.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/alexklibisz/terrifi/internal/unifi"
)

// ListSites returns every site visible to the authenticated user. Hits the
// /api/self/sites endpoint which is not site-scoped.
func (c *Client) ListSites(ctx context.Context) ([]unifi.Site, error) {
	var resp struct {
		Meta json.RawMessage `json:"meta"`
		Data []unifi.Site    `json:"data"`
	}
	if err := c.doV2Request(ctx, http.MethodGet,
		fmt.Sprintf("%s%s/api/self/sites", c.BaseURL, c.APIPath),
		nil, &resp); err != nil {
		return nil, err
	}
	if err := checkV1Meta(resp.Meta); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// ListAPGroup returns all AP groups for the site. v2 endpoint, no envelope.
func (c *Client) ListAPGroup(ctx context.Context, site string) ([]unifi.APGroup, error) {
	var result []unifi.APGroup
	if err := c.doV2Request(ctx, http.MethodGet,
		fmt.Sprintf("%s%s/v2/api/site/%s/apgroups", c.BaseURL, c.APIPath, site),
		nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ListClientGroup returns all user/QoS groups for the site. v1 envelope.
// (The Go method name reflects the resource layer's "client group" terminology;
// the JSON endpoint path is /rest/usergroup.)
func (c *Client) ListClientGroup(ctx context.Context, site string) ([]unifi.ClientGroup, error) {
	var resp struct {
		Meta json.RawMessage     `json:"meta"`
		Data []unifi.ClientGroup `json:"data"`
	}
	if err := c.doV2Request(ctx, http.MethodGet,
		fmt.Sprintf("%s%s/api/s/%s/rest/usergroup", c.BaseURL, c.APIPath, site),
		nil, &resp); err != nil {
		return nil, err
	}
	if err := checkV1Meta(resp.Meta); err != nil {
		return nil, err
	}
	return resp.Data, nil
}
