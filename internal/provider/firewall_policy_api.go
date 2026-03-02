package provider

// TODO(go-unifi): This file works around bugs in the go-unifi SDK for firewall
// policy CRUD operations. When the upstream SDK fixes these issues, this file
// can be deleted and the resource can use the SDK's built-in methods directly
// (c.ApiClient.Create/Update/DeleteFirewallPolicy). The bugs are:
//
//  1. SDK's DeleteFirewallPolicy only treats HTTP 200 as success. The v2
//     firewall policy DELETE endpoint returns 204 No Content on success, which
//     the SDK misinterprets as an error. (Same bug as firewall zones.)
//     Fix needed in SDK: accept any 2xx status code as success.
//
//  2. SDK serializes all boolean fields without omitempty (enabled, logging,
//     match_ip_sec, create_allow_respond, predefined, match_opposite_protocol)
//     and sends `connection_states: null`, which may cause API issues.
//     Fix needed in SDK: add omitempty to boolean fields and handle nil slices.
//
//  3. SDK's UpdateFirewallPolicy does not include `_id` in the PUT request
//     body (only in the URL path). The v2 API requires it in both places.
//     Fix needed in SDK: include ID in the request body for PUT calls.
//
//  4. SDK's FirewallPolicySource/Destination structs define `port` as *int64,
//     but the v2 API returns `port` as a JSON string (e.g. "443"). The SDK
//     fails to unmarshal this, breaking all GET/list operations.
//     Fix needed in SDK: use json.Number or a custom unmarshaler for port.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/ubiquiti-community/go-unifi/unifi"
)

// firewallPolicyCreateRequest is the payload for POST /v2/api/site/{site}/firewall-policies.
// Uses a bespoke struct to control omitempty on boolean and slice fields.
type firewallPolicyCreateRequest struct {
	Name                string                         `json:"name"`
	Description         string                         `json:"description,omitempty"`
	Enabled             *bool                          `json:"enabled,omitempty"`
	Action              string                         `json:"action"`
	IPVersion           string                         `json:"ip_version,omitempty"`
	Protocol            string                         `json:"protocol,omitempty"`
	ConnectionStateType string                         `json:"connection_state_type,omitempty"`
	ConnectionStates    []string                       `json:"connection_states,omitempty"`
	MatchIPSec          *bool                          `json:"match_ip_sec,omitempty"`
	Logging             *bool                          `json:"logging,omitempty"`
	CreateAllowRespond  *bool                          `json:"create_allow_respond,omitempty"`
	Index               *int64                         `json:"index,omitempty"`
	Source              *firewallPolicyEndpointRequest `json:"source,omitempty"`
	Destination         *firewallPolicyEndpointRequest `json:"destination,omitempty"`
	Schedule            *firewallPolicyScheduleRequest `json:"schedule,omitempty"`
	ICMPTypename        string                         `json:"icmp_typename,omitempty"`
	ICMPV6Typename      string                         `json:"icmp_v6_typename,omitempty"`
}

// firewallPolicyUpdateRequest is the payload for PUT /v2/api/site/{site}/firewall-policies/{id}.
// Includes _id to work around SDK bug #3 above.
type firewallPolicyUpdateRequest struct {
	ID string `json:"_id"`
	firewallPolicyCreateRequest
}

// firewallPolicyEndpointRequest is the source/destination nested object.
type firewallPolicyEndpointRequest struct {
	ZoneID             string   `json:"zone_id"`
	MatchingTarget     string   `json:"matching_target,omitempty"`
	MatchingTargetType string   `json:"matching_target_type,omitempty"`
	IPs                []string `json:"ips,omitempty"`
	MACs               []string `json:"macs,omitempty"`
	PortMatchingType   string   `json:"port_matching_type,omitempty"`
	Port               *int64   `json:"port,omitempty"`
	PortGroupID        string   `json:"port_group_id,omitempty"`
}

// firewallPolicyScheduleRequest is the schedule nested object.
type firewallPolicyScheduleRequest struct {
	Mode           string   `json:"mode,omitempty"`
	Date           string   `json:"date,omitempty"`
	TimeAllDay     *bool    `json:"time_all_day,omitempty"`
	TimeRangeStart string   `json:"time_range_start,omitempty"`
	TimeRangeEnd   string   `json:"time_range_end,omitempty"`
	RepeatOnDays   []string `json:"repeat_on_days,omitempty"`
}

