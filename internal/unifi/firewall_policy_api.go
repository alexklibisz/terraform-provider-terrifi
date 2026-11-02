package unifi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
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
	Schedule            *FirewallPolicyScheduleRequest `json:"schedule,omitempty"`
	ICMPTypename        string                         `json:"icmp_typename,omitempty"`
	ICMPV6Typename      string                         `json:"icmp_v6_typename,omitempty"`
}

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
	NetworkIDs         []string `json:"network_ids,omitempty"`
	PortMatchingType   string   `json:"port_matching_type,omitempty"`
	Port               *int64   `json:"port,omitempty"`
	PortGroupID        string   `json:"port_group_id,omitempty"`
	MatchOppositePorts *bool    `json:"match_opposite_ports,omitempty"`
	MatchOppositeIPs   *bool    `json:"match_opposite_ips,omitempty"`
	// NETWORK targets use their own invert flag instead of match_opposite_ips.
	MatchOppositeNetworks *bool `json:"match_opposite_networks,omitempty"`
}

// FirewallPolicyScheduleRequest is the schedule nested object.
type FirewallPolicyScheduleRequest struct {
	Mode           string   `json:"mode,omitempty"`
	Date           string   `json:"date,omitempty"`
	TimeAllDay     *bool    `json:"time_all_day,omitempty"`
	TimeRangeStart string   `json:"time_range_start,omitempty"`
	TimeRangeEnd   string   `json:"time_range_end,omitempty"`
	RepeatOnDays   []string `json:"repeat_on_days,omitempty"`
	DateStart      string   `json:"date_start,omitempty"`
	DateEnd        string   `json:"date_end,omitempty"`
}

type FirewallPolicyFull struct {
	*FirewallPolicy
	RawSchedule *FirewallPolicyScheduleRequest
}

func (c *APIClient) CreateFirewallPolicy(ctx context.Context, site string, d *FirewallPolicy, schedOverride *FirewallPolicyScheduleRequest) (*FirewallPolicyFull, error) {
	payload := BuildFirewallPolicyCreateRequest(d, schedOverride)

	var result FirewallPolicyResponse
	err := c.Do(ctx, http.MethodPost,
		fmt.Sprintf("%s%s/v2/api/site/%s/firewall-policies", c.BaseURL, c.APIPath, site),
		payload, &result)
	if err != nil {
		return nil, err
	}
	return result.toFull(), nil
}

func (c *APIClient) UpdateFirewallPolicy(ctx context.Context, site string, d *FirewallPolicy, schedOverride *FirewallPolicyScheduleRequest) (*FirewallPolicyFull, error) {
	create := BuildFirewallPolicyCreateRequest(d, schedOverride)
	payload := firewallPolicyUpdateRequest{
		ID:                          d.ID,
		firewallPolicyCreateRequest: create,
	}

	var result FirewallPolicyResponse
	err := c.Do(ctx, http.MethodPut,
		fmt.Sprintf("%s%s/v2/api/site/%s/firewall-policies/%s", c.BaseURL, c.APIPath, site, d.ID),
		payload, &result)
	if err != nil {
		return nil, err
	}
	return result.toFull(), nil
}

func (c *APIClient) DeleteFirewallPolicy(ctx context.Context, site string, id string) error {
	return c.Do(ctx, http.MethodDelete,
		fmt.Sprintf("%s%s/v2/api/site/%s/firewall-policies/%s", c.BaseURL, c.APIPath, site, id),
		struct{}{}, nil)
}

// ListFirewallPolicies returns all firewall policies for the given site.
// Reuses the same workaround as GetFirewallPolicy (custom response struct with
// string port field).
func (c *APIClient) ListFirewallPolicies(ctx context.Context, site string) ([]*FirewallPolicy, error) {
	var rawPolicies []FirewallPolicyResponse
	err := c.Do(ctx, http.MethodGet,
		fmt.Sprintf("%s%s/v2/api/site/%s/firewall-policies", c.BaseURL, c.APIPath, site),
		struct{}{}, &rawPolicies)
	if err != nil {
		return nil, err
	}

	policies := make([]*FirewallPolicy, len(rawPolicies))
	for i := range rawPolicies {
		policies[i] = rawPolicies[i].ToPolicy()
	}
	return policies, nil
}

