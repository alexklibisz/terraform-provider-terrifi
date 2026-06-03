# examples/complete — zone-based firewall slice.
#
# terrifi implements UniFi's zone-based firewall (terrifi_firewall_zone,
# terrifi_firewall_policy, terrifi_firewall_policy_order). These resources
# REQUIRE an adopted gateway: the controller only seeds the default zones
# (Internal, External, DMZ, Hotspot, IoT, VPN) once a UGW reaches the
# Connected state, and a user zone cannot be created until the "Hotspot"
# parent zone exists. Neither the docker simulation nor a fresh UOS Server in
# simulation mode seeds those zones (confirmed on Network app 10.3.58 and
# 10.4.57 — see testing/README.md "firewall zones require real adopted
# hardware"). So these are gated behind a toggle, OFF by default, and applied
# only against a real hardware target.
#
#   terraform apply                              # ZBF off (docker / UOS sim)
#   terraform apply -var=enable_zone_firewall=true   # ZBF on (real hardware)

variable "enable_zone_firewall" {
  type        = bool
  default     = false
  description = "Create the zone-based firewall resources. Requires an adopted gateway with the default zones seeded; leave false on docker/UOS simulation."
}

resource "terrifi_firewall_zone" "trusted" {
  count = var.enable_zone_firewall ? 1 : 0
  name  = "tfacc-zone-trusted"
  network_ids = [
    terrifi_network.trusted.id,
  ]
}

resource "terrifi_firewall_zone" "iot" {
  count = var.enable_zone_firewall ? 1 : 0
  name  = "tfacc-zone-iot"
  network_ids = [
    terrifi_network.iot.id,
  ]
}

# Allow IoT -> Trusted only on the Home-Assistant / MQTT / HTTPS ports.
resource "terrifi_firewall_policy" "iot_to_trusted_mgmt" {
  count    = var.enable_zone_firewall ? 1 : 0
  name     = "tfacc-fw-iot-to-trusted-mgmt"
  action   = "ALLOW"
  protocol = "tcp"

  source {
    zone_id = terrifi_firewall_zone.iot[0].id
  }

  destination {
    zone_id        = terrifi_firewall_zone.trusted[0].id
    port_group_id  = terrifi_firewall_group.iot_mgmt_allowed.id
  }
}

# Block everything else IoT -> Trusted.
resource "terrifi_firewall_policy" "iot_to_trusted_block" {
  count  = var.enable_zone_firewall ? 1 : 0
  name   = "tfacc-fw-iot-to-trusted-block"
  action = "BLOCK"

  source {
    zone_id = terrifi_firewall_zone.iot[0].id
  }

  destination {
    zone_id = terrifi_firewall_zone.trusted[0].id
  }
}

# Allow the management ports first, then the catch-all block.
resource "terrifi_firewall_policy_order" "iot_to_trusted" {
  count               = var.enable_zone_firewall ? 1 : 0
  source_zone_id      = terrifi_firewall_zone.iot[0].id
  destination_zone_id = terrifi_firewall_zone.trusted[0].id

  policy_ids = [
    terrifi_firewall_policy.iot_to_trusted_mgmt[0].id,
    terrifi_firewall_policy.iot_to_trusted_block[0].id,
  ]
}
