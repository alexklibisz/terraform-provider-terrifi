package unifi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
)

var ipv6RaPreferredLifetimeStringRE = regexp.MustCompile(`"ipv6_ra_preferred_lifetime"\s*:\s*"(-?\d+)"`)

var ipv6RaPreferredLifetimeEmptyRE = regexp.MustCompile(`"ipv6_ra_preferred_lifetime"\s*:\s*""`)

func fixNetworkBytes(b []byte) []byte {
	b = ipv6RaPreferredLifetimeEmptyRE.ReplaceAll(b, []byte(`"ipv6_ra_preferred_lifetime":null`))
	b = ipv6RaPreferredLifetimeStringRE.ReplaceAll(b, []byte(`"ipv6_ra_preferred_lifetime":$1`))
	return b
}

// networkListResponse is the v1 rest/networkconf envelope. We unmarshal into
// json.RawMessage first so we can patch the bytes before decoding into
// Network. See fixNetworkBytes.
type networkListResponse struct {
	Meta struct {
		RC  string `json:"rc"`
		Msg string `json:"msg,omitempty"`
	} `json:"meta"`
	Data []json.RawMessage `json:"data"`
}

// fetchNetworkList performs a GET against the v1 rest/networkconf endpoint,
// applies the field coercions, and decodes each entry into Network.
// When suffix is non-empty it is appended as `/<suffix>` (used to fetch a
// single network by ID).
func (c *APIClient) fetchNetworkList(ctx context.Context, site, suffix string) ([]Network, error) {
	var raw json.RawMessage
	if err := c.Do(ctx, http.MethodGet, c.NetworkURL(site, suffix), nil, &raw); err != nil {
		return nil, err
	}

	var envelope networkListResponse
	if err := json.Unmarshal(fixNetworkBytes(raw), &envelope); err != nil {
		return nil, fmt.Errorf("decoding network envelope: %w", err)
	}

	if envelope.Meta.RC != "" && envelope.Meta.RC != "ok" {
		return nil, fmt.Errorf("controller returned rc=%s msg=%s", envelope.Meta.RC, envelope.Meta.Msg)
	}

	networks := make([]Network, 0, len(envelope.Data))
	for i, item := range envelope.Data {
		var n Network
		if err := json.Unmarshal(item, &n); err != nil {
			return nil, fmt.Errorf("decoding network[%d]: %w", i, err)
		}
		networks = append(networks, n)
	}
	return networks, nil
}

func (c *APIClient) ListNetwork(ctx context.Context, site string, params ...[]struct {
	key string
	val string
}) ([]Network, error) {
	return c.fetchNetworkList(ctx, site, "")
}

func (c *APIClient) GetNetwork(ctx context.Context, site, id string) (*Network, error) {
	networks, err := c.fetchNetworkList(ctx, site, id)
	if err != nil {
		return nil, err
	}
	if len(networks) != 1 {
		return nil, &NotFoundError{}
	}
	n := networks[0]
	return &n, nil
}

// sendNetwork issues a write (POST or PUT) against the v1 rest/networkconf
// endpoint with the given body and returns the decoded result, applying the
// same field coercions as the read paths. Shared by CreateNetwork and
// UpdateNetwork.
func (c *APIClient) sendNetwork(ctx context.Context, method, site, suffix string, body any) (*Network, error) {
	url := c.NetworkURL(site, suffix)

	var raw json.RawMessage
	if err := c.Do(ctx, method, url, body, &raw); err != nil {
		return nil, err
	}

	var envelope networkListResponse
	if err := json.Unmarshal(fixNetworkBytes(raw), &envelope); err != nil {
		return nil, fmt.Errorf("decoding network envelope: %w", err)
	}

	if envelope.Meta.RC != "" && envelope.Meta.RC != "ok" {
		return nil, fmt.Errorf("controller returned rc=%s msg=%s", envelope.Meta.RC, envelope.Meta.Msg)
	}

	if len(envelope.Data) == 0 && method == http.MethodPut {
		return c.GetNetwork(ctx, site, suffix)
	}

	if len(envelope.Data) != 1 {
		return nil, &NotFoundError{}
	}

	var n Network
	if err := json.Unmarshal(envelope.Data[0], &n); err != nil {
		return nil, fmt.Errorf("decoding network: %w", err)
	}
	return &n, nil
}

func (c *APIClient) CreateNetwork(ctx context.Context, site string, d *Network) (*Network, error) {
	return c.sendNetwork(ctx, http.MethodPost, site, "", d)
}

func (c *APIClient) UpdateNetwork(ctx context.Context, site string, d *Network) (*Network, error) {
	return c.sendNetwork(ctx, http.MethodPut, site, d.ID, d)
}

// NetworkURL returns the v1 networkconf URL, for one network if id is set.
func (c *APIClient) NetworkURL(site, id string) string {
	url := fmt.Sprintf("%s%s/api/s/%s/rest/networkconf", c.BaseURL, c.APIPath, site)
	if id != "" {
		url += "/" + id
	}
	return url
}

func (c *APIClient) UpdateNetworkManaged(ctx context.Context, site, id string, managed map[string]any) (*Network, error) {
	// The PUT below invalidates the cache anyway; drop it now so the read is
	// fresh and changes made in the UI since an earlier read are kept.
	if c.cache != nil {
		c.cache.invalidateAll()
	}

	var current struct {
		Meta struct {
			RC  string `json:"rc"`
			Msg string `json:"msg,omitempty"`
		} `json:"meta"`
		Data []map[string]json.RawMessage `json:"data"`
	}
	if err := c.Do(ctx, http.MethodGet, c.NetworkURL(site, id), nil, &current); err != nil {
		return nil, err
	}
	if current.Meta.RC != "" && current.Meta.RC != "ok" {
		return nil, fmt.Errorf("controller returned rc=%s msg=%s", current.Meta.RC, current.Meta.Msg)
	}
	if len(current.Data) != 1 {
		return nil, &NotFoundError{}
	}

	payload, err := MergeNetwork(current.Data[0], managed)
	if err != nil {
		return nil, err
	}
	return c.sendNetwork(ctx, http.MethodPut, site, id, payload)
}

// MergeNetwork overlays managed onto the raw stored network.
func MergeNetwork(existing map[string]json.RawMessage, managed map[string]any) (map[string]json.RawMessage, error) {
	for k, v := range managed {
		b, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("marshaling network field %s: %w", k, err)
		}
		existing[k] = b
	}
	return existing, nil
}

func (c *APIClient) DeleteNetwork(ctx context.Context, site, id, name string) error {
	var response struct {
		Meta json.RawMessage `json:"meta"`
	}
	if err := c.doV1Request(ctx, http.MethodDelete, c.NetworkURL(site, id), map[string]string{"name": name}, &response); err != nil {
		return err
	}
	return CheckV1Meta(response.Meta)
}
