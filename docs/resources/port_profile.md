---
page_title: "terrifi_port_profile Resource - Terrifi"
subcategory: ""
description: |-
  Manages a switch port profile on the UniFi controller.
---

# terrifi_port_profile (Resource)

Manages a switch port profile on the UniFi controller. Port profiles are reusable port configurations (VLANs, PoE, link speed and the UI's advanced settings) that can be assigned to switch and gateway ports.

A profile only defines settings. Creating or changing one doesn't affect any port until the profile is assigned to that port, with [`terrifi_device_ports`](device_ports.md) or in the UI.

Which settings take effect depends on the device a profile is assigned to. For example, a gateway's built-in switch ports may ignore settings that a managed switch honors.

## Tagged networks: `tagged_network_ids` vs `excluded_network_ids`

The UniFi UI shows the networks tagged on a port, but the controller stores the opposite: a list of *excluded* networks, containing every other network. The two attributes are the two ways of expressing that:

- **`tagged_network_ids`** (recommended) matches what you select in the UI. Terrifi converts it to the excluded list when writing, and back when reading.
- **`excluded_network_ids`** is the stored list, as is.

Only networks with the `corporate` purpose are considered when converting `tagged_network_ids`.

### New networks

When a network is created, whether in the UI or through the API, the controller adds it to the excluded list of every profile with `tagged_vlan_mgmt = "custom"`, so a new network is never tagged on existing custom ports. It does this in the background, about a second after the network is created. Deleting a network removes it from the excluded lists.

- With `tagged_network_ids`, this is exactly what the config says: the new network isn't in the list, so there's no planned change. (A plan run within a second or two of creating the network may still show it being removed from the port, because the controller hasn't excluded it yet.)
- With `excluded_network_ids`, the new network appears in the stored list but not in your config, so the next plan removes it, and applying that tags the new network on the port. Add new networks to the list to keep them off the port.

A network created in the same apply as a profile that tags it is still being processed when the profile is written, and the controller would exclude it from that profile too. To prevent this, when a profile tags a network created within the last minute, Terrifi waits a few seconds after writing the profile, checks it, and writes it again if the controller has excluded the network. Other applies aren't delayed.

## Advanced settings

Settings in the UI's **Advanced** section (everything from `egress_rate_limit_kbps` down in the schema) only apply when the profile's Advanced mode is **Manual**. Terrifi always sets Manual, including when it updates a profile that was created in the UI with Advanced left on Auto.

## Example Usage

### Trunk port for a hypervisor

An untagged management network, plus two tagged networks.

```terraform
resource "terrifi_port_profile" "hypervisor_trunk" {
  name              = "Trunk - Hypervisor"
  native_network_id = terrifi_network.management.id
  tagged_vlan_mgmt  = "custom"

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
  tagged_vlan_mgmt  = "block_all"
}
```

### Phone port with a voice VLAN

```terraform
resource "terrifi_port_profile" "desk_phone" {
  name              = "Desk + Phone"
  native_network_id = terrifi_network.office.id
  voice_network_id  = terrifi_network.voice.id
}
```

### Fixed link speed, rate limiting and storm control

```terraform
resource "terrifi_port_profile" "legacy_device" {
  name        = "Legacy 100M"
  autoneg     = false
  speed       = 100
  full_duplex = true

  egress_rate_limit_kbps = 10000
  link_debounce_ms       = 1000

  storm_control = {
    type      = "level"
    broadcast = 20
    multicast = 30
  }
}
```

### MAC address filter

```terraform
resource "terrifi_port_profile" "locked_down" {
  name                        = "Locked Down"
  port_security_enabled       = true
  port_security_mac_addresses = ["00:00:5e:00:53:01"]
}
```

## Schema

### Required

- `name` (String) — The name of the port profile.

### Optional

- `site` (String) — The site to associate the port profile with. Defaults to the provider site. Changing this forces a new resource.

VLANs:

- `native_network_id` (String) — The ID of the native (untagged) network. When unset, the port has no native network (the UI's "None"). Note that the UI defaults new profiles to the Default network instead.
- `tagged_vlan_mgmt` (String) — Tagged VLAN management. `auto` (UI "Allow All") tags every network, `block_all` tags none, and `custom` tags the networks given by `tagged_network_ids` or `excluded_network_ids`. Default: `auto`.
- `tagged_network_ids` (Set of String) — IDs of the networks tagged on the port, as selected in the UI. Only with `tagged_vlan_mgmt = "custom"`; conflicts with `excluded_network_ids`. Must not include the native network, and must not be empty (use `tagged_vlan_mgmt = "block_all"` to tag no networks; the controller rewrites an empty custom selection to `block_all`). See [above](#tagged-networks-tagged_network_ids-vs-excluded_network_ids).
- `excluded_network_ids` (Set of String) — IDs of the networks **not** tagged on the port, exactly as the controller stores them. Only with `tagged_vlan_mgmt = "custom"`; conflicts with `tagged_network_ids`. Must not include the native network. Excluding every network is rewritten by the controller to `block_all`, which then shows as a planned change; use `block_all` instead.
- `voice_network_id` (String) — The ID of the voice VLAN network, advertised over LLDP-MED. Requires `lldpmed_enabled`, and must be tagged on the port: any network with `auto`, one of `tagged_network_ids` (or not in `excluded_network_ids`) with `custom`, and not allowed with `block_all`.

Link:

- `poe_mode` (String) — PoE mode. `auto` is the UI's "Auto PoE" checkbox ticked, `off` unticked. Default: `auto`.
- `autoneg` (Boolean) — Whether link speed and duplex are auto-negotiated. Set to `false` to use `speed` and `full_duplex`. Default: `true`.
- `speed` (Number) — Fixed link speed in Mbps. Required when `autoneg` is `false`, not allowed otherwise. One of: `10`, `100`, `1000`, `2500`, `5000`, `10000`, `20000`, `25000`, `40000`, `50000`, `100000`.
- `full_duplex` (Boolean) — Whether a fixed-speed link runs full duplex. Can only be `true` when `autoneg` is `false`. Default: `false`.

Advanced:

`flow_control_enabled`, `ptp_enabled`, `eee_enabled` and `link_debounce_ms` need a recent Network version. Network Application 10.3.58 doesn't store them, which shows up as "Provider produced inconsistent result after apply".

- `egress_rate_limit_kbps` (Number) — Egress rate limit in kbps (64–9999999). When unset, egress rate limiting is disabled.
- `flow_control_enabled` (Boolean) — Whether flow control is enabled. Default: `true`.
- `ptp_enabled` (Boolean) — Whether Precision Time Protocol is enabled. Default: `true`.
- `isolation` (Boolean) — Whether port isolation is enabled. Default: `false`.
- `stp_enabled` (Boolean) — Whether spanning tree (STP) is enabled. Default: `true`.
- `stp_uplink` (Boolean) — Whether the port is an STP uplink. Default: `false`.
- `bpdu_guard_enabled` (Boolean) — Whether BPDU guard is enabled. Default: `false`.
- `loop_protection_enabled` (Boolean) — Whether non-STP loop protection is enabled (`port_keepalive_enabled` in the API). Default: `false`.
- `eee_enabled` (Boolean) — Whether Energy Efficient Ethernet is enabled. Default: `false`.
- `lldpmed_enabled` (Boolean) — Whether LLDP-MED is enabled. Default: `true`.
- `link_debounce_ms` (Number) — Link debounce. Unset is the UI's "Auto", `0` is "Off", and 100–5000 (in steps of 100) is "Custom" in milliseconds.
- `port_security_enabled` (Boolean) — Whether the MAC address filter (port security) is enabled. Default: `false`.
- `port_security_mac_addresses` (Set of String) — MAC addresses allowed by the MAC address filter. Default: `[]`.
- `dot1x_ctrl` (String) — 802.1X control. One of: `auto`, `force_authorized`, `force_unauthorized`, `mac_based`, `multi_host` (the UI's "Multi-auth"). Default: `force_authorized`.
- `storm_control` (Attributes) — Storm control. When unset, storm control is disabled. See [below for nested schema](#nestedatt--storm_control).

### Read-Only

- `id` (String) — The ID of the port profile.

<a id="nestedatt--storm_control"></a>
### Nested Schema for `storm_control`

At least one of `broadcast`, `multicast` or `unicast` must be set.

- `type` (String) — How thresholds are expressed: `level` (the UI's "percentage" of link bandwidth, 0–100) or `rate` (packets per second, 0–14880000). Default: `level`.
- `broadcast` (Number) — Broadcast threshold. When unset, broadcast storm control is disabled.
- `multicast` (Number) — Multicast threshold. When unset, multicast storm control is disabled.
- `unicast` (Number) — Unknown-unicast threshold. When unset, unicast storm control is disabled.

Not yet supported: QoS mode and policies, and the multicast router port setting. Terrifi leaves these unchanged when it updates a profile.

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
