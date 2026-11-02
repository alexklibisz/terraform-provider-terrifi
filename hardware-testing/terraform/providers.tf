terraform {
  required_version = ">= 1.10.0"

  required_providers {
    terrifi = {
      source  = "registry.opentofu.org/alexklibisz/terrifi"
      version = "0.11.0"
    }
  }
}

# Controller connection and credentials are supplied through UNIFI_* env vars.
provider "terrifi" {}
