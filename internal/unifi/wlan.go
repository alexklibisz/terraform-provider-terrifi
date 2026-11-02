package unifi

// WLAN is the local controller API contract.
type WLAN struct {
	ID                          string                     `json:"_id,omitempty"`
	SiteID                      string                     `json:"site_id,omitempty"`
	Hidden                      bool                       `json:"attr_hidden,omitempty"`
	ApGroupIDs                  []string                   `json:"ap_group_ids,omitempty"`
	ApGroupMode                 string                     `json:"ap_group_mode,omitempty"`
	Enabled                     bool                       `json:"enabled"`
	EnhancedIot                 bool                       `json:"enhanced_iot"`
	HideSSID                    bool                       `json:"hide_ssid"`
	IsGuest                     bool                       `json:"is_guest"`
	Name                        string                     `json:"name,omitempty"`
	NetworkID                   string                     `json:"networkconf_id,omitempty"`
	OptimizeIotWifiConnectivity bool                       `json:"optimize_iot_wifi_connectivity"`
	Priority                    string                     `json:"priority,omitempty"`
	Schedule                    []string                   `json:"schedule,omitempty"`
	ScheduleWithDuration        []WLANScheduleWithDuration `json:"schedule_with_duration"`
	Security                    string                     `json:"security,omitempty"`
	SettingPreference           string                     `json:"setting_preference,omitempty"`
	UserGroupID                 string                     `json:"usergroup_id,omitempty"`
	VLAN                        *int64                     `json:"vlan,omitempty"`
	VLANEnabled                 bool                       `json:"vlan_enabled"`
	WLANBand                    string                     `json:"wlan_band,omitempty"`
	WLANGroupID                 string                     `json:"wlangroup_id"`
	WPA3Support                 bool                       `json:"wpa3_support"`
	WPA3Transition              bool                       `json:"wpa3_transition"`
	WPAMode                     string                     `json:"wpa_mode,omitempty"`
	XPassphrase                 string                     `json:"x_passphrase,omitempty"`
}
