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

# Block all traffic from IoT to Trusted.
resource "terrifi_firewall_policy" "block_iot_to_trusted" {
  name   = "Block IoT to Trusted"
  action = "BLOCK"

  source {
    zone_id = terrifi_firewall_zone.iot.id
  }

  destination {
    zone_id = terrifi_firewall_zone.trusted.id
  }
}

output "firewall_policy_id" {
  value = terrifi_firewall_policy.block_iot_to_trusted.id
}
