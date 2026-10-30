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

- `network_ids` (Set of String) — Set of network IDs to associate with this firewall zone.
- `site` (String) — The site to associate the firewall zone with. Defaults to the provider site. Changing this forces a new resource.

### Read-Only

- `id` (String) — The ID of the firewall zone.
- `zone_key` (String) — The zone key assigned by the controller.

## Validation coverage

✅ Covered · ❌ Not covered · ⚠️ Conditional. See [validation coverage](../guides/validation-coverage.md) for definitions and limitations.

| Setting | Usage | Docker controller | Real controller |
| --- | --- | :---: | :---: |
| `name` | Create a named zone | ❌ | ✅ |
| `name` | Rename a zone | ❌ | ✅ |
| `network_ids` | Assign a network to a zone | ❌ | ✅ |
| `network_ids` | Add a network | ❌ | ✅ |
| `network_ids` | Remove a network | ❌ | ✅ |
| `network_ids` | Replace the member set | ❌ | ✅ |
| `network_ids` | Use an empty set | ❌ | ✅ |

Zone lifecycle and membership acceptance tests require `TERRIFI_ACC_TARGET=hardware` with an adopted gateway and skip on Docker.

Evidence: [firewall_zone_resource_test.go](https://github.com/alexklibisz/terraform-provider-terrifi/blob/main/internal/provider/firewall_zone_resource_test.go).

## Import

Firewall zones can be imported using the zone ID:

```shell
terraform import terrifi_firewall_zone.iot <id>
```

To import a firewall zone from a non-default site, use the `site:id` format:

```shell
terraform import terrifi_firewall_zone.iot <site>:<id>
```

You can also use the [Terrifi CLI](../index.md#cli) to generate import blocks for all firewall zones automatically:

```shell
terrifi generate-imports terrifi_firewall_zone
```
