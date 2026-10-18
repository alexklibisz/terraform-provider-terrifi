variable "internal_zone_id" {
  type = string
}

resource "terrifi_network" "extra" {
  name         = "hil-reset-fixture-network"
  purpose      = "corporate"
  vlan_id      = 3990
  subnet       = "192.168.99.1/24"
  dhcp_enabled = true
  dhcp_start   = "192.168.99.10"
  dhcp_stop    = "192.168.99.100"
}
resource "terrifi_client_group" "extra" {
  name = "hil-reset-fixture-clients"
}
resource "terrifi_client_device" "extra" {
  mac              = "02:00:00:39:90:01"
  name             = "hil-reset-fixture-client"
  blocked          = true
  fixed_ip         = "192.168.99.5"
  network_id       = terrifi_network.extra.id
  client_group_ids = [terrifi_client_group.extra.id]
}
resource "terrifi_dns_record" "extra" {
  name        = "hil-reset-fixture.invalid"
  value       = "192.0.2.50"
  record_type = "A"
}
resource "terrifi_firewall_group" "extra" {
  name    = "hil-reset-fixture-ports"
  type    = "port-group"
  members = ["8000"]
}
resource "terrifi_firewall_zone" "extra" {
  name        = "hil-reset-fixture-zone"
  network_ids = [terrifi_network.extra.id]
}
resource "terrifi_firewall_policy" "extra" {
  name   = "hil-reset-fixture-policy"
  action = "BLOCK"
  source {
    zone_id = terrifi_firewall_zone.extra.id
  }
  destination {
    zone_id            = var.internal_zone_id
    port_matching_type = "OBJECT"
    port_group_id      = terrifi_firewall_group.extra.id
  }
}
resource "terrifi_firewall_policy_order" "extra" {
  source_zone_id      = terrifi_firewall_zone.extra.id
  destination_zone_id = var.internal_zone_id
  policy_ids          = [terrifi_firewall_policy.extra.id]
}
resource "terrifi_wlan" "extra" {
  name       = "hil-reset-fixture-wifi"
  security   = "open"
  enabled    = false
  network_id = terrifi_network.extra.id
}

resource "terrifi_wlan" "secured" {
  name       = "hil-reset-fixture-secure-wifi"
  passphrase = "fixture-only-passphrase"
  enabled    = false
  network_id = terrifi_network.extra.id
}
