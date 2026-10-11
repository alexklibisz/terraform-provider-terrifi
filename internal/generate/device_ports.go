package generate

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/alexklibisz/terrifi/internal/unifi"
)

// DevicePortsBlocks generates import + resource blocks for device ports: one
// block per device with at least one named or profiled port, listing those
// ports (the same set the resource imports).
func DevicePortsBlocks(devices []unifi.DevicePorts) []ResourceBlock {
	var blocks []ResourceBlock
	for _, d := range devices {
		type port struct {
			idx             int64
			name, profileID string
		}
		var ports []port
		for _, e := range d.PortOverrides {
			idx, name, profileID, err := unifi.ParsePortOverride(e)
			if err == nil && (name != "" || profileID != "") {
				ports = append(ports, port{idx, name, profileID})
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
			fmt.Fprintf(&b, "    %q = { %s }\n", fmt.Sprint(p.idx), strings.Join(fields, ", "))
		}
		b.WriteString("  }")

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
				{Key: "ports", Value: b.String(), Comment: "TODO: reference the corresponding terrifi_port_profile resources"},
			},
		})
	}
	return blocks
}
