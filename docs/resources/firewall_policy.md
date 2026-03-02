---
page_title: "terrifi_firewall_policy Resource - Terrifi"
subcategory: ""
description: |-
  Manages a firewall policy on the UniFi controller.
---

# terrifi_firewall_policy (Resource)

Manages a firewall policy on the UniFi controller. Firewall policies define traffic rules between firewall zones (e.g., block traffic from an IoT zone to a Trusted zone).

~> **Prerequisite:** Zone-based firewall must be enabled on your controller before using this resource. See `terrifi_firewall_zone` for setup instructions.

## Example Usage

### Block traffic between zones

```terraform
resource "terrifi_firewall_zone" "iot" {
  name = "IoT"
}

resource "terrifi_firewall_zone" "trusted" {
  name = "Trusted"
}

resource "terrifi_firewall_policy" "block_iot_to_trusted" {
  name   = "Block IoT to Trusted"
  action = "BLOCK"

  source {
    zone_id = terrifi_firewall_zone.iot.id
  }

  destination {
    zone_id = terrifi_firewall_zone.trusted.id
  }
}
```

### Allow specific IPs and port

```terraform
resource "terrifi_firewall_policy" "allow_https" {
  name      = "Allow HTTPS from management"
  action    = "ALLOW"
  protocol  = "tcp"

  source {
    zone_id = terrifi_firewall_zone.management.id
    ips     = ["10.0.0.0/24"]
  }

  destination {
    zone_id            = terrifi_firewall_zone.servers.id
    port_matching_type = "SPECIFIC"
    port               = 443
  }
}
```

### Block by MAC address

```terraform
resource "terrifi_firewall_policy" "block_mac" {
  name   = "Block specific device"
  action = "BLOCK"

  source {
    zone_id       = terrifi_firewall_zone.iot.id
    mac_addresses = ["aa:bb:cc:dd:ee:ff"]
  }

  destination {
    zone_id = terrifi_firewall_zone.trusted.id
  }
}
```

### Weekly schedule

```terraform
resource "terrifi_firewall_policy" "weekday_block" {
  name   = "Block during work hours"
  action = "BLOCK"

  source {
    zone_id = terrifi_firewall_zone.guest.id
  }

  destination {
    zone_id = terrifi_firewall_zone.internal.id
  }

  schedule {
    mode             = "EVERY_WEEK"
    time_range_start = "08:00"
    time_range_end   = "17:00"
    repeat_on_days   = ["mon", "tue", "wed", "thu", "fri"]
  }
}
```

## Schema

### Required

- `name` (String) — The name of the firewall policy.
- `action` (String) — The action to take. Valid values: `ALLOW`, `BLOCK`, `REJECT`.
- `source` (Block) — Source endpoint configuration. See [Source/Destination](#sourcedestination) below.
- `destination` (Block) — Destination endpoint configuration. See [Source/Destination](#sourcedestination) below.

### Optional

- `description` (String) — A description of the firewall policy.
- `enabled` (Boolean) — Whether the policy is enabled. Default: `true`.
- `ip_version` (String) — IP version to match. Valid values: `BOTH`, `IPV4`, `IPV6`. Default: `BOTH`.
- `protocol` (String) — Protocol to match. Valid values: `all`, `tcp`, `udp`, `tcp_udp`. Default: `all`.
- `connection_state_type` (String) — Connection state type. Valid values: `ALL`, `RESPOND_ONLY`. Default: `ALL`.
- `connection_states` (Set of String) — Connection states to match (e.g. `NEW`, `ESTABLISHED`, `RELATED`, `INVALID`).
- `match_ipsec` (Boolean) — Whether to match IPsec traffic.
- `logging` (Boolean) — Whether to enable syslog logging for matched traffic.
- `create_allow_respond` (Boolean) — Whether to create a corresponding allow-respond rule.
- `schedule` (Block) — Schedule configuration. See [Schedule](#schedule) below.
- `site` (String) — The site. Defaults to the provider site. Changing this forces a new resource.

### Read-Only

- `id` (String) — The ID of the firewall policy.
- `index` (Number) — The ordering index of the policy, assigned by the controller.

### Source/Destination

- `zone_id` (String, Required) — The firewall zone ID.
- `ips` (Set of String) — IP addresses or CIDR ranges to match.
- `mac_addresses` (Set of String) — MAC addresses to match. **Note:** Currently only supported in the `source` block. The UniFi v2 API uses different enum types for source vs. destination matching targets, and the destination enum does not include `MAC` (see [#69](https://github.com/alexklibisz/terraform-provider-terrifi/issues/69)).
- `network_ids` (Set of String) — Network IDs to match.
- `device_ids` (Set of String) — Client device MAC addresses to match. Use the `mac` attribute from `terrifi_client_device` resources.
- `port_matching_type` (String) — Port matching type. Valid values: `ANY`, `SPECIFIC`, `LIST`. Default: `ANY`.
- `port` (Number) — Specific port number (when `port_matching_type` is `SPECIFIC`).
- `port_group_id` (String) — Port group ID (when `port_matching_type` is `LIST`).
- `match_opposite_ports` (Boolean) — Inverts the port matching. When `true` and action is `ALLOW`, all ports _except_ the specified ones are allowed. When `true` and action is `BLOCK`, all ports _except_ the specified ones are blocked.
- `match_opposite_ips` (Boolean) — Inverts the IP matching. When `true` and action is `ALLOW`, all IPs _except_ the specified ones are allowed. When `true` and action is `BLOCK`, all IPs _except_ the specified ones are blocked.

At most one of `ips`, `mac_addresses`, `network_ids`, or `device_ids` may be set. When none is set, the endpoint matches any target.

### Schedule

- `mode` (String, Required) — Schedule mode. Valid values: `ALWAYS`, `EVERY_DAY`, `EVERY_WEEK`, `ONE_TIME_ONLY`.
- `date` (String) — Date for one-time schedules.
- `time_all_day` (Boolean) — Whether the schedule applies all day.
- `time_range_start` (String) — Start time (e.g. `08:00`).
- `time_range_end` (String) — End time (e.g. `17:00`).
- `repeat_on_days` (Set of String) — Days of the week. Valid values: `mon`, `tue`, `wed`, `thu`, `fri`, `sat`, `sun`.

## Import

Firewall policies can be imported using the policy ID:

```shell
terraform import terrifi_firewall_policy.example <id>
```

To import from a non-default site, use the `site:id` format:

```shell
terraform import terrifi_firewall_policy.example <site>:<id>
```

You can also use the [Terrifi CLI](../cli.md) to generate import blocks for all firewall policies automatically:

```shell
terrifi generate-imports terrifi_firewall_policy
```
