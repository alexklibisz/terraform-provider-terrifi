---
page_title: "terrifi_firewall_zone Resource - Terrifi"
subcategory: ""
description: |-
  Manages a firewall zone on the UniFi controller.
---

# terrifi_firewall_zone (Resource)

Manages a firewall zone on the UniFi controller. Firewall zones group networks together for firewall rule management.

~> **Prerequisite:** Zone-based firewall must be enabled on your controller before using this resource. In the UniFi UI, navigate to **Settings > Security > Traffic & Firewall Rules** and click **Upgrade to the New Zone-Based Firewall**. This is a one-time operation that creates the default zones and enables the zone API.

## Example Usage

### Basic zone

```terraform
resource "terrifi_firewall_zone" "iot" {
  name = "IoT"
}
```

### Zone with networks

```terraform
resource "terrifi_network" "iot" {
  name    = "IoT"
  purpose = "corporate"
  vlan_id = 33
  subnet  = "192.168.33.1/24"
}

resource "terrifi_firewall_zone" "iot" {
  name        = "IoT"
  network_ids = [terrifi_network.iot.id]
}
```

## Schema

### Required

- `name` (String) — The name of the firewall zone.

### Optional

- `network_ids` (List of String) — List of network IDs to associate with this firewall zone.
- `site` (String) — The site to associate the firewall zone with. Defaults to the provider site. Changing this forces a new resource.

### Read-Only

- `id` (String) — The ID of the firewall zone.
- `zone_key` (String) — The zone key assigned by the controller.

## Import

Firewall zones can be imported using the zone ID:

```shell
terraform import terrifi_firewall_zone.iot <id>
```

To import a firewall zone from a non-default site, use the `site:id` format:

```shell
terraform import terrifi_firewall_zone.iot <site>:<id>
```

You can also use the [Terrifi CLI](../cli.md) to generate import blocks for all firewall zones automatically:

```shell
terrifi generate-imports terrifi_firewall_zone
```
