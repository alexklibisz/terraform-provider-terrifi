---
page_title: "terrifi_device_ports Resource - Terrifi"
subcategory: ""
description: |-
  Assigns port profiles and names to the ports of an adopted UniFi switch or gateway.
---

# terrifi_device_ports (Resource)

Assigns [port profiles](port_profile.md) and names to the ports of an adopted UniFi switch or gateway.

Only the ports listed in `ports` are managed. Every other port on the device is left as it is, including ports configured in the UniFi UI.

Use **one `terrifi_device_ports` per device**. The controller stores all of a device's ports as a single list, so two resources for the same device overwrite each other. Device-level settings (name, LEDs, SNMP, ...) belong in [`terrifi_device`](device.md).

## How ports are written

- **With `port_profile_id`**, the profile controls every setting of the port. Per-port settings made in the UI are cleared, as the UI does when you pick a profile. That includes PoE: set `poe_mode` on the profile.
- **Without `port_profile_id`**, any profile is removed and the port becomes a default port (all networks). Per-port settings made in the UI are kept.
- **Without `name`**, the port shows the controller's default name (`Port N`).
- **Removing a port from `ports`**, or destroying the resource, removes the port's name and profile. Per-port settings made in the UI are kept. The device itself is never changed otherwise.

A profile can't be deleted while a port still uses it; the controller refuses. Because `port_profile_id` references the profile, Terraform removes the assignment before it deletes the profile, so `terraform destroy` works as expected.

On a gateway, port profiles only apply to LAN ports. Assigning a profile to a WAN port (set under the gateway's WAN settings) is an error, because the controller strips the profile when a port becomes a WAN port.

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
- `port_profile_id` (String) The ID of the `terrifi_port_profile` to assign. Omit to remove the profile.

## Import

Device ports can be imported using the device MAC address. Every port with a name or a profile is imported:

```shell
terraform import terrifi_device_ports.core_switch aa:bb:cc:dd:ee:ff
```

To import from a non-default site, use the `site:mac` format:

```shell
terraform import terrifi_device_ports.core_switch <site>:aa:bb:cc:dd:ee:ff
```

You can also use the [Terrifi CLI](../index.md#cli) to generate import blocks for every device with named or profiled ports:

```shell
terrifi generate-imports terrifi_device_ports
```
