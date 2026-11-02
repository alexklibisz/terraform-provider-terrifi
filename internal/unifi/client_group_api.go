package unifi

import (
	"context"
	"fmt"
	"net/http"
)

func (c *APIClient) CreateNetworkMembersGroup(
	ctx context.Context,
	site string,
	d *NetworkMembersGroup,
) (*NetworkMembersGroup, error) {
	payload := *d
	if payload.Members == nil {
		payload.Members = []string{}
	}

	var result NetworkMembersGroup
	err := c.Do(ctx, http.MethodPost,
		fmt.Sprintf("%s%s/v2/api/site/%s/network-members-group", c.BaseURL, c.APIPath, site),
		payload, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *APIClient) DeleteNetworkMembersGroup(ctx context.Context, site string, id string) error {
	return c.Do(ctx, http.MethodDelete,
		fmt.Sprintf("%s%s/v2/api/site/%s/network-members-group/%s", c.BaseURL, c.APIPath, site, id),
		struct{}{}, nil)
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
