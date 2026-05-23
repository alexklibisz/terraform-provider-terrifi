package provider

// TODO(go-unifi): The SDK declares several Network fields (e.g.
// IPV6RaPreferredLifetime) as *int64, but the controller emits them as JSON
// strings on some sites, so unmarshal fails with: "unable to unmarshal alias:
// json: cannot unmarshal string into Go struct field .Alias.<field> of type
// int64". This file bypasses the SDK's listNetwork/getNetwork (which use the
// embedded ApiClient's do()) and pre-processes the raw JSON to coerce known
// string-encoded numeric fields back into JSON numbers before decoding into
// unifi.Network.
// Fix needed in SDK: affected fields should accept either a string or a number
// on the wire (e.g. via a custom UnmarshalJSON using json.Number).

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"

	"github.com/ubiquiti-community/go-unifi/unifi"
)

// ipv6RaPreferredLifetimeStringRE matches `"ipv6_ra_preferred_lifetime": "<digits>"`
// so we can strip the quotes before handing the bytes to the SDK's
// Network.UnmarshalJSON, which expects a JSON number for the underlying *int64.
var ipv6RaPreferredLifetimeStringRE = regexp.MustCompile(`"ipv6_ra_preferred_lifetime"\s*:\s*"(-?\d+)"`)

// ipv6RaPreferredLifetimeEmptyRE matches `"ipv6_ra_preferred_lifetime": ""` so
// we can replace it with null (the SDK field is *int64, so null is the correct
// representation of "unset").
var ipv6RaPreferredLifetimeEmptyRE = regexp.MustCompile(`"ipv6_ra_preferred_lifetime"\s*:\s*""`)

// fixNetworkBytes coerces JSON-string-encoded numeric Network fields back to
// JSON numbers so the SDK's UnmarshalJSON succeeds. See the file-level TODO
// for details.
func fixNetworkBytes(b []byte) []byte {
	b = ipv6RaPreferredLifetimeEmptyRE.ReplaceAll(b, []byte(`"ipv6_ra_preferred_lifetime":null`))
	b = ipv6RaPreferredLifetimeStringRE.ReplaceAll(b, []byte(`"ipv6_ra_preferred_lifetime":$1`))
	return b
}

// networkListResponse is the v1 rest/networkconf envelope. We unmarshal into
// json.RawMessage first so we can patch the bytes before decoding into
// unifi.Network. See fixNetworkBytes.
type networkListResponse struct {
	Meta struct {
		RC  string `json:"rc"`
		Msg string `json:"msg,omitempty"`
	} `json:"meta"`
	Data []json.RawMessage `json:"data"`
}

// fetchNetworkList performs a GET against the v1 rest/networkconf endpoint,
// applies the field coercions, and decodes each entry into unifi.Network.
// When suffix is non-empty it is appended as `/<suffix>` (used to fetch a
// single network by ID).
func (c *Client) fetchNetworkList(ctx context.Context, site, suffix string) ([]unifi.Network, error) {
	url := fmt.Sprintf("%s%s/api/s/%s/rest/networkconf", c.BaseURL, c.APIPath, site)
	if suffix != "" {
		url += "/" + suffix
	}

	var raw json.RawMessage
	if err := c.doV2Request(ctx, http.MethodGet, url, nil, &raw); err != nil {
		return nil, err
	}

	var envelope networkListResponse
	if err := json.Unmarshal(fixNetworkBytes(raw), &envelope); err != nil {
		return nil, fmt.Errorf("decoding network envelope: %w", err)
	}

	if envelope.Meta.RC != "" && envelope.Meta.RC != "ok" {
		return nil, fmt.Errorf("controller returned rc=%s msg=%s", envelope.Meta.RC, envelope.Meta.Msg)
	}

	networks := make([]unifi.Network, 0, len(envelope.Data))
	for i, item := range envelope.Data {
		var n unifi.Network
		if err := json.Unmarshal(item, &n); err != nil {
			return nil, fmt.Errorf("decoding network[%d]: %w", i, err)
		}
		networks = append(networks, n)
	}
	return networks, nil
}

// ListNetwork returns all networks for a site, working around the SDK's
// unmarshal bugs for string-encoded numeric fields. Matches the SDK signature
// so it overrides the embedded ApiClient.ListNetwork via method promotion.
func (c *Client) ListNetwork(ctx context.Context, site string, params ...[]struct {
	key string
	val string
}) ([]unifi.Network, error) {
	return c.fetchNetworkList(ctx, site, "")
}

// GetNetwork fetches a network by ID, working around the SDK's unmarshal bugs
// for string-encoded numeric fields. Matches the SDK signature so it overrides
// the embedded ApiClient.GetNetwork via method promotion.
func (c *Client) GetNetwork(ctx context.Context, site, id string) (*unifi.Network, error) {
	networks, err := c.fetchNetworkList(ctx, site, id)
	if err != nil {
		return nil, err
	}
	if len(networks) != 1 {
		return nil, &unifi.NotFoundError{}
	}
	n := networks[0]
	return &n, nil
}

// sendNetwork issues a write (POST or PUT) against the v1 rest/networkconf
// endpoint with the given body and returns the decoded result, applying the
// same field coercions as the read paths. Shared by CreateNetwork and
// UpdateNetwork.
func (c *Client) sendNetwork(ctx context.Context, method, site, suffix string, body *unifi.Network) (*unifi.Network, error) {
	url := fmt.Sprintf("%s%s/api/s/%s/rest/networkconf", c.BaseURL, c.APIPath, site)
	if suffix != "" {
		url += "/" + suffix
	}

	var raw json.RawMessage
	if err := c.doV2Request(ctx, method, url, body, &raw); err != nil {
		return nil, err
	}

	var envelope networkListResponse
	if err := json.Unmarshal(fixNetworkBytes(raw), &envelope); err != nil {
		return nil, fmt.Errorf("decoding network envelope: %w", err)
	}

	if envelope.Meta.RC != "" && envelope.Meta.RC != "ok" {
		return nil, fmt.Errorf("controller returned rc=%s msg=%s", envelope.Meta.RC, envelope.Meta.Msg)
	}

	// UDM SE returns an empty data array on successful PUT; fall back to GET in
	// that case to mirror the SDK's updateNetwork behavior.
	if len(envelope.Data) == 0 && method == http.MethodPut {
		return c.GetNetwork(ctx, site, body.ID)
	}

	if len(envelope.Data) != 1 {
		return nil, &unifi.NotFoundError{}
	}

	var n unifi.Network
	if err := json.Unmarshal(envelope.Data[0], &n); err != nil {
		return nil, fmt.Errorf("decoding network: %w", err)
	}
	return &n, nil
}

// CreateNetwork creates a network, working around the SDK's unmarshal bugs in
// the response payload.
func (c *Client) CreateNetwork(ctx context.Context, site string, d *unifi.Network) (*unifi.Network, error) {
	return c.sendNetwork(ctx, http.MethodPost, site, "", d)
}

// UpdateNetwork updates a network, working around the SDK's unmarshal bugs in
// the response payload.
func (c *Client) UpdateNetwork(ctx context.Context, site string, d *unifi.Network) (*unifi.Network, error) {
	return c.sendNetwork(ctx, http.MethodPut, site, d.ID, d)
}
