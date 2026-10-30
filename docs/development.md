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

## Testing the provider locally with a Terraform/OpenTofu project

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

## Testing the CLI locally

1. Build the CLI:

    ```sh
    $ cd /path/to/terrifi
    $ task build:cli
    ```

2. Run it with `--version` to verify the install:

    ```sh
    $ terrifi --version
    terrifi version v0.9.2-0.20260430142350-658b70734fe1
    ```

You should see a version that ends with a Git hash.

### Switch back to the released CLI

```sh
$ go install github.com/alexklibisz/terrifi/cmd/terrifi@latest
$ terrifi --version
terrifi version v0.9.1
```

### Troubleshooting

The `task build:cli` puts the binary ont
If you don't, that means `task build:cli` wasn't able to get the binary onto your `$GOBIN`, or that your `$GOBIN` is not on your `$PATH`.

```sh
$ echo $GOBIN
/Users/alex/.asdf/installs/golang/1.26.1/bin
$ which terrifi
/Users/alex/.asdf/installs/golang/1.26.1/bin/terrifi
```

## Running tests

Resource and data-source pages use the shared [validation coverage](guides/validation-coverage.md) convention. Update their tables when adding settings or changing test coverage. A HIL test that checks saved controller configuration establishes Controller/API coverage; Hardware behavior coverage requires observing the effect on a device or connected client.

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

## Installing a pre-release version

If a release is available on GitHub but has not appeared in the OpenTofu/Terraform registry yet, use the [prerelease installation example](../examples/using-pre-release/README.md). Its installer downloads the release into a local filesystem mirror, so normal initialization, version selection, and dependency locking work without registry publication.

Run the installer from your infrastructure configuration directory:

```sh
/path/to/terraform-provider-terrifi/examples/using-pre-release/install.sh v0.11.0-RC0
```

Then pin `version = "= 0.11.0-RC0"` with `source = "alexklibisz/terrifi"` in your `required_providers` block, and run `tofu init -upgrade` followed by `tofu plan`. The example covers explicit CLI installation settings, Terraform compatibility, and switching back to registry installation.

The installer uses an unpacked mirror to preserve uppercase prerelease suffixes such as `RC0`; OpenTofu lowercases ZIP filenames when discovering packed mirrors. Remove any Terrifi `dev_overrides` entry from your active CLI configuration before using the release. In particular, the provider repository's development environment selects its generated `.terraformrc` through `TF_CLI_CONFIG_FILE`; use your infrastructure project's CLI configuration when testing a release.

## Releasing

1. Go to the [Release workflow](https://github.com/alexklibisz/terraform-provider-terrifi/actions/workflows/release.yml) in GitHub Actions.
2. Click "Run workflow", select the branch or ref to release (normally `main`), and enter the version tag (e.g., `v0.11.0`).
3. Run the workflow. It creates and pushes the tag at the selected ref, then builds provider and CLI binaries for Linux and macOS (amd64/arm64).
4. Confirm the workflow succeeds and the GitHub Release contains the archives, checksums, detached checksum signature, and registry manifest. The signed release can then be indexed by the Terraform/OpenTofu registries.

## On-demand CodeRabbit PR reviews

CodeRabbit reviews run through its GitHub App.

The root `.coderabbit.yaml` disables automatic reviews and automatic incremental
reviews on pushes. To request a complete review, post a comment on the PR:

```text
@coderabbitai full review
```

For an incremental review of changes since the previous review:

```text
@coderabbitai review
```

Manual review commands also work for PRs targeting a branch other than `main`,
including stacked PRs. Review progress and findings appear on the PR from
`coderabbitai[bot]`. Leave **Autofix** unchecked when you only want feedback.

