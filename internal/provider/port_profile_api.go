package provider

// Local CRUD methods for the v1 REST port profile endpoint
// (/api/s/{site}/rest/portconf), using the local internal/unifi types rather
// than the go-unifi SDK — see issue #157.
//
// Update is a read-modify-write on the raw controller object: the fields the
// provider sets are overlaid, and every other key the controller returned
// (qos_profile, priority queues, multicast router networks, ...) is sent back
// untouched. The controller
// merges PUTs (keys left out keep their stored value), but sending the full
// object keeps the update correct even if that changes.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/alexklibisz/terrifi/internal/unifi"
)

type portProfileEnvelope struct {
	Meta json.RawMessage     `json:"meta"`
	Data []unifi.PortProfile `json:"data"`
}

type portProfileRawEnvelope struct {
	Meta json.RawMessage              `json:"meta"`
	Data []map[string]json.RawMessage `json:"data"`
}

func (c *Client) portProfileURL(site, id string) string {
	url := fmt.Sprintf("%s%s/api/s/%s/rest/portconf", c.BaseURL, c.APIPath, site)
	if id != "" {
		url += "/" + id
	}
	return url
}

// doPortProfileRequest wraps doV2Request and maps HTTP 404 to the local
// unifi.NotFoundError. (doV1Request maps it to the go-unifi SDK's type.)
func (c *Client) doPortProfileRequest(ctx context.Context, method, url string, body, result any) error {
	err := c.doV2Request(ctx, method, url, body, result)
	if err != nil && strings.Contains(err.Error(), "(404)") {
		return &unifi.NotFoundError{}
	}
	return err
}

// normalizePortProfile returns a copy with nil list fields replaced by empty
// slices, so the controller receives an authoritative empty array when the
// user removes every element.
func normalizePortProfile(d *unifi.PortProfile) unifi.PortProfile {
	p := *d
	if p.ExcludedNetworkIDs == nil {
		p.ExcludedNetworkIDs = []string{}
	}
	if p.PortSecurityMACAddress == nil {
		p.PortSecurityMACAddress = []string{}
	}
	return p
}

