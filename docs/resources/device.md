---
page_title: "terrifi_device Resource - Terrifi"
subcategory: ""
description: |-
  Manages settings on an adopted UniFi network device.
---

# terrifi_device (Resource)

Manages settings on an adopted UniFi network device (access point, switch, or gateway). The device must already be adopted by the controller. This resource does not adopt or forget devices — it only manages configurable properties like name, LED behavior, and SNMP settings. Removing the resource from Terraform state does not affect the device on the controller.

To assign port profiles and names to a device's ports, use [`terrifi_device_ports`](device_ports.md).

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
    channel             = "auto"
    channel_width       = 40
    transmit_power_mode = "auto"
  }

  radio_5 = {
    channel             = "auto"
    channel_width       = 80
    transmit_power_mode = "auto"
  }
}
```

See [radio configuration notes](#radio-settings) for how omitted settings are handled.

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

### DHCP management IP

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

All fields are optional. See [radio configuration notes](#radio-settings) for how omitted settings are handled.

- `channel` (String) — Channel number or `auto`. Valid channels depend on the radio band and country code.
- `channel_width` (Number) — Channel width in MHz. Accepted values: `20`, `40`, `80`, `160`, `240`, `320`.
- `transmit_power_mode` (String) — Transmit power mode: `auto`, `high`, `medium`, `low`, `custom`, or `disabled`.
- `transmit_power` (String) — Transmit power in dBm. Only used when `transmit_power_mode` is `custom`.
- `min_rssi_enabled` (Boolean) — Whether the minimum RSSI client association threshold is enabled.
- `min_rssi` (Number) — Minimum RSSI threshold for client association (dBm, -90 to -67).

### Read-Only

- `id` (String) — The ID of the device.
- `model` (String) — The hardware model (e.g. `U6-LR`, `US-16-XG`).
- `type` (String) — The device type (e.g. `uap`, `usw`, `ugw`).
- `ip` (String) — The current IP address.
- `adopted` (Boolean) — Whether the device is adopted.
- `state` (Number) — The device state (0 = unknown, 1 = connected, 2 = pending, 4 = upgrading, 5 = provisioning, 6 = heartbeat missed).

## Validation coverage

✅ Covered · ❌ Not covered · ◻️ Unavailable on this target · ⚠️ Conditional. See [validation coverage](../guides/validation-coverage.md) for definitions and limitations.

| Setting | Usage | Simulated | Hardware-in-the-loop |
| --- | --- | :---: | :---: |
| `name` | Rename an adopted device | ✅ | ✅ |
| `name` | Remove the configured name | ✅ | ✅ |
| `led_enabled` | Set to `false` | ✅ | ✅ |
| `led_enabled` | Set to `true` | ✅ | ✅ |
| `led_enabled` | Omit to follow the site default | ✅ | ✅ |
| `led_color` | Set an explicit color | ❌ | ❌ |
| `led_brightness` | Set an explicit brightness | ❌ | ❌ |
| `outdoor_mode_override` | Set to `on` | ❌ | ❌ |
| `locked` | Set to `true` | ✅ | ✅ |
| `locked` | Set to `false` | ✅ | ✅ |
| `disabled` | Set to `true` | ✅ | ✅ |
| `disabled` | Set to `false` | ✅ | ✅ |
| `snmp_contact` | Set a contact string | ✅ | ✅ |
| `snmp_contact` | Remove the contact string | ✅ | ✅ |
| `snmp_location` | Set a location string | ✅ | ✅ |
| `snmp_location` | Remove the location string | ✅ | ✅ |
| `volume` | Set an explicit volume | ❌ | ❌ |
| `config_network.type` | Set to `dhcp` | ✅ | ✅ |
| `config_network.type` | Set to `static` (opt-in test) | ❌ | ⚠️ |
| `config_network.ip` | Assign a static address (opt-in test) | ❌ | ⚠️ |
| `config_network.netmask` | Set a static netmask (opt-in test) | ❌ | ⚠️ |
| `config_network.gateway` | Set a static gateway (opt-in test) | ❌ | ⚠️ |
| `config_network.dns1` | Set a primary DNS server | ❌ | ❌ |
| `config_network.dns2` | Set a secondary DNS server | ❌ | ❌ |
| `radio_24.channel` | Set to `auto` | ✅ | ✅ |
| `radio_24.channel_width` | Set to `40` | ✅ | ✅ |
| `radio_24.transmit_power_mode` | Set to `medium` | ✅ | ✅ |
| `radio_24.transmit_power_mode` | Set to `auto` | ✅ | ✅ |
| `radio_5.channel` | Set to `auto` | ✅ | ✅ |
| `radio_5.channel_width` | Set to `80` | ✅ | ✅ |
| `radio_5.transmit_power_mode` | Set to `auto` | ✅ | ✅ |
| `radio_24.transmit_power` | Set custom transmit power | ❌ | ❌ |
| `radio_24.min_rssi_enabled` | Enable the minimum RSSI threshold | ❌ | ❌ |
| `radio_24.min_rssi` | Set the minimum RSSI threshold | ❌ | ❌ |
| `radio_5.transmit_power` | Set custom transmit power | ❌ | ❌ |
| `radio_5.min_rssi_enabled` | Enable the minimum RSSI threshold | ❌ | ❌ |
| `radio_5.min_rssi` | Set the minimum RSSI threshold | ❌ | ❌ |
| `radio_6.channel` | Set a channel | ❌ | ❌ |
| `radio_6.channel_width` | Set a channel width | ❌ | ❌ |
| `radio_6.transmit_power_mode` | Set a transmit-power mode | ❌ | ❌ |
| `radio_6.transmit_power` | Set custom transmit power | ❌ | ❌ |
| `radio_6.min_rssi_enabled` | Enable the minimum RSSI threshold | ❌ | ❌ |
| `radio_6.min_rssi` | Set the minimum RSSI threshold | ❌ | ❌ |

The suite uses virtual adopted devices on the Simulated target and adopted hardware on the Hardware-in-the-loop target. Radio tests require an adopted AP, and multi-radio tests skip if it has fewer than two radios. ⚠️ Static management tests are opt-in: they require `TERRIFI_ACC_DEVICE_STATIC_IP`, `TERRIFI_ACC_DEVICE_STATIC_NETMASK`, and `TERRIFI_ACC_DEVICE_STATIC_GATEWAY`; the standard HIL workflow does not configure these variables.

Evidence: [device_resource_test.go](https://github.com/alexklibisz/terraform-provider-terrifi/blob/main/internal/provider/device_resource_test.go).

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

## Additional Notes

### Radio settings

Each radio block is independent — omit a block to leave that radio's settings unchanged. Omit all blocks to leave the controller's current radio configuration untouched.

Within a configured radio block, omitted fields are also left unchanged on the controller.

### Management address changes

Omitting `config_network` leaves the device's current management configuration unchanged. To revert to DHCP, change `type` to `"dhcp"` and remove the addressing fields, as shown in the [DHCP management IP example](#dhcp-management-ip).
