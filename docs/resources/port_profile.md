---
page_title: "terrifi_port_profile Resource - Terrifi"
subcategory: ""
description: |-
  Manages a switch port profile on the UniFi controller.
---

# terrifi_port_profile (Resource)

Manages reusable switch port configurations on the UniFi controller.
Assign a profile to a switch or gateway port with [`terrifi_device_ports`](device_ports.md) or in the UniFi UI.
Creating a profile does not configure any ports until it is assigned.

Which settings take effect depends on the device.
For example, a gateway's built-in switch ports may ignore settings that a managed switch honors.

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

### Access port with isolation and loop protection

An IoT port with isolation, STP, BPDU guard, non-STP loop protection, and a broadcast storm-control threshold of 20 percent.
PoE is disabled.

```terraform
resource "terrifi_port_profile" "protected_iot" {
  name                    = "Protected - IoT"
  native_network_id       = terrifi_network.iot.id
  tagged_vlan_mode        = "block_all"
  poe_mode                = "off"
  port_isolation_enabled  = true
  stp_enabled             = true
  bpdu_guard_enabled      = true
  loop_protection_enabled = true
  storm_control_broadcast = 20
}
```

## Schema

### Required

- `name` (String) — The name of the port profile, between 1 and 128 characters.

### Optional

- `site` (String) — The site to associate the port profile with. Defaults to the provider site. Changing this forces a new resource.

Removing an optional attribute with a documented default resets it to that default.
Removing `native_network_id` clears the native network, removing `link_debounce_ms` selects Auto, and removing a storm-control threshold disables its traffic class.
Terraform manages these settings even when omitted and corrects out-of-band changes on apply.

#### VLANs

