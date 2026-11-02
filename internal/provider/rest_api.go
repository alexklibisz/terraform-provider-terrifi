package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/alexklibisz/terrifi/internal/unifi"
	"net/http"
)

type restEnvelope[T any] struct {
	Meta json.RawMessage `json:"meta"`
	Data []T             `json:"data"`
}

func restList[T any](ctx context.Context, c *Client, method, url string, body any) ([]T, error) {
	var response restEnvelope[T]
	if err := c.doV1Request(ctx, method, url, body, &response); err != nil {
		return nil, err
	}
	if err := checkV1Meta(response.Meta); err != nil {
		return nil, err
	}
	return response.Data, nil
}

func restOne[T any](ctx context.Context, c *Client, method, url string, body any) (*T, error) {
	data, err := restList[T](ctx, c, method, url, body)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 && method == http.MethodPut {
		return restOne[T](ctx, c, http.MethodGet, url, nil)
	}
	if len(data) != 1 {
		return nil, &unifi.NotFoundError{}
	}
	return &data[0], nil
}

func restDelete(ctx context.Context, c *Client, url string) error {
	var response struct {
		Meta json.RawMessage `json:"meta"`
	}
	if err := c.doV1Request(ctx, http.MethodDelete, url, nil, &response); err != nil {
		return err
	}
	return checkV1Meta(response.Meta)
}

func (c *Client) ListFirewallGroup(ctx context.Context, site string) ([]unifi.FirewallGroup, error) {
	return restList[unifi.FirewallGroup](ctx, c, http.MethodGet, fmt.Sprintf("%s%s/api/s/%s/rest/firewallgroup", c.BaseURL, c.APIPath, site), nil)
}
func (c *Client) GetFirewallGroup(ctx context.Context, site, id string) (*unifi.FirewallGroup, error) {
	return restOne[unifi.FirewallGroup](ctx, c, http.MethodGet, fmt.Sprintf("%s%s/api/s/%s/rest/firewallgroup/%s", c.BaseURL, c.APIPath, site, id), nil)
}
func (c *Client) CreateFirewallGroup(ctx context.Context, site string, d *unifi.FirewallGroup) (*unifi.FirewallGroup, error) {
	copy := *d
	d = &copy
	if d.GroupMembers == nil {
		d.GroupMembers = []string{}
	}
	return restOne[unifi.FirewallGroup](ctx, c, http.MethodPost, fmt.Sprintf("%s%s/api/s/%s/rest/firewallgroup", c.BaseURL, c.APIPath, site), d)
}
func (c *Client) UpdateFirewallGroup(ctx context.Context, site string, d *unifi.FirewallGroup) (*unifi.FirewallGroup, error) {
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
	return restOne[unifi.FirewallGroup](ctx, c, http.MethodPut, url, payload)
}
func (c *Client) DeleteFirewallGroup(ctx context.Context, site, id string) error {
	return restDelete(ctx, c, fmt.Sprintf("%s%s/api/s/%s/rest/firewallgroup/%s", c.BaseURL, c.APIPath, site, id))
}

func (c *Client) ListWLAN(ctx context.Context, site string) ([]unifi.WLAN, error) {
	return restList[unifi.WLAN](ctx, c, http.MethodGet, fmt.Sprintf("%s%s/api/s/%s/rest/wlanconf", c.BaseURL, c.APIPath, site), nil)
}
func (c *Client) GetWLAN(ctx context.Context, site, id string) (*unifi.WLAN, error) {
	return restOne[unifi.WLAN](ctx, c, http.MethodGet, fmt.Sprintf("%s%s/api/s/%s/rest/wlanconf/%s", c.BaseURL, c.APIPath, site, id), nil)
}
func (c *Client) CreateWLAN(ctx context.Context, site string, d *unifi.WLAN) (*unifi.WLAN, error) {
	return restOne[unifi.WLAN](ctx, c, http.MethodPost, fmt.Sprintf("%s%s/api/s/%s/rest/wlanconf", c.BaseURL, c.APIPath, site), d)
}
func (c *Client) UpdateWLAN(ctx context.Context, site string, d *unifi.WLAN) (*unifi.WLAN, error) {
	url := fmt.Sprintf("%s%s/api/s/%s/rest/wlanconf/%s", c.BaseURL, c.APIPath, site, d.ID)
	payload, err := c.mergeRESTObject(ctx, url, d)
	if err != nil {
		return nil, err
	}
	return restOne[unifi.WLAN](ctx, c, http.MethodPut, url, payload)
}
func (c *Client) DeleteWLAN(ctx context.Context, site, id string) error {
	return restDelete(ctx, c, fmt.Sprintf("%s%s/api/s/%s/rest/wlanconf/%s", c.BaseURL, c.APIPath, site, id))
}

