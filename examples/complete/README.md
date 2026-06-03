# examples/complete

The EXAMPLE_NETWORK reference network (a role-based, dual-WAN/multi-VLAN
homelab shape, stripped of identifying detail) expressed with terrifi's
implemented resources. It is the fixture the end-to-end acceptance test
(`TestAccCompleteNetwork_basic`) applies, re-plans for idempotency, and
destroys.

## Files

| File | What |
|---|---|
| `main.tf` | The appliable core — networks, WLANs, firewall groups, DNS records, client group, client reservations. Applies cleanly on the `docker` and `uos` targets (no adopted gateway needed). |
| `firewall-zbf.tf` | Zone-based firewall (zones, policies, ordering), gated behind `enable_zone_firewall` (default `false`). Needs an adopted gateway — see note below. |
| `stubs.tf` | Commented-out blocks for resources terrifi does not implement yet (WAN, port profiles, port forwards, routes, RADIUS, dynamic DNS), each tagged to its reference section. |

## Apply

```sh
# Appliable core only (docker / UOS simulation):
terraform apply

# Add the zone-based firewall (real adopted hardware only):
terraform apply -var=enable_zone_firewall=true
```

Provider credentials come from the environment (`UNIFI_API`,
`UNIFI_API_KEY` or `UNIFI_USERNAME`/`UNIFI_PASSWORD`, `UNIFI_INSECURE`).

This directory is also the fixture for `TestAccCompleteNetwork_basic`, which
applies it in-process. terraform-plugin-testing requires a test-applied
config to declare no providers of its own, so — unlike the other examples —
there is no `terraform { required_providers {} }` block here. Under the
repo's `dev_overrides` workflow you don't need one. To run against a registry
build, drop a `versions.tf` in this directory:

```hcl
terraform {
  required_providers {
    terrifi = { source = "alexklibisz/terrifi" }
  }
}
```

## Why the zone-based firewall is gated off

`terrifi_firewall_zone` / `_policy` / `_policy_order` require the controller
to have seeded its default zones (Internal, External, DMZ, Hotspot, IoT,
VPN), which only happens once a real gateway is adopted and Connected.
Neither the docker simulation nor a fresh UOS Server in simulation mode
seeds them, so a zone create returns `api.err.CouldNotFindHotspotFirewallZone`.
Confirmed on Network app 10.3.58 and 10.4.57 — see
`testing/README.md` → "firewall zones require real adopted hardware". The
toggle keeps the core example applying everywhere while still exercising the
ZBF resources on a hardware target.

## A note on controller load

`TestAccCompleteNetwork_basic` applies this whole directory at once, so
Terraform creates ~14 resources with its default parallelism (10). Against a
fresh docker controller this completes in a few seconds. Against a modestly
resourced controller that is **also running adopted (or simulated) devices**,
the burst of concurrent `networkconf` writes can each exceed the provider's
30 s HTTP timeout, because the controller recomputes device configuration on
every network change. If you hit `context deadline exceeded` running this on
such a target, run it against an idle controller (no adopted/simulated
devices), or apply with reduced parallelism (`terraform apply
-parallelism=2`). The per-resource acceptance tests already prove each
resource individually; this fixture proves they compose.

## Coverage vs. the reference

Implemented and exercised here: `terrifi_network` (×5), `terrifi_wlan` (×3),
`terrifi_firewall_group` (×2), `terrifi_dns_record` (×3),
`terrifi_client_group` (×1), `terrifi_client_device` (×3), and — behind the
toggle — `terrifi_firewall_zone` / `_policy` / `_policy_order`.

Not yet implemented (see `stubs.tf`): WAN, `purpose=guest` networks, port
profiles, port forwards / DNAT, static & traffic routes, RADIUS profile/user,
dynamic DNS. The reference (`tmp/EXAMPLE_NETWORK.md`) is the contract; these
stubs are the running coverage gap.
