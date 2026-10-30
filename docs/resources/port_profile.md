---
page_title: "terrifi_port_profile Resource - Terrifi"
subcategory: ""
description: |-
  Manages a switch port profile on the UniFi controller.
---

# terrifi_port_profile (Resource)

Manages a switch port profile on the UniFi controller. Port profiles are reusable port configurations (VLANs, PoE, port isolation and LLDP-MED) that can be assigned to switch and gateway ports. Other profile settings are not managed; see [unmanaged settings](#unmanaged-settings).

A profile only defines settings. Creating or changing one doesn't affect any port until the profile is assigned to that port, with [`terrifi_device_ports`](device_ports.md) or in the UI.

Which settings take effect depends on the device a profile is assigned to. For example, a gateway's built-in switch ports may ignore settings that a managed switch honors.

## Example Usage

### Trunk port for a hypervisor

An untagged management network, plus two tagged networks.

```terraform
resource "terrifi_port_profile" "hypervisor_trunk" {
  name              = "Trunk - Hypervisor"
  native_network_id = terrifi_network.management.id
  tagged_vlan_mode  = "custom"

  tagged_network_ids = [
    terrifi_network.servers.id,
    terrifi_network.dmz.id,
  ]

  poe_mode = "off"
}
```

### Access port

A single untagged network, with all tagged traffic blocked.

```terraform
resource "terrifi_port_profile" "iot_access" {
  name              = "Access - IoT"
  native_network_id = terrifi_network.iot.id
  tagged_vlan_mode  = "block_all"
}
```

### Isolated port

An IoT port that can't reach other isolated ports, without PoE.

```terraform
resource "terrifi_port_profile" "isolated_iot" {
  name                   = "Isolated - IoT"
  native_network_id      = terrifi_network.iot.id
  tagged_vlan_mode       = "block_all"
  poe_mode               = "off"
  port_isolation_enabled = true
}
```

## Schema

### Required

- `name` (String) — The name of the port profile.

### Optional

- `site` (String) — The site to associate the port profile with. Defaults to the provider site. Changing this forces a new resource.

VLANs:

- `native_network_id` (String) — The ID of the native (untagged) network. When unset, the port has no native network (the UI's "None"). Note that the UI defaults new profiles to the Default network instead.
- `tagged_vlan_mode` (String) — Tagged VLAN management. `allow_all` (UI "Allow All") tags every network, `block_all` tags none, and `custom` tags the networks given by `tagged_network_ids`. Default: `allow_all`.
- `tagged_network_ids` (Set of String) — IDs of the networks tagged on the port, as selected in the UI. Required with `tagged_vlan_mode = "custom"`, not allowed otherwise. Must not include the native network, and must not be empty (use `tagged_vlan_mode = "block_all"` to tag no networks; the controller rewrites an empty custom selection to `block_all`). See [tagged networks](#tagged-networks).

Other:

- `poe_mode` (String) — PoE mode. `auto` is the UI's "Auto PoE" checkbox ticked, `off` unticked. Default: `auto`.
- `port_isolation_enabled` (Boolean) — Whether port isolation is enabled. Default: `false`.
- `lldpmed_enabled` (Boolean) — Whether LLDP-MED is enabled. Default: `true`.

### Read-Only

- `id` (String) — The ID of the port profile.

## Validation coverage

✅ Covered · ❌ Not covered · ⚠️ Conditional. See [validation coverage](../guides/validation-coverage.md) for definitions and limitations.

| Setting | Usage | Docker controller | Real controller |
| --- | --- | :---: | :---: |
| `name` | Rename a profile | ✅ | ✅ |
| `native_network_id` | Assign a native network | ✅ | ✅ |
| `native_network_id` | Change the native network | ✅ | ✅ |
| `native_network_id` | Remove the native network | ✅ | ✅ |
| `tagged_vlan_mode` | Set to `custom` | ✅ | ✅ |
| `tagged_vlan_mode` | Set to `block_all` | ✅ | ✅ |
| `tagged_vlan_mode` | Omit to restore `allow_all` | ✅ | ✅ |
| `tagged_network_ids` | Select a tagged network | ✅ | ✅ |
| `tagged_network_ids` | Add a tagged network | ✅ | ✅ |
| `poe_mode` | Set to `off` | ✅ | ✅ |
| `poe_mode` | Remove to restore `auto` | ✅ | ✅ |
| `port_isolation_enabled` | Set to `true` | ✅ | ✅ |
| `port_isolation_enabled` | Remove to restore `false` | ✅ | ✅ |
| `lldpmed_enabled` | Set to `false` | ✅ | ✅ |
| `lldpmed_enabled` | Remove to restore `true` | ✅ | ✅ |

Profile configuration acceptance tests run against both controller targets, including imports, defaults, drift, and preservation of unmanaged fields. These profile CRUD tests run on the real controller even though its USW Flex Mini cannot accept custom-profile assignments. Assignment coverage is documented separately on `terrifi_device_ports`.

Evidence: [port_profile_resource_test.go](https://github.com/alexklibisz/terraform-provider-terrifi/blob/main/internal/provider/port_profile_resource_test.go).

## Import

Port profiles can be imported using the profile ID:

```shell
terraform import terrifi_port_profile.hypervisor_trunk <id>
```

To import a profile from a non-default site, use the `site:id` format:

```shell
terraform import terrifi_port_profile.hypervisor_trunk <site>:<id>
```

An imported profile with custom tagged VLANs is read as `tagged_network_ids`.

You can also use the [Terrifi CLI](../index.md#cli) to generate import blocks for all port profiles automatically:

```shell
terrifi generate-imports terrifi_port_profile
```

## Additional Notes

### Tagged networks

The UniFi UI shows the networks tagged on a port, but the controller stores the opposite: a list of *excluded* networks, containing every other network. `tagged_network_ids` matches what you select in the UI. Terrifi converts it to the excluded list when writing, and back when reading.

Only networks with the `corporate` purpose are considered when converting `tagged_network_ids`.

#### New networks

When a network is created, whether in the UI or through the API, the controller adds it to the excluded list of every profile with `tagged_vlan_mode = "custom"`, so a new network is never tagged on existing custom ports. It does this in the background, about a second after the network is created. Deleting a network removes it from the excluded lists.

This is exactly what `tagged_network_ids` says: the new network isn't in the list, so there's no planned change. (A plan run within a second or two of creating the network may still show it being removed from the port, because the controller hasn't excluded it yet.)

A network created in the same apply as a profile that tags it is still being processed when the profile is written, and the controller would exclude it from that profile too. To prevent this, when a profile tags a network created within the last minute, Terrifi waits a few seconds after writing the profile, checks it, and writes it again if the controller has excluded the network. Other applies aren't delayed.

### Unmanaged settings

Profile settings without an attribute are not managed: link speed and duplex, rate limiting, flow control, PTP, Energy Efficient Ethernet, link debounce, STP and loop protection, the MAC address filter, 802.1X, storm control, the voice VLAN, QoS and the multicast router port. A profile created by Terrifi gets the same values for these as one created in the UI. Updates leave them unchanged, and changes made to them in the UI don't show up as drift.

### Advanced settings

Settings in the UI's **Advanced** section, including port isolation and LLDP-MED, only apply when the profile's Advanced mode is **Manual**. Terrifi always sets Manual, including when it updates a profile that was created in the UI with Advanced left on Auto.
