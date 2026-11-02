package unifi

// FirewallZone is the local controller API contract.
type FirewallZone struct {
	ID          string   `json:"_id,omitempty"`
	SiteID      string   `json:"site_id,omitempty"`
	Hidden      bool     `json:"attr_hidden,omitempty"`
	DefaultZone bool     `json:"default_zone"`
	Name        string   `json:"name,omitempty"`
	NetworkIDs  []string `json:"network_ids,omitempty"`
	ZoneKey     string   `json:"zone_key,omitempty"`
}
