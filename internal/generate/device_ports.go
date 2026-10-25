package generate

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/alexklibisz/terrifi/internal/unifi"
)

// DevicePortsBlocks generates import + resource blocks for device ports: one
// block per device with named, profiled, or inline VLAN ports, listing those
// ports (the same set the resource imports).
func DevicePortsBlocks(devices []unifi.DevicePorts) []ResourceBlock {
	var blocks []ResourceBlock
	for _, d := range devices {
		type port struct {
			idx             int64
			name, profileID string
			native, mode    string
		}
		var ports []port
		for _, e := range d.PortOverrides {
			idx, name, profileID, err := unifi.ParsePortOverride(e)
			native, mode := unifi.PortVLANSettings(e)
			if err == nil && (name != "" || profileID != "" || native != "") {
				ports = append(ports, port{idx, name, profileID, native, mode})
			}
		}
		if len(ports) == 0 {
			continue
		}
		slices.SortFunc(ports, func(a, b port) int { return cmp.Compare(a.idx, b.idx) })

		var b strings.Builder
		b.WriteString("{\n")
		for _, p := range ports {
			var fields []string
			if p.name != "" {
				fields = append(fields, "name = "+HCLString(p.name))
			}
			if p.profileID != "" {
				fields = append(fields, "port_profile_id = "+HCLString(p.profileID))
			}
			if p.native != "" {
				fields = append(fields, "native_network_id = "+HCLString(p.native))
				if p.mode != "" {
					fields = append(fields, "tagged_vlan_mode = "+HCLString(p.mode))
				}
			}
			fmt.Fprintf(&b, "    %q = { %s }\n", fmt.Sprint(p.idx), strings.Join(fields, ", "))
		}
		b.WriteString("  }")

		comment := "TODO: reference the corresponding terrifi_port_profile resources"
		for _, p := range ports {
			if p.native != "" {
				comment = "TODO: reference the corresponding terrifi_network and terrifi_port_profile resources"
				break
			}
		}
		name := d.Name
		if name == "" {
			name = d.MAC
		}
		blocks = append(blocks, ResourceBlock{
			ResourceType: "terrifi_device_ports",
			ResourceName: ToTerraformName(name),
			ImportID:     d.MAC,
			Attributes: []Attr{
				{Key: "device_mac", Value: HCLString(d.MAC)},
				{Key: "ports", Value: b.String(), Comment: comment},
			},
		})
	}
	DeduplicateNames(blocks)
	return blocks
}
