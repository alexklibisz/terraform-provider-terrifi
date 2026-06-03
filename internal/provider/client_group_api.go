package provider

// Local CRUD methods for the v2 network-members-group endpoint. These shadow
// the promoted go-unifi methods on *Client and use the local internal/unifi
// types instead, removing the SDK dependency for this resource — see
// issue #157.
//
// One non-obvious URL quirk: LIST uses the plural path
// (/network-members-groups) while every other verb uses the singular path
// (/network-members-group[/id]). This is what the controller actually
// expects; the SDK's CreateNetworkMembersGroup posting to the plural URL
// was the original motivation for bypassing it.

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/alexklibisz/terrifi/internal/unifi"
)

// CreateNetworkMembersGroup posts a new group to the singular URL.
func (c *Client) CreateNetworkMembersGroup(ctx context.Context, site string, d *unifi.NetworkMembersGroup) (*unifi.NetworkMembersGroup, error) {
	payload := *d
	if payload.Members == nil {
		payload.Members = []string{}
	}

	var result unifi.NetworkMembersGroup
	if err := c.doClientGroupRequest(ctx, http.MethodPost,
		fmt.Sprintf("%s%s/v2/api/site/%s/network-members-group", c.BaseURL, c.APIPath, site),
		payload, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetNetworkMembersGroup reads a group by ID.
func (c *Client) GetNetworkMembersGroup(ctx context.Context, site, id string) (*unifi.NetworkMembersGroup, error) {
	var result unifi.NetworkMembersGroup
	if err := c.doClientGroupRequest(ctx, http.MethodGet,
		fmt.Sprintf("%s%s/v2/api/site/%s/network-members-group/%s", c.BaseURL, c.APIPath, site, id),
		nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateNetworkMembersGroup writes the full group at its ID.
func (c *Client) UpdateNetworkMembersGroup(ctx context.Context, site string, d *unifi.NetworkMembersGroup) (*unifi.NetworkMembersGroup, error) {
	payload := *d
	if payload.Members == nil {
		payload.Members = []string{}
	}

	var result unifi.NetworkMembersGroup
	if err := c.doClientGroupRequest(ctx, http.MethodPut,
		fmt.Sprintf("%s%s/v2/api/site/%s/network-members-group/%s", c.BaseURL, c.APIPath, site, d.ID),
		payload, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteNetworkMembersGroup removes a group by ID.
func (c *Client) DeleteNetworkMembersGroup(ctx context.Context, site, id string) error {
	return c.doClientGroupRequest(ctx, http.MethodDelete,
		fmt.Sprintf("%s%s/v2/api/site/%s/network-members-group/%s", c.BaseURL, c.APIPath, site, id),
		nil, nil)
}

// ListNetworkMembersGroups returns all groups for the site. Uses the plural
// URL; see the file-level note above.
func (c *Client) ListNetworkMembersGroups(ctx context.Context, site string) ([]unifi.NetworkMembersGroup, error) {
	var result []unifi.NetworkMembersGroup
	if err := c.doClientGroupRequest(ctx, http.MethodGet,
		fmt.Sprintf("%s%s/v2/api/site/%s/network-members-groups", c.BaseURL, c.APIPath, site),
		nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// doClientGroupRequest reuses the shared HTTP layer but translates 404
// responses into the local *unifi.NotFoundError so resource Read() can
// distinguish "deleted externally" from real errors.
func (c *Client) doClientGroupRequest(ctx context.Context, method, url string, body, result any) error {
	err := c.doV2Request(ctx, method, url, body, result)
	if err != nil && strings.Contains(err.Error(), "(404)") {
		return &unifi.NotFoundError{}
	}
	return err
}
