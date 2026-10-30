---
page_title: "terrifi_device Data Source - Terrifi"
subcategory: ""
description: |-
  Looks up a UniFi network device (access point, switch, or gateway).
---

# terrifi_device (Data Source)

Looks up a UniFi network device (access point, switch, or gateway) by name or MAC address. Use this data source to reference device attributes in other resources — for example, to pin a client device to a specific access point.

## Example Usage

### Look up by name

```terraform
data "terrifi_device" "living_room_ap" {
  name = "Living Room AP"
}
```

### Look up by MAC

```terraform
data "terrifi_device" "office_ap" {
  mac = "aa:bb:cc:dd:ee:ff"
}
```

### Pin a client to an access point

```terraform
data "terrifi_device" "office_ap" {
  name = "Office AP"
}

resource "terrifi_client_device" "laptop" {
  mac          = "11:22:33:44:55:66"
  name         = "Work Laptop"
  fixed_ap_mac = data.terrifi_device.office_ap.mac
}
```

## Schema

### Required (one of)

Exactly one of `name` or `mac` must be specified.

- `name` (String) — The name of the device to look up.
- `mac` (String) — The MAC address of the device to look up (e.g. `aa:bb:cc:dd:ee:ff`).

### Optional

- `site` (String) — The site to look up the device in. Defaults to the provider site.

### Read-Only

- `id` (String) — The ID of the device.
- `model` (String) — The hardware model of the device (e.g. `U6-LR`, `US-16-XG`).
- `type` (String) — The device type (e.g. `uap` for access point, `usw` for switch, `ugw` for gateway).
- `ip` (String) — The current IP address of the device.
- `disabled` (Boolean) — Whether the device is administratively disabled.
- `adopted` (Boolean) — Whether the device has been adopted by the controller.
- `state` (Number) — The device state. 0 = unknown, 1 = connected, 2 = pending, 4 = upgrading, 5 = provisioning, 6 = heartbeat missed.

## Validation coverage

✅ Covered · ❌ Not covered · ⚠️ Conditional. See [validation coverage](../guides/validation-coverage.md) for definitions and limitations.

| Setting | Usage | Simulated | Hardware-in-the-loop |
| --- | --- | :---: | :---: |
| `name` | Look up an adopted device by name | ✅ | ✅ |
| `mac` | Look up an adopted device by MAC | ✅ | ✅ |
| `id` | Read from the controller | ✅ | ✅ |
| `model` | Read from the controller | ✅ | ✅ |
| `type` | Read from the controller | ✅ | ✅ |
| `adopted` | Read from the controller | ✅ | ✅ |

Lookup tests run against both controller targets. They require an adopted device and skip if none is available; lookup by name also requires that device to have a name.

Evidence: [device_data_source_test.go](https://github.com/alexklibisz/terraform-provider-terrifi/blob/main/internal/provider/device_data_source_test.go).
