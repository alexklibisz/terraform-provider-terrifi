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

Pre-release versions (e.g. `0.4.0-RC2`) are published to the OpenTofu/Terraform registry but may take time to appear. If the version isn't available yet, download the binary directly from the GitHub release and use `dev_overrides`:

```sh
VERSION="0.4.0-RC2"
OS="linux"
ARCH="amd64"
PROVIDER="terraform-provider-terrifi"
OWNER="alexklibisz"

curl -L "https://github.com/${OWNER}/${PROVIDER}/releases/download/v${VERSION}/${PROVIDER}_${VERSION}_${OS}_${ARCH}.zip" \
  -o /tmp/${PROVIDER}-${VERSION}.zip && \
  unzip -o /tmp/${PROVIDER}-${VERSION}.zip -d ~/${PROVIDER}-${VERSION}/
```

Then add to `~/.tofurc` (or `~/.terraformrc`):

```hcl
provider_installation {
  dev_overrides {
    "alexklibisz/terrifi" = "/home/<you>/terraform-provider-terrifi-0.4.0-RC2"
  }
  direct {}
}
```

With `dev_overrides` active, skip `tofu init` and run `tofu plan` directly.

## Releasing

1. Go to the [Tag workflow](../../actions/workflows/tag.yml) in GitHub Actions.
2. Click "Run workflow", enter the version tag (e.g., `v0.1.0`), and run it.
3. The tag workflow creates and pushes the tag, which triggers the [Release workflow](../../actions/workflows/release.yml).
4. The release workflow builds binaries for linux/darwin (amd64/arm64) and publishes them as a GitHub Release.

## On-demand Claude PR reviews

After the workflow is merged into `main`, **alexklibisz** can request a review by
posting a new comment in an open PR's **Conversation** tab containing exactly:

```text
@claude review
```

Only that account (also checked by numeric GitHub account ID) can trigger the job.
Other commenters, ordinary issues, edited comments, inline review comments, new
PRs, and pushes do not start a Claude review. GitHub may show a skipped workflow
for other new comments. Post a new command to review subsequent changes.

### One-time setup

1. Sign into Claude Code using the Claude Pro subscription and run
   `claude setup-token` locally.
2. Save the generated token as the **repository Actions secret**
   `CLAUDE_CODE_OAUTH_TOKEN` in
   [Terrifi's Actions secrets](https://github.com/alexklibisz/terraform-provider-terrifi/settings/secrets/actions).
   Do not paste the token into a PR, issue, or committed file.
3. Merge `.github/workflows/claude-review.yml` into the default branch. GitHub's
   `issue_comment` event requires the workflow there before comments can trigger it.
4. Post the command above on an open PR and check the **Claude on-demand PR review**
   run in Actions. Claude posts its findings as a PR conversation comment under
   `github-actions[bot]`.

No Claude GitHub App installation, personal access token, API key, or self-hosted
runner is needed. The workflow passes GitHub's built-in token to the official
Claude action with `contents: read` and `pull-requests: write`, scoped to this
repository. The job also checks the repository name, so copied workflows in forks
do not run. OAuth authentication uses the Claude subscription allowance; it does
not provide unlimited reviews. GitHub Actions usage is separate. Each run uses
Sonnet with a 20-turn limit and a 15-minute timeout.

The reviewer reads the current PR diff and default-branch context and reports the
reviewed head SHA. It does not run tests or execute PR code. Findings are advisory;
normal testing and human review still apply. To disable reviews, disable the
workflow in Actions or remove it. To rotate the token, replace the repository secret.

References: [Claude action setup](https://github.com/anthropics/claude-code-action/blob/main/docs/setup.md),
[custom GitHub token authentication](https://github.com/anthropics/claude-code-action/blob/main/docs/faq.md#why-am-i-getting-oidc-authentication-errors).
