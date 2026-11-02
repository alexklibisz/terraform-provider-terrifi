package unifi

import (
	"context"
	"fmt"
	"net/http"
)

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
