---
page_title: "terrifi_client_device Resource - Terrifi"
subcategory: ""
description: |-
  Manages a client device on the UniFi controller.
---

# terrifi_client_device (Resource)

Manages a client device on the UniFi controller. Use this resource to set aliases, notes, fixed IPs, VLAN overrides, local DNS records, custom device icons, AP locking, and blocked status for known clients.

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

### Custom device icon

Use `device_type_id` to set a custom icon for the device in the UniFi UI. Browse available IDs with `terrifi list-device-types` (CSV) or `terrifi list-device-types --html` to generate a browsable HTML page with icons, fuzzy search, and filterable dropdowns. See the [CLI docs](../index.md#list-device-types) for details.

```terraform
resource "terrifi_client_device" "server" {
  mac            = "aa:bb:cc:11:22:33"
  name           = "Proxmox Server"
  device_type_id = 1084
}
```

### Lock to access point

```terraform
resource "terrifi_client_device" "laptop" {
  mac          = "aa:bb:cc:dd:ee:ff"
  name         = "Work Laptop"
  fixed_ap_mac = "11:22:33:44:55:66"
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
- `note` (String) — A free-text note for the client device. When the configuration does not set it, Terrifi keeps the note the controller already holds, such as one added in the UniFi UI, instead of reporting drift. Removing `note` from the configuration does not clear the note on the controller; the provider does not support clearing notes. An empty string is rejected.
- `fixed_ip` (String) — A fixed IP address to assign via DHCP reservation. Requires `network_id` or `network_override_id`.
- `network_id` (String) — The network ID for fixed IP assignment. Required when `fixed_ip` is set unless `network_override_id` provides the network context.
- `network_override_id` (String) — The network ID for VLAN/network override.
- `local_dns_record` (String) — A local DNS hostname for this client device. Requires `fixed_ip`.
- `client_group_ids` (Set of String) — Set of client group IDs to assign this device to. Use `terrifi_client_group` to manage groups.
- `device_type_id` (Number) — The device type ID (fingerprint override) to set a custom icon. Use `terrifi list-device-types` to list IDs as CSV, or `terrifi list-device-types --html` to generate a browsable page with icons and fuzzy search.
- `fixed_ap_mac` (String) — The MAC address of the access point to lock this client to (e.g. `aa:bb:cc:dd:ee:ff`). When set, the client will only connect to this AP.
- `blocked` (Boolean) — Whether the client device is blocked from network access. Defaults to `false`.
- `site` (String) — The site to associate the client device with. Defaults to the provider site. Changing this forces a new resource.

### Read-Only

- `id` (String) — The ID of the client device.

## Validation coverage

✅ Covered · ❌ Not covered · ◻️ Unavailable on this target · ⚠️ Conditional. See [validation coverage](../guides/validation-coverage.md) for definitions and limitations.

| Setting | Usage | Simulated | Hardware-in-the-loop |
| --- | --- | :---: | :---: |
| `id` | Import by internal ID or `site:ID` | ✅ | ✅ |
| `id` | Import a DHCP reservation by MAC or `site:MAC`, refresh, and update | ✅ | ✅ |
| `name` | Set a client alias | ✅ | ✅ |
| `name` | Rename a client | ✅ | ✅ |
| `note` | Set a note | ✅ | ✅ |
| `note` | Import a client with a note | ✅ | ✅ |
| `note` | Omit to keep a note set outside Terraform | ✅ | ✅ |
| `note` | Remove from the configuration to keep the existing note | ✅ | ✅ |
| `fixed_ip` | Assign a DHCP reservation | ✅ | ✅ |
| `fixed_ip` | Remove the reservation | ✅ | ✅ |
| `network_id` | Select the reservation network | ✅ | ✅ |
| `network_override_id` | Assign a network override | ✅ | ✅ |
| `local_dns_record` | Set a local hostname | ✅ | ✅ |
| `local_dns_record` | Remove the hostname | ✅ | ✅ |
| `client_group_ids` | Assign group membership | ✅ | ✅ |
| `client_group_ids` | Remove a group membership | ✅ | ✅ |
| `device_type_id` | Set an icon override | ✅ | ✅ |
| `device_type_id` | Remove the override | ✅ | ✅ |
| `fixed_ap_mac` | Lock a client to an AP | ✅ | ✅ |
| `fixed_ap_mac` | Remove the AP lock | ✅ | ✅ |
| `blocked` | Set to `true` | ✅ | ✅ |
| `blocked` | Set to `false` | ✅ | ✅ |
| `blocked` | Omit to use `false` | ✅ | ✅ |

The acceptance suite runs against both controller targets and checks saved client settings. Tests use generated client MAC addresses.

Evidence: [client_device_resource_test.go](https://github.com/alexklibisz/terraform-provider-terrifi/blob/main/internal/provider/client_device_resource_test.go).

## Import

Client devices can be imported using the controller's internal device ID or a colon-separated MAC address.
Both forms accept an optional site prefix.
Without a prefix, the import uses the provider's `site` setting, falling back to `UNIFI_SITE`, then `default` if neither is set.
Terrifi looks up the client only in that site; it does not search other sites or move the client between sites.
The resolved site is saved in Terraform state during the first read.

Import by internal ID:

```shell
terraform import terrifi_client_device.printer <id>
```

To specify a site, use the `site:ID` format:

```shell
terraform import terrifi_client_device.printer <site>:<id>
```

Import by MAC address, with or without a site prefix:

```shell
terraform import terrifi_client_device.printer aa:bb:cc:dd:ee:ff
terraform import terrifi_client_device.printer default:aa:bb:cc:dd:ee:ff
```

MAC addresses are matched case-insensitively.
The client must already exist on the controller in the selected site.
The first read resolves the MAC to the controller's internal ID, which is stored in Terraform state and used for subsequent operations.
DHCP reservation network IDs and group memberships are populated from the controller response during import.
Some controller versions omit group membership from client responses; keep `client_group_ids` in the resource configuration and apply it after import when necessary.
The CLI generates imports using internal IDs.

You can also use the [Terrifi CLI](../index.md#cli) to generate import blocks for all client devices automatically:

```shell
terrifi generate-imports terrifi_client_device
```
