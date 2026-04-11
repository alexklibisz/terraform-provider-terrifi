---
page_title: "terrifi_device Resource - Terrifi"
subcategory: ""
description: |-
  Manages settings on an adopted UniFi network device.
---

# terrifi_device (Resource)

Manages settings on an adopted UniFi network device (access point, switch, or gateway). The device must already be adopted by the controller. This resource does not adopt or forget devices — it only manages configurable properties like name, LED behavior, and SNMP settings. Removing the resource from Terraform state does not affect the device on the controller.

## Example Usage

### Basic — set device name

```terraform
resource "terrifi_device" "living_room_ap" {
  mac  = "aa:bb:cc:dd:ee:ff"
  name = "Living Room AP"
}
```

### LED override

```terraform
resource "terrifi_device" "office_ap" {
  mac          = "aa:bb:cc:dd:ee:ff"
  name         = "Office AP"
  led_override = "off"
}
```

### SNMP settings

```terraform
resource "terrifi_device" "core_switch" {
  mac           = "11:22:33:44:55:66"
  name          = "Core Switch"
  snmp_contact  = "admin@example.com"
  snmp_location = "Server Room A, Rack 1"
}
```

### Multiple settings

```terraform
resource "terrifi_device" "gateway" {
  mac           = "aa:bb:cc:11:22:33"
  name          = "Main Gateway"
  led_override  = "on"
  locked        = true
  snmp_contact  = "noc@example.com"
  snmp_location = "DC1"
}
```

### Using with device data source

```terraform
data "terrifi_device" "ap" {
  name = "Living Room AP"
}

resource "terrifi_device" "ap" {
  mac          = data.terrifi_device.ap.mac
  name         = "Living Room AP"
  led_override = "off"
  locked       = true
}
```

## Schema

### Required

- `mac` (String) — The MAC address of the device (e.g. `aa:bb:cc:dd:ee:ff`). The device must already be adopted by the controller. Changing this forces a new resource.

### Optional

- `name` (String) — The display name for the device.
- `led_override` (String) — LED behavior override: `default` (follows site setting), `on`, or `off`.
- `led_override_color` (String) — LED color override as a hex string (e.g. `#0000ff`).
- `led_override_color_brightness` (Number) — LED color brightness override (0–100).
- `outdoor_mode_override` (String) — Outdoor mode override: `default`, `on`, or `off`.
- `locked` (Boolean) — Whether the device is locked to prevent accidental removal.
- `disabled` (Boolean) — Whether the device is administratively disabled.
- `snmp_contact` (String) — SNMP contact string (max 255 characters).
- `snmp_location` (String) — SNMP location string (max 255 characters).
- `volume` (Number) — Speaker volume (0–100). Only applicable to devices with speakers.
- `site` (String) — The site the device belongs to. Defaults to the provider site. Changing this forces a new resource.

### Read-Only

- `id` (String) — The ID of the device.
- `model` (String) — The hardware model (e.g. `U6-LR`, `US-16-XG`).
- `type` (String) — The device type (e.g. `uap`, `usw`, `ugw`).
- `ip` (String) — The current IP address.
- `adopted` (Boolean) — Whether the device is adopted.
- `state` (Number) — The device state (0 = unknown, 1 = connected, 2 = pending, 4 = upgrading, 5 = provisioning, 6 = heartbeat missed).

## Import

Devices can be imported using their MAC address:

```shell
terraform import terrifi_device.ap aa:bb:cc:dd:ee:ff
```

To import from a non-default site, use the `site:mac` format:

```shell
terraform import terrifi_device.ap mysite:aa:bb:cc:dd:ee:ff
```

You can also use the [Terrifi CLI](../index.md#cli) to generate import blocks for all devices automatically:

```shell
terrifi generate-imports terrifi_device
```
