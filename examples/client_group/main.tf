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

output "client_group_id" {
  value = terrifi_client_group.iot.id
}
