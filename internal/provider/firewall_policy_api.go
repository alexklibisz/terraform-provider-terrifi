package provider

// Local CRUD methods for the v2 firewall-policies endpoint. These shadow the
// promoted go-unifi methods on *Client and use the local internal/unifi types
// instead, removing the SDK dependency for this resource — see issue #157.
//
// The controller's wire-format quirks for this endpoint:
//
//  1. DELETE returns 204 No Content. doV2Request treats any 2xx as success.
//
//  2. The boolean fields the controller cares about (enabled, logging,
//     match_ip_sec, create_allow_respond) cannot be serialized as plain bool
//     because we need to distinguish "send false" from "omit entirely" for a
//     few of them. The bespoke firewallPolicyCreateRequest below uses *bool
//     with omitempty for that fine-grained control.
//
//  3. PUT requires `_id` in the body (not just the URL); firewallPolicyUpdateRequest
//     embeds it so it always ships.
//
//  4. The endpoint emits `port` (inside source/destination) as a JSON string
//     on read but expects a number on write. firewallPolicyEndpointResponse
//     stores it as json.RawMessage and parsePort() coerces both shapes.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/alexklibisz/terrifi/internal/unifi"
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
	ClientMACs         []string `json:"client_macs,omitempty"`
	PortMatchingType   string   `json:"port_matching_type,omitempty"`
	Port               *int64   `json:"port,omitempty"`
	PortGroupID        string   `json:"port_group_id,omitempty"`
	MatchOppositePorts *bool    `json:"match_opposite_ports,omitempty"`
	MatchOppositeIPs   *bool    `json:"match_opposite_ips,omitempty"`
}

// firewallPolicyScheduleRequest is the schedule nested object.
type firewallPolicyScheduleRequest struct {
	Mode           string   `json:"mode,omitempty"`
	Date           string   `json:"date,omitempty"`
	TimeAllDay     *bool    `json:"time_all_day,omitempty"`
	TimeRangeStart string   `json:"time_range_start,omitempty"`
	TimeRangeEnd   string   `json:"time_range_end,omitempty"`
	RepeatOnDays   []string `json:"repeat_on_days,omitempty"`
	DateStart string `json:"date_start,omitempty"`
	DateEnd   string `json:"date_end,omitempty"`
}

// firewallPolicyFull wraps *unifi.FirewallPolicy with the raw schedule from the
// API response, preserving fields (date_range_start, date_range_end) that are not
// present in the SDK's FirewallPolicySchedule struct.
type firewallPolicyFull struct {
	*unifi.FirewallPolicy
	RawSchedule *firewallPolicyScheduleRequest
}

// CreateFirewallPolicy creates a firewall policy via the v2 API, bypassing the
// SDK to control boolean serialization. schedOverride, when non-nil, is used as
// the schedule payload instead of deriving it from d.Schedule, allowing callers
// to include fields (e.g. date_range_start, date_range_end) not in the SDK struct.
func (c *Client) CreateFirewallPolicy(ctx context.Context, site string, d *unifi.FirewallPolicy, schedOverride *firewallPolicyScheduleRequest) (*firewallPolicyFull, error) {
	payload := buildFirewallPolicyCreateRequest(d, schedOverride)

	var result firewallPolicyResponse
	err := c.doV2Request(ctx, http.MethodPost,
		fmt.Sprintf("%s%s/v2/api/site/%s/firewall-policies", c.BaseURL, c.APIPath, site),
		payload, &result)
	if err != nil {
		return nil, err
	}
	return result.toFull(), nil
}

