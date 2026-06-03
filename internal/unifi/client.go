package unifi

// Client mirrors entries at /api/s/{site}/rest/user — a controller's
// representation of a known wireless/wired client device. The provider reads
// and writes the subset of fields below; everything else the controller
// returns (dpi stats, presence flags, vendor metadata, etc.) is dropped on
// decode.
//
// Wire-format quirks for create/update are handled in client_device_api.go
// via the bespoke clientDeviceRequest type — most notably, *bool +
// omitempty for boolean toggles that we must NOT send as bare false.
type Client struct {
	ID     string `json:"_id,omitempty"`
	SiteID string `json:"site_id,omitempty"`

	Hidden   bool   `json:"attr_hidden,omitempty"`
	HiddenID string `json:"attr_hidden_id,omitempty"`
	NoDelete bool   `json:"attr_no_delete,omitempty"`
	NoEdit   bool   `json:"attr_no_edit,omitempty"`

	Blocked                       *bool    `json:"blocked,omitempty"`
	FixedApEnabled                bool     `json:"fixed_ap_enabled"`
	FixedApMAC                    string   `json:"fixed_ap_mac,omitempty"`
	FixedIP                       string   `json:"fixed_ip,omitempty"`
	LocalDNSRecord                string   `json:"local_dns_record,omitempty"`
	LocalDNSRecordEnabled         bool     `json:"local_dns_record_enabled"`
	MAC                           string   `json:"mac,omitempty"`
	Name                          string   `json:"name,omitempty"`
	NetworkID                     string   `json:"network_id,omitempty"`
	NetworkMembersGroupIDs        []string `json:"network_members_group_ids,omitempty"`
	Note                          string   `json:"note,omitempty"`
	UseFixedIP                    bool     `json:"use_fixedip"`
	VirtualNetworkOverrideEnabled *bool    `json:"virtual_network_override_enabled,omitempty"`
	VirtualNetworkOverrideID      string   `json:"virtual_network_override_id,omitempty"`
}
