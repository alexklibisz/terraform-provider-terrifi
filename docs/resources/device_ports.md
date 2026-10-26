---
page_title: "terrifi_device_ports Resource - Terrifi"
subcategory: ""
description: |-
  Configures inline VLAN settings, port profiles, and names on adopted UniFi switches and gateways.
---

# terrifi_device_ports (Resource)

Configures VLAN settings and names on device ports, either directly or through reusable [port profiles](port_profile.md).

Only the ports listed in `ports` are managed. Every other port on the device is left as it is, including ports configured in the UniFi UI.

Use **one `terrifi_device_ports` per device**. The controller stores all of a device's ports as a single list, so two resources for the same device overwrite each other. Device-level settings (name, LEDs, SNMP, ...) belong in [`terrifi_device`](device.md).

## Example Usage

```terraform
resource "terrifi_device_ports" "core_switch" {
  device_mac = "aa:bb:cc:dd:ee:ff"

  ports = {
    "1" = {
      name            = "Hypervisor 1"
      port_profile_id = terrifi_port_profile.hypervisor_trunk.id
    }
    "2" = {
      name              = "Server"
      native_network_id = terrifi_network.servers.id
      tagged_vlan_mode  = "block_all"
    }
    "8" = {
      name = "Uplink"
    }
  }
}
```

## Schema

### Required

- `device_mac` (String) The MAC address of the adopted device (e.g. `aa:bb:cc:dd:ee:ff`). Changing it replaces this resource.
- `ports` (Attributes Map) The managed ports, keyed by port number (`"1"`, `"2"`, ...). (see [below for nested schema](#nestedatt--ports))

### Optional

- `site` (String) The site the device belongs to. Defaults to the provider site.

### Read-Only

- `id` (String) The ID of the device.

<a id="nestedatt--ports"></a>
### Nested Schema for `ports`

Optional:

- `name` (String) The port name. Omit to use the controller's default (`Port N`).
- `port_profile_id` (String) The ID of the `terrifi_port_profile` to assign. Conflicts with inline VLAN attributes.
- `native_network_id` (String) The native (untagged) LAN network on the same controller and site as the device. Requires `tagged_vlan_mode`.
- `tagged_vlan_mode` (String) `allow_all` or `block_all`. Requires `native_network_id`.

## Import

Device ports can be imported using the device MAC address. Ports with supported configurations are imported, including their names, profile assignments, or inline VLAN settings:

```shell
terraform import terrifi_device_ports.core_switch aa:bb:cc:dd:ee:ff
```

To import from a non-default site, use the `site:mac` format:

```shell
terraform import terrifi_device_ports.core_switch <site>:aa:bb:cc:dd:ee:ff
```

You can also use the [Terrifi CLI](../index.md#cli) to generate import blocks for every device with named, profiled, or inline-configured ports:

```shell
terrifi generate-imports terrifi_device_ports
```

Ports with unsupported inline VLAN settings or forwarding modes (including
disabled ports and inline voice VLANs) are excluded entirely from import, including their names, and
produce a warning. They remain outside Terraform's
ownership, so updates and destroy leave them untouched. The CLI includes these
ports only as commented-out entries:

```terraform
ports = {
  # Port 3 excluded from import: inline VLAN mode "custom" is unsupported.
  # "3" = { name = "Server" }
}
```

## Additional Notes

### How ports are written

- **With `port_profile_id`**, the profile controls every setting of the port. Per-port settings made in the UI are cleared, as the UI does when you pick a profile. That includes PoE: set `poe_mode` on the profile.
- **With `native_network_id` and `tagged_vlan_mode`**, the port is configured directly. `block_all` permits only the native network; `allow_all` also permits tagged VLAN traffic. Both attributes must be set together and conflict with `port_profile_id`. Any previous VLAN settings on the port, including a voice VLAN, are replaced. Other inline settings are preserved.
- **With a name alone**, any profile is removed and existing inline settings are preserved. Removing previously managed inline VLAN attributes restores the default VLAN configuration.
- **Without `name`**, the port shows the controller's default name (`Port N`).
- **Removing a port from `ports`**, or destroying the resource, deletes its entire port override. See [port ownership and removal](#port-ownership-and-removal).

On a gateway, inline VLAN settings and port profiles only apply to LAN ports. Assigning a profile to a WAN port (set under the gateway's WAN settings) is an error, because the controller strips the profile when a port becomes a WAN port.

Each change re-provisions the device. On the switches tested this didn't interrupt traffic on other ports.

### Port ownership and removal

Listing a port in `ports` puts its entire controller override under Terraform's management. Updates preserve unspecified inline settings, but removal deletes the whole override. For example, a name-only entry can preserve an existing inline VLAN and PoE configuration; removing that entry from the map and applying also removes those preserved settings.

Terrifi removes the override rather than sending a hard-coded set of defaults. The controller determines how the port behaves without it. This avoids field-by-field reset rules for settings that interact with each other, such as profiles, native networks, and tagged VLANs. Unlisted ports and device-level settings are untouched.

Deleting the override also releases its profile assignment. When `port_profile_id` references a managed profile, Terraform removes the assignment before deleting the profile; the controller refuses to delete profiles that are still assigned.

Removing the resource with `terraform state rm` only stops tracking it; it does not change any port settings.
