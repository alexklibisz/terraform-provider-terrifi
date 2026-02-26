terraform {
  required_providers {
    terrifi = {
      source = "alexklibisz/terrifi"
    }
  }
}

provider "terrifi" {}

# Create a client group for WiFi smart plugs.
resource "terrifi_client_group" "smart_plugs" {
  name = "WiFi Smart Plugs"
}

# Assign client devices to the group.
resource "terrifi_client_device" "plug_living_room" {
  mac             = "aa:bb:cc:dd:ee:01"
  name            = "Living Room Plug"
  client_group_id = terrifi_client_group.smart_plugs.id
}

resource "terrifi_client_device" "plug_bedroom" {
  mac             = "aa:bb:cc:dd:ee:02"
  name            = "Bedroom Plug"
  client_group_id = terrifi_client_group.smart_plugs.id
}

output "client_group_id" {
  value = terrifi_client_group.smart_plugs.id
}
