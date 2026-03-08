# Development

## Prerequisites

- [Go](https://go.dev/) (see `go.mod` for the required version)
- [Task](https://taskfile.dev/) (build runner)
- [OpenTofu](https://opentofu.org/) or [Terraform](https://www.terraform.io/)

## Building

Build the provider and CLI binaries:

```sh
task build
```

This does three things:

1. Builds the `terraform-provider-terrifi` binary and installs it to your `GOBIN`
2. Builds the `terrifi` CLI binary and installs it to your `GOBIN`
3. Generates a `.terraformrc` file with `dev_overrides` pointing at the locally-built provider

## Testing locally with a Terraform/OpenTofu project

After running `task build`, you can use the locally-built provider in any Terraform/OpenTofu project:

1. Build the provider:

    ```sh
    cd /path/to/terrifi
    task build
    ```

2. In the terminal where you run your Terraform/OpenTofu commands, point at the generated `.terraformrc`:

    ```sh
    export TF_CLI_CONFIG_FILE=/path/to/terrifi/.terraformrc
    ```

3. Run your plan/apply as usual:

    ```sh
    cd /path/to/your/terraform/project
    tofu apply
    ```

The `dev_overrides` in `.terraformrc` tell Terraform/OpenTofu to use the locally-built binary instead of downloading from the registry. No `terraform init` or `tofu init` is needed.

## Running tests

Unit tests (fast, no network needed):

```sh
task test:unit
```

Run a single test:

```sh
task test:unit -- -run TestCheckV1Meta
```

Acceptance tests against a Docker-based UniFi controller:

```sh
task test:acc
```

Acceptance tests against real hardware (requires `UNIFI_*` env vars):

```sh
task test:acc:hardware
```

Run a single acceptance test:

```sh
task test:acc -- -run TestAccDNSRecord_basic
```

## Linting

```sh
task lint
```

## Releasing

1. Go to the [Tag workflow](../../actions/workflows/tag.yml) in GitHub Actions.
2. Click "Run workflow", enter the version tag (e.g., `v0.1.0`), and run it.
3. The tag workflow creates and pushes the tag, which triggers the [Release workflow](../../actions/workflows/release.yml).
4. The release workflow builds binaries for linux/darwin (amd64/arm64) and publishes them as a GitHub Release.