func (c *APIClient) GetFirewallPolicy(ctx context.Context, site string, id string) (*FirewallPolicyFull, error) {
	return c.GetFirewallPolicyWithCache(ctx, site, id, true)
}

// GetFirewallPolicyWithCache can bypass cached reads when verifying a controller write.
func (c *APIClient) GetFirewallPolicyWithCache(ctx context.Context, site string, id string, useCached bool) (*FirewallPolicyFull, error) {
	var rawPolicies []FirewallPolicyResponse
	err := c.doRequest(ctx, http.MethodGet,
		fmt.Sprintf("%s%s/v2/api/site/%s/firewall-policies", c.BaseURL, c.APIPath, site),
		struct{}{}, &rawPolicies, useCached)
	if err != nil {
		return nil, err
	}

	for _, raw := range rawPolicies {
		if raw.ID == id {
			return raw.toFull(), nil
		}
	}
	return nil, &NotFoundError{}
}

type FirewallPolicyResponse struct {
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
	Source              *FirewallPolicyEndpointResponse `json:"source"`
	Destination         *FirewallPolicyEndpointResponse `json:"destination"`
	Schedule            *FirewallPolicyScheduleRequest  `json:"schedule"`
}

type FirewallPolicyEndpointResponse struct {
	ZoneID                string          `json:"zone_id"`
	MatchingTarget        string          `json:"matching_target"`
	IPs                   []string        `json:"ips"`
	MACs                  []string        `json:"macs"`
	ClientMACs            []string        `json:"client_macs"`
	NetworkIDs            []string        `json:"network_ids"`
	PortMatchingType      string          `json:"port_matching_type"`
	Port                  json.RawMessage `json:"port"`
	PortGroupID           string          `json:"port_group_id"`
	MatchOppositePorts    bool            `json:"match_opposite_ports"`
	MatchOppositeIPs      bool            `json:"match_opposite_ips"`
	MatchOppositeNetworks bool            `json:"match_opposite_networks"`
}

func (r *FirewallPolicyResponse) toFull() *FirewallPolicyFull {
	return &FirewallPolicyFull{
		FirewallPolicy: r.ToPolicy(),
		RawSchedule:    r.Schedule,
	}
}

