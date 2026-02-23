terraform {
  required_providers {
    terrifi = {
      source = "alexklibisz/terrifi"
    }
  }
}

provider "terrifi" {}

# Create a network to associate with the zone.
resource "terrifi_network" "iot" {
  name    = "IoT"
  purpose = "corporate"
  vlan_id = 33
  subnet  = "192.168.33.1/24"
}

# Create a firewall zone grouping the IoT network.
resource "terrifi_firewall_zone" "iot" {
  name        = "IoT Zone"
  network_ids = [terrifi_network.iot.id]
}

output "firewall_zone_id" {
  value = terrifi_firewall_zone.iot.id
}
