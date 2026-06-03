# Controller findings catalog (F-001 … F-014)

The mock-controller investigation behind the `uos` test target catalogued 14
controller / provider findings (F-001 … F-014). That investigation was run
against the upstream `terraform-provider-unifi` provider, so each finding has
to be re-mapped to terrifi, which has a different (and smaller) resource set
and owns its own HTTP layer.

**Bottom line: none of the 14 findings map to a currently-failing terrifi
test.** Each one is either (a) already handled by terrifi and proven by a
passing acceptance test, (b) not applicable because terrifi doesn't model the
field, or (c) dormant because terrifi hasn't ported the affected resource
yet. So there are no `t.Skip("F-NNN…")` markers to add — adding skips to green
tests would be wrong. This file is the catalog the improvement-plan §8 L08
calls for; revisit it when a dormant resource lands.

Legend: **handled** = terrifi does the right thing, covered by a passing test ·
**n/a** = terrifi doesn't expose the affected field · **dormant** = resource
not ported; re-evaluate when it lands.

| Finding | Upstream subject | terrifi status | Notes |
|---|---|---|---|
| F-001 | `unifi_network` doesn't expose `purpose` | **handled (partial)** | terrifi_network exposes `purpose` = `corporate` \| `vlan-only`. `purpose = "guest"` is **not** supported — dormant gap, recorded in `examples/complete/stubs.tf`. Covered by `TestAccNetwork_*` (6/6). |
| F-002 | `subnet` required even for vlan-only | **handled** | terrifi_network applies a vlan-only network with no subnet. Covered by `TestAccNetwork_vlanOnly`. |
| F-003 | `data.ap_group` requires `name` | dormant | No terrifi AP-group data source. Re-evaluate if one is added. |
| F-004 | `data.client_qos_rate` requires `name` | dormant | No terrifi client-QoS-rate resource/data source. |
| F-005 | default AP group is `"All APs"` not `"Default"` | dormant | Same as F-003 — no AP-group lookup in terrifi. |
| F-006 | `network.domain_name` round-trips empty | **n/a** | local `unifi.Network` doesn't declare `domain_name`, so it's never sent or diffed. |
| F-007 | `unifi_firewall_rule` bool `null` vs `false` | dormant | terrifi has no `firewall_rule` (it uses zone-based firewall). The analogous bool-omitempty concern in `firewall_policy` is already handled (see `firewall_policy_api.go`). |
| F-008 | `port_forward.wan.interface` rejects LAN IDs | dormant | No terrifi port-forward resource. |
| F-009 | `traffic_route` fails on zero-device controller | dormant | No terrifi traffic-route resource. |
| F-010 | `port_profile` round-trips 3 fields wrong (2 parts) | dormant | No terrifi port-profile resource. |
| F-011 | `unifi_wlan` create → `InvalidPayload` from `schedule_with_duration: null` | **handled** | terrifi sends `schedule_with_duration` as `[]`, never `null` (`normalizeWLAN` in `wlan_api.go`). Covered by `TestAccWLAN_*` (23/23 on UOS 5.1.15). |
| F-012 | `dynamic_dns` → `UnsupportedDynamicDns` | dormant | No terrifi dynamic-DNS resource. |
| F-013 | `firewall_rule` WAN_IN index 20000+ rejected with devices | dormant | No terrifi `firewall_rule`. |
| F-014 | `traffic_route` POST 400 (post-F-009) | dormant | No terrifi traffic-route resource. |

## Separate from the F-findings: the zone-based-firewall simulation gap

terrifi's `firewall_zone` / `firewall_policy` / `firewall_policy_order` tests
are gated `requireHardware(t)` because the controller only seeds the default
zones (incl. the `Hotspot` parent every user zone needs) once a real gateway
is adopted and Connected — neither docker nor UOS simulation does this. That
is **not** one of F-001 … F-014; it is documented in
`testing/README.md` → "firewall zones require real adopted hardware" and in
`examples/complete/firewall-zbf.tf`.

## When a dormant resource lands

When terrifi gains one of the unported resources (port profile, port forward,
static/traffic route, RADIUS profile/user, dynamic DNS, AP-group data
source), re-read the matching finding above and the full write-up in the
investigation doc, add the resource's acceptance tests, and — only if a test
genuinely fails for the documented reason — add a
`t.Skip("F-NNN: <one-line>")` until the provider catches up. Update this
catalog's status column at the same time.
