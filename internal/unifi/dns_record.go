package unifi

// DNSRecord is the local controller API contract.
type DNSRecord struct {
	ID         string `json:"_id,omitempty"`
	SiteID     string `json:"site_id,omitempty"`
	Hidden     bool   `json:"attr_hidden,omitempty"`
	Enabled    bool   `json:"enabled"`
	Key        string `json:"key,omitempty"`
	Port       *int64 `json:"port,omitempty"`
	Priority   int64  `json:"priority,omitempty"`
	RecordType string `json:"record_type,omitempty"`
	Ttl        int64  `json:"ttl,omitempty"`
	Value      string `json:"value,omitempty"`
	Weight     int64  `json:"weight,omitempty"`
}
