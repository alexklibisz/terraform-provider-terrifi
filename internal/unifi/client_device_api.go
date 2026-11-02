package unifi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// clientDeviceRequest is the payload for POST/PUT to api/s/{site}/rest/user.
// Uses *bool + omitempty for boolean fields so we only send fields we manage.
type clientDeviceRequest struct {
	MAC                           string   `json:"mac"`
	Name                          string   `json:"name,omitempty"`
	Note                          string   `json:"note,omitempty"`
	Noted                         *bool    `json:"noted,omitempty"`
	FixedIP                       string   `json:"fixed_ip,omitempty"`
	NetworkID                     string   `json:"network_id,omitempty"`
	UseFixedIP                    *bool    `json:"use_fixedip,omitempty"`
	LocalDNSRecord                string   `json:"local_dns_record,omitempty"`
	LocalDNSRecordEnabled         *bool    `json:"local_dns_record_enabled,omitempty"`
	VirtualNetworkOverrideEnabled *bool    `json:"virtual_network_override_enabled,omitempty"`
	VirtualNetworkOverrideID      string   `json:"virtual_network_override_id,omitempty"`
	NetworkMembersGroupIDs        []string `json:"network_members_group_ids"`
	FixedApEnabled                *bool    `json:"fixed_ap_enabled,omitempty"`
	FixedApMAC                    string   `json:"fixed_ap_mac,omitempty"`
	Blocked                       *bool    `json:"blocked,omitempty"`
}

// clientDeviceUpdateRequest adds _id to the request for PUT operations.
type clientDeviceUpdateRequest struct {
	ID string `json:"_id"`
	clientDeviceRequest
}

func (c *APIClient) CreateClientDevice(ctx context.Context, site string, d *Client) (*Client, error) {
	payload := BuildClientDeviceRequest(d)

	var respBody struct {
		Meta json.RawMessage `json:"meta"`
		Data []Client        `json:"data"`
	}
	err := c.doV1Request(ctx, http.MethodPost,
		fmt.Sprintf("%s%s/api/s/%s/rest/user", c.BaseURL, c.APIPath, site),
		payload, &respBody)
	if err != nil {
		return nil, err
	}
	if err := CheckV1Meta(respBody.Meta); err != nil {
		return nil, err
	}
	if len(respBody.Data) != 1 {
		return nil, &NotFoundError{}
	}
	return &respBody.Data[0], nil
}

func (c *APIClient) UpdateClientDevice(ctx context.Context, site string, d *Client) (*Client, error) {
	req := BuildClientDeviceRequest(d)
	payload := clientDeviceUpdateRequest{
		ID:                  d.ID,
		clientDeviceRequest: req,
	}

	var respBody struct {
		Meta json.RawMessage `json:"meta"`
		Data []Client        `json:"data"`
	}
	err := c.doV1Request(ctx, http.MethodPut,
		fmt.Sprintf("%s%s/api/s/%s/rest/user/%s", c.BaseURL, c.APIPath, site, d.ID),
		payload, &respBody)
	if err != nil {
		return nil, err
	}
	if err := CheckV1Meta(respBody.Meta); err != nil {
		return nil, err
	}
	if len(respBody.Data) == 1 {
		return &respBody.Data[0], nil
	}
	// The controller sometimes returns an empty data array for no-op updates
	// (when the PUT payload is identical to the current state). Fall back to
	// GET to retrieve the current record instead of treating this as an error.
	return c.GetClientDevice(ctx, site, d.ID)
}

func (c *APIClient) GetClientDevice(ctx context.Context, site string, id string) (*Client, error) {
	var respBody struct {
		Meta json.RawMessage `json:"meta"`
		Data []Client        `json:"data"`
	}
	err := c.doV1Request(ctx, http.MethodGet,
		fmt.Sprintf("%s%s/api/s/%s/rest/user/%s", c.BaseURL, c.APIPath, site, id),
		nil, &respBody)
	if err != nil {
		return nil, err
	}
	if err := CheckV1Meta(respBody.Meta); err != nil {
		return nil, err
	}
	if len(respBody.Data) != 1 {
		return nil, &NotFoundError{}
	}
	return &respBody.Data[0], nil
}