// CreateFirewallPolicy creates a firewall policy via the v2 API, bypassing the
// SDK to control boolean serialization.
func (c *Client) CreateFirewallPolicy(ctx context.Context, site string, d *unifi.FirewallPolicy) (*unifi.FirewallPolicy, error) {
	payload := buildFirewallPolicyCreateRequest(d)

	var result firewallPolicyResponse
	err := c.doV2Request(ctx, http.MethodPost,
		fmt.Sprintf("%s%s/v2/api/site/%s/firewall-policies", c.BaseURL, c.APIPath, site),
		payload, &result)
	if err != nil {
		return nil, err
	}
	return result.toSDK(), nil
}

// UpdateFirewallPolicy updates a firewall policy via the v2 API, bypassing the
// SDK to include _id in the PUT body and control boolean serialization.
func (c *Client) UpdateFirewallPolicy(ctx context.Context, site string, d *unifi.FirewallPolicy) (*unifi.FirewallPolicy, error) {
	create := buildFirewallPolicyCreateRequest(d)
	payload := firewallPolicyUpdateRequest{
		ID:                          d.ID,
		firewallPolicyCreateRequest: create,
	}

	var result firewallPolicyResponse
	err := c.doV2Request(ctx, http.MethodPut,
		fmt.Sprintf("%s%s/v2/api/site/%s/firewall-policies/%s", c.BaseURL, c.APIPath, site, d.ID),
		payload, &result)
	if err != nil {
		return nil, err
	}
	return result.toSDK(), nil
}

// DeleteFirewallPolicy deletes a firewall policy via the v2 API, bypassing the
// SDK to handle 204 No Content responses.
func (c *Client) DeleteFirewallPolicy(ctx context.Context, site string, id string) error {
	return c.doV2Request(ctx, http.MethodDelete,
		fmt.Sprintf("%s%s/v2/api/site/%s/firewall-policies/%s", c.BaseURL, c.APIPath, site, id),
		struct{}{}, nil)
}

// ListFirewallPolicies returns all firewall policies for the given site.
// Reuses the same workaround as GetFirewallPolicy (custom response struct with
// string port field).
func (c *Client) ListFirewallPolicies(ctx context.Context, site string) ([]*unifi.FirewallPolicy, error) {
	var rawPolicies []firewallPolicyResponse
	err := c.doV2Request(ctx, http.MethodGet,
		fmt.Sprintf("%s%s/v2/api/site/%s/firewall-policies", c.BaseURL, c.APIPath, site),
		struct{}{}, &rawPolicies)
	if err != nil {
		return nil, err
	}

	policies := make([]*unifi.FirewallPolicy, len(rawPolicies))
	for i := range rawPolicies {
		policies[i] = rawPolicies[i].toSDK()
	}
	return policies, nil
}

// TODO(go-unifi): GetFirewallPolicy uses a custom implementation because the
// SDK's generated FirewallPolicySource/Destination struct defines `port` as
// *int64, but the v2 API returns `port` as a JSON string (e.g. "443"). The SDK
// fails to unmarshal this. When the SDK fixes the port field type (or adds a
// custom unmarshaler), this can be replaced with c.ApiClient.GetFirewallPolicy().
func (c *Client) GetFirewallPolicy(ctx context.Context, site string, id string) (*unifi.FirewallPolicy, error) {
	var rawPolicies []firewallPolicyResponse
	err := c.doV2Request(ctx, http.MethodGet,
		fmt.Sprintf("%s%s/v2/api/site/%s/firewall-policies", c.BaseURL, c.APIPath, site),
		struct{}{}, &rawPolicies)
	if err != nil {
		return nil, err
	}

	for _, raw := range rawPolicies {
		if raw.ID == id {
			return raw.toSDK(), nil
		}
	}
	return nil, &unifi.NotFoundError{}
}

