---
page_title: "terrifi_firewall_policy_order Resource - Terrifi"
subcategory: ""
description: |-
  Manages the evaluation order of firewall policies for a zone pair.
---

# terrifi_firewall_policy_order (Resource)

Manages the evaluation order of firewall policies for a specific zone pair on the UniFi controller. Policies are evaluated in the order specified by `policy_ids`, before any predefined (system) policies.

Create the policies separately using [`terrifi_firewall_policy`](firewall_policy.md).

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

## Validation coverage

✅ Covered · ❌ Not covered · ◻️ Unavailable on this target · ⚠️ Conditional. See [validation coverage](../guides/validation-coverage.md) for definitions and limitations.

| Setting | Usage | Simulated | Hardware-in-the-loop |
| --- | --- | :---: | :---: |
| `source_zone_id` | Select the source zone | ◻️ | ✅ |
| `destination_zone_id` | Select the destination zone | ◻️ | ✅ |
| `policy_ids` | Set an initial policy order | ◻️ | ✅ |
| `policy_ids` | Reorder existing policies | ◻️ | ✅ |
| `policy_ids` | Verify the imported policy order | ◻️ | ✅ |

Ordering acceptance tests require `TERRIFI_ACC_TARGET=hardware` and skip on the Simulated target. ◻️ The zone-based firewall v2 API requires a fully adopted gateway, which the current simulator does not provide. `TestAccFirewallPolicyOrder_import` verifies the imported `policy_ids`, in order, against the configured order. `TestFirewallPolicyOrderImportReadsFullOrder` is a unit test of the same import path against a fake API.

Evidence: [firewall_policy_order_resource_test.go](https://github.com/alexklibisz/terraform-provider-terrifi/blob/main/internal/provider/firewall_policy_order_resource_test.go).

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

## Additional Notes

### Import behavior

Import reads every custom (non-predefined) policy of the zone pair, in the order the controller evaluates them.
The first plan after import therefore shows a real reorder when the configured `policy_ids` differ from the controller's order, and no change when they match.
If `policy_ids` lists only some of the zone pair's policies, that first plan also shows the unlisted ones leaving the list.
After that, refreshes keep only the policies listed in `policy_ids`, so policies managed elsewhere in the same zone pair don't appear in state.

### Destroy behavior

Destroying this resource removes it from Terraform state but does not change the ordering on the controller.
