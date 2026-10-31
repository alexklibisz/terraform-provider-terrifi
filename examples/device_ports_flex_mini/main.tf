terraform {
  required_providers {
    terrifi = {
      source = "alexklibisz/terrifi"
    }
  }
}

# Configure the controller that manages the switch using UNIFI_* variables.
provider "terrifi" {}

variable "switch_mac" {
  type        = string
  description = "MAC of the adopted USW Flex Mini."
}

variable "hil_uplink_network_id" {
  type        = string
  description = "ID of the HIL uplink VLAN on the switch's controller and site."
}

resource "terrifi_device_ports" "switch" {
  device_mac = var.switch_mac

  # Choose a port that is safe to reconfigure. Unlisted ports are untouched.
  ports = {
    "3" = {
      name              = "HIL gateway WAN"
      native_network_id = var.hil_uplink_network_id
      tagged_vlan_mode  = "block_all"
    }
  }
}

# Removing port 3 or destroying this resource restores that port's defaults.
