package unifi

// FirewallGroup mirrors the controller's representation of a firewall group at
// /api/s/{site}/rest/firewallgroup. Only fields the provider needs are typed;
// extra fields the controller returns are ignored on read and dropped on write
// (the v1 API does not require full-object passthrough for this endpoint).
type FirewallGroup struct {
	ID           string   `json:"_id,omitempty"`
	SiteID       string   `json:"site_id,omitempty"`
	Name         string   `json:"name,omitempty"`
	GroupType    string   `json:"group_type,omitempty"`
	GroupMembers []string `json:"group_members"`
}
