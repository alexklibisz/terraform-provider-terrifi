package provider

// TODO(go-unifi): This file works around a bug in the go-unifi SDK for Client
// CRUD. The SDK's Client struct serializes boolean fields use_fixedip,
// local_dns_record_enabled, and fixed_ap_enabled without omitempty, which means
// they always appear as false in the JSON body. This can clear settings managed
// outside Terraform (like fixed AP binding). A custom request struct lets us
// control exactly which fields are serialized.
//
// When the upstream SDK adds omitempty to these fields, this file can be deleted
// and the resource can use the SDK's built-in createClient/updateClient methods.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/ubiquiti-community/go-unifi/unifi"
)

// clientDeviceRequest is the payload for POST/PUT to api/s/{site}/rest/user.
// Uses *bool + omitempty for boolean fields so we only send fields we manage.
type clientDeviceRequest struct {
	MAC                           string `json:"mac"`
	Name                          string `json:"name,omitempty"`
	Note                          string `json:"note,omitempty"`
	Noted                         *bool  `json:"noted,omitempty"`
	FixedIP                       string `json:"fixed_ip,omitempty"`
	NetworkID                     string `json:"network_id,omitempty"`
	UseFixedIP                    *bool  `json:"use_fixedip,omitempty"`
	LocalDNSRecord                string `json:"local_dns_record,omitempty"`
	LocalDNSRecordEnabled         *bool  `json:"local_dns_record_enabled,omitempty"`
	VirtualNetworkOverrideEnabled *bool  `json:"virtual_network_override_enabled,omitempty"`
	VirtualNetworkOverrideID      string `json:"virtual_network_override_id,omitempty"`
	Blocked                       *bool  `json:"blocked,omitempty"`
}

// clientDeviceUpdateRequest adds _id to the request for PUT operations.
type clientDeviceUpdateRequest struct {
	ID string `json:"_id"`
	clientDeviceRequest
}

// CreateClientDevice creates a client device entry via the v1 REST API,
// bypassing the SDK to control boolean serialization.
func (c *Client) CreateClientDevice(ctx context.Context, site string, d *unifi.Client) (*unifi.Client, error) {
	payload := buildClientDeviceRequest(d)

	var respBody struct {
		Meta json.RawMessage `json:"meta"`
		Data []unifi.Client  `json:"data"`
	}
	err := c.doV1Request(ctx, http.MethodPost,
		fmt.Sprintf("%s%s/api/s/%s/rest/user", c.BaseURL, c.APIPath, site),
		payload, &respBody)
	if err != nil {
		return nil, err
	}
	if len(respBody.Data) != 1 {
		return nil, &unifi.NotFoundError{}
	}
	return &respBody.Data[0], nil
}

// UpdateClientDevice updates a client device entry via the v1 REST API,
// bypassing the SDK to control boolean serialization.
func (c *Client) UpdateClientDevice(ctx context.Context, site string, d *unifi.Client) (*unifi.Client, error) {
	req := buildClientDeviceRequest(d)
	payload := clientDeviceUpdateRequest{
		ID:                  d.ID,
		clientDeviceRequest: req,
	}

	var respBody struct {
		Meta json.RawMessage `json:"meta"`
		Data []unifi.Client  `json:"data"`
	}
	err := c.doV1Request(ctx, http.MethodPut,
		fmt.Sprintf("%s%s/api/s/%s/rest/user/%s", c.BaseURL, c.APIPath, site, d.ID),
		payload, &respBody)
	if err != nil {
		return nil, err
	}
	if len(respBody.Data) != 1 {
		return nil, &unifi.NotFoundError{}
	}
	return &respBody.Data[0], nil
}

// GetClientDevice reads a client device via the SDK's getClient (GET has no body
// serialization issue).
func (c *Client) GetClientDevice(ctx context.Context, site string, id string) (*unifi.Client, error) {
	var respBody struct {
		Meta json.RawMessage `json:"meta"`
		Data []unifi.Client  `json:"data"`
	}
	err := c.doV1Request(ctx, http.MethodGet,
		fmt.Sprintf("%s%s/api/s/%s/rest/user/%s", c.BaseURL, c.APIPath, site, id),
		nil, &respBody)
	if err != nil {
		return nil, err
	}
	if len(respBody.Data) != 1 {
		return nil, &unifi.NotFoundError{}
	}
	return &respBody.Data[0], nil
}

// DeleteClientDevice deletes a client device via the v1 REST API.
func (c *Client) DeleteClientDevice(ctx context.Context, site string, id string) error {
	return c.doV1Request(ctx, http.MethodDelete,
		fmt.Sprintf("%s%s/api/s/%s/rest/user/%s", c.BaseURL, c.APIPath, site, id),
		struct{}{}, nil)
}

// doV1Request makes an authenticated HTTP request to the UniFi v1 REST API.
// It reuses the HTTP mechanics from doV2Request and converts HTTP 404 responses
// to *unifi.NotFoundError for consistent handling by callers.
func (c *Client) doV1Request(ctx context.Context, method, url string, body any, result any) error {
	err := c.doV2Request(ctx, method, url, body, result)
	if err != nil && strings.Contains(err.Error(), "(404)") {
		return &unifi.NotFoundError{}
	}
	return err
}

// buildClientDeviceRequest converts a *unifi.Client to a clientDeviceRequest,
// deriving boolean enable flags from the presence of their associated values.
func buildClientDeviceRequest(d *unifi.Client) clientDeviceRequest {
	req := clientDeviceRequest{
		MAC:  d.MAC,
		Name: d.Name,
	}

	// Note: the "noted" field tells the UI a note exists
	if d.Note != "" {
		req.Note = d.Note
		req.Noted = boolPtr(true)
	}

	// Fixed IP: derive use_fixedip from whether fixed_ip is set
	if d.FixedIP != "" {
		req.FixedIP = d.FixedIP
		req.NetworkID = d.NetworkID
		req.UseFixedIP = boolPtr(true)
	} else {
		req.UseFixedIP = boolPtr(false)
	}

	// Local DNS record: derive enabled from whether record is set
	if d.LocalDNSRecord != "" {
		req.LocalDNSRecord = d.LocalDNSRecord
		req.LocalDNSRecordEnabled = boolPtr(true)
	} else {
		req.LocalDNSRecordEnabled = boolPtr(false)
	}

	// Virtual network override: derive enabled from whether ID is set
	if d.VirtualNetworkOverrideID != "" {
		req.VirtualNetworkOverrideID = d.VirtualNetworkOverrideID
		req.VirtualNetworkOverrideEnabled = boolPtr(true)
	} else {
		req.VirtualNetworkOverrideEnabled = boolPtr(false)
	}

	// Blocked: pass through as-is
	req.Blocked = d.Blocked

	return req
}
