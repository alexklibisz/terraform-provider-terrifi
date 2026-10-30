---
page_title: "terrifi_network Resource - Terrifi"
subcategory: ""
description: |-
  Manages a network on the UniFi controller.
---

# terrifi_network (Resource)

Manages a network on the UniFi controller. Supports `corporate` networks with VLAN configuration and DHCP settings, and `vlan-only` networks that carry no IP configuration.

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

### VLAN-only network

```terraform
resource "terrifi_network" "cameras" {
  name    = "Cameras"
  purpose = "vlan-only"
  vlan_id = 20
}
```

## Schema

### Required

- `name` (String) — The name of the network.
- `purpose` (String) — The purpose of the network. One of: `corporate`, `vlan-only`.

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

## Validation coverage

✅ Covered · ❌ Not covered · ◻️ Unavailable on this target · ⚠️ Conditional. See [validation coverage](../guides/validation-coverage.md) for definitions and limitations.

| Setting | Usage | Simulated | Hardware-in-the-loop |
| --- | --- | :---: | :---: |
| `name` | Create a named network | ✅ | ✅ |
| `purpose` | Set to `corporate` | ✅ | ✅ |
| `purpose` | Set to `vlan-only` | ✅ | ✅ |
| `vlan_id` | Assign a VLAN ID | ✅ | ✅ |
| `subnet` | Assign an IPv4 subnet | ✅ | ✅ |
| `network_group` | Set to `LAN` | ✅ | ✅ |
| `dhcp_enabled` | Set to `true` | ✅ | ✅ |
| `dhcp_enabled` | Set to `false` | ✅ | ✅ |
| `dhcp_start` | Set the pool start address | ✅ | ✅ |
| `dhcp_start` | Change the pool start address | ✅ | ✅ |
| `dhcp_stop` | Set the pool end address | ✅ | ✅ |
| `dhcp_stop` | Change the pool end address | ✅ | ✅ |
| `dhcp_lease` | Change the lease duration | ✅ | ✅ |
| `dhcp_dns` | Set the DNS server list | ✅ | ✅ |
| `dhcp_dns` | Replace the DNS server list | ✅ | ✅ |
| `internet_access_enabled` | Set to `true` | ✅ | ✅ |
| `internet_access_enabled` | Set to `false` | ✅ | ✅ |

Network and DHCP configuration acceptance tests run against both controller targets. Setting `internet_access_enabled` to `false` has no dedicated acceptance test.

Evidence: [network_resource_test.go](https://github.com/alexklibisz/terraform-provider-terrifi/blob/main/internal/provider/network_resource_test.go).

## Import

Networks can be imported using the network ID:

```shell
terraform import terrifi_network.iot <id>
```

To import a network from a non-default site, use the `site:id` format:

```shell
terraform import terrifi_network.iot <site>:<id>
```

You can also use the [Terrifi CLI](../index.md#cli) to generate import blocks for all networks automatically:

```shell
terrifi generate-imports terrifi_network
```
