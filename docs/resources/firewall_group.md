---
page_title: "terrifi_firewall_group Resource - Terrifi"
subcategory: ""
description: |-
  Manages a firewall group on the UniFi controller.
---

# terrifi_firewall_group (Resource)

Manages a firewall group on the UniFi controller. Firewall groups are named collections of ports or addresses that can be referenced by firewall policies and rules.

## Example Usage

### Port group

```terraform
resource "terrifi_firewall_group" "web_ports" {
  name    = "Web Ports"
  type    = "port-group"
  members = ["80", "443", "8080"]
}
```

### Address group

```terraform
resource "terrifi_firewall_group" "trusted_ips" {
  name    = "Trusted IPs"
  type    = "address-group"
  members = ["10.0.0.0/24", "192.168.1.0/24"]
}
```

### IPv6 address group

```terraform
resource "terrifi_firewall_group" "ipv6_trusted" {
  name    = "IPv6 Trusted"
  type    = "ipv6-address-group"
  members = ["fd00::/64", "2001:db8::/32"]
}
```

### Usage in a firewall policy

```terraform
resource "terrifi_firewall_group" "web_ports" {
  name    = "Web Ports"
  type    = "port-group"
  members = ["80", "443"]
}

resource "terrifi_firewall_policy" "allow_web" {
  name   = "Allow Web"
  action = "ALLOW"

  source {
    zone_id = terrifi_firewall_zone.lan.id
  }

  destination {
    zone_id            = terrifi_firewall_zone.wan.id
    port_matching_type = "LIST"
    port_group_id      = terrifi_firewall_group.web_ports.id
  }
}
```

## Schema

### Required

- `name` (String) — The name of the firewall group.
- `type` (String) — The type of firewall group. One of: `port-group`, `address-group`, `ipv6-address-group`. Changing this forces a new resource.
- `members` (Set of String) — The members of the firewall group. For `port-group`, these are port numbers or port ranges (e.g. `"80"`, `"8080-8090"`). For `address-group`, these are IPv4 addresses or CIDRs. For `ipv6-address-group`, these are IPv6 addresses or CIDRs.

### Optional

- `site` (String) — The site to associate the firewall group with. Defaults to the provider site. Changing this forces a new resource.

### Read-Only

- `id` (String) — The ID of the firewall group.

## Import

Firewall groups can be imported using the group ID:

```shell
terraform import terrifi_firewall_group.web_ports <id>
```

To import a group from a non-default site, use the `site:id` format:

```shell
terraform import terrifi_firewall_group.web_ports <site>:<id>
```

You can also use the [Terrifi CLI](../cli.md) to generate import blocks for all firewall groups automatically:

```shell
terrifi generate-imports terrifi_firewall_group
```