// firewallPolicyResponse mirrors the API's JSON response shape where `port`
// is a string instead of int64. We unmarshal into this and convert to the SDK
// struct.
type firewallPolicyResponse struct {
	ID                  string                          `json:"_id"`
	Name                string                          `json:"name"`
	Description         string                          `json:"description"`
	Enabled             bool                            `json:"enabled"`
	Action              string                          `json:"action"`
	IPVersion           string                          `json:"ip_version"`
	Protocol            string                          `json:"protocol"`
	ConnectionStateType string                          `json:"connection_state_type"`
	ConnectionStates    []string                        `json:"connection_states"`
	CreateAllowRespond  bool                            `json:"create_allow_respond"`
	Logging             bool                            `json:"logging"`
	MatchIPSec          bool                            `json:"match_ip_sec"`
	Index               *int64                          `json:"index"`
	Source              *firewallPolicyEndpointResponse `json:"source"`
	Destination         *firewallPolicyEndpointResponse `json:"destination"`
	Schedule            *firewallPolicyScheduleRequest  `json:"schedule"`
}

type firewallPolicyEndpointResponse struct {
	ZoneID           string          `json:"zone_id"`
	MatchingTarget   string          `json:"matching_target"`
	IPs              []string        `json:"ips"`
	MACs             []string        `json:"macs"`
	PortMatchingType string          `json:"port_matching_type"`
	Port             json.RawMessage `json:"port"`
	PortGroupID      string          `json:"port_group_id"`
}

func (r *firewallPolicyResponse) toSDK() *unifi.FirewallPolicy {
	p := &unifi.FirewallPolicy{
		ID:                  r.ID,
		Name:                r.Name,
		Description:         r.Description,
		Enabled:             r.Enabled,
		Action:              r.Action,
		IPVersion:           r.IPVersion,
		Protocol:            r.Protocol,
		ConnectionStateType: r.ConnectionStateType,
		ConnectionStates:    r.ConnectionStates,
		CreateAllowRespond:  r.CreateAllowRespond,
		Logging:             r.Logging,
		MatchIPSec:          r.MatchIPSec,
		Index:               r.Index,
	}

	if r.Source != nil {
		p.Source = r.Source.toSDKSource()
	}
	if r.Destination != nil {
		p.Destination = r.Destination.toSDKDestination()
	}
	if r.Schedule != nil {
		p.Schedule = &unifi.FirewallPolicySchedule{
			Mode:           r.Schedule.Mode,
			Date:           r.Schedule.Date,
			TimeRangeStart: r.Schedule.TimeRangeStart,
			TimeRangeEnd:   r.Schedule.TimeRangeEnd,
			RepeatOnDays:   r.Schedule.RepeatOnDays,
		}
		if r.Schedule.TimeAllDay != nil {
			p.Schedule.TimeAllDay = *r.Schedule.TimeAllDay
		}
	}

	return p
}

func (ep *firewallPolicyEndpointResponse) parsePort() *int64 {
	if len(ep.Port) == 0 || string(ep.Port) == "null" {
		return nil
	}
	// Try parsing as number first, then as quoted string.
	var n int64
	if err := json.Unmarshal(ep.Port, &n); err == nil {
		return &n
	}
	var s string
	if err := json.Unmarshal(ep.Port, &s); err == nil {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil {
			return &v
		}
	}
	return nil
}

func (ep *firewallPolicyEndpointResponse) toSDKSource() *unifi.FirewallPolicySource {
	return &unifi.FirewallPolicySource{
		ZoneID:           ep.ZoneID,
		MatchingTarget:   ep.MatchingTarget,
		IPs:              ep.resolveIPs(),
		PortMatchingType: ep.PortMatchingType,
		Port:             ep.parsePort(),
		PortGroupID:      ep.PortGroupID,
	}
}

func (ep *firewallPolicyEndpointResponse) toSDKDestination() *unifi.FirewallPolicyDestination {
	return &unifi.FirewallPolicyDestination{
		ZoneID:           ep.ZoneID,
		MatchingTarget:   ep.MatchingTarget,
		IPs:              ep.resolveIPs(),
		PortMatchingType: ep.PortMatchingType,
		Port:             ep.parsePort(),
		PortGroupID:      ep.PortGroupID,
	}
}