// ListPortProfiles returns every port profile on the site.
func (c *Client) ListPortProfiles(ctx context.Context, site string) ([]unifi.PortProfile, error) {
	var resp portProfileEnvelope
	if err := c.doPortProfileRequest(ctx, http.MethodGet, c.portProfileURL(site, ""), nil, &resp); err != nil {
		return nil, err
	}
	if err := checkV1Meta(resp.Meta); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// GetPortProfile reads a port profile by ID. It looks the ID up in the full
// list rather than GETting /rest/portconf/{id}, so a missing profile is always
// reported as unifi.NotFoundError regardless of how the controller signals an
// unknown ID on the single-object endpoint.
func (c *Client) GetPortProfile(ctx context.Context, site, id string) (*unifi.PortProfile, error) {
	profiles, err := c.ListPortProfiles(ctx, site)
	if err != nil {
		return nil, err
	}
	for i := range profiles {
		if profiles[i].ID == id {
			return &profiles[i], nil
		}
	}
	return nil, &unifi.NotFoundError{}
}

// CreatePortProfile posts a new port profile.
func (c *Client) CreatePortProfile(ctx context.Context, site string, d *unifi.PortProfile) (*unifi.PortProfile, error) {
	payload := normalizePortProfile(d)
	var resp portProfileEnvelope
	if err := c.doPortProfileRequest(ctx, http.MethodPost, c.portProfileURL(site, ""), payload, &resp); err != nil {
		return nil, err
	}
	if err := checkV1Meta(resp.Meta); err != nil {
		return nil, err
	}
	if len(resp.Data) != 1 {
		return nil, fmt.Errorf("creating port profile: controller returned %d objects, expected 1", len(resp.Data))
	}
	return &resp.Data[0], nil
}

// UpdatePortProfile replaces the managed fields of an existing port profile.
// See the file-level comment for the read-modify-write rationale.
func (c *Client) UpdatePortProfile(ctx context.Context, site string, d *unifi.PortProfile) (*unifi.PortProfile, error) {
	var current portProfileRawEnvelope
	if err := c.doPortProfileRequest(ctx, http.MethodGet, c.portProfileURL(site, ""), nil, &current); err != nil {
		return nil, err
	}
	if err := checkV1Meta(current.Meta); err != nil {
		return nil, err
	}

	var existing map[string]json.RawMessage
	for _, obj := range current.Data {
		var id string
		if err := json.Unmarshal(obj["_id"], &id); err == nil && id == d.ID {
			existing = obj
			break
		}
	}
	if existing == nil {
		return nil, &unifi.NotFoundError{}
	}

	payload, err := mergePortProfile(existing, d)
	if err != nil {
		return nil, err
	}

	var resp portProfileEnvelope
	if err := c.doPortProfileRequest(ctx, http.MethodPut, c.portProfileURL(site, d.ID), payload, &resp); err != nil {
		return nil, err
	}
	if err := checkV1Meta(resp.Meta); err != nil {
		return nil, err
	}
	if len(resp.Data) == 1 {
		return &resp.Data[0], nil
	}
	// The controller can return an empty data array for no-op updates. Fall
	// back to a read instead of treating that as an error.
	return c.GetPortProfile(ctx, site, d.ID)
}

// mergePortProfile overlays the fields of d onto the raw controller object.
// Fields d leaves out (nil pointers, empty strings) keep their stored value;
// settings that must be cleared are always sent with an explicit value
// (false, "", []) instead.
func mergePortProfile(existing map[string]json.RawMessage, d *unifi.PortProfile) (map[string]json.RawMessage, error) {
	b, err := json.Marshal(normalizePortProfile(d))
	if err != nil {
		return nil, fmt.Errorf("marshaling port profile: %w", err)
	}
	if err := json.Unmarshal(b, &existing); err != nil {
		return nil, fmt.Errorf("unmarshaling port profile: %w", err)
	}
	return existing, nil
}

// DeletePortProfile deletes a port profile. A profile that is already gone is
// treated as deleted.
func (c *Client) DeletePortProfile(ctx context.Context, site, id string) error {
	var resp portProfileEnvelope
	err := c.doPortProfileRequest(ctx, http.MethodDelete, c.portProfileURL(site, id), struct{}{}, &resp)
	var nf *unifi.NotFoundError
	if errors.As(err, &nf) {
		return nil
	}
	if err != nil && strings.Contains(err.Error(), "api.err.ObjectReferredByDevice") {
		return fmt.Errorf("the port profile is still assigned to a device port (the response names the device); "+
			"remove it from terrifi_device_ports or the UniFi UI first: %w", err)
	}
	if err != nil {
		return err
	}
	return checkV1Meta(resp.Meta)
}

// ListLANNetworkIDs returns the IDs of the site's LAN networks: the set the
// controller's excluded_networkconf_ids deny-list is drawn from. Only
// "corporate" networks have been observed in exclude lists; "guest" and
// "vlan-only" are left out until verified (a documented limitation). It
// decodes only _id and purpose, so it is independent of the full network type.
func (c *Client) ListLANNetworkIDs(ctx context.Context, site string) ([]string, error) {
	var resp struct {
		Meta json.RawMessage `json:"meta"`
		Data []struct {
			ID      string `json:"_id"`
			Purpose string `json:"purpose"`
		} `json:"data"`
	}
	url := fmt.Sprintf("%s%s/api/s/%s/rest/networkconf", c.BaseURL, c.APIPath, site)
	if err := c.doPortProfileRequest(ctx, http.MethodGet, url, nil, &resp); err != nil {
		return nil, err
	}
	if err := checkV1Meta(resp.Meta); err != nil {
		return nil, err
	}
	var ids []string
	for _, n := range resp.Data {
		if n.Purpose == "corporate" {
			ids = append(ids, n.ID)
		}
	}
	return ids, nil
}
