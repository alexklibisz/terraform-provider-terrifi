---
page_title: "terrifi_firewall_policy_order Resource - Terrifi"
subcategory: ""
description: |-
  Manages the evaluation order of firewall policies for a zone pair.
---

# terrifi_firewall_policy_order (Resource)

Manages the evaluation order of firewall policies for a specific zone pair on the UniFi controller. Policies are evaluated in the order specified by `policy_ids`, before any predefined (system) policies.

~> **Note:** This resource only controls ordering. The policies themselves must be created separately using `terrifi_firewall_policy`. Destroying this resource removes it from Terraform state but does not change the ordering on the controller.

## Example Usage

### Order policies between two zones

```terraform
resource "terrifi_firewall_zone" "iot" {
  name = "IoT"
}

resource "terrifi_firewall_zone" "trusted" {
  name = "Trusted"
}

resource "terrifi_firewall_policy" "allow_dns" {
  name   = "Allow DNS"
  action = "ALLOW"

  source {
    zone_id = terrifi_firewall_zone.iot.id
  }

  destination {
    zone_id            = terrifi_firewall_zone.trusted.id
    port_matching_type = "SPECIFIC"
    port               = 53
  }
}

resource "terrifi_firewall_policy" "block_all" {
  name   = "Block All"
  action = "BLOCK"

  source {
    zone_id = terrifi_firewall_zone.iot.id
  }

  destination {
    zone_id = terrifi_firewall_zone.trusted.id
  }
}

resource "terrifi_firewall_policy_order" "iot_to_trusted" {
  source_zone_id      = terrifi_firewall_zone.iot.id
  destination_zone_id = terrifi_firewall_zone.trusted.id

  policy_ids = [
    terrifi_firewall_policy.allow_dns.id,  # evaluated first
    terrifi_firewall_policy.block_all.id,  # evaluated second
  ]
}
```

## Schema

### Required

- `source_zone_id` (String) — The ID of the source firewall zone. Changing this forces a new resource.
- `destination_zone_id` (String) — The ID of the destination firewall zone. Changing this forces a new resource.
- `policy_ids` (List of String) — Ordered list of firewall policy IDs. Policies are evaluated in this order, before any predefined (system) policies.

### Optional

- `site` (String) — The site. Defaults to the provider site. Changing this forces a new resource.

### Read-Only

- `id` (String) — Synthetic ID in the format `source_zone_id:destination_zone_id`.

## Import

Firewall policy ordering can be imported using the zone pair:

```shell
terraform import terrifi_firewall_policy_order.example <source_zone_id>:<destination_zone_id>
```

To import from a non-default site:

```shell
terraform import terrifi_firewall_policy_order.example <site>:<source_zone_id>:<destination_zone_id>
```

You can also use the [Terrifi CLI](../index.md#cli) to generate import blocks automatically:

```shell
terrifi generate-imports terrifi_firewall_policy_order
```
