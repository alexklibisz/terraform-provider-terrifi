package unifi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// ListDevicePorts reads the ports of every device on the site, or of the
// device with the given MAC.
func (c *APIClient) ListDevicePorts(ctx context.Context, site, mac string) ([]DevicePorts, error) {
	var resp struct {
		Meta json.RawMessage `json:"meta"`
		Data []DevicePorts   `json:"data"`
	}
	url := fmt.Sprintf("%s%s/api/s/%s/stat/device", c.BaseURL, c.APIPath, site)
	if mac != "" {
		url += "/" + strings.ToLower(mac)
	}
	if err := c.doRequest(ctx, http.MethodGet, url, nil, &resp, false); err != nil {
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

// GetDevicePorts reads a device's ports by MAC.
func (c *APIClient) GetDevicePorts(ctx context.Context, site, mac string) (*DevicePorts, error) {
	devices, err := c.ListDevicePorts(ctx, site, mac)
	if err != nil {
		return nil, err
	}
	if len(devices) != 1 {
		return nil, &NotFoundError{}
	}
	return &devices[0], nil
}

// PutPortOverrides replaces a device's port_overrides array. Only that field
// is sent; the controller leaves the rest of the device as it is.
func (c *APIClient) PutPortOverrides(ctx context.Context, site, deviceID string, overrides []map[string]json.RawMessage) error {
	var resp struct {
		Meta json.RawMessage `json:"meta"`
	}
	url := fmt.Sprintf("%s%s/api/s/%s/rest/device/%s", c.BaseURL, c.APIPath, site, deviceID)
	body := map[string]any{"port_overrides": overrides}
	if err := c.DoREST(ctx, http.MethodPut, url, body, &resp); err != nil {
		return err
	}
	return CheckV1Meta(resp.Meta)
}

func (c *APIClient) ListPortNetworks(ctx context.Context, site string) ([]PortNetwork, error) {
	var resp struct {
		Meta json.RawMessage `json:"meta"`
		Data []PortNetwork   `json:"data"`
	}
	url := fmt.Sprintf("%s%s/api/s/%s/rest/networkconf", c.BaseURL, c.APIPath, site)
	if err := c.doRequest(ctx, http.MethodGet, url, nil, &resp, false); err != nil {
		return nil, err
	}
	if err := CheckV1Meta(resp.Meta); err != nil {
		return nil, err
	}
	return resp.Data, nil
}
