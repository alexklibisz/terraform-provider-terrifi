# Example: Assign a port profile and names to ports on an adopted switch.
#
# This reconfigures live ports. Pick ports that are safe to change.
#
# Prerequisites:
#
# 1. Build and install the provider:
#      task build
#
# 2. Configure Terraform to use your local build instead of downloading
#    from the registry. Add this to ~/.terraformrc (create if it doesn't exist):
#
#      provider_installation {
#        dev_overrides {
#          "alexklibisz/terrifi" = "/Users/alex/go/bin"  # or wherever `go env GOBIN` points
#        }
#        direct {}
#      }
#
# 3. Set your controller credentials (via .envrc or manually):
#      export UNIFI_API="https://192.168.1.12:8443"
#      export UNIFI_USERNAME=root
#      export UNIFI_PASSWORD='your-password'
#      export UNIFI_INSECURE=true
#
# 4. Run:
#      terraform plan  -var switch_mac=aa:bb:cc:dd:ee:ff
#      terraform apply -var switch_mac=aa:bb:cc:dd:ee:ff
#      terraform destroy -var switch_mac=aa:bb:cc:dd:ee:ff # unassign and clean up
terraform {
  required_providers {
    terrifi = {
      source = "alexklibisz/terrifi"
    }
  }
}

# Provider configuration comes from environment variables.
provider "terrifi" {}

variable "switch_mac" {
  type        = string
  description = "MAC address of an adopted switch."
}

resource "terrifi_network" "servers" {
  name         = "Servers"
  purpose      = "corporate"
  vlan_id      = 200
  subnet       = "10.0.200.1/24"
  dhcp_enabled = false
}

# Access port on the servers network.
resource "terrifi_port_profile" "servers_access" {
  name              = "Access - Servers"
  native_network_id = terrifi_network.servers.id
  tagged_vlan_mgmt  = "block_all"
}

# Only ports 7 and 8 are managed; every other port is left as it is.
resource "terrifi_device_ports" "switch" {
  device_mac = var.switch_mac

  ports = {
    "7" = {
      name            = "Server 1"
      port_profile_id = terrifi_port_profile.servers_access.id
    }
    "8" = {
      name = "Spare"
    }
  }
}
