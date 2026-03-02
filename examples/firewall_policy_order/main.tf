terraform {
  required_providers {
    terrifi = {
      source = "alexklibisz/terrifi"
    }
  }
}

provider "terrifi" {}

# Create two firewall zones.
resource "terrifi_firewall_zone" "iot" {
  name = "IoT Zone"
}

resource "terrifi_firewall_zone" "trusted" {
  name = "Trusted Zone"
}

# Create policies for the IoT -> Trusted direction.
resource "terrifi_firewall_policy" "allow_dns" {
  name     = "Allow DNS"
  action   = "ALLOW"
  protocol = "udp"

  source {
    zone_id = terrifi_firewall_zone.iot.id
  }

  destination {
    zone_id            = terrifi_firewall_zone.trusted.id
    port_matching_type = "SPECIFIC"
    port               = 53
  }
}

resource "terrifi_firewall_policy" "block_all" {
  name   = "Block All"
  action = "BLOCK"

  source {
    zone_id = terrifi_firewall_zone.iot.id
  }

  destination {
    zone_id = terrifi_firewall_zone.trusted.id
  }
}

# Control the evaluation order: allow DNS first, then block everything else.
resource "terrifi_firewall_policy_order" "iot_to_trusted" {
  source_zone_id      = terrifi_firewall_zone.iot.id
  destination_zone_id = terrifi_firewall_zone.trusted.id

  policy_ids = [
    terrifi_firewall_policy.allow_dns.id,  # evaluated first
    terrifi_firewall_policy.block_all.id,  # evaluated second
  ]
}
