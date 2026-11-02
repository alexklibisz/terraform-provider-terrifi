# HIL Terraform

This directory contains a Terraform project used to provision the HIL-testing hardware.

## Environment variables

Set these before running the Task commands:

| Variable | Value |
| --- | --- |
| `TERRIFI_STATE_BUCKET` | Private R2 bucket containing the Terraform state. |
| `AWS_ENDPOINT_URL_S3` | `https://<account-id>.r2.cloudflarestorage.com` |
| `AWS_ACCESS_KEY_ID` | R2 access key ID, scoped to the state bucket. |
| `AWS_SECRET_ACCESS_KEY` | Corresponding R2 secret access key. |
| `UNIFI_API` | Controller URL, e.g. `https://<hil-controller>:11443`. |
| `UNIFI_API_KEY` | UniFi controller API key. |

For a self-signed controller certificate, also set `UNIFI_INSECURE=true`.
`UNIFI_SITE` is optional and defaults to `default`.

You can copy `.env.example` to the ignored `.env`, fill in the values, and load
them with `source .env` from this directory. Task does not load `.env` automatically.

## Provider version

`providers.tf` pins an exact released provider version, and `.terraform.lock.hcl` records its package hashes for `linux_amd64`, `linux_arm64`, `darwin_amd64`, and `darwin_arm64`.
`terraform.rc` installs the pinned release from the OpenTofu registry even when the parent repository enables `dev_overrides`.

The registry can take a while to index a new release, including release candidates pinned for validation.
Before each reset, HIL CI runs `provider-mirror.sh --configure`, which does nothing if the registry lists the pinned version.
Otherwise it downloads the release zip from GitHub, verifies it against the release's `SHA256SUMS`, unpacks it into a filesystem mirror, and points the CI checkout's `terraform.rc` at that mirror.
The lock file still pins the package hashes, so the reset's read-only `tofu init` rejects a package that does not match.

To pin a release the registry has not indexed yet, generate the lock file from a mirror:

```sh
./provider-mirror.sh /tmp/terrifi-mirror linux_amd64 linux_arm64 darwin_amd64 darwin_arm64
rm .terraform.lock.hcl
TF_CLI_CONFIG_FILE="$PWD/terraform.rc" tofu providers lock -fs-mirror=/tmp/terrifi-mirror \
  -platform=linux_amd64 -platform=linux_arm64 -platform=darwin_amd64 -platform=darwin_arm64
```

A lock file generated this way contains only `h1:` hashes, which also match the packages the registry serves once it indexes the release.
Do not use `--configure` locally: it rewrites the tracked `terraform.rc`.

## Reset before HIL tests

From the repository root, build the CLI (`task build:cli`) and run
`task hil:terraform:reset`. The `terrifi-tests reset` command restores the baseline
and passes `tofu show -json` to the CLI's
`--exclude-state` flag. It imports extra objects without modifying them,
then removes their temporary definitions and applies the deletes. A final plan
must be empty before tests start. Failed resets can be retried using the R2 state.
`task test:reset:hardware` exercises the reset with disposable resources of every
deletable resource type; run it separately from other HIL tests.

This deletes extra supported user-managed resources on the HIL controller, including objects
without the `tfacc-` prefix. Built-in firewall zones/policies and adopted devices
are preserved. Device settings are restored only where configured in the baseline;
this is not a factory reset. Do not run it concurrently with tests or manual applies.

HIL CI runs the reset before checking out test code, with R2 credentials available
only to the reset step. Fork runs use the baseline from `main`. Configure these
repository secrets using the same values as the local environment:

| GitHub secret | Local variable |
| --- | --- |
| `TERRIFI_HIL_STATE_BUCKET` | `TERRIFI_STATE_BUCKET` |
| `TERRIFI_HIL_STATE_ENDPOINT` | `AWS_ENDPOINT_URL_S3` |
| `TERRIFI_HIL_STATE_ACCESS_KEY_ID` | `AWS_ACCESS_KEY_ID` |
| `TERRIFI_HIL_STATE_SECRET_ACCESS_KEY` | `AWS_SECRET_ACCESS_KEY` |

### Switch port tests

Ports 4 and 5 on the baseline Flex Mini (`68:d7:9a:3a:79:ec`) are reserved
for destructive port lifecycle tests and physically marked with RJ45 plugs.
The hardware tests use this fixed allocation directly; no CI port-selection
variables are needed. Tests restore the original override entries afterwards. Custom-profile assignment
tests skip on the original Flex Mini, while inline VLAN lifecycle tests run.

### Staged switch port configuration

The commented configuration in `switch_ports.tf` defines port 2 as an access port
on Default and port 3 as an access port on the Switch lab network (VLAN 20, `192.168.20.0/24`, DHCP
`192.168.20.10`–`192.168.20.99`). Both block tagged traffic. Port 1 remains the
uplink, and ports 4 and 5 remain reserved for acceptance tests.

The pinned provider supports `terrifi_device_ports`, but the configuration remains commented out until it is applied to the HIL hardware.
To enable it, uncomment the configuration and plan/apply the baseline.
Until then, the baseline leaves the ports unchanged.
