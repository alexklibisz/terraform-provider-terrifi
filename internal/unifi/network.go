package unifi

import (
	"encoding/json"
	"fmt"
)

// Network mirrors the controller's representation of a network at
// /api/s/{site}/rest/networkconf. Only the fields the resource reads or writes
// are typed; the controller's many other fields (ipv6_*, wan_*, igmp, etc.)
// are dropped on decode because json.Unmarshal ignores unknown fields.
//
// Notably, IPV6RaPreferredLifetime is intentionally absent: the controller
// emits it as a JSON string on some sites (e.g. "14400"), which would fail to
// decode into an *int64 typed field. Issue #154 worked around that in the
// provider layer; dropping the field at the type level eliminates the need
// for the workaround entirely.
type Network struct {
	ID     string `json:"_id,omitempty"`
	SiteID string `json:"site_id,omitempty"`

	Hidden   bool   `json:"attr_hidden,omitempty"`
	HiddenID string `json:"attr_hidden_id,omitempty"`
	NoDelete bool   `json:"attr_no_delete,omitempty"`
	NoEdit   bool   `json:"attr_no_edit,omitempty"`

	Name *string `json:"name,omitempty"`

	Purpose      string  `json:"purpose"`
	NetworkGroup *string `json:"networkgroup,omitempty"`
	Enabled      bool    `json:"enabled"`

	VLAN        *int64 `json:"vlan,omitempty"`
	VLANEnabled bool   `json:"vlan_enabled"`

	IPSubnet *string `json:"ip_subnet,omitempty"`

	DHCPDEnabled   bool    `json:"dhcpd_enabled"`
	DHCPDStart     *string `json:"dhcpd_start,omitempty"`
	DHCPDStop      *string `json:"dhcpd_stop,omitempty"`
	DHCPDLeaseTime *int64  `json:"dhcpd_leasetime,omitempty"`
	DHCPDDNS1      string  `json:"dhcpd_dns_1"`
	DHCPDDNS2      string  `json:"dhcpd_dns_2"`
	DHCPDDNS3      string  `json:"dhcpd_dns_3"`
	DHCPDDNS4      string  `json:"dhcpd_dns_4"`

	InternetAccessEnabled bool    `json:"internet_access_enabled"`
	SettingPreference     *string `json:"setting_preference,omitempty"`
}

// UnmarshalJSON applies the SDK's internet_access_enabled default-to-true
// behavior: a missing or null field decodes as true, matching what the
// controller implicitly assumes.
func (dst *Network) UnmarshalJSON(b []byte) error {
	type Alias Network
	aux := &struct {
		InternetAccessEnabled *bool `json:"internet_access_enabled"`
		*Alias
	}{
		Alias: (*Alias)(dst),
	}
	if err := json.Unmarshal(b, aux); err != nil {
		return fmt.Errorf("unmarshal Network: %w", err)
	}
	dst.InternetAccessEnabled = aux.InternetAccessEnabled == nil || *aux.InternetAccessEnabled
	return nil
}
