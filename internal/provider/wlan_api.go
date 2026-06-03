package provider

// Local CRUD methods for the v1 REST wlanconf and wlangroup endpoints. These
// shadow the promoted go-unifi methods on *Client and use the local
// internal/unifi types instead, removing the SDK dependency for this resource
// — see issue #157.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/alexklibisz/terrifi/internal/unifi"
)

type wlanEnvelope struct {
	Meta json.RawMessage `json:"meta"`
	Data []unifi.WLAN    `json:"data"`
}

type wlanGroupEnvelope struct {
	Meta json.RawMessage   `json:"meta"`
	Data []unifi.WLANGroup `json:"data"`
}

// CreateWLAN posts a new WLAN to /api/s/{site}/rest/wlanconf.
func (c *Client) CreateWLAN(ctx context.Context, site string, d *unifi.WLAN) (*unifi.WLAN, error) {
	payload := normalizeWLAN(d)
	var resp wlanEnvelope
	if err := c.doWLANRequest(ctx, http.MethodPost,
		fmt.Sprintf("%s%s/api/s/%s/rest/wlanconf", c.BaseURL, c.APIPath, site),
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

// GetWLAN reads a WLAN by ID.
func (c *Client) GetWLAN(ctx context.Context, site, id string) (*unifi.WLAN, error) {
	var resp wlanEnvelope
	if err := c.doWLANRequest(ctx, http.MethodGet,
		fmt.Sprintf("%s%s/api/s/%s/rest/wlanconf/%s", c.BaseURL, c.APIPath, site, id),
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

// UpdateWLAN writes the full WLAN at its ID.
func (c *Client) UpdateWLAN(ctx context.Context, site string, d *unifi.WLAN) (*unifi.WLAN, error) {
	payload := normalizeWLAN(d)
	var resp wlanEnvelope
	if err := c.doWLANRequest(ctx, http.MethodPut,
		fmt.Sprintf("%s%s/api/s/%s/rest/wlanconf/%s", c.BaseURL, c.APIPath, site, d.ID),
		payload, &resp); err != nil {
		return nil, err
	}
	if err := checkV1Meta(resp.Meta); err != nil {
		return nil, err
	}
	if len(resp.Data) == 1 {
		return &resp.Data[0], nil
	}
	return c.GetWLAN(ctx, site, d.ID)
}

// DeleteWLAN removes the WLAN by ID.
func (c *Client) DeleteWLAN(ctx context.Context, site, id string) error {
	var resp wlanEnvelope
	if err := c.doWLANRequest(ctx, http.MethodDelete,
		fmt.Sprintf("%s%s/api/s/%s/rest/wlanconf/%s", c.BaseURL, c.APIPath, site, id),
		nil, &resp); err != nil {
		return err
	}
	return checkV1Meta(resp.Meta)
}

// ListWLAN returns all WLANs for the site.
func (c *Client) ListWLAN(ctx context.Context, site string) ([]unifi.WLAN, error) {
	var resp wlanEnvelope
	if err := c.doWLANRequest(ctx, http.MethodGet,
		fmt.Sprintf("%s%s/api/s/%s/rest/wlanconf", c.BaseURL, c.APIPath, site),
		nil, &resp); err != nil {
		return nil, err
	}
	if err := checkV1Meta(resp.Meta); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// ListWLANGroup returns all WLAN groups for the site.
func (c *Client) ListWLANGroup(ctx context.Context, site string) ([]unifi.WLANGroup, error) {
	var resp wlanGroupEnvelope
	if err := c.doWLANRequest(ctx, http.MethodGet,
		fmt.Sprintf("%s%s/api/s/%s/rest/wlangroup", c.BaseURL, c.APIPath, site),
		nil, &resp); err != nil {
		return nil, err
	}
	if err := checkV1Meta(resp.Meta); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// normalizeWLAN ensures ScheduleWithDuration is a non-nil slice so it
// serializes as [] rather than null. The controller treats a missing
// schedule_with_duration as "leave unchanged"; we always want to send the
// authoritative set from the plan.
func normalizeWLAN(d *unifi.WLAN) unifi.WLAN {
	out := *d
	if out.ScheduleWithDuration == nil {
		out.ScheduleWithDuration = []unifi.WLANScheduleWithDuration{}
	}
	return out
}

// doWLANRequest reuses the shared HTTP layer but translates 404 responses
// into the local *unifi.NotFoundError so resource Read() can distinguish
// "deleted externally" from real errors.
func (c *Client) doWLANRequest(ctx context.Context, method, url string, body, result any) error {
	err := c.doV2Request(ctx, method, url, body, result)
	if err != nil && strings.Contains(err.Error(), "(404)") {
		return &unifi.NotFoundError{}
	}
	return err
}
