# Using a Pre-Release

This example shows how to use a pre-released version of the terrifi provider from GitHub Releases.

## Setup

### 1. Install the provider

Run the install script from the directory containing your OpenTofu configuration. It downloads the provider binary from GitHub Releases into an unpacked local mirror under `terraform.d/plugins` in that directory.

```sh
# Install the latest release
./install.sh

# Or install a specific version
./install.sh v0.11.0-RC0
```

Requires the [GitHub CLI](https://cli.github.com/) (`gh`) with access to the repo, and `unzip`. When using your own configuration directory, invoke the script by its absolute path. Add `terraform.d/` to your `.gitignore`.

### 2. Pin the installed version

Use the version printed by the installer. Prereleases need an explicit version constraint, including the exact case of the prerelease suffix:

```hcl
terraform {
  required_providers {
    terrifi = {
      source  = "alexklibisz/terrifi"
      version = "= 0.11.0-RC0"
    }
  }
}
```

OpenTofu discovers `terraform.d/plugins` automatically with default CLI settings. If you already have a `provider_installation` block in `~/.tofurc` or the file selected by `TF_CLI_CONFIG_FILE`, add a mirror to that block:

```hcl
filesystem_mirror {
  path    = "/absolute/path/to/your/configuration/terraform.d/plugins"
  include = ["registry.opentofu.org/alexklibisz/terrifi"]
}
```

Exclude `registry.opentofu.org/alexklibisz/terrifi` from the block's `direct` method, and remove any Terrifi `dev_overrides` entry so the installed release is used.

For Terraform, run `TERRIFI_REGISTRY_HOST=registry.terraform.io ./install.sh v0.11.0-RC0`, use that hostname in any explicit mirror configuration, and run `terraform` instead of `tofu`. The unqualified provider source above works with either CLI.

### 3. Set controller credentials

```sh
export UNIFI_API="https://192.168.1.12:8443"
export UNIFI_API_KEY="your-api-key"
export UNIFI_INSECURE=true
```

### 4. Initialize and review the plan

```sh
tofu init -upgrade
tofu plan
tofu apply
```

Review the plan before applying. A local installation may produce a warning that the lock file only contains checksums for your current platform; other platforms need their own release packages and checksums.

Once your desired version is available in the registry, remove Terrifi's local mirror directory and any explicit mirror rule/exclusion, update the version constraint if needed, and run `tofu init -upgrade` to resume registry installation.
