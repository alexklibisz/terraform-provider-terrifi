---
page_title: "terrifi_device_ports Resource - Terrifi"
subcategory: ""
description: |-
  Configures inline VLAN settings, port profiles, and names on adopted UniFi switches and gateways.
---

# terrifi_device_ports (Resource)

Configures ports directly, or assigns reusable [port profiles](port_profile.md). The original USW Flex Mini uses direct configuration because it does not support custom profiles.

Only the ports listed in `ports` are managed. Every other port on the device is left as it is, including ports configured in the UniFi UI.

Use **one `terrifi_device_ports` per device**. The controller stores all of a device's ports as a single list, so two resources for the same device overwrite each other. Device-level settings (name, LEDs, SNMP, ...) belong in [`terrifi_device`](device.md).

## How ports are written

- **With `port_profile_id`**, the profile controls every setting of the port. Per-port settings made in the UI are cleared, as the UI does when you pick a profile. That includes PoE: set `poe_mode` on the profile.
- **With `native_network_id` and `tagged_vlan_mode`**, the port is configured directly. `block_all` permits only the native network; `allow_all` also permits tagged VLAN traffic. Both attributes must be set together and conflict with `port_profile_id`. Other inline settings are preserved.
- **With a name alone**, any profile is removed and existing inline settings are preserved. Removing previously managed inline VLAN attributes restores the default VLAN configuration.
- **Without `name`**, the port shows the controller's default name (`Port N`).
- **Removing a port from `ports`**, or destroying the resource, removes its entire port override and restores controller defaults, including settings originally made in the UI. Unlisted ports are untouched. This does not factory-reset or unadopt the device.

A profile can't be deleted while a port still uses it; the controller refuses. Because `port_profile_id` references the profile, Terraform removes the assignment before it deletes the profile, so `terraform destroy` works as expected.

On a gateway, inline VLAN settings and port profiles only apply to LAN ports. Assigning a profile to a WAN port (set under the gateway's WAN settings) is an error, because the controller strips the profile when a port becomes a WAN port.

Each change re-provisions the device. On the switches tested this didn't interrupt traffic on other ports.

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
      name            = "Hypervisor 2"
      port_profile_id = terrifi_port_profile.hypervisor_trunk.id
    }
    "8" = {
      name = "Uplink"
    }
  }
}
```

## USW Flex Mini

The original `USW-Flex-Mini` (`USMINI`) supports inline access ports using a native network and `block_all`. `allow_all` is supported only with its default native network. Custom profiles are rejected before any device write. Selective tagged VLAN lists are not currently supported by the inline API.

For example, the switch on your home controller can place the HIL gateway's WAN connection on a dedicated home VLAN:

```terraform
resource "terrifi_device_ports" "home_switch" {
  provider   = terrifi.home
  device_mac = terrifi_device.home_switch.mac

  ports = {
    "3" = {
      name              = "HIL gateway WAN"
      native_network_id = terrifi_network.hil_uplink.id
      tagged_vlan_mode  = "block_all"
    }
  }
}
```

The network reference must belong to the same controller and site as the switch. Firewall rules controlling access from that VLAN are configured separately.

## Schema

### Required

- `device_mac` (String) The MAC address of the adopted device (e.g. `aa:bb:cc:dd:ee:ff`). Changing it moves the assignments to another device.
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
- `native_network_id` (String) The native (untagged) LAN network. Requires `tagged_vlan_mode`.
- `tagged_vlan_mode` (String) `allow_all` or `block_all`. Requires `native_network_id`.

## Import

Device ports can be imported using the device MAC address. Every port with a name, a profile, or an inline native network is imported:

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
