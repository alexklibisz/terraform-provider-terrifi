package generate

import (
	"github.com/ubiquiti-community/go-unifi/unifi"
)

// NetworkBlocks generates import + resource blocks for networks.
// Only corporate networks are included (the provider only supports corporate).
func NetworkBlocks(networks []unifi.Network) []ResourceBlock {
	blocks := make([]ResourceBlock, 0, len(networks))
	for _, n := range networks {
		if n.Purpose != "corporate" {
			continue
		}

		name := ""
		if n.Name != nil {
			name = *n.Name
		}

		block := ResourceBlock{
			ResourceType: "terrifi_network",
			ResourceName: ToTerraformName(name),
			ImportID:     n.ID,
		}

		block.Attributes = append(block.Attributes, Attr{Key: "name", Value: HCLString(name)})
		block.Attributes = append(block.Attributes, Attr{Key: "purpose", Value: HCLString("corporate")})

		if n.VLAN != nil && *n.VLAN != 0 {
			block.Attributes = append(block.Attributes, Attr{Key: "vlan_id", Value: HCLInt64(*n.VLAN)})
		}
		if n.IPSubnet != nil && *n.IPSubnet != "" {
			block.Attributes = append(block.Attributes, Attr{Key: "subnet", Value: HCLString(*n.IPSubnet)})
		}
		if n.NetworkGroup != nil && *n.NetworkGroup != "" && *n.NetworkGroup != "LAN" {
			block.Attributes = append(block.Attributes, Attr{Key: "network_group", Value: HCLString(*n.NetworkGroup)})
		}
		if n.DHCPDEnabled {
			block.Attributes = append(block.Attributes, Attr{Key: "dhcp_enabled", Value: HCLBool(true)})
			if n.DHCPDStart != nil && *n.DHCPDStart != "" {
				block.Attributes = append(block.Attributes, Attr{Key: "dhcp_start", Value: HCLString(*n.DHCPDStart)})
			}
			if n.DHCPDStop != nil && *n.DHCPDStop != "" {
				block.Attributes = append(block.Attributes, Attr{Key: "dhcp_stop", Value: HCLString(*n.DHCPDStop)})
			}
			if n.DHCPDLeaseTime != nil && *n.DHCPDLeaseTime != 86400 {
				block.Attributes = append(block.Attributes, Attr{Key: "dhcp_lease", Value: HCLInt64(*n.DHCPDLeaseTime)})
			}

			var dnsServers []string
			if n.DHCPDDNS1 != "" {
				dnsServers = append(dnsServers, n.DHCPDDNS1)
			}
			if n.DHCPDDNS2 != "" {
				dnsServers = append(dnsServers, n.DHCPDDNS2)
			}
			if n.DHCPDDNS3 != "" {
				dnsServers = append(dnsServers, n.DHCPDDNS3)
			}
			if n.DHCPDDNS4 != "" {
				dnsServers = append(dnsServers, n.DHCPDDNS4)
			}
			if len(dnsServers) > 0 {
				block.Attributes = append(block.Attributes, Attr{Key: "dhcp_dns", Value: HCLStringList(dnsServers)})
			}
		}
		if !n.InternetAccessEnabled {
			block.Attributes = append(block.Attributes, Attr{Key: "internet_access_enabled", Value: HCLBool(false)})
		}

		blocks = append(blocks, block)
	}
	DeduplicateNames(blocks)
	return blocks
}
