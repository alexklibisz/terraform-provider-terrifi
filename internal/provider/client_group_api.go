package provider

// TODO(go-unifi): This file works around bugs in the go-unifi SDK
// (github.com/ubiquiti-community/go-unifi) for the v2 network-members-group
// endpoint. When the upstream SDK fixes these issues, this file can be deleted
// and the client_group resource can use the SDK's built-in methods directly.
// The upstream bugs are:
//
//  1. SDK's CreateNetworkMembersGroup POSTs to
//     `v2/api/site/{site}/network-members-groups` (plural). The controller
//     only exposes POST on the singular path
//     `v2/api/site/{site}/network-members-group`; the plural path returns 405.
//     Fix needed in SDK: change the POST URL to the singular form.
//
//  2. SDK's do() only treats HTTP 200 as success. The v2
//     network-members-group POST returns 201 Created and DELETE returns 204
//     No Content, both of which the SDK misinterprets as errors.
//     Fix needed in SDK: accept any 2xx status code as success.

import (
	"context"
	"fmt"
	"net/http"

	"github.com/ubiquiti-community/go-unifi/unifi"
)

// CreateNetworkMembersGroup creates a network-members-group via the v2 API,
// bypassing the SDK to avoid bugs #1 (wrong POST URL) and #2 (201 status
// treated as error).
func (c *Client) CreateNetworkMembersGroup(
	ctx context.Context,
	site string,
	d *unifi.NetworkMembersGroup,
) (*unifi.NetworkMembersGroup, error) {
	payload := *d
	if payload.Members == nil {
		payload.Members = []string{}
	}

	var result unifi.NetworkMembersGroup
	err := c.doV2Request(ctx, http.MethodPost,
		fmt.Sprintf("%s%s/v2/api/site/%s/network-members-group", c.BaseURL, c.APIPath, site),
		payload, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteNetworkMembersGroup deletes a network-members-group via the v2 API,
// bypassing the SDK to avoid bug #2 (204 No Content treated as error).
func (c *Client) DeleteNetworkMembersGroup(ctx context.Context, site string, id string) error {
	return c.doV2Request(ctx, http.MethodDelete,
		fmt.Sprintf("%s%s/v2/api/site/%s/network-members-group/%s", c.BaseURL, c.APIPath, site, id),
		struct{}{}, nil)
}
