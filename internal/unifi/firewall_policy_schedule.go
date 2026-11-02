package unifi

// FirewallPolicySchedule is the local controller API contract.
type FirewallPolicySchedule struct {
	Date           string   `json:"date,omitempty"`
	Mode           string   `json:"mode,omitempty"`
	RepeatOnDays   []string `json:"repeat_on_days,omitempty"`
	TimeAllDay     bool     `json:"time_all_day"`
	TimeRangeEnd   string   `json:"time_range_end,omitempty"`
	TimeRangeStart string   `json:"time_range_start,omitempty"`
}
