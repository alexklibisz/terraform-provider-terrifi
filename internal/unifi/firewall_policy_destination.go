package unifi

// FirewallPolicyDestination is the local controller API contract.
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
