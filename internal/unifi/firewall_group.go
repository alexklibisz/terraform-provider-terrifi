package unifi

// FirewallGroup is the local controller API contract.
type FirewallGroup struct {
	ID           string   `json:"_id,omitempty"`
	SiteID       string   `json:"site_id,omitempty"`
	Hidden       bool     `json:"attr_hidden,omitempty"`
	GroupMembers []string `json:"group_members"`
	GroupType    string   `json:"group_type,omitempty"`
	Name         string   `json:"name,omitempty"`
}
