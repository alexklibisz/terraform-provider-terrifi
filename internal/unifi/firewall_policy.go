package unifi

// FirewallPolicy is the local controller API contract.
type FirewallPolicy struct {
	ID                  string                     `json:"_id,omitempty"`
	SiteID              string                     `json:"site_id,omitempty"`
	Hidden              bool                       `json:"attr_hidden,omitempty"`
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
