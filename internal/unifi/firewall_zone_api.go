package unifi

import (
	"context"
	"fmt"
	"net/http"
)

type firewallZoneCreateRequest struct {
	Name       string   `json:"name,omitempty"`
	NetworkIDs []string `json:"network_ids"`
}

type firewallZoneUpdateRequest struct {
	ID         string   `json:"_id"`
	Name       string   `json:"name,omitempty"`
	NetworkIDs []string `json:"network_ids"`
}

func (c *APIClient) GetFirewallZone(ctx context.Context, site string, id string) (*FirewallZone, error) {
	var zones []FirewallZone
	err := c.Do(ctx, http.MethodGet,
		fmt.Sprintf("%s%s/v2/api/site/%s/firewall/zone", c.BaseURL, c.APIPath, site),
		struct{}{}, &zones)
	if err != nil {
		return nil, err
	}

	for i := range zones {
		if zones[i].ID == id {
			return &zones[i], nil
		}
	}
	return nil, &NotFoundError{}
}

func (c *APIClient) CreateFirewallZone(ctx context.Context, site string, d *FirewallZone) (*FirewallZone, error) {
	payload := firewallZoneCreateRequest{
		Name:       d.Name,
		NetworkIDs: d.NetworkIDs,
	}
	if payload.NetworkIDs == nil {
		payload.NetworkIDs = []string{}
	}

	var result FirewallZone
	err := c.Do(ctx, http.MethodPost,
		fmt.Sprintf("%s%s/v2/api/site/%s/firewall/zone", c.BaseURL, c.APIPath, site),
		payload, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *APIClient) UpdateFirewallZone(ctx context.Context, site string, d *FirewallZone) (*FirewallZone, error) {
	payload := firewallZoneUpdateRequest{
		ID:         d.ID,
		Name:       d.Name,
		NetworkIDs: d.NetworkIDs,
	}
	if payload.NetworkIDs == nil {
		payload.NetworkIDs = []string{}
	}

	var result FirewallZone
	err := c.Do(ctx, http.MethodPut,
		fmt.Sprintf("%s%s/v2/api/site/%s/firewall/zone/%s", c.BaseURL, c.APIPath, site, d.ID),
		payload, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *APIClient) DeleteFirewallZone(ctx context.Context, site string, id string) error {
	return c.Do(ctx, http.MethodDelete,
		fmt.Sprintf("%s%s/v2/api/site/%s/firewall/zone/%s", c.BaseURL, c.APIPath, site, id),
		struct{}{}, nil)
}
