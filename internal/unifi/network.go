package unifi

// Network is the local controller API contract.
type Network struct {
	ID                      string  `json:"_id,omitempty"`
	SiteID                  string  `json:"site_id,omitempty"`
	Hidden                  bool    `json:"attr_hidden,omitempty"`
	DHCPDDNS1               string  `json:"dhcpd_dns_1"`
	DHCPDDNS2               string  `json:"dhcpd_dns_2"`
	DHCPDDNS3               string  `json:"dhcpd_dns_3"`
	DHCPDDNS4               string  `json:"dhcpd_dns_4"`
	DHCPDEnabled            bool    `json:"dhcpd_enabled"`
	DHCPDLeaseTime          *int64  `json:"dhcpd_leasetime,omitempty"`
	DHCPDStart              *string `json:"dhcpd_start,omitempty"`
	DHCPDStop               *string `json:"dhcpd_stop,omitempty"`
	Enabled                 bool    `json:"enabled"`
	IPSubnet                *string `json:"ip_subnet,omitempty"`
	IPV6RaPreferredLifetime *int64  `json:"ipv6_ra_preferred_lifetime,omitempty"`
	InternetAccessEnabled   bool    `json:"internet_access_enabled"`
	Name                    *string `json:"name,omitempty"`
	NetworkGroup            *string `json:"networkgroup,omitempty"`
	Priority                *int64  `json:"priority,omitempty"`
	Purpose                 string  `json:"purpose"`
	SettingPreference       *string `json:"setting_preference,omitempty"`
	UserGroupID             *string `json:"usergroup_id,omitempty"`
	VLAN                    *int64  `json:"vlan,omitempty"`
	VLANEnabled             bool    `json:"vlan_enabled"`
}
