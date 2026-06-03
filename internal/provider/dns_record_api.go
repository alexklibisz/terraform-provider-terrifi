package provider

// CRUD methods for the v2 static-dns endpoint
// (/v2/api/site/{site}/static-dns). No envelope; per-ID GET is not exposed
// so GetDNSRecord lists and filters.

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/alexklibisz/terrifi/internal/unifi"
)

// CreateDNSRecord posts a new DNS record to /v2/api/site/{site}/static-dns.
func (c *Client) CreateDNSRecord(ctx context.Context, site string, d *unifi.DNSRecord) (*unifi.DNSRecord, error) {
	var result unifi.DNSRecord
	if err := c.doDNSRecordRequest(ctx, http.MethodPost,
		fmt.Sprintf("%s%s/v2/api/site/%s/static-dns", c.BaseURL, c.APIPath, site),
		d, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetDNSRecord reads a DNS record by ID. The v2 endpoint does not expose a
// per-ID GET, so we list and filter — matches the SDK's behavior.
func (c *Client) GetDNSRecord(ctx context.Context, site, id string) (*unifi.DNSRecord, error) {
	records, err := c.ListDNSRecord(ctx, site)
	if err != nil {
		return nil, err
	}
	for i := range records {
		if records[i].ID == id {
			return &records[i], nil
		}
	}
	return nil, &unifi.NotFoundError{}
}

// ListDNSRecord returns all DNS records for the site.
func (c *Client) ListDNSRecord(ctx context.Context, site string) ([]unifi.DNSRecord, error) {
	var records []unifi.DNSRecord
	if err := c.doDNSRecordRequest(ctx, http.MethodGet,
		fmt.Sprintf("%s%s/v2/api/site/%s/static-dns", c.BaseURL, c.APIPath, site),
		nil, &records); err != nil {
		return nil, err
	}
	return records, nil
}

// UpdateDNSRecord writes the full DNS record at its ID.
func (c *Client) UpdateDNSRecord(ctx context.Context, site string, d *unifi.DNSRecord) (*unifi.DNSRecord, error) {
	var result unifi.DNSRecord
	if err := c.doDNSRecordRequest(ctx, http.MethodPut,
		fmt.Sprintf("%s%s/v2/api/site/%s/static-dns/%s", c.BaseURL, c.APIPath, site, d.ID),
		d, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteDNSRecord removes the DNS record by ID.
func (c *Client) DeleteDNSRecord(ctx context.Context, site, id string) error {
	return c.doDNSRecordRequest(ctx, http.MethodDelete,
		fmt.Sprintf("%s%s/v2/api/site/%s/static-dns/%s", c.BaseURL, c.APIPath, site, id),
		nil, nil)
}

// doDNSRecordRequest reuses the shared HTTP layer but translates 404 responses
// into the local *unifi.NotFoundError so resource Read() can distinguish
// "deleted externally" from real errors.
func (c *Client) doDNSRecordRequest(ctx context.Context, method, url string, body, result any) error {
	err := c.doV2Request(ctx, method, url, body, result)
	if err != nil && strings.Contains(err.Error(), "(404)") {
		return &unifi.NotFoundError{}
	}
	return err
}
