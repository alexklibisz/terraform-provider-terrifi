package unifi

// WLAN mirrors the controller's representation of a wireless network at
// /api/s/{site}/rest/wlanconf. Only the fields this provider reads or writes
// are typed; the controller's other fields are dropped on decode (json.Unmarshal
// ignores unknown fields).
type WLAN struct {
	ID     string `json:"_id,omitempty"`
	SiteID string `json:"site_id,omitempty"`

	Hidden   bool   `json:"attr_hidden,omitempty"`
	HiddenID string `json:"attr_hidden_id,omitempty"`
	NoDelete bool   `json:"attr_no_delete,omitempty"`
	NoEdit   bool   `json:"attr_no_edit,omitempty"`

	ApGroupIDs                  []string                   `json:"ap_group_ids,omitempty"`
	ApGroupMode                 string                     `json:"ap_group_mode,omitempty"`
	Enabled                     bool                       `json:"enabled"`
	EnhancedIot                 bool                       `json:"enhanced_iot"`
	HideSSID                    bool                       `json:"hide_ssid"`
	IsGuest                     bool                       `json:"is_guest"`
	Name                        string                     `json:"name,omitempty"`
	NetworkID                   string                     `json:"networkconf_id,omitempty"`
	OptimizeIotWifiConnectivity bool                       `json:"optimize_iot_wifi_connectivity"`
	ScheduleWithDuration        []WLANScheduleWithDuration `json:"schedule_with_duration"`
	Security                    string                     `json:"security,omitempty"`
	UserGroupID                 string                     `json:"usergroup_id,omitempty"`
	WLANBand                    string                     `json:"wlan_band,omitempty"`
	WLANGroupID                 string                     `json:"wlangroup_id"`
	WPA3Support                 bool                       `json:"wpa3_support"`
	WPA3Transition              bool                       `json:"wpa3_transition"`
	WPAMode                     string                     `json:"wpa_mode,omitempty"`
	XPassphrase                 string                     `json:"x_passphrase,omitempty"`
}

// WLANScheduleWithDuration is referenced by WLAN.ScheduleWithDuration. The
// provider never reads or writes scheduled-entry fields — it only sends an
// empty slice on Create to ensure the controller receives [] rather than null.
// Inner fields the controller may return are dropped on decode.
type WLANScheduleWithDuration struct{}

// WLANGroup mirrors /api/s/{site}/rest/wlangroup entries. The provider uses
// only ID (to look up the default group); other fields are passthrough.
type WLANGroup struct {
	ID     string `json:"_id,omitempty"`
	SiteID string `json:"site_id,omitempty"`
	Name   string `json:"name,omitempty"`
}
