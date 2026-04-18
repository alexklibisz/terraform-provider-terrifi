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
  led_enabled = false
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

### Radio settings (access points)

```terraform
resource "terrifi_device" "office_ap" {
  mac  = "aa:bb:cc:dd:ee:ff"
  name = "Office AP"

  radio_24 = {
    channel       = "auto"
    ht            = 40
    tx_power_mode = "auto"
  }

  radio_5 = {
    channel       = "auto"
    ht            = 80
    tx_power_mode = "auto"
  }
}
```

Each radio block is independent — omit a block to leave that radio's settings unchanged. Omit all blocks to leave the controller's current radio configuration untouched.

### Multiple settings

```terraform
resource "terrifi_device" "gateway" {
  mac           = "aa:bb:cc:11:22:33"
  name          = "Main Gateway"
  led_enabled  = "on"
  locked        = true
  snmp_contact  = "noc@example.com"
  snmp_location = "DC1"
}
```

### Static management IP

```terraform
resource "terrifi_device" "core_switch" {
  mac  = "11:22:33:44:55:66"
  name = "Core Switch"
  config_network = {
    type    = "static"
    ip      = "192.168.1.5"
    netmask = "255.255.255.0"
    gateway = "192.168.1.1"
    dns1    = "1.1.1.1"
    dns2    = "8.8.8.8"
  }
}
```

To revert to DHCP, change `type` to `"dhcp"` and remove the addressing fields:

```terraform
resource "terrifi_device" "core_switch" {
  mac  = "11:22:33:44:55:66"
  name = "Core Switch"
  config_network = {
    type = "dhcp"
  }
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
  led_enabled = false
  locked       = true
}
```

## Schema

### Required

- `mac` (String) — The MAC address of the device (e.g. `aa:bb:cc:dd:ee:ff`). The device must already be adopted by the controller. Changing this forces a new resource.

### Optional

- `name` (String) — The display name for the device.
- `led_enabled` (Boolean) — Whether LEDs are enabled. `true` forces on, `false` forces off. Omit to follow site default.
- `led_color` (String) — LED color as a hex string (e.g. `#0000ff`).
- `led_brightness` (Number) — LED brightness (0–100).
- `outdoor_mode_override` (String) — Outdoor mode override: `default`, `on`, or `off`.
- `locked` (Boolean) — Whether the device is locked to prevent accidental removal.
- `disabled` (Boolean) — Whether the device is administratively disabled.
- `snmp_contact` (String) — [SNMP](https://en.wikipedia.org/wiki/Simple_Network_Management_Protocol) contact string (max 255 characters). Identifies who is responsible for the device; read by network monitoring tools like Nagios, PRTG, or LibreNMS.
- `snmp_location` (String) — [SNMP](https://en.wikipedia.org/wiki/Simple_Network_Management_Protocol) location string (max 255 characters). Describes where the device is physically located; read by network monitoring tools.
- `volume` (Number) — Speaker volume (0–100). Only applicable to devices with speakers.
- `radio_24` (Attributes) — Settings for the 2.4 GHz radio (UniFi `ng` radio) on an access point. See [nested schema for radio blocks](#nested-schema-for-radio-blocks).
- `radio_5` (Attributes) — Settings for the 5 GHz radio (UniFi `na` radio) on an access point. See [nested schema for radio blocks](#nested-schema-for-radio-blocks).
- `radio_6` (Attributes) — Settings for the 6 GHz radio (UniFi `6e` radio) on an access point. See [nested schema for radio blocks](#nested-schema-for-radio-blocks).
- `site` (String) — The site the device belongs to. Defaults to the provider site. Changing this forces a new resource.
- `config_network` (Block) — Management network configuration. Omit to leave the device's current configuration untouched. See [below](#config_network).

### config_network

- `type` (String, Required) — Addressing mode: `dhcp` or `static`.
- `ip` (String) — Static IPv4 address. Required when `type = "static"`; must not be set when `type = "dhcp"`.
- `netmask` (String) — Subnet mask (e.g. `255.255.255.0`). Required when `type = "static"`.
- `gateway` (String) — Default gateway IPv4 address. Required when `type = "static"`.
- `dns1` (String) — Primary DNS server.
- `dns2` (String) — Secondary DNS server.

### Nested schema for radio blocks

All fields are optional. Omitting the block entirely leaves the radio's current settings unchanged. Within a configured block, omitted fields are also left unchanged on the controller.

- `channel` (String) — Channel number or `auto`. Valid channels depend on the radio band and country code.
- `ht` (Number) — Channel width in MHz. Accepted values: `20`, `40`, `80`, `160`, `240`, `320`.
- `tx_power_mode` (String) — Transmit power mode: `auto`, `high`, `medium`, `low`, `custom`, or `disabled`.
- `tx_power` (String) — Transmit power in dBm. Only used when `tx_power_mode` is `custom`.
- `min_rssi_enabled` (Boolean) — Whether the minimum RSSI client association threshold is enabled.
- `min_rssi` (Number) — Minimum RSSI threshold for client association (dBm, -90 to -67).

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
