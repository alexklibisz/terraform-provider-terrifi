package provider

import (
	"context"

	"github.com/alexklibisz/terrifi/internal/unifi"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Client adds Terraform site resolution to the independent controller client.
type Client struct{ *unifi.APIClient }

type ClientConfig = unifi.Config

var ClientConfigFromEnv = unifi.ConfigFromEnv

func NewClient(ctx context.Context, cfg ClientConfig) (*Client, error) {
	api, err := unifi.New(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &Client{APIClient: api}, nil
}

// SiteOrDefault returns the given site if non-empty, otherwise falls back to the
// provider's default site. Every resource calls this to resolve which site to operate on,
// since the site attribute is optional on individual resources.
func (c *Client) SiteOrDefault(site types.String) string {
	if v := site.ValueString(); v != "" {
		return v
	}
	return c.Site
}
