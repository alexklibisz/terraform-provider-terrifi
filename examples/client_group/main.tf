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

# Manage client devices alongside the group.
resource "terrifi_client_device" "thermostat" {
  mac  = "aa:bb:cc:dd:ee:01"
  name = "Thermostat"
}

resource "terrifi_client_device" "camera" {
  mac  = "aa:bb:cc:dd:ee:02"
  name = "Security Camera"
}

output "client_group_id" {
  value = terrifi_client_group.iot.id
}
