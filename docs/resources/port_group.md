---
page_title: "terrifi_port_group Resource - Terrifi"
subcategory: ""
description: |-
  Manages a port group on the UniFi controller.
---

# terrifi_port_group (Resource)

Manages a port group on the UniFi controller. Port groups are named collections of port numbers or port ranges that can be referenced by firewall policies and rules.

## Example Usage

### Basic port group

```terraform
resource "terrifi_port_group" "web" {
  name  = "Web Ports"
  ports = ["80", "443"]
}
```

### Port range

```terraform
resource "terrifi_port_group" "ephemeral" {
  name  = "Ephemeral Ports"
  ports = ["1024-65535"]
}
```

### Usage in a firewall policy

```terraform
resource "terrifi_port_group" "web" {
  name  = "Web Ports"
  ports = ["80", "443"]
}

resource "terrifi_firewall_policy" "allow_web" {
  name   = "Allow Web"
  action = "ALLOW"

  source {
    zone_id = terrifi_firewall_zone.lan.id
  }

  destination {
    zone_id            = terrifi_firewall_zone.wan.id
    port_matching_type = "OBJECT"
    port_group_id      = terrifi_port_group.web.id
  }
}
```

## Schema

### Required

- `name` (String) — The name of the port group.
- `ports` (Set of String) — The ports in this group. Each entry is a port number (e.g. `"80"`) or a port range (e.g. `"8080-8090"`).

### Optional

- `site` (String) — The site to associate the port group with. Defaults to the provider site. Changing this forces a new resource.

### Read-Only

- `id` (String) — The ID of the port group.

## Import

Port groups can be imported using the group ID:

```shell
terraform import terrifi_port_group.web <id>
```

To import a port group from a non-default site, use the `site:id` format:

```shell
terraform import terrifi_port_group.web <site>:<id>
```

You can also use the [Terrifi CLI](../index.md#cli) to generate import blocks for all port groups automatically:

```shell
terrifi generate-imports terrifi_port_group
```
