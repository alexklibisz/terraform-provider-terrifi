package unifi

// FirewallZone mirrors entries at /v2/api/site/{site}/firewall/zone. The
// provider reads ID, Name, NetworkIDs, and ZoneKey; the controller's
// default_zone flag is intentionally omitted — see firewall_zone_api.go
// for the rationale.
type FirewallZone struct {
	ID     string `json:"_id,omitempty"`
	SiteID string `json:"site_id,omitempty"`

	Hidden   bool   `json:"attr_hidden,omitempty"`
	HiddenID string `json:"attr_hidden_id,omitempty"`
	NoDelete bool   `json:"attr_no_delete,omitempty"`
	NoEdit   bool   `json:"attr_no_edit,omitempty"`

	Name       string   `json:"name,omitempty"`
	NetworkIDs []string `json:"network_ids"`
	ZoneKey    string   `json:"zone_key,omitempty"`
}
