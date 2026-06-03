package provider

// Local CRUD methods for the v1 REST networkconf endpoint. These shadow the
// promoted go-unifi methods on *Client and use the local internal/unifi types
// instead, removing the SDK dependency for this resource — see issue #157.
//
// Replaces the previous SDK-bypass layer that pre-processed raw JSON to coerce
// ipv6_ra_preferred_lifetime (issue #154). The local unifi.Network type does
// not declare that field at all, so the coercion is no longer needed: unknown
// wire fields are silently ignored by json.Unmarshal.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/alexklibisz/terrifi/internal/unifi"
)

type networkEnvelope struct {
	Meta json.RawMessage `json:"meta"`
	Data []unifi.Network `json:"data"`
}

// CreateNetwork posts a new network to /api/s/{site}/rest/networkconf.
func (c *Client) CreateNetwork(ctx context.Context, site string, d *unifi.Network) (*unifi.Network, error) {
	var resp networkEnvelope
	if err := c.doNetworkRequest(ctx, http.MethodPost,
		fmt.Sprintf("%s%s/api/s/%s/rest/networkconf", c.BaseURL, c.APIPath, site),
		d, &resp); err != nil {
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

// GetNetwork reads a network by ID.
func (c *Client) GetNetwork(ctx context.Context, site, id string) (*unifi.Network, error) {
	var resp networkEnvelope
	if err := c.doNetworkRequest(ctx, http.MethodGet,
		fmt.Sprintf("%s%s/api/s/%s/rest/networkconf/%s", c.BaseURL, c.APIPath, site, id),
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

// UpdateNetwork writes the full network at its ID.
func (c *Client) UpdateNetwork(ctx context.Context, site string, d *unifi.Network) (*unifi.Network, error) {
	var resp networkEnvelope
	if err := c.doNetworkRequest(ctx, http.MethodPut,
		fmt.Sprintf("%s%s/api/s/%s/rest/networkconf/%s", c.BaseURL, c.APIPath, site, d.ID),
		d, &resp); err != nil {
		return nil, err
	}
	if err := checkV1Meta(resp.Meta); err != nil {
		return nil, err
	}
	if len(resp.Data) == 1 {
		return &resp.Data[0], nil
	}
	// UDM SE returns an empty data array on successful PUT; fall back to GET.
	return c.GetNetwork(ctx, site, d.ID)
}

// DeleteNetwork removes the network by ID. The v1 endpoint requires the network
// name in the request body alongside the URL ID — matches the SDK signature.
func (c *Client) DeleteNetwork(ctx context.Context, site, id, name string) error {
	var resp networkEnvelope
	body := struct {
		Name string `json:"name"`
	}{Name: name}
	if err := c.doNetworkRequest(ctx, http.MethodDelete,
		fmt.Sprintf("%s%s/api/s/%s/rest/networkconf/%s", c.BaseURL, c.APIPath, site, id),
		body, &resp); err != nil {
		return err
	}
	return checkV1Meta(resp.Meta)
}

// ListNetwork returns all networks for a site. Matches the SDK signature
// (variadic params) so it overrides the embedded ApiClient.ListNetwork via
// method promotion.
func (c *Client) ListNetwork(ctx context.Context, site string, params ...[]struct {
	key string
	val string
}) ([]unifi.Network, error) {
	var resp networkEnvelope
	if err := c.doNetworkRequest(ctx, http.MethodGet,
		fmt.Sprintf("%s%s/api/s/%s/rest/networkconf", c.BaseURL, c.APIPath, site),
		nil, &resp); err != nil {
		return nil, err
	}
	if err := checkV1Meta(resp.Meta); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// doNetworkRequest reuses the shared HTTP layer but translates 404 responses
// into the local *unifi.NotFoundError so resource Read() can distinguish
// "deleted externally" from real errors.
func (c *Client) doNetworkRequest(ctx context.Context, method, url string, body, result any) error {
	err := c.doV2Request(ctx, method, url, body, result)
	if err != nil && strings.Contains(err.Error(), "(404)") {
		return &unifi.NotFoundError{}
	}
	return err
}