- `native_network_id` (String) — The ID of the native (untagged) network. Unset means no native network (the UI's "None"). The UI defaults new profiles to the Default network instead.
- `tagged_vlan_mode` (String) — `allow_all` permits all tagged networks, `block_all` permits none, and `custom` permits the networks in `tagged_network_ids`. Default: `allow_all`.
- `tagged_network_ids` (Set of String) — IDs of the tagged networks. Required with `tagged_vlan_mode = "custom"` and not allowed otherwise. The set must be nonempty and must not contain the native network. Use `block_all` to permit no tagged networks. See [tagged networks](#tagged-networks).

#### Link settings

- `poe_mode` (String) — PoE mode: `auto` enables Auto PoE and `off` disables it. Default: `auto`.
- `eee_enabled` (Boolean) — Whether Energy Efficient Ethernet is enabled. Default: `false`.
- `flow_control_enabled` (Boolean) — Whether flow control is enabled. Default: `true`.
- `ptp_enabled` (Boolean) — Whether Precision Time Protocol is enabled. Default: `true`.
- `link_debounce_ms` (Number) — Link debounce in milliseconds. Unset selects Auto, `0` selects Off, and `100`–`5000` in steps of `100` selects Custom.

#### Port isolation and discovery

- `port_isolation_enabled` (Boolean) — Whether port isolation is enabled. Default: `false`.
- `lldpmed_enabled` (Boolean) — Whether LLDP-MED is enabled. Default: `true`.

#### Loop protection

- `stp_enabled` (Boolean) — Whether spanning tree is enabled. Default: `true`. Disabling STP removes its protection against switching loops.
- `bpdu_guard_enabled` (Boolean) — Whether BPDU guard is enabled. Default: `false`.
- `loop_protection_enabled` (Boolean) — Whether non-STP loop protection is enabled. Default: `false`.

#### Storm control

- `storm_control_type` (String) — Threshold unit: `percentage` or `packets_per_second`. Default: `percentage`.
- `storm_control_broadcast` (Number) — Broadcast threshold. Unset disables broadcast storm control.
- `storm_control_multicast` (Number) — Multicast threshold. Unset disables multicast storm control.
- `storm_control_unknown_unicast` (Number) — Unknown-unicast threshold. Unset disables unknown-unicast storm control.

Thresholds accept integers from `0` to `100` for `percentage` and from `0` to `14880000` for `packets_per_second`.
An explicit `0` enables the traffic class with a zero threshold.
Omitting all three thresholds disables storm control.

### Read-Only

- `id` (String) — The ID of the port profile.

## Validation coverage

Acceptance tests verify saving and reading profile configuration on both controller targets.
They do not verify physical-port enforcement.
The HIL switch (USW Flex Mini) does not support custom-profile assignment, and the Simulated target has no adopted devices or usable switch ports.
Profile-assignment tests are therefore unavailable on both targets.

✅ Covered · ❌ Not covered · ◻️ Unavailable on this target · ⚠️ Conditional.
See [validation coverage](../guides/validation-coverage.md) for definitions and limitations.

| Setting | Usage | Simulated | Hardware-in-the-loop |
| --- | --- | :---: | :---: |
| `terrifi_device_ports.ports.<port>.port_profile_id` | Assign this profile to a port | ◻️ | ◻️ |
| `name` | Rename a profile | ✅ | ✅ |
| `native_network_id` | Assign a native network | ✅ | ✅ |
| `native_network_id` | Change the native network | ✅ | ✅ |
| `native_network_id` | Remove the native network | ✅ | ✅ |
| `tagged_vlan_mode` | Set to `custom` | ✅ | ✅ |
| `tagged_vlan_mode` | Set to `block_all` | ✅ | ✅ |
| `tagged_vlan_mode` | Omit to reset to `allow_all` | ✅ | ✅ |
| `tagged_network_ids` | Select a tagged network | ✅ | ✅ |
| `tagged_network_ids` | Add a tagged network | ✅ | ✅ |
| `poe_mode` | Set to `off` | ✅ | ✅ |
| `poe_mode` | Omit to reset to `auto` | ✅ | ✅ |
| `eee_enabled` | Set to `true` | ✅ | ✅ |
| `eee_enabled` | Omit to reset to `false` | ✅ | ✅ |
| `flow_control_enabled` | Set to `false` | ✅ | ✅ |
| `flow_control_enabled` | Omit to reset to `true` | ✅ | ✅ |
| `ptp_enabled` | Set to `false` | ✅ | ✅ |
| `ptp_enabled` | Omit to reset to `true` | ✅ | ✅ |
| `link_debounce_ms` | Set to Off (`0`) or Custom (`1200`, `5000`) | ✅ | ✅ |
| `link_debounce_ms` | Omit to select Auto | ✅ | ✅ |
| `port_isolation_enabled` | Set to `true` | ✅ | ✅ |
| `port_isolation_enabled` | Omit to reset to `false` | ✅ | ✅ |
| `lldpmed_enabled` | Set to `false` | ✅ | ✅ |
| `lldpmed_enabled` | Omit to reset to `true` | ✅ | ✅ |
| `stp_enabled` | Set to `false` | ✅ | ✅ |
| `stp_enabled` | Omit to reset to `true` | ✅ | ✅ |
| `bpdu_guard_enabled` | Set to `true` | ✅ | ✅ |
| `bpdu_guard_enabled` | Omit to reset to `false` | ✅ | ✅ |
| `loop_protection_enabled` | Set to `true` | ✅ | ✅ |
| `loop_protection_enabled` | Omit to reset to `false` | ✅ | ✅ |
| `storm_control_type` | Change between percentage and packets per second | ✅ | ✅ |
| `storm_control_type` | Omit to reset to `percentage` | ✅ | ✅ |
| `storm_control_broadcast` | Set, change, and remove a threshold | ✅ | ✅ |
| `storm_control_multicast` | Set, change, and remove a threshold | ✅ | ✅ |
| `storm_control_unknown_unicast` | Set, change, and remove a threshold | ✅ | ✅ |

Both targets cover imports, defaults, values computed by other resources, and preservation of unmanaged fields.
Drift-repair tests cover STP, BPDU guard, non-STP loop protection, EEE, flow control, PTP, link debounce, and broadcast storm control.
Simulation tests use Network 10.6.106.

Evidence: [device_ports_resource_test.go](https://github.com/alexklibisz/terraform-provider-terrifi/blob/main/internal/provider/device_ports_resource_test.go), [port_profile_advanced_test.go](https://github.com/alexklibisz/terraform-provider-terrifi/blob/main/internal/provider/port_profile_advanced_test.go) and [port_profile_resource_test.go](https://github.com/alexklibisz/terraform-provider-terrifi/blob/main/internal/provider/port_profile_resource_test.go).

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

The UniFi UI shows the networks tagged on a port, but the controller stores a list of excluded networks.
`tagged_network_ids` is the allow-list selected in the UI.
Terrifi converts it to the controller's exclude list when writing and back when reading.
The native network is not included in either list.

Only networks with the `corporate` purpose are considered when converting `tagged_network_ids`.

#### Network creation and deletion

When a network is created, the controller adds it to the exclude list of every profile with `tagged_vlan_mode = "custom"`.
A network omitted from `tagged_network_ids` therefore remains excluded without a Terraform update.
Deleting a network removes it from the exclude lists.

The controller processes network creation in the background, typically within a few seconds.
A plan run before this processing finishes may show the network being removed from the profile's tagged networks.

When a profile tags a network created within the last minute, Terrifi waits a few seconds after saving the profile and reads it back.
If the controller has excluded that network, Terrifi reapplies the tag selection.
This applies to profile creation and updates.

### Unmanaged settings

Terrifi does not manage link speed and duplex, rate limiting, STP uplink mode, MAC address filtering, 802.1X, the voice VLAN, QoS, or the multicast router port.
Profiles created by Terrifi use the UI defaults for these settings.
Updates preserve their stored values, and changes made to them outside Terraform do not appear as drift.

### Advanced settings

Settings in the UI's **Advanced** section apply when the profile's Advanced mode is **Manual**.
Terrifi sets this mode on every profile creation and update, including updates to profiles imported with Advanced set to Auto.
