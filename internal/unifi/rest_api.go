package unifi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type restEnvelope[T any] struct {
	Meta json.RawMessage `json:"meta"`
	Data []T             `json:"data"`
}

func restList[T any](ctx context.Context, c *APIClient, method, url string, body any) ([]T, error) {
	var response restEnvelope[T]
	if err := c.doV1Request(ctx, method, url, body, &response); err != nil {
		return nil, err
	}
	if err := CheckV1Meta(response.Meta); err != nil {
		return nil, err
	}
	return response.Data, nil
}

func restOne[T any](ctx context.Context, c *APIClient, method, url string, body any) (*T, error) {
	data, err := restList[T](ctx, c, method, url, body)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 && method == http.MethodPut {
		return restOne[T](ctx, c, http.MethodGet, url, nil)
	}
	if len(data) != 1 {
		return nil, &NotFoundError{}
	}
	return &data[0], nil
}

func restDelete(ctx context.Context, c *APIClient, url string) error {
	var response struct {
		Meta json.RawMessage `json:"meta"`
	}
	if err := c.doV1Request(ctx, http.MethodDelete, url, nil, &response); err != nil {
		return err
	}
	return CheckV1Meta(response.Meta)
}

func (c *APIClient) ListFirewallGroup(ctx context.Context, site string) ([]FirewallGroup, error) {
	return restList[FirewallGroup](ctx, c, http.MethodGet, fmt.Sprintf("%s%s/api/s/%s/rest/firewallgroup", c.BaseURL, c.APIPath, site), nil)
}
func (c *APIClient) GetFirewallGroup(ctx context.Context, site, id string) (*FirewallGroup, error) {
	return restOne[FirewallGroup](ctx, c, http.MethodGet, fmt.Sprintf("%s%s/api/s/%s/rest/firewallgroup/%s", c.BaseURL, c.APIPath, site, id), nil)
}
func (c *APIClient) CreateFirewallGroup(ctx context.Context, site string, d *FirewallGroup) (*FirewallGroup, error) {
	copy := *d
	d = &copy
	if d.GroupMembers == nil {
		d.GroupMembers = []string{}
	}
	return restOne[FirewallGroup](ctx, c, http.MethodPost, fmt.Sprintf("%s%s/api/s/%s/rest/firewallgroup", c.BaseURL, c.APIPath, site), d)
}
func (c *APIClient) UpdateFirewallGroup(ctx context.Context, site string, d *FirewallGroup) (*FirewallGroup, error) {
	copy := *d
	d = &copy
	if d.GroupMembers == nil {
		d.GroupMembers = []string{}
	}
	url := fmt.Sprintf("%s%s/api/s/%s/rest/firewallgroup/%s", c.BaseURL, c.APIPath, site, d.ID)
	payload, err := c.mergeRESTObject(ctx, url, d)
	if err != nil {
		return nil, err
	}
	return restOne[FirewallGroup](ctx, c, http.MethodPut, url, payload)
}
func (c *APIClient) DeleteFirewallGroup(ctx context.Context, site, id string) error {
	return restDelete(ctx, c, fmt.Sprintf("%s%s/api/s/%s/rest/firewallgroup/%s", c.BaseURL, c.APIPath, site, id))
}

func (c *APIClient) ListWLAN(ctx context.Context, site string) ([]WLAN, error) {
	return restList[WLAN](ctx, c, http.MethodGet, fmt.Sprintf("%s%s/api/s/%s/rest/wlanconf", c.BaseURL, c.APIPath, site), nil)
}
func (c *APIClient) GetWLAN(ctx context.Context, site, id string) (*WLAN, error) {
	return restOne[WLAN](ctx, c, http.MethodGet, fmt.Sprintf("%s%s/api/s/%s/rest/wlanconf/%s", c.BaseURL, c.APIPath, site, id), nil)
}
func (c *APIClient) CreateWLAN(ctx context.Context, site string, d *WLAN) (*WLAN, error) {
	return restOne[WLAN](ctx, c, http.MethodPost, fmt.Sprintf("%s%s/api/s/%s/rest/wlanconf", c.BaseURL, c.APIPath, site), d)
}
func (c *APIClient) UpdateWLAN(ctx context.Context, site string, d *WLAN) (*WLAN, error) {
	url := fmt.Sprintf("%s%s/api/s/%s/rest/wlanconf/%s", c.BaseURL, c.APIPath, site, d.ID)
	payload, err := c.mergeRESTObject(ctx, url, d)
	if err != nil {
		return nil, err
	}
	return restOne[WLAN](ctx, c, http.MethodPut, url, payload)
}
func (c *APIClient) DeleteWLAN(ctx context.Context, site, id string) error {
	return restDelete(ctx, c, fmt.Sprintf("%s%s/api/s/%s/rest/wlanconf/%s", c.BaseURL, c.APIPath, site, id))
}