// ListClientDevices returns all configured client devices for the given site.
func (c *APIClient) ListClientDevices(ctx context.Context, site string) ([]Client, error) {
	var respBody struct {
		Meta json.RawMessage `json:"meta"`
		Data []Client        `json:"data"`
	}
	err := c.doV1Request(ctx, http.MethodGet,
		fmt.Sprintf("%s%s/api/s/%s/rest/user", c.BaseURL, c.APIPath, site),
		nil, &respBody)
	if err != nil {
		return nil, err
	}
	if err := CheckV1Meta(respBody.Meta); err != nil {
		return nil, err
	}
	return respBody.Data, nil
}

// GetClientDeviceByMAC looks up a client device by MAC address. The REST API's
// ?mac= query parameter is not honored by all controller versions, so this
// falls back to listing all users and filtering client-side.
func (c *APIClient) GetClientDeviceByMAC(ctx context.Context, site string, mac string) (*Client, error) {
	mac = strings.ToLower(mac)
	clients, err := c.ListClientDevices(ctx, site)
	if err != nil {
		return nil, err
	}
	for i := range clients {
		if strings.ToLower(clients[i].MAC) == mac {
			return &clients[i], nil
		}
	}
	return nil, &NotFoundError{}
}

// ForgetClientDevicesByMAC removes one or more known client devices via the
// stamgr "forget-sta" command. UniFi controllers do not honor DELETE on
// /rest/user/{id} (they return 404), so we use the documented stamgr endpoint
// instead. Accepts a batch of MACs to minimize controller round-trips when
// cleaning up many records at once.
func (c *APIClient) ForgetClientDevicesByMAC(ctx context.Context, site string, macs []string) error {
	if len(macs) == 0 {
		return nil
	}
	payload := map[string]any{
		"cmd":  "forget-sta",
		"macs": macs,
	}
	var respBody struct {
		Meta json.RawMessage `json:"meta"`
		Data []Client        `json:"data"`
	}
	err := c.doV1Request(ctx, http.MethodPost,
		fmt.Sprintf("%s%s/api/s/%s/cmd/stamgr", c.BaseURL, c.APIPath, site),
		payload, &respBody)
	if err != nil {
		return err
	}
	return CheckV1Meta(respBody.Meta)
}

// doV1Request makes an authenticated HTTP request to the UniFi v1 REST API.
// It reuses the HTTP mechanics from Do and converts HTTP 404 responses
// to *NotFoundError for consistent handling by callers.
func (c *APIClient) doV1Request(ctx context.Context, method, url string, body any, result any) error {
	err := c.Do(ctx, method, url, body, result)
	if err != nil && strings.Contains(err.Error(), "(404)") {
		return &NotFoundError{}
	}
	return err
}

// GetFingerprintOverride reads the current fingerprint override for a client
// device via the v2 client info API. Returns 0 if no override is set.
//
// The fingerprint_override endpoint only supports PUT/DELETE, not GET. So we
// read the override from the v2 client info endpoint which includes the
// fingerprint data with dev_id_override.
func (c *APIClient) GetFingerprintOverride(ctx context.Context, site string, mac string) (int64, error) {
	var respBody struct {
		Fingerprint struct {
			DevIdOverride *int64 `json:"dev_id_override,omitempty"`
			HasOverride   bool   `json:"has_override,omitempty"`
		} `json:"fingerprint"`
	}
	err := c.Do(ctx, http.MethodGet,
		fmt.Sprintf("%s%s/v2/api/site/%s/clients/local/%s?includeUnifiDevices=true", c.BaseURL, c.APIPath, site, mac),
		nil, &respBody)
	if err != nil {
		// A 404 means the client isn't known yet — no override.
		if strings.Contains(err.Error(), "(404)") {
			return 0, nil
		}
		return 0, err
	}
	if respBody.Fingerprint.DevIdOverride != nil {
		return *respBody.Fingerprint.DevIdOverride, nil
	}
	return 0, nil
}

