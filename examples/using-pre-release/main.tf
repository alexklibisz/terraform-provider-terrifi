# Example: Using a pre-released version of the terrifi provider.
#
# Prerequisites:
#
# 1. Run the install script to download the provider binary:
#      ./install.sh             # latest release
#      ./install.sh v0.1.0-RC1  # specific version
#
# 2. Add a filesystem mirror to ~/.terraformrc:
#
#      provider_installation {
#        filesystem_mirror {
#          path = "/home/<you>/.terraform.d/plugins"
#        }
#        direct {}
#      }
#
# 3. Set your controller credentials:
#      export UNIFI_API="https://192.168.1.12:8443"
#      export UNIFI_API_KEY="your-api-key"
#      export UNIFI_INSECURE=true
#
# 4. Run:
#      terraform init
#      terraform plan
#      terraform apply

terraform {
  required_providers {
    terrifi = {
      source = "github.com/alexklibisz/terrifi"
    }
  }
}

provider "terrifi" {}

resource "terrifi_dns_record" "example" {
  name        = "example.home"
  value       = "192.168.1.100"
  record_type = "A"
  enabled     = true
}
