package provider

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
)

// GetControllerVersion returns the UniFi Network Application version string
// (e.g. "9.0.114") by querying the sysinfo endpoint.
func (c *Client) GetControllerVersion(ctx context.Context, site string) (string, error) {
	var resp struct {
		Data []struct {
			Version string `json:"version"`
		} `json:"data"`
	}
	err := c.doV1Request(ctx, http.MethodGet,
		fmt.Sprintf("%s%s/api/s/%s/stat/sysinfo", c.BaseURL, c.APIPath, site),
		nil, &resp)
	if err != nil {
		return "", err
	}
	if len(resp.Data) > 0 && resp.Data[0].Version != "" {
		return resp.Data[0].Version, nil
	}
	return "unknown", nil
}

// FingerprintDevice represents a device type entry from the UniFi controller's
// fingerprint database. The ID is used with dev_id_override to set custom
// icons on client devices.
type FingerprintDevice struct {
	ID       int64
	Name     string
	DevType  string
	Family   string
	Vendor   string
}

// fingerprintAPIResponse is the response from GET v2/api/fingerprint_devices/{version}.
type fingerprintAPIResponse struct {
	DevIDs     map[string]fingerprintDevEntry `json:"dev_ids"`
	DevTypeIDs map[string]string             `json:"dev_type_ids"`
	FamilyIDs  map[string]string             `json:"family_ids"`
	VendorIDs  map[string]string             `json:"vendor_ids"`
}

type fingerprintDevEntry struct {
	Name      string `json:"name"`
	DevTypeID string `json:"dev_type_id"`
	FamilyID  string `json:"family_id"`
	VendorID  string `json:"vendor_id"`
}

// ListFingerprintDevices fetches all known device types from the controller's
// fingerprint database. These can be used as dev_id_override values to set
// custom icons on client devices.
//
// The version parameter selects the fingerprint database edition:
//   - 0: full/expanded database (~5,600 devices)
//   - 1: smaller legacy subset (~1,000 devices)
//
// Icon URLs follow the pattern:
//
//	https://static.ui.com/fingerprint/0/{id}_128x128.png
func (c *Client) ListFingerprintDevices(ctx context.Context, version int) ([]FingerprintDevice, error) {
	var resp fingerprintAPIResponse
	err := c.doV2Request(ctx, http.MethodGet,
		fmt.Sprintf("%s%s/v2/api/fingerprint_devices/%d", c.BaseURL, c.APIPath, version),
		nil, &resp)
	if err != nil {
		return nil, err
	}

	devices := make([]FingerprintDevice, 0, len(resp.DevIDs))
	for idStr, entry := range resp.DevIDs {
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			continue
		}
		devices = append(devices, FingerprintDevice{
			ID:      id,
			Name:    entry.Name,
			DevType: resp.DevTypeIDs[entry.DevTypeID],
			Family:  resp.FamilyIDs[entry.FamilyID],
			Vendor:  resp.VendorIDs[entry.VendorID],
		})
	}

	sort.Slice(devices, func(i, j int) bool {
		return devices[i].ID < devices[j].ID
	})

	return devices, nil
}

