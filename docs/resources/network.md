---
page_title: "terrifi_network Resource - Terrifi"
subcategory: ""
description: |-
  Manages a network on the UniFi controller.
---

# terrifi_network (Resource)

Manages a network on the UniFi controller. Supports corporate network types with VLAN configuration and DHCP settings.

## Example Usage

### Corporate network with VLAN and DHCP

```terraform
resource "terrifi_network" "iot" {
  name                     = "IoT"
  purpose                  = "corporate"
  vlan_id                  = 33
  subnet                   = "192.168.33.0/24"
  network_group            = "LAN"
  dhcp_enabled             = true
  dhcp_start               = "192.168.33.10"
  dhcp_stop                = "192.168.33.250"
  dhcp_lease               = 86400
  dhcp_dns                 = ["8.8.8.8", "8.8.4.4"]
  internet_access_enabled  = true
}
```

## Schema

### Required

- `name` (String) — The name of the network.
- `purpose` (String) — The purpose of the network. Must be `corporate`. Changing this forces a new resource.

### Optional

- `vlan_id` (Number) — The VLAN ID for the network. Must be between 2 and 4095.
- `subnet` (String) — The subnet for the network in CIDR notation (e.g., `192.168.33.0/24`).
- `network_group` (String) — The network group. Defaults to `LAN`.
- `dhcp_enabled` (Boolean) — Whether DHCP is enabled on this network. Defaults to `false`.
- `dhcp_start` (String) — The starting IP address for the DHCP pool. Computed by the API if not specified.
- `dhcp_stop` (String) — The ending IP address for the DHCP pool. Computed by the API if not specified.
- `dhcp_lease` (Number) — The DHCP lease time in seconds. Defaults to `86400` (24 hours).
- `dhcp_dns` (List of String) — List of DNS servers for DHCP clients. Maximum 4 servers.
- `internet_access_enabled` (Boolean) — Whether internet access is enabled on this network. Defaults to `true`.
- `site` (String) — The site to associate the network with. Defaults to the provider site. Changing this forces a new resource.

### Read-Only

- `id` (String) — The ID of the network.

## Import

Networks can be imported using the network ID:

```shell
terraform import terrifi_network.iot <id>
```

To import a network from a non-default site, use the `site:id` format:

```shell
terraform import terrifi_network.iot <site>:<id>
```
