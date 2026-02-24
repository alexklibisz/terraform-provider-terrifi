package generate

import (
	"github.com/ubiquiti-community/go-unifi/unifi"
)

// FirewallPolicyBlocks generates import + resource blocks for firewall policies.
func FirewallPolicyBlocks(policies []*unifi.FirewallPolicy) []ResourceBlock {
	blocks := make([]ResourceBlock, 0, len(policies))
	for _, p := range policies {
		block := ResourceBlock{
			ResourceType: "terrifi_firewall_policy",
			ResourceName: ToTerraformName(p.Name),
			ImportID:     p.ID,
		}

		block.Attributes = append(block.Attributes, Attr{Key: "name", Value: HCLString(p.Name)})

		if p.Description != "" {
			block.Attributes = append(block.Attributes, Attr{Key: "description", Value: HCLString(p.Description)})
		}
		if !p.Enabled {
			block.Attributes = append(block.Attributes, Attr{Key: "enabled", Value: HCLBool(false)})
		}

		block.Attributes = append(block.Attributes, Attr{Key: "action", Value: HCLString(p.Action)})

		if p.IPVersion != "" && p.IPVersion != "BOTH" {
			block.Attributes = append(block.Attributes, Attr{Key: "ip_version", Value: HCLString(p.IPVersion)})
		}
		if p.Protocol != "" && p.Protocol != "all" {
			block.Attributes = append(block.Attributes, Attr{Key: "protocol", Value: HCLString(p.Protocol)})
		}
		if p.ConnectionStateType != "" && p.ConnectionStateType != "ALL" {
			block.Attributes = append(block.Attributes, Attr{Key: "connection_state_type", Value: HCLString(p.ConnectionStateType)})
		}
		if len(p.ConnectionStates) > 0 {
			block.Attributes = append(block.Attributes, Attr{Key: "connection_states", Value: HCLStringList(p.ConnectionStates)})
		}
		if p.MatchIPSec {
			block.Attributes = append(block.Attributes, Attr{Key: "match_ipsec", Value: HCLBool(true)})
		}
		if p.Logging {
			block.Attributes = append(block.Attributes, Attr{Key: "logging", Value: HCLBool(true)})
		}
		if p.CreateAllowRespond {
			block.Attributes = append(block.Attributes, Attr{Key: "create_allow_respond", Value: HCLBool(true)})
		}
		if p.Index != nil {
			block.Attributes = append(block.Attributes, Attr{Key: "index", Value: HCLInt64(*p.Index)})
		}

		if p.Source != nil {
			block.Blocks = append(block.Blocks, buildEndpointBlock("source", p.Source.ZoneID, p.Source.MatchingTarget, p.Source.IPs, p.Source.PortMatchingType, p.Source.Port, p.Source.PortGroupID))
		}
		if p.Destination != nil {
			block.Blocks = append(block.Blocks, buildEndpointBlock("destination", p.Destination.ZoneID, p.Destination.MatchingTarget, p.Destination.IPs, p.Destination.PortMatchingType, p.Destination.Port, p.Destination.PortGroupID))
		}

		if p.Schedule != nil && p.Schedule.Mode != "" && p.Schedule.Mode != "ALWAYS" {
			block.Blocks = append(block.Blocks, buildScheduleBlock(p.Schedule))
		}

		blocks = append(blocks, block)
	}
	DeduplicateNames(blocks)
	return blocks
}

func buildEndpointBlock(name, zoneID, matchingTarget string, ips []string, portMatchingType string, port *int64, portGroupID string) NestedBlock {
	nb := NestedBlock{Name: name}

	nb.Attributes = append(nb.Attributes, Attr{
		Key:     "zone_id",
		Value:   HCLString(zoneID),
		Comment: "TODO: find and reference corresponding terrifi_firewall_zone resource",
	})

	if matchingTarget != "" && matchingTarget != "ANY" && len(ips) > 0 {
		switch matchingTarget {
		case "IP":
			nb.Attributes = append(nb.Attributes, Attr{Key: "ips", Value: HCLStringList(ips)})
		case "MAC":
			nb.Attributes = append(nb.Attributes, Attr{Key: "mac_addresses", Value: HCLStringList(ips)})
		case "NETWORK":
			nb.Attributes = append(nb.Attributes, Attr{
				Key:     "network_ids",
				Value:   HCLStringList(ips),
				Comment: "TODO: find and reference corresponding terrifi_network resources",
			})
		case "DEVICE":
			nb.Attributes = append(nb.Attributes, Attr{
				Key:     "device_ids",
				Value:   HCLStringList(ips),
				Comment: "TODO: find and reference corresponding terrifi_client_device resources",
			})
		}
	}

	if portMatchingType != "" && portMatchingType != "ANY" {
		nb.Attributes = append(nb.Attributes, Attr{Key: "port_matching_type", Value: HCLString(portMatchingType)})
	}
	if port != nil {
		nb.Attributes = append(nb.Attributes, Attr{Key: "port", Value: HCLInt64(*port)})
	}
	if portGroupID != "" {
		nb.Attributes = append(nb.Attributes, Attr{Key: "port_group_id", Value: HCLString(portGroupID)})
	}

	return nb
}

func buildScheduleBlock(s *unifi.FirewallPolicySchedule) NestedBlock {
	nb := NestedBlock{Name: "schedule"}

	nb.Attributes = append(nb.Attributes, Attr{Key: "mode", Value: HCLString(s.Mode)})

	if s.Date != "" {
		nb.Attributes = append(nb.Attributes, Attr{Key: "date", Value: HCLString(s.Date)})
	}
	if s.TimeAllDay {
		nb.Attributes = append(nb.Attributes, Attr{Key: "time_all_day", Value: HCLBool(true)})
	}
	if s.TimeRangeStart != "" {
		nb.Attributes = append(nb.Attributes, Attr{Key: "time_range_start", Value: HCLString(s.TimeRangeStart)})
	}
	if s.TimeRangeEnd != "" {
		nb.Attributes = append(nb.Attributes, Attr{Key: "time_range_end", Value: HCLString(s.TimeRangeEnd)})
	}
	if len(s.RepeatOnDays) > 0 {
		nb.Attributes = append(nb.Attributes, Attr{Key: "repeat_on_days", Value: HCLStringList(s.RepeatOnDays)})
	}

	return nb
}
