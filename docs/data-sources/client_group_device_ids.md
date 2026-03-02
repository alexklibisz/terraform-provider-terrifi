---
page_title: "terrifi_client_group_device_ids Data Source - Terrifi"
subcategory: ""
description: |-
  Returns the IDs of client devices belonging to a particular client group.
---

# terrifi_client_group_device_ids (Data Source)

Returns the IDs of client devices belonging to a particular client group. Use this to reference grouped devices in firewall policies.

## Example Usage

### Look up device IDs for a client group

```terraform
resource "terrifi_client_group" "iot" {
  name = "IoT Devices"
}

resource "terrifi_client_device" "sensor" {
  mac             = "aa:bb:cc:dd:ee:01"
  name            = "Temperature Sensor"
  client_group_id = terrifi_client_group.iot.id
}

resource "terrifi_client_device" "camera" {
  mac             = "aa:bb:cc:dd:ee:02"
  name            = "IP Camera"
  client_group_id = terrifi_client_group.iot.id
}

data "terrifi_client_group_device_ids" "iot" {
  client_group_id = terrifi_client_group.iot.id

  depends_on = [
    terrifi_client_device.sensor,
    terrifi_client_device.camera,
  ]
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
