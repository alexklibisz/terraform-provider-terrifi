package provider

// TODO(go-unifi): This file implements the batch-reorder endpoint for firewall
// policies, which is not supported by the go-unifi SDK. When the SDK adds
// support for PUT /v2/api/site/{site}/firewall-policies/batch-reorder, this
// file can be deleted and the resource can use the SDK's built-in method.

import (
	"context"
	"fmt"
	"net/http"
	"sort"
)

// firewallPolicyReorderRequest is the payload for
// PUT /v2/api/site/{site}/firewall-policies/batch-reorder.
type firewallPolicyReorderRequest struct {
	SourceZoneID        string   `json:"source_zone_id"`
	DestinationZoneID   string   `json:"destination_zone_id"`
	BeforePredefinedIDs []string `json:"before_predefined_ids"`
	AfterPredefinedIDs  []string `json:"after_predefined_ids"`
}

// ReorderFirewallPolicies calls the batch-reorder endpoint to set the
// evaluation order of firewall policies for a given zone pair. The policyIDs
// are placed in before_predefined_ids (custom policies ordered before system
// defaults).
func (c *Client) ReorderFirewallPolicies(ctx context.Context, site, sourceZoneID, destZoneID string, policyIDs []string) ([]firewallPolicyResponse, error) {
	payload := firewallPolicyReorderRequest{
		SourceZoneID:        sourceZoneID,
		DestinationZoneID:   destZoneID,
		BeforePredefinedIDs: policyIDs,
		AfterPredefinedIDs:  []string{},
	}

	var result []firewallPolicyResponse
	err := c.doV2Request(ctx, http.MethodPut,
		fmt.Sprintf("%s%s/v2/api/site/%s/firewall-policies/batch-reorder", c.BaseURL, c.APIPath, site),
		payload, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// GetFirewallPolicyOrdering returns the ordered list of non-predefined policy
// IDs for a given zone pair, sorted by their controller-assigned index.
func (c *Client) GetFirewallPolicyOrdering(ctx context.Context, site, sourceZoneID, destZoneID string) ([]string, error) {
	policies, err := c.ListFirewallPolicies(ctx, site)
	if err != nil {
		return nil, err
	}

	// Filter to policies matching the zone pair, excluding predefined ones.
	type indexedPolicy struct {
		id    string
		index int64
	}
	var matched []indexedPolicy
	for _, p := range policies {
		if p.Predefined {
			continue
		}
		if p.Source == nil || p.Destination == nil {
			continue
		}
		if p.Source.ZoneID == sourceZoneID && p.Destination.ZoneID == destZoneID {
			idx := int64(0)
			if p.Index != nil {
				idx = *p.Index
			}
			matched = append(matched, indexedPolicy{id: p.ID, index: idx})
		}
	}

	sort.Slice(matched, func(i, j int) bool {
		return matched[i].index < matched[j].index
	})

	ids := make([]string, len(matched))
	for i, m := range matched {
		ids[i] = m.id
	}
	return ids, nil
}
