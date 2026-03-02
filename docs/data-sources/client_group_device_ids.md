---
page_title: "terrifi_client_group_device_ids Data Source - Terrifi"
subcategory: ""
description: |-
  Returns the IDs of client devices belonging to a particular client group.
---

# terrifi_client_group_device_ids (Data Source)

Returns the IDs of client devices belonging to a particular client group. Use this to reference grouped devices in firewall policies.

This data source is most useful when devices are assigned to groups outside of Terraform (e.g. via the UniFi UI) and you want to reference those group memberships in your Terraform config.

## Example Usage

### Look up devices assigned to a group via the UI

```terraform
data "terrifi_client_group_device_ids" "iot" {
  client_group_id = "existing-group-id"
}
```

### Use with a firewall policy

```terraform
resource "terrifi_firewall_policy" "block_iot_wan" {
  name    = "Block IoT to WAN"
  action  = "BLOCK"
  enabled = true

  source {
    zone_id    = terrifi_firewall_zone.iot.id
    device_ids = data.terrifi_client_group_device_ids.iot.ids
  }

  destination {
    zone_id = terrifi_firewall_zone.wan.id
  }
}
```

## Schema

### Required

- `client_group_id` (String) — The ID of the client group to look up.

### Optional

- `site` (String) — The site to query. Defaults to the provider site.

### Read-Only

- `ids` (Set of String) — The set of client device IDs belonging to the group.
