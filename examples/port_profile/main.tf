# Example: Create port profiles on the UniFi controller.
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
#      terraform plan    # see what would be created
#      terraform apply   # create it
#      terraform destroy # clean up
terraform {
  required_providers {
    terrifi = {
      source = "alexklibisz/terrifi"
    }
  }
}

# Provider configuration comes from environment variables.
provider "terrifi" {}

resource "terrifi_network" "management" {
  name         = "Management"
  purpose      = "corporate"
  vlan_id      = 100
  subnet       = "10.0.100.1/24"
  dhcp_enabled = false
}

resource "terrifi_network" "servers" {
  name         = "Servers"
  purpose      = "corporate"
  vlan_id      = 200
  subnet       = "10.0.200.1/24"
  dhcp_enabled = false
}

resource "terrifi_network" "guest" {
  name         = "Guest"
  purpose      = "corporate"
  vlan_id      = 300
  subnet       = "10.3.0.1/24"
  dhcp_enabled = false
}

# Trunk for a hypervisor: management untagged, servers tagged, nothing else.
resource "terrifi_port_profile" "hypervisor_trunk" {
  name              = "Trunk - Hypervisor"
  native_network_id = terrifi_network.management.id
  tagged_vlan_mgmt  = "custom"

  tagged_network_ids = [
    terrifi_network.servers.id,
  ]

  poe_mode = "off"
}

# Access port: guest network only, no tagged traffic.
resource "terrifi_port_profile" "guest_access" {
  name              = "Access - Guest"
  native_network_id = terrifi_network.guest.id
  tagged_vlan_mgmt  = "block_all"

  storm_control = {
    broadcast = 20
  }
}

output "hypervisor_trunk_id" {
  value = terrifi_port_profile.hypervisor_trunk.id
}

output "guest_access_id" {
  value = terrifi_port_profile.guest_access.id
}
