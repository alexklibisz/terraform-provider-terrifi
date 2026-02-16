# Example: Create a DNS A record on the UniFi controller.
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
# You could also set attributes directly here:
#   provider "terrifi" {
#     api_url        = "https://192.168.1.12:8443"
#     username       = "admin"
#     password       = "secret"
#     allow_insecure = true
#   }
provider "terrifi" {}

resource "terrifi_dns_record" "test" {
  name        = "test.home"
  value       = "192.168.1.100"
  record_type = "A"
  enabled     = true
}

output "dns_record_id" {
  value = terrifi_dns_record.test.id
}