// mergeRESTObject preserves controller fields outside the local write contract.
func (c *APIClient) mergeRESTObject(ctx context.Context, url string, managed any) (map[string]json.RawMessage, error) {
	current, err := restOne[map[string]json.RawMessage](ctx, c, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(managed)
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(b, &fields); err != nil {
		return nil, err
	}
	for k, v := range fields {
		(*current)[k] = v
	}
	return *current, nil
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

type controllerGroup struct {
	ID   string `json:"_id"`
	Name string `json:"name"`
}

func (c *APIClient) ListWLANGroup(ctx context.Context, site string) ([]controllerGroup, error) {
	return restList[controllerGroup](ctx, c, http.MethodGet, fmt.Sprintf("%s%s/api/s/%s/rest/wlangroup", c.BaseURL, c.APIPath, site), nil)
}
func (c *APIClient) ListClientGroup(ctx context.Context, site string) ([]controllerGroup, error) {
	return restList[controllerGroup](ctx, c, http.MethodGet, fmt.Sprintf("%s%s/api/s/%s/rest/usergroup", c.BaseURL, c.APIPath, site), nil)
}
func (c *APIClient) ListAPGroup(ctx context.Context, site string) ([]controllerGroup, error) {
	var data []controllerGroup
	err := c.Do(ctx, http.MethodGet, fmt.Sprintf("%s%s/v2/api/site/%s/apgroups", c.BaseURL, c.APIPath, site), nil, &data)
	return data, err
}
func (c *APIClient) ListSites(ctx context.Context) ([]controllerGroup, error) {
	return restList[controllerGroup](ctx, c, http.MethodGet, fmt.Sprintf("%s%s/api/self/sites", c.BaseURL, c.APIPath), nil)
}
func (c *APIClient) ListFirewallZone(ctx context.Context, site string) ([]FirewallZone, error) {
	var data []FirewallZone
	err := c.Do(ctx, http.MethodGet, fmt.Sprintf("%s%s/v2/api/site/%s/firewall/zone", c.BaseURL, c.APIPath, site), nil, &data)
	return data, err
}
func (c *APIClient) ListNetworkMembersGroups(ctx context.Context, site string) ([]NetworkMembersGroup, error) {
	var data []NetworkMembersGroup
	err := c.Do(ctx, http.MethodGet, fmt.Sprintf("%s%s/v2/api/site/%s/network-members-groups", c.BaseURL, c.APIPath, site), nil, &data)
	return data, err
}
func (c *APIClient) GetNetworkMembersGroup(ctx context.Context, site, id string) (*NetworkMembersGroup, error) {
	var data NetworkMembersGroup
	err := c.doV1Request(ctx, http.MethodGet, fmt.Sprintf("%s%s/v2/api/site/%s/network-members-group/%s", c.BaseURL, c.APIPath, site, id), nil, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}
func (c *APIClient) UpdateNetworkMembersGroup(ctx context.Context, site string, d *NetworkMembersGroup) (*NetworkMembersGroup, error) {
	payload := *d
	if payload.Members == nil {
		payload.Members = []string{}
	}
	var data NetworkMembersGroup
	err := c.Do(ctx, http.MethodPut, fmt.Sprintf("%s%s/v2/api/site/%s/network-members-group/%s", c.BaseURL, c.APIPath, site, d.ID), &payload, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}
func (c *APIClient) ListDNSRecord(ctx context.Context, site string) ([]DNSRecord, error) {
	var data []DNSRecord
	err := c.Do(ctx, http.MethodGet, fmt.Sprintf("%s%s/v2/api/site/%s/static-dns", c.BaseURL, c.APIPath, site), nil, &data)
	return data, err
}
func (c *APIClient) GetDNSRecord(ctx context.Context, site, id string) (*DNSRecord, error) {
	data, err := c.ListDNSRecord(ctx, site)
	if err != nil {
		return nil, err
	}
	for i := range data {
		if data[i].ID == id {
			return &data[i], nil
		}
	}
	return nil, &NotFoundError{}
}
func (c *APIClient) CreateDNSRecord(ctx context.Context, site string, d *DNSRecord) (*DNSRecord, error) {
	var result DNSRecord
	err := c.Do(ctx, http.MethodPost, fmt.Sprintf("%s%s/v2/api/site/%s/static-dns", c.BaseURL, c.APIPath, site), d, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}
func (c *APIClient) UpdateDNSRecord(ctx context.Context, site string, d *DNSRecord) (*DNSRecord, error) {
	var result DNSRecord
	err := c.Do(ctx, http.MethodPut, fmt.Sprintf("%s%s/v2/api/site/%s/static-dns/%s", c.BaseURL, c.APIPath, site, d.ID), d, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}
func (c *APIClient) DeleteDNSRecord(ctx context.Context, site, id string) error {
	return c.Do(ctx, http.MethodDelete, fmt.Sprintf("%s%s/v2/api/site/%s/static-dns/%s", c.BaseURL, c.APIPath, site, id), nil, nil)
}