// SetFingerprintOverride sets or clears the fingerprint override for a client
// device via the v2 API. Pass 0 to clear the override (sends DELETE), or a
// positive device type ID to set it (sends PUT).
func (c *APIClient) SetFingerprintOverride(ctx context.Context, site string, mac string, deviceTypeID int64) error {
	url := fmt.Sprintf("%s%s/v2/api/site/%s/station/%s/fingerprint_override", c.BaseURL, c.APIPath, site, mac)

	if deviceTypeID == 0 {
		return c.Do(ctx, http.MethodDelete, url,
			map[string]any{"mac": mac, "dev_id_override": 0, "search_query": ""}, nil)
	}

	return c.Do(ctx, http.MethodPut, url,
		map[string]any{"mac": mac, "dev_id_override": deviceTypeID, "search_query": ""}, nil)
}

// v1Meta represents the meta field in UniFi v1 API responses.
type v1Meta struct {
	RC  string `json:"rc"`
	Msg string `json:"msg"`
}

// CheckV1Meta parses a v1 API response meta field and returns an error if the
// controller reported an error (rc != "ok"), even when the HTTP status was 200.
// This prevents silent error swallowing where the controller returns HTTP 200
// with an error in the meta field and an empty data array.
func CheckV1Meta(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	var meta v1Meta
	if err := json.Unmarshal(raw, &meta); err != nil {
		return nil
	}
	if meta.RC == "error" {
		return fmt.Errorf("controller error: %s", meta.Msg)
	}
	return nil
}

// BuildClientDeviceRequest converts a *Client to a clientDeviceRequest,
// deriving boolean enable flags from the presence of their associated values.
func BuildClientDeviceRequest(d *Client) clientDeviceRequest {
	req := clientDeviceRequest{
		MAC:  d.MAC,
		Name: d.Name,
	}

	// Note: the "noted" field tells the UI a note exists
	if d.Note != "" {
		req.Note = d.Note
		req.Noted = BoolPtr(true)
	}

	// Fixed IP: the controller requires a valid network_id to resolve the
	// DHCP scope; sending use_fixedip=true without one returns "not found:
	// type=". When no explicit network_id is provided, fall back to the
	// virtual_network_override_id so that fixed_ip + network_override_id
	// works without requiring the user to duplicate the ID into network_id.
	if d.FixedIP != "" {
		netID := d.NetworkID
		if netID == "" {
			netID = d.VirtualNetworkOverrideID
		}
		if netID != "" {
			req.FixedIP = d.FixedIP
			req.NetworkID = netID
			req.UseFixedIP = BoolPtr(true)
		} else {
			req.UseFixedIP = BoolPtr(false)
		}
	} else {
		req.UseFixedIP = BoolPtr(false)
	}

	// Local DNS record: derive enabled from whether record is set
	if d.LocalDNSRecord != "" {
		req.LocalDNSRecord = d.LocalDNSRecord
		req.LocalDNSRecordEnabled = BoolPtr(true)
	} else {
		req.LocalDNSRecordEnabled = BoolPtr(false)
	}

	// Virtual network override: derive enabled from whether ID is set
	if d.VirtualNetworkOverrideID != "" {
		req.VirtualNetworkOverrideID = d.VirtualNetworkOverrideID
		req.VirtualNetworkOverrideEnabled = BoolPtr(true)
	} else {
		req.VirtualNetworkOverrideEnabled = BoolPtr(false)
	}

	// Client group assignment — always set the slice so that an empty slice
	// explicitly clears group references (needed during Delete to remove
	// references before deleting the groups themselves).
	if d.NetworkMembersGroupIDs != nil {
		req.NetworkMembersGroupIDs = d.NetworkMembersGroupIDs
	} else {
		req.NetworkMembersGroupIDs = []string{}
	}

	// Fixed AP: derive enabled from whether MAC is set
	if d.FixedApMAC != "" {
		req.FixedApMAC = d.FixedApMAC
		req.FixedApEnabled = BoolPtr(true)
	} else {
		req.FixedApEnabled = BoolPtr(false)
	}

	// Blocked: pass through as-is
	req.Blocked = d.Blocked

	return req
}
