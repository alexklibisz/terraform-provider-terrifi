package unifi

// FirewallPolicy mirrors entries at /v2/api/site/{site}/firewall-policies.
// Wire-format serialization is handled by firewall_policy_api.go's bespoke
// request/response structs (boolean omitempty, _id-in-PUT-body, string-encoded
// port field on decode); these typed structs are just the in-process shape
// that resource code reads and writes.
type FirewallPolicy struct {
	ID     string `json:"_id,omitempty"`
	SiteID string `json:"site_id,omitempty"`

	Hidden   bool   `json:"attr_hidden,omitempty"`
	HiddenID string `json:"attr_hidden_id,omitempty"`
	NoDelete bool   `json:"attr_no_delete,omitempty"`
	NoEdit   bool   `json:"attr_no_edit,omitempty"`

	Action              string                     `json:"action,omitempty"`
	ConnectionStateType string                     `json:"connection_state_type,omitempty"`
	ConnectionStates    []string                   `json:"connection_states"`
	CreateAllowRespond  bool                       `json:"create_allow_respond"`
	Description         string                     `json:"description,omitempty"`
	Destination         *FirewallPolicyDestination `json:"destination,omitempty"`
	Enabled             bool                       `json:"enabled"`
	ICMPTypename        string                     `json:"icmp_typename,omitempty"`
	ICMPV6Typename      string                     `json:"icmp_v6_typename,omitempty"`
	IPVersion           string                     `json:"ip_version,omitempty"`
	Index               *int64                     `json:"index,omitempty"`
	Logging             bool                       `json:"logging"`
	MatchIPSec          bool                       `json:"match_ip_sec"`
	Name                string                     `json:"name,omitempty"`
	Predefined          bool                       `json:"predefined"`
	Protocol            string                     `json:"protocol,omitempty"`
	Schedule            *FirewallPolicySchedule    `json:"schedule,omitempty"`
	Source              *FirewallPolicySource      `json:"source,omitempty"`
}

// FirewallPolicySource is the source endpoint of a FirewallPolicy.
type FirewallPolicySource struct {
	IPs                []string `json:"ips,omitempty"`
	MatchOppositeIPs   bool     `json:"match_opposite_ips"`
	MatchOppositePorts bool     `json:"match_opposite_ports"`
	MatchingTarget     string   `json:"matching_target,omitempty"`
	MatchingTargetType string   `json:"matching_target_type,omitempty"`
	Port               *int64   `json:"port,omitempty"`
	PortGroupID        string   `json:"port_group_id,omitempty"`
	PortMatchingType   string   `json:"port_matching_type,omitempty"`
	ZoneID             string   `json:"zone_id,omitempty"`
}

// FirewallPolicyDestination is the destination endpoint of a FirewallPolicy.
type FirewallPolicyDestination struct {
	IPs                []string `json:"ips,omitempty"`
	MatchOppositeIPs   bool     `json:"match_opposite_ips"`
	MatchOppositePorts bool     `json:"match_opposite_ports"`
	MatchingTarget     string   `json:"matching_target,omitempty"`
	MatchingTargetType string   `json:"matching_target_type,omitempty"`
	Port               *int64   `json:"port,omitempty"`
	PortGroupID        string   `json:"port_group_id,omitempty"`
	PortMatchingType   string   `json:"port_matching_type,omitempty"`
	ZoneID             string   `json:"zone_id,omitempty"`
}

// FirewallPolicySchedule is the schedule for a FirewallPolicy.
type FirewallPolicySchedule struct {
	Date           string   `json:"date,omitempty"`
	Mode           string   `json:"mode,omitempty"`
	RepeatOnDays   []string `json:"repeat_on_days,omitempty"`
	TimeAllDay     bool     `json:"time_all_day"`
	TimeRangeEnd   string   `json:"time_range_end,omitempty"`
	TimeRangeStart string   `json:"time_range_start,omitempty"`
}