// resolveIPs returns the endpoint values, merging the "macs" field back into
// a single slice so the resource layer can handle all target types uniformly
// via the IPs field on the SDK struct. Both MAC and CLIENT matching targets
// store their values in the API's "macs" JSON field.
func (ep *firewallPolicyEndpointResponse) resolveIPs() []string {
	switch ep.MatchingTarget {
	case "MAC", "CLIENT":
		if len(ep.MACs) > 0 {
			return ep.MACs
		}
	}
	return ep.IPs
}

func buildFirewallPolicyCreateRequest(d *unifi.FirewallPolicy) firewallPolicyCreateRequest {
	req := firewallPolicyCreateRequest{
		Name:                d.Name,
		Description:         d.Description,
		Action:              d.Action,
		IPVersion:           d.IPVersion,
		Protocol:            d.Protocol,
		ConnectionStateType: d.ConnectionStateType,
		ConnectionStates:    d.ConnectionStates,
		ICMPTypename:        d.ICMPTypename,
		ICMPV6Typename:      d.ICMPV6Typename,
		Index:               d.Index,
	}

	// Send all booleans that the API expects. The enabled and
	// create_allow_respond fields must always be present; the rest are only
	// included when true to avoid sending unwanted false values.
	req.Enabled = boolPtr(d.Enabled)
	req.CreateAllowRespond = boolPtr(d.CreateAllowRespond)
	if d.Logging {
		req.Logging = boolPtr(true)
	}
	if d.MatchIPSec {
		req.MatchIPSec = boolPtr(true)
	}

	if d.Source != nil {
		req.Source = buildEndpointRequest(d.Source.ZoneID, d.Source.MatchingTarget, d.Source.IPs, d.Source.PortMatchingType, d.Source.Port, d.Source.PortGroupID)
	}

	if d.Destination != nil {
		req.Destination = buildEndpointRequest(d.Destination.ZoneID, d.Destination.MatchingTarget, d.Destination.IPs, d.Destination.PortMatchingType, d.Destination.Port, d.Destination.PortGroupID)
	}

	if d.Schedule != nil {
		sched := &firewallPolicyScheduleRequest{
			Mode:           d.Schedule.Mode,
			Date:           d.Schedule.Date,
			TimeRangeStart: d.Schedule.TimeRangeStart,
			TimeRangeEnd:   d.Schedule.TimeRangeEnd,
			RepeatOnDays:   d.Schedule.RepeatOnDays,
		}
		if d.Schedule.TimeAllDay {
			sched.TimeAllDay = boolPtr(true)
		}
		req.Schedule = sched
	} else {
		// The UniFi API requires schedule to be non-null; default to ALWAYS.
		req.Schedule = &firewallPolicyScheduleRequest{
			Mode: "ALWAYS",
		}
	}

	return req
}

func buildEndpointRequest(zoneID, matchingTarget string, ips []string, portMatchingType string, port *int64, portGroupID string) *firewallPolicyEndpointRequest {
	ep := &firewallPolicyEndpointRequest{
		ZoneID:             zoneID,
		MatchingTarget:     matchingTarget,
		MatchingTargetType: matchingTargetType(matchingTarget),
		PortMatchingType:   portMatchingType,
		Port:               port,
		PortGroupID:        portGroupID,
	}
	// The API expects MAC and CLIENT values in the "macs" field, not "ips".
	if matchingTarget == "MAC" || matchingTarget == "CLIENT" {
		ep.MACs = ips
	} else {
		ep.IPs = ips
	}
	return ep
}

func boolPtr(b bool) *bool { return &b }

// matchingTargetType derives matching_target_type from matching_target.
// The v2 API requires both fields: matching_target identifies WHAT to match
// (IP, NETWORK, etc.) and matching_target_type indicates how (ANY vs SPECIFIC).
// matchingTargetType derives matching_target_type from matching_target.
// The v2 API requires this field when matching_target is not ANY. The enum
// only accepts SPECIFIC or OBJECT (not ANY), so we omit it for ANY targets.
func matchingTargetType(matchingTarget string) string {
	if matchingTarget == "" || matchingTarget == "ANY" {
		return "" // omitempty will exclude it from the JSON
	}
	return "SPECIFIC"
}
