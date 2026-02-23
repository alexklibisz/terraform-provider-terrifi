terraform {
  required_providers {
    terrifi = {
      source = "alexklibisz/terrifi"
    }
  }
}

provider "terrifi" {}

# Create a client group for IoT devices.
resource "terrifi_client_group" "iot" {
  name = "IoT Devices"
}

# Assign client devices to the group.
resource "terrifi_client_device" "thermostat" {
  mac             = "aa:bb:cc:dd:ee:01"
  name            = "Thermostat"
  client_group_id = terrifi_client_group.iot.id
}

resource "terrifi_client_device" "camera" {
  mac             = "aa:bb:cc:dd:ee:02"
  name            = "Security Camera"
  client_group_id = terrifi_client_group.iot.id
}

output "client_group_id" {
  value = terrifi_client_group.iot.id
}
