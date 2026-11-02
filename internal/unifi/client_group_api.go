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