func (r *FirewallPolicyResponse) ToPolicy() *FirewallPolicy {
	p := &FirewallPolicy{
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
		p.Source = r.Source.ToSource()
	}
	if r.Destination != nil {
		p.Destination = r.Destination.ToDestination()
	}
	if r.Schedule != nil {
		p.Schedule = &FirewallPolicySchedule{
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

func (ep *FirewallPolicyEndpointResponse) ParsePort() *int64 {
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

func (ep *FirewallPolicyEndpointResponse) ToSource() *FirewallPolicySource {
	return &FirewallPolicySource{
		ZoneID:             ep.ZoneID,
		MatchingTarget:     ep.MatchingTarget,
		IPs:                ep.ResolveIPs(),
		PortMatchingType:   ep.PortMatchingType,
		Port:               ep.ParsePort(),
		PortGroupID:        ep.PortGroupID,
		MatchOppositePorts: ep.MatchOppositePorts,
		MatchOppositeIPs:   ep.ResolveMatchOpposite(),
	}
}

func (ep *FirewallPolicyEndpointResponse) ToDestination() *FirewallPolicyDestination {
	return &FirewallPolicyDestination{
		ZoneID:             ep.ZoneID,
		MatchingTarget:     ep.MatchingTarget,
		IPs:                ep.ResolveIPs(),
		PortMatchingType:   ep.PortMatchingType,
		Port:               ep.ParsePort(),
		PortGroupID:        ep.PortGroupID,
		MatchOppositePorts: ep.MatchOppositePorts,
		MatchOppositeIPs:   ep.ResolveMatchOpposite(),
	}
}

func (ep *FirewallPolicyEndpointResponse) ResolveIPs() []string {
	switch ep.MatchingTarget {
	case "NETWORK":
		if len(ep.NetworkIDs) > 0 {
			return ep.NetworkIDs
		}
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

func (ep *FirewallPolicyEndpointResponse) ResolveMatchOpposite() bool {
	if ep.MatchingTarget == "NETWORK" {
		return ep.MatchOppositeNetworks
	}
	return ep.MatchOppositeIPs
}

func BuildFirewallPolicyCreateRequest(d *FirewallPolicy, schedOverride *FirewallPolicyScheduleRequest) firewallPolicyCreateRequest {
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
	req.Enabled = BoolPtr(d.Enabled)
	req.CreateAllowRespond = BoolPtr(d.CreateAllowRespond)
	if d.Logging {
		req.Logging = BoolPtr(true)
	}
	if d.MatchIPSec {
		req.MatchIPSec = BoolPtr(true)
	}

	if d.Source != nil {
		req.Source = BuildEndpointRequest(d.Source.ZoneID, d.Source.MatchingTarget, d.Source.IPs, d.Source.PortMatchingType, d.Source.Port, d.Source.PortGroupID, d.Source.MatchOppositePorts, d.Source.MatchOppositeIPs)
	}

	if d.Destination != nil {
		req.Destination = BuildEndpointRequest(d.Destination.ZoneID, d.Destination.MatchingTarget, d.Destination.IPs, d.Destination.PortMatchingType, d.Destination.Port, d.Destination.PortGroupID, d.Destination.MatchOppositePorts, d.Destination.MatchOppositeIPs)
	}

	if schedOverride != nil {
		req.Schedule = schedOverride
	} else if d.Schedule != nil {
		sched := &FirewallPolicyScheduleRequest{
			Mode:           d.Schedule.Mode,
			Date:           d.Schedule.Date,
			TimeRangeStart: d.Schedule.TimeRangeStart,
			TimeRangeEnd:   d.Schedule.TimeRangeEnd,
			RepeatOnDays:   d.Schedule.RepeatOnDays,
		}
		if d.Schedule.TimeAllDay {
			sched.TimeAllDay = BoolPtr(true)
		}
		req.Schedule = sched
	} else {
		// The UniFi API requires schedule to be non-null; default to ALWAYS.
		req.Schedule = &FirewallPolicyScheduleRequest{
			Mode: "ALWAYS",
		}
	}

	return req
}

func BuildEndpointRequest(zoneID, matchingTarget string, ips []string, portMatchingType string, port *int64, portGroupID string, matchOppositePorts, matchOppositeIPs bool) *firewallPolicyEndpointRequest {
	ep := &firewallPolicyEndpointRequest{
		ZoneID:             zoneID,
		MatchingTarget:     matchingTarget,
		MatchingTargetType: MatchingTargetType(matchingTarget),
		PortMatchingType:   ResolvePortMatchingType(portMatchingType, port, portGroupID),
		Port:               port,
		PortGroupID:        portGroupID,
	}
	if matchOppositePorts {
		ep.MatchOppositePorts = BoolPtr(true)
	}
	if matchOppositeIPs {
		if matchingTarget == "NETWORK" {
			ep.MatchOppositeNetworks = BoolPtr(true)
		} else {
			ep.MatchOppositeIPs = BoolPtr(true)
		}
	}
	// The API expects MAC values in the "macs" field, device values in the
	// "client_macs" field and network IDs in the "network_ids" field, not "ips".
	switch matchingTarget {
	case "MAC":
		ep.MACs = ips
	case "CLIENT":
		ep.ClientMACs = ips
	case "NETWORK":
		ep.NetworkIDs = ips
	default:
		ep.IPs = ips
	}
	return ep
}

func BoolPtr(b bool) *bool { return &b }

// ResolvePortMatchingType derives the correct port_matching_type for the API.
// The v2 API accepts SPECIFIC (when a port number is set), OBJECT (when a port
// group ID is set), or ANY (no port filter). This function auto-derives the
// value from what's set, so users don't need to specify port_matching_type
// explicitly.
func ResolvePortMatchingType(portMatchingType string, port *int64, portGroupID string) string {
	if portGroupID != "" {
		return "OBJECT"
	}
	if port != nil {
		return "SPECIFIC"
	}
	return portMatchingType
}

// MatchingTargetType derives matching_target_type from matching_target.
// The v2 API requires this field when matching_target is not ANY. The enum
// only accepts SPECIFIC or OBJECT (not ANY), so we omit it for ANY targets.
func MatchingTargetType(matchingTarget string) string {
	if matchingTarget == "" || matchingTarget == "ANY" {
		return "" // omitempty will exclude it from the JSON
	}
	if matchingTarget == "CLIENT" {
		return "OBJECT"
	}
	return "SPECIFIC"
}
