# Using a Pre-Release

This example shows how to use a pre-released version of the terrifi provider from GitHub Releases.

## Setup

### 1. Install the provider

The install script downloads the provider binary and places it in a local filesystem mirror.

```sh
# Install the latest release
./install.sh

# Or install a specific version
./install.sh v0.1.0-RC1
```

Requires the [GitHub CLI](https://cli.github.com/) (`gh`) with access to the repo.

### 2. Configure Terraform

Add a filesystem mirror to `~/.terraformrc`:

```hcl
provider_installation {
  filesystem_mirror {
    path = "/home/<you>/.terraform.d/plugins"
  }
  direct {}
}
```

### 3. Set controller credentials

```sh
export UNIFI_API="https://192.168.1.12:8443"
export UNIFI_API_KEY="your-api-key"
export UNIFI_INSECURE=true
```

### 4. Run Terraform

```sh
terraform init
terraform plan
terraform apply
```
