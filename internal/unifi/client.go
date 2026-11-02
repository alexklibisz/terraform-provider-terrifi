package unifi

// Client is the local controller API contract.
type Client struct {
	ID                            string   `json:"_id,omitempty"`
	SiteID                        string   `json:"site_id,omitempty"`
	Hidden                        bool     `json:"attr_hidden,omitempty"`
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
	UserGroupID                   string   `json:"usergroup_id,omitempty"`
	VirtualNetworkOverrideEnabled *bool    `json:"virtual_network_override_enabled,omitempty"`
	VirtualNetworkOverrideID      string   `json:"virtual_network_override_id,omitempty"`
}
