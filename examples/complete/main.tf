# examples/complete — the EXAMPLE_NETWORK reference, expressed in terrifi.
#
# A role-based ("cattle, not pets") UniFi Network configuration that exercises
# the breadth of terrifi's implemented resources. Every name carries the
# `tfacc-` prefix so the test sweep can find and delete leftovers. See
# tmp/EXAMPLE_NETWORK.md for the full reference and tmp/TERRIFI_IMPROVEMENT_PLAN.md
# §8 (L06/L07) for how the end-to-end test consumes this.
#
# Scope: this file holds the resources that apply cleanly against the docker
# simulation and a fresh UOS Server (no adopted gateway required) —
# networks, WLANs, firewall groups, DNS records, client groups, and client
# reservations. The zone-based firewall resources live in firewall-zbf.tf,
# gated behind a toggle because they need an adopted gateway (see that file).
# Resources terrifi does not implement yet are catalogued in stubs.tf.

# NOTE: this directory is consumed by the TestAccCompleteNetwork_basic
# acceptance test (improvement-plan §8 L07), which applies it via
# ConfigDirectory and injects the provider in-process. terraform-plugin-testing
# requires that a test-applied config NOT declare providers itself, so there is
# deliberately no `terraform { required_providers {} }` or `provider "terrifi"`
# block here. Provider config comes from UNIFI_* env vars.
#
# To run this example by hand against a registry build, add a versions.tf:
#
#   terraform {
#     required_providers {
#       terrifi = { source = "alexklibisz/terrifi" }
#     }
#   }
#
# (Under the documented dev_overrides workflow you don't even need that — see
# the repo README.) Then `terraform apply`.

variable "wifi_passphrase" {
  type        = string
  sensitive   = true
  default     = "tfacc-wifi-test-passphrase"
  description = "WPA2 passphrase shared by the test WLANs. A fixed sentinel — never a real one."
}

# ---------------------------------------------------------------------------
# Networks (VLANs)
#
# Reference has 5 user networks + 2 WANs. terrifi has no WAN resource, so the
# two WANs are omitted (tracked in stubs.tf). terrifi_network.purpose only
# supports "corporate" and "vlan-only" — the reference's purpose="guest"
# network is modeled as corporate here, with client isolation enforced at the
# WLAN layer instead. That purpose=guest gap is noted in stubs.tf.
# ---------------------------------------------------------------------------

resource "terrifi_network" "mgmt" {
  name       = "tfacc-net-mgmt"
  purpose    = "corporate"
  vlan_id    = 10
  subnet     = "192.168.10.1/24"
  dhcp_start = "192.168.10.30"
  dhcp_stop  = "192.168.10.240"
}

resource "terrifi_network" "trusted" {
  name       = "tfacc-net-trusted"
  purpose    = "corporate"
  vlan_id    = 20
  subnet     = "192.168.20.1/24"
  dhcp_start = "192.168.20.30"
  dhcp_stop  = "192.168.20.240"
}

resource "terrifi_network" "iot" {
  name       = "tfacc-net-iot"
  purpose    = "corporate"
  vlan_id    = 30
  subnet     = "192.168.30.1/24"
  dhcp_start = "192.168.30.30"
  dhcp_stop  = "192.168.30.240"
}

# Reference purpose=guest; terrifi has no guest purpose, so corporate +
# WLAN-level isolation. See stubs.tf "purpose=guest".
resource "terrifi_network" "guest" {
  name       = "tfacc-net-guest"
  purpose    = "corporate"
  vlan_id    = 40
  subnet     = "192.168.40.1/24"
  dhcp_start = "192.168.40.30"
  dhcp_stop  = "192.168.40.240"
}

# VLAN-only network (no subnet, no DHCP) — exercises the vlan-only branch.
resource "terrifi_network" "inner" {
  name    = "tfacc-net-inner"
  purpose = "vlan-only"
  vlan_id = 99
}

# ---------------------------------------------------------------------------
# WLANs — all three broadcast the WPA2-PSK test passphrase. Guest + IoT get
# client isolation via application=hotspot/iot (terrifi's isolation knob).
# ---------------------------------------------------------------------------

resource "terrifi_wlan" "trusted" {
  name       = "tfacc-ssid-trusted"
  passphrase = var.wifi_passphrase
  network_id = terrifi_network.trusted.id
}

resource "terrifi_wlan" "iot" {
  name        = "tfacc-ssid-iot"
  passphrase  = var.wifi_passphrase
  network_id  = terrifi_network.iot.id
  application = "iot"
}

resource "terrifi_wlan" "guest" {
  name        = "tfacc-ssid-guest"
  passphrase  = var.wifi_passphrase
  network_id  = terrifi_network.guest.id
  application = "hotspot"
}

# ---------------------------------------------------------------------------
# Firewall groups
# ---------------------------------------------------------------------------

resource "terrifi_firewall_group" "rfc1918" {
  name    = "tfacc-fg-rfc1918"
  type    = "address-group"
  members = ["10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"]
}

resource "terrifi_firewall_group" "iot_mgmt_allowed" {
  name    = "tfacc-fg-iot-mgmt-allowed"
  type    = "port-group"
  members = ["8123", "1883", "443"]
}

# ---------------------------------------------------------------------------
# DNS records (controller-side overrides)
# ---------------------------------------------------------------------------

resource "terrifi_dns_record" "nas" {
  name        = "nas.example.lan"
  record_type = "A"
  value       = "192.168.10.20"
}

resource "terrifi_dns_record" "auth" {
  name        = "auth.example.lan"
  record_type = "A"
  value       = "192.168.20.10"
}

resource "terrifi_dns_record" "cname" {
  name        = "ha.example.lan"
  record_type = "CNAME"
  value       = "nas.example.lan"
}

# ---------------------------------------------------------------------------
# Client group + reservations (DHCP fixed IPs)
#
# MACs use the locally-administered, unicast prefix 02:00:00:ca:ff: so they
# never collide with anything real on the test bench.
# ---------------------------------------------------------------------------

resource "terrifi_client_group" "iot_devices" {
  name = "tfacc-cg-iot"
}

resource "terrifi_client_device" "nas" {
  mac        = "02:00:00:ca:ff:01"
  name       = "tfacc-client-nas"
  fixed_ip   = "192.168.10.20"
  network_id = terrifi_network.mgmt.id
}

resource "terrifi_client_device" "auth" {
  mac        = "02:00:00:ca:ff:02"
  name       = "tfacc-client-auth"
  fixed_ip   = "192.168.20.10"
  network_id = terrifi_network.trusted.id
}

resource "terrifi_client_device" "iot_hub" {
  mac              = "02:00:00:ca:ff:03"
  name             = "tfacc-client-iot-hub"
  fixed_ip         = "192.168.30.20"
  network_id       = terrifi_network.iot.id
  client_group_ids = [terrifi_client_group.iot_devices.id]
}

# ---------------------------------------------------------------------------
# Device data source — devices are adopted out-of-band (zero-device CI by
# default), so this is a data source, not a managed resource. Looked up by
# the role-based hostname the reference expects. Commented out by default
# because it errors when no device with that name exists (the common CI case).
# ---------------------------------------------------------------------------

# data "terrifi_device" "gateway" {
#   name = "tfacc-gw01"
# }