// mergeRESTObject preserves controller fields outside the local write contract.
func (c *Client) mergeRESTObject(ctx context.Context, url string, managed any) (map[string]json.RawMessage, error) {
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

func (c *Client) DeleteNetwork(ctx context.Context, site, id, name string) error {
	var response struct {
		Meta json.RawMessage `json:"meta"`
	}
	if err := c.doV1Request(ctx, http.MethodDelete, c.networkURL(site, id), map[string]string{"name": name}, &response); err != nil {
		return err
	}
	return checkV1Meta(response.Meta)
}

type controllerGroup struct {
	ID   string `json:"_id"`
	Name string `json:"name"`
}

func (c *Client) ListWLANGroup(ctx context.Context, site string) ([]controllerGroup, error) {
	return restList[controllerGroup](ctx, c, http.MethodGet, fmt.Sprintf("%s%s/api/s/%s/rest/wlangroup", c.BaseURL, c.APIPath, site), nil)
}
func (c *Client) ListClientGroup(ctx context.Context, site string) ([]controllerGroup, error) {
	return restList[controllerGroup](ctx, c, http.MethodGet, fmt.Sprintf("%s%s/api/s/%s/rest/usergroup", c.BaseURL, c.APIPath, site), nil)
}
func (c *Client) ListAPGroup(ctx context.Context, site string) ([]controllerGroup, error) {
	var data []controllerGroup
	err := c.doV2Request(ctx, http.MethodGet, fmt.Sprintf("%s%s/v2/api/site/%s/apgroups", c.BaseURL, c.APIPath, site), nil, &data)
	return data, err
}
func (c *Client) ListSites(ctx context.Context) ([]controllerGroup, error) {
	return restList[controllerGroup](ctx, c, http.MethodGet, fmt.Sprintf("%s%s/api/self/sites", c.BaseURL, c.APIPath), nil)
}
func (c *Client) ListFirewallZone(ctx context.Context, site string) ([]unifi.FirewallZone, error) {
	var data []unifi.FirewallZone
	err := c.doV2Request(ctx, http.MethodGet, fmt.Sprintf("%s%s/v2/api/site/%s/firewall/zone", c.BaseURL, c.APIPath, site), nil, &data)
	return data, err
}
func (c *Client) ListNetworkMembersGroups(ctx context.Context, site string) ([]unifi.NetworkMembersGroup, error) {
	var data []unifi.NetworkMembersGroup
	err := c.doV2Request(ctx, http.MethodGet, fmt.Sprintf("%s%s/v2/api/site/%s/network-members-groups", c.BaseURL, c.APIPath, site), nil, &data)
	return data, err
}
func (c *Client) GetNetworkMembersGroup(ctx context.Context, site, id string) (*unifi.NetworkMembersGroup, error) {
	var data unifi.NetworkMembersGroup
	err := c.doV1Request(ctx, http.MethodGet, fmt.Sprintf("%s%s/v2/api/site/%s/network-members-group/%s", c.BaseURL, c.APIPath, site, id), nil, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}
func (c *Client) UpdateNetworkMembersGroup(ctx context.Context, site string, d *unifi.NetworkMembersGroup) (*unifi.NetworkMembersGroup, error) {
	payload := *d
	if payload.Members == nil {
		payload.Members = []string{}
	}
	var data unifi.NetworkMembersGroup
	err := c.doV2Request(ctx, http.MethodPut, fmt.Sprintf("%s%s/v2/api/site/%s/network-members-group/%s", c.BaseURL, c.APIPath, site, d.ID), &payload, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}
func (c *Client) ListDNSRecord(ctx context.Context, site string) ([]unifi.DNSRecord, error) {
	var data []unifi.DNSRecord
	err := c.doV2Request(ctx, http.MethodGet, fmt.Sprintf("%s%s/v2/api/site/%s/static-dns", c.BaseURL, c.APIPath, site), nil, &data)
	return data, err
}
func (c *Client) GetDNSRecord(ctx context.Context, site, id string) (*unifi.DNSRecord, error) {
	data, err := c.ListDNSRecord(ctx, site)
	if err != nil {
		return nil, err
	}
	for i := range data {
		if data[i].ID == id {
			return &data[i], nil
		}
	}
	return nil, &unifi.NotFoundError{}
}
func (c *Client) CreateDNSRecord(ctx context.Context, site string, d *unifi.DNSRecord) (*unifi.DNSRecord, error) {
	var result unifi.DNSRecord
	err := c.doV2Request(ctx, http.MethodPost, fmt.Sprintf("%s%s/v2/api/site/%s/static-dns", c.BaseURL, c.APIPath, site), d, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}
func (c *Client) UpdateDNSRecord(ctx context.Context, site string, d *unifi.DNSRecord) (*unifi.DNSRecord, error) {
	var result unifi.DNSRecord
	err := c.doV2Request(ctx, http.MethodPut, fmt.Sprintf("%s%s/v2/api/site/%s/static-dns/%s", c.BaseURL, c.APIPath, site, d.ID), d, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}
func (c *Client) DeleteDNSRecord(ctx context.Context, site, id string) error {
	return c.doV2Request(ctx, http.MethodDelete, fmt.Sprintf("%s%s/v2/api/site/%s/static-dns/%s", c.BaseURL, c.APIPath, site, id), nil, nil)
}
