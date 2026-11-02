package unifi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type portProfileEnvelope struct {
	Meta json.RawMessage `json:"meta"`
	Data []PortProfile   `json:"data"`
}

type portProfileRawEnvelope struct {
	Meta json.RawMessage              `json:"meta"`
	Data []map[string]json.RawMessage `json:"data"`
}

func (c *APIClient) PortProfileURL(site, id string) string {
	url := fmt.Sprintf("%s%s/api/s/%s/rest/portconf", c.BaseURL, c.APIPath, site)
	if id != "" {
		url += "/" + id
	}
	return url
}

func (c *APIClient) DoREST(ctx context.Context, method, url string, body, result any) error {
	err := c.Do(ctx, method, url, body, result)
	if err != nil && strings.Contains(err.Error(), "(404)") {
		return &NotFoundError{}
	}
	return err
}

// NormalizePortProfile returns a copy with nil list fields replaced by empty
// slices, so the controller receives an authoritative empty array when the
// user removes every element.
func NormalizePortProfile(d *PortProfile) PortProfile {
	p := *d
	if p.ExcludedNetworkIDs == nil {
		p.ExcludedNetworkIDs = []string{}
	}
	return p
}

// PortProfileUnmanagedDefaults returns the UI defaults (UniFi OS 5.1 /
// Network 10.x) of settings the provider doesn't manage. Create sends them so
// that a profile created by Terraform matches one created in the UI: the
// provider forces the Advanced mode to manual, which makes these settings
// apply, so they must not be left to whatever the controller stores for a
// missing key.
func PortProfileUnmanagedDefaults() map[string]json.RawMessage {
	return map[string]json.RawMessage{
		"autoneg":                        json.RawMessage(`true`),
		"full_duplex":                    json.RawMessage(`false`),
		"voice_networkconf_id":           json.RawMessage(`""`),
		"egress_rate_limit_kbps_enabled": json.RawMessage(`false`),
		"stp_uplink":                     json.RawMessage(`false`),
		"port_security_enabled":          json.RawMessage(`false`),
		"port_security_mac_address":      json.RawMessage(`[]`),
		"dot1x_ctrl":                     json.RawMessage(`"force_authorized"`),
	}
}

// ListPortProfiles returns every port profile on the site.
func (c *APIClient) ListPortProfiles(ctx context.Context, site string) ([]PortProfile, error) {
	return c.listPortProfiles(ctx, site, true)
}

func (c *APIClient) listPortProfiles(ctx context.Context, site string, useCached bool) ([]PortProfile, error) {
	var resp portProfileEnvelope
	if err := c.doRequest(ctx, http.MethodGet, c.PortProfileURL(site, ""), nil, &resp, useCached); err != nil {
		if strings.Contains(err.Error(), "(404)") {
			return nil, &NotFoundError{}
		}
		return nil, err
	}
	if err := CheckV1Meta(resp.Meta); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// GetPortProfile reads a port profile by ID. It looks the ID up in the full
// list rather than GETting /rest/portconf/{id}, so a missing profile is always
// reported as NotFoundError regardless of how the controller signals an
// unknown ID on the single-object endpoint.
func (c *APIClient) GetPortProfile(ctx context.Context, site, id string) (*PortProfile, error) {
	return c.GetPortProfileWithCache(ctx, site, id, true)
}

func (c *APIClient) GetPortProfileWithCache(ctx context.Context, site, id string, useCached bool) (*PortProfile, error) {
	profiles, err := c.listPortProfiles(ctx, site, useCached)
	if err != nil {
		return nil, err
	}
	for i := range profiles {
		if profiles[i].ID == id {
			return &profiles[i], nil
		}
	}
	return nil, &NotFoundError{}
}

// CreatePortProfile posts a new port profile, with the UI defaults for every
// setting the provider doesn't manage.
func (c *APIClient) CreatePortProfile(ctx context.Context, site string, d *PortProfile) (*PortProfile, error) {
	payload, err := MergePortProfile(PortProfileUnmanagedDefaults(), d)
	if err != nil {
		return nil, err
	}
	var resp portProfileEnvelope
	if err := c.DoREST(ctx, http.MethodPost, c.PortProfileURL(site, ""), payload, &resp); err != nil {
		return nil, err
	}
	if err := CheckV1Meta(resp.Meta); err != nil {
		return nil, err
	}
	if len(resp.Data) != 1 {
		return nil, fmt.Errorf("creating port profile: controller returned %d objects, expected 1", len(resp.Data))
	}
	return &resp.Data[0], nil
}

// UpdatePortProfile replaces the managed fields of an existing port profile.
// See the file-level comment for the read-modify-write rationale.
func (c *APIClient) UpdatePortProfile(ctx context.Context, site string, d *PortProfile) (*PortProfile, error) {
	var current portProfileRawEnvelope
	if err := c.DoREST(ctx, http.MethodGet, c.PortProfileURL(site, ""), nil, &current); err != nil {
		return nil, err
	}
	if err := CheckV1Meta(current.Meta); err != nil {
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
		return nil, &NotFoundError{}
	}

	payload, err := MergePortProfile(existing, d)
	if err != nil {
		return nil, err
	}

	var resp portProfileEnvelope
	if err := c.DoREST(ctx, http.MethodPut, c.PortProfileURL(site, d.ID), payload, &resp); err != nil {
		return nil, err
	}
	if err := CheckV1Meta(resp.Meta); err != nil {
		return nil, err
	}
	if len(resp.Data) == 1 {
		return &resp.Data[0], nil
	}
	// The controller can return an empty data array for no-op updates. Fall
	// back to a read instead of treating that as an error.
	return c.GetPortProfile(ctx, site, d.ID)
}

// MergePortProfile overlays the fields of d onto the raw controller object.
// Fields d leaves out (nil pointers, empty strings) keep their stored value;
// settings that must be cleared are always sent with an explicit value
// (false, "", []) instead.
func MergePortProfile(existing map[string]json.RawMessage, d *PortProfile) (map[string]json.RawMessage, error) {
	b, err := json.Marshal(NormalizePortProfile(d))
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
func (c *APIClient) DeletePortProfile(ctx context.Context, site, id string) error {
	var resp portProfileEnvelope
	err := c.DoREST(ctx, http.MethodDelete, c.PortProfileURL(site, id), struct{}{}, &resp)
	var nf *NotFoundError
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
	return CheckV1Meta(resp.Meta)
}

// ListLANNetworkIDs returns the IDs of the site's LAN networks: the set the
// controller's excluded_networkconf_ids deny-list is drawn from. Only
// "corporate" networks have been observed in exclude lists; "guest" and
// "vlan-only" are left out until verified (a documented limitation). It
// decodes only _id and purpose, so it is independent of the full network type.
func (c *APIClient) ListLANNetworkIDs(ctx context.Context, site string) ([]string, error) {
	var resp struct {
		Meta json.RawMessage `json:"meta"`
		Data []struct {
			ID      string `json:"_id"`
			Purpose string `json:"purpose"`
		} `json:"data"`
	}
	url := fmt.Sprintf("%s%s/api/s/%s/rest/networkconf", c.BaseURL, c.APIPath, site)
	if err := c.DoREST(ctx, http.MethodGet, url, nil, &resp); err != nil {
		return nil, err
	}
	if err := CheckV1Meta(resp.Meta); err != nil {
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
