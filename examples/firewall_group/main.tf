# Example: Create firewall groups on the UniFi controller.
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

resource "terrifi_firewall_group" "web_ports" {
  name    = "Web Ports"
  type    = "port-group"
  members = ["80", "443", "8080"]
}

resource "terrifi_firewall_group" "trusted_ips" {
  name    = "Trusted IPs"
  type    = "address-group"
  members = ["10.0.0.0/24", "192.168.1.0/24"]
}

output "web_ports_id" {
  value = terrifi_firewall_group.web_ports.id
}

output "trusted_ips_id" {
  value = terrifi_firewall_group.trusted_ips.id
}
