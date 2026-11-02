package unifi

import (
	"context"
	"fmt"
	"net/http"
)

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
