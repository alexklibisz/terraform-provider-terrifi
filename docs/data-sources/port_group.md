---
page_title: "terrifi_port_group Data Source - Terrifi"
subcategory: ""
description: |-
  Looks up a UniFi port group by name.
---

# terrifi_port_group (Data Source)

Looks up a UniFi port group by name. Use this data source to reference an existing port group in firewall policies without managing it directly.

## Example Usage

### Look up a port group

```terraform
data "terrifi_port_group" "web" {
  name = "Web Ports"
}
```

### Reference in a firewall policy

```terraform
data "terrifi_port_group" "web" {
  name = "Web Ports"
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
    port_group_id      = data.terrifi_port_group.web.id
  }
}
```

## Schema

### Required

- `name` (String) — The name of the port group to look up.

### Optional

- `site` (String) — The site to look up the port group in. Defaults to the provider site.

### Read-Only

- `id` (String) — The ID of the port group.
- `ports` (Set of String) — The ports in this group. Each entry is a port number (e.g. `"80"`) or a port range (e.g. `"8080-8090"`).