// UpdateFirewallPolicy updates a firewall policy via the v2 API, bypassing the
// SDK to include _id in the PUT body and control boolean serialization. schedOverride,
// when non-nil, is used as the schedule payload instead of deriving it from d.Schedule.
func (c *Client) UpdateFirewallPolicy(ctx context.Context, site string, d *unifi.FirewallPolicy, schedOverride *firewallPolicyScheduleRequest) (*firewallPolicyFull, error) {
	create := buildFirewallPolicyCreateRequest(d, schedOverride)
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
	return result.toFull(), nil
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

// GetFirewallPolicy lists all policies and filters by ID. The v2 endpoint
// does not expose a per-ID GET, and listing-then-filtering matches the
// pattern used by GetFirewallZone.
func (c *Client) GetFirewallPolicy(ctx context.Context, site string, id string) (*firewallPolicyFull, error) {
	var rawPolicies []firewallPolicyResponse
	err := c.doV2Request(ctx, http.MethodGet,
		fmt.Sprintf("%s%s/v2/api/site/%s/firewall-policies", c.BaseURL, c.APIPath, site),
		struct{}{}, &rawPolicies)
	if err != nil {
		return nil, err
	}

	for _, raw := range rawPolicies {
		if raw.ID == id {
			return raw.toFull(), nil
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
	Predefined          bool                            `json:"predefined"`
	Index               *int64                          `json:"index"`
	Source              *firewallPolicyEndpointResponse `json:"source"`
	Destination         *firewallPolicyEndpointResponse `json:"destination"`
	Schedule            *firewallPolicyScheduleRequest  `json:"schedule"`
}

type firewallPolicyEndpointResponse struct {
	ZoneID             string          `json:"zone_id"`
	MatchingTarget     string          `json:"matching_target"`
	IPs                []string        `json:"ips"`
	MACs               []string        `json:"macs"`
	ClientMACs         []string        `json:"client_macs"`
	PortMatchingType   string          `json:"port_matching_type"`
	Port               json.RawMessage `json:"port"`
	PortGroupID        string          `json:"port_group_id"`
	MatchOppositePorts bool            `json:"match_opposite_ports"`
	MatchOppositeIPs   bool            `json:"match_opposite_ips"`
}

func (r *firewallPolicyResponse) toFull() *firewallPolicyFull {
	return &firewallPolicyFull{
		FirewallPolicy: r.toSDK(),
		RawSchedule:    r.Schedule,
	}
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
		Predefined:          r.Predefined,
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
		ZoneID:             ep.ZoneID,
		MatchingTarget:     ep.MatchingTarget,
		IPs:                ep.resolveIPs(),
		PortMatchingType:   ep.PortMatchingType,
		Port:               ep.parsePort(),
		PortGroupID:        ep.PortGroupID,
		MatchOppositePorts: ep.MatchOppositePorts,
		MatchOppositeIPs:   ep.MatchOppositeIPs,
	}
}

func (ep *firewallPolicyEndpointResponse) toSDKDestination() *unifi.FirewallPolicyDestination {
	return &unifi.FirewallPolicyDestination{
		ZoneID:             ep.ZoneID,
		MatchingTarget:     ep.MatchingTarget,
		IPs:                ep.resolveIPs(),
		PortMatchingType:   ep.PortMatchingType,
		Port:               ep.parsePort(),
		PortGroupID:        ep.PortGroupID,
		MatchOppositePorts: ep.MatchOppositePorts,
		MatchOppositeIPs:   ep.MatchOppositeIPs,
	}
}

// resolveIPs returns the endpoint values, merging the "macs" or "client_macs"
// field back into a single slice so the resource layer can handle all target
// types uniformly via the IPs field on the SDK struct.
func (ep *firewallPolicyEndpointResponse) resolveIPs() []string {
	switch ep.MatchingTarget {
	case "IID", "MAC", "CLIENT":
		if len(ep.MACs) > 0 {
			return ep.MACs
		}
	}
	if ep.MatchingTarget == "CLIENT" && len(ep.ClientMACs) > 0 {
		return ep.ClientMACs
	}
	return ep.IPs
}

func buildFirewallPolicyCreateRequest(d *unifi.FirewallPolicy, schedOverride *firewallPolicyScheduleRequest) firewallPolicyCreateRequest {
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
		req.Source = buildEndpointRequest(d.Source.ZoneID, d.Source.MatchingTarget, d.Source.IPs, d.Source.PortMatchingType, d.Source.Port, d.Source.PortGroupID, d.Source.MatchOppositePorts, d.Source.MatchOppositeIPs)
	}

	if d.Destination != nil {
		req.Destination = buildEndpointRequest(d.Destination.ZoneID, d.Destination.MatchingTarget, d.Destination.IPs, d.Destination.PortMatchingType, d.Destination.Port, d.Destination.PortGroupID, d.Destination.MatchOppositePorts, d.Destination.MatchOppositeIPs)
	}

	if schedOverride != nil {
		req.Schedule = schedOverride
	} else if d.Schedule != nil {
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

func buildEndpointRequest(zoneID, matchingTarget string, ips []string, portMatchingType string, port *int64, portGroupID string, matchOppositePorts, matchOppositeIPs bool) *firewallPolicyEndpointRequest {
	ep := &firewallPolicyEndpointRequest{
		ZoneID:             zoneID,
		MatchingTarget:     matchingTarget,
		MatchingTargetType: matchingTargetType(matchingTarget),
		PortMatchingType:   resolvePortMatchingType(portMatchingType, port, portGroupID),
		Port:               port,
		PortGroupID:        portGroupID,
	}
	if matchOppositePorts {
		ep.MatchOppositePorts = boolPtr(true)
	}
	if matchOppositeIPs {
		ep.MatchOppositeIPs = boolPtr(true)
	}
	// The API expects MAC values in the "macs" field and device values in
	// the "client_macs" field, not "ips".
	if matchingTarget == "MAC" {
		ep.MACs = ips
	} else if matchingTarget == "CLIENT" {
		ep.ClientMACs = ips
	} else {
		ep.IPs = ips
	}
	return ep
}

func boolPtr(b bool) *bool { return &b }

// resolvePortMatchingType derives the correct port_matching_type for the API.
// The v2 API accepts SPECIFIC (when a port number is set), OBJECT (when a port
// group ID is set), or ANY (no port filter). This function auto-derives the
// value from what's set, so users don't need to specify port_matching_type
// explicitly.
func resolvePortMatchingType(portMatchingType string, port *int64, portGroupID string) string {
	if portGroupID != "" {
		return "OBJECT"
	}
	if port != nil {
		return "SPECIFIC"
	}
	return portMatchingType
}

// matchingTargetType derives matching_target_type from matching_target.
// The v2 API requires this field when matching_target is not ANY. The enum
// only accepts SPECIFIC or OBJECT (not ANY), so we omit it for ANY targets.
func matchingTargetType(matchingTarget string) string {
	if matchingTarget == "" || matchingTarget == "ANY" {
		return "" // omitempty will exclude it from the JSON
	}
	if matchingTarget == "CLIENT" {
		return "OBJECT"
	}
	return "SPECIFIC"
}
