# Example: Using a pre-released version of the terrifi provider.
#
# Prerequisites:
#
# 1. Run the install script to download the provider binary:
#      ./install.sh v0.11.0-RC0
#    Requires gh (authenticated for GitHub downloads) and unzip.
#    Run from this directory, or invoke the script by its absolute path from
#    your own configuration directory. It writes terraform.d/plugins there.
#    Add terraform.d/ to your .gitignore.
#
# 2. OpenTofu discovers this mirror automatically with default CLI settings.
#    If you already configure provider_installation explicitly, add this to
#    that block in your CLI config (~/.tofurc or TF_CLI_CONFIG_FILE):
#
#        filesystem_mirror {
#          path    = "/absolute/path/to/your/configuration/terraform.d/plugins"
#          include = ["registry.opentofu.org/alexklibisz/terrifi"]
#        }
#    Exclude registry.opentofu.org/alexklibisz/terrifi from your direct block.
#    Remove any dev_overrides entry for Terrifi to use the installed release.
#    For Terraform instead, run the installer with
#    TERRIFI_REGISTRY_HOST=registry.terraform.io and use that hostname above.
#    The unqualified provider source below works with either CLI.
#
# 3. Set your controller credentials:
#      export UNIFI_API="https://192.168.1.12:8443"
#      export UNIFI_API_KEY="your-api-key"
#      export UNIFI_INSECURE=true
#
# 4. Run:
#      tofu init -upgrade
#      tofu plan
#      tofu apply
#    ./install.sh with no arguments selects the latest release, including
#    prereleases; update the exact version below to match its output.

terraform {
  required_providers {
    terrifi = {
      source  = "alexklibisz/terrifi"
      version = "= 0.11.0-RC0"
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
