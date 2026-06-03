package unifi

import (
	"encoding/json"
	"fmt"
)

// DNSRecord mirrors the controller's representation of a static DNS record at
// /v2/api/site/{site}/static-dns. Numeric fields (port, priority, ttl, weight)
// arrive as either JSON numbers or quoted strings depending on controller
// version, so UnmarshalJSON coerces both shapes via Number.
type DNSRecord struct {
	ID     string `json:"_id,omitempty"`
	SiteID string `json:"site_id,omitempty"`

	Hidden   bool   `json:"attr_hidden,omitempty"`
	HiddenID string `json:"attr_hidden_id,omitempty"`
	NoDelete bool   `json:"attr_no_delete,omitempty"`
	NoEdit   bool   `json:"attr_no_edit,omitempty"`

	Enabled    bool   `json:"enabled"`
	Key        string `json:"key,omitempty"`
	Port       *int64 `json:"port,omitempty"`
	Priority   int64  `json:"priority,omitempty"`
	RecordType string `json:"record_type,omitempty"`
	Ttl        int64  `json:"ttl,omitempty"`
	Value      string `json:"value,omitempty"`
	Weight     int64  `json:"weight,omitempty"`
}

func (dst *DNSRecord) UnmarshalJSON(b []byte) error {
	type Alias DNSRecord
	aux := &struct {
		Port     *Number `json:"port"`
		Priority Number  `json:"priority"`
		Ttl      Number  `json:"ttl"`
		Weight   Number  `json:"weight"`
		*Alias
	}{
		Alias: (*Alias)(dst),
	}
	if err := json.Unmarshal(b, aux); err != nil {
		return fmt.Errorf("unmarshal DNSRecord: %w", err)
	}
	if aux.Port != nil {
		if v, err := aux.Port.Int64(); err == nil {
			dst.Port = &v
		} else if string(*aux.Port) == "" {
			var zero int64
			dst.Port = &zero
		}
	}
	if v, err := aux.Priority.Int64(); err == nil {
		dst.Priority = v
	}
	if v, err := aux.Ttl.Int64(); err == nil {
		dst.Ttl = v
	}
	if v, err := aux.Weight.Int64(); err == nil {
		dst.Weight = v
	}
	return nil
}
