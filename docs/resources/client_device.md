---
page_title: "terrifi_client_device Resource - Terrifi"
subcategory: ""
description: |-
  Manages a client device on the UniFi controller.
---

# terrifi_client_device (Resource)

Manages a client device on the UniFi controller. Use this resource to set aliases, notes, fixed IPs, VLAN overrides, local DNS records, and blocked status for known clients.

## Example Usage

### Basic alias

```terraform
resource "terrifi_client_device" "printer" {
  mac  = "aa:bb:cc:dd:ee:ff"
  name = "Office Printer"
}
```

### Fixed IP with network

```terraform
resource "terrifi_network" "lan" {
  name         = "Office LAN"
  purpose      = "corporate"
  vlan_id      = 10
  subnet       = "192.168.10.1/24"
  dhcp_enabled = true
  dhcp_start   = "192.168.10.6"
  dhcp_stop    = "192.168.10.254"
}

resource "terrifi_client_device" "server" {
  mac        = "11:22:33:44:55:66"
  name       = "Home Server"
  fixed_ip   = "192.168.10.50"
  network_id = terrifi_network.lan.id
}
```

### Fixed IP with network override

When using a network override, `network_id` is not required — the override provides the network context.

```terraform
resource "terrifi_client_device" "laptop" {
  mac                 = "22:33:44:55:66:77"
  name                = "Work Laptop"
  fixed_ip            = "192.168.10.20"
  network_override_id = terrifi_network.lan.id
}
```

### Local DNS record

Local DNS records require a fixed IP assignment (controller requirement).

```terraform
resource "terrifi_client_device" "nas" {
  mac              = "aa:bb:cc:11:22:33"
  name             = "NAS"
  fixed_ip         = "192.168.10.100"
  network_id       = terrifi_network.lan.id
  local_dns_record = "nas.home"
}
```

### Assign to client groups

```terraform
resource "terrifi_client_group" "iot" {
  name = "IoT Devices"
}

resource "terrifi_client_group" "monitored" {
  name = "Monitored Devices"
}

resource "terrifi_client_device" "sensor" {
  mac              = "aa:bb:cc:dd:ee:01"
  name             = "Temperature Sensor"
  client_group_ids = [terrifi_client_group.iot.id, terrifi_client_group.monitored.id]
}
```

### Block a client

```terraform
resource "terrifi_client_device" "blocked" {
  mac     = "de:ad:be:ef:00:01"
  name    = "Blocked Device"
  blocked = true
}
```

## Schema

### Required

- `mac` (String) — The MAC address of the client device (e.g. `aa:bb:cc:dd:ee:ff`). Changing this forces a new resource.

### Optional

- `name` (String) — The alias/display name for the client device.
- `note` (String) — A free-text note for the client device.
- `fixed_ip` (String) — A fixed IP address to assign via DHCP reservation. Requires `network_id` or `network_override_id`.
- `network_id` (String) — The network ID for fixed IP assignment. Required when `fixed_ip` is set unless `network_override_id` provides the network context.
- `network_override_id` (String) — The network ID for VLAN/network override.
- `local_dns_record` (String) — A local DNS hostname for this client device. Requires `fixed_ip`.
- `client_group_ids` (Set of String) — Set of client group IDs to assign this device to. Use `terrifi_client_group` to manage groups.
- `blocked` (Boolean) — Whether the client device is blocked from network access.
- `site` (String) — The site to associate the client device with. Defaults to the provider site. Changing this forces a new resource.

### Read-Only

- `id` (String) — The ID of the client device.

## Import

Client devices can be imported using the device ID:

```shell
terraform import terrifi_client_device.printer <id>
```

To import from a non-default site, use the `site:id` format:

```shell
terraform import terrifi_client_device.printer <site>:<id>
```

You can also use the [Terrifi CLI](../cli.md) to generate import blocks for all client devices automatically:

```shell
terrifi generate-imports terrifi_client_device
```
