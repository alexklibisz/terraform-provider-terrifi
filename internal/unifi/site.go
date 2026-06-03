package unifi

// Site mirrors entries at /api/self/sites. The provider's CLI reads Name to
// print the list of available sites; the description and ID are for
// completeness.
type Site struct {
	ID          string `json:"_id,omitempty"`
	Name        string `json:"name"`
	Description string `json:"desc"`
}

// APGroup mirrors entries at /v2/api/site/{site}/apgroups. Only ID is read by
// the provider (to look up the default AP group on WLAN create); Name and
// DeviceMacs are passthrough for display in the CLI.
type APGroup struct {
	ID         string   `json:"_id,omitempty"`
	Name       string   `json:"name"`
	DeviceMacs []string `json:"device_macs"`
}

// ClientGroup mirrors entries at /api/s/{site}/rest/usergroup — the legacy
// QoS user-group object. The provider reads only ID (to look up the default
// group on WLAN create); rate-limit fields are passthrough.
type ClientGroup struct {
	ID             string `json:"_id,omitempty"`
	SiteID         string `json:"site_id,omitempty"`
	Name           string `json:"name,omitempty"`
	QOSRateMaxDown *int64 `json:"qos_rate_max_down,omitempty"`
	QOSRateMaxUp   *int64 `json:"qos_rate_max_up,omitempty"`
}
