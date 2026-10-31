# AGENTS.md

This file provides guidance to coding agents working in this repository.

## Repository Skills

For UniFi controller API exploration, reverse-engineering, and debugging, read
[the unifi-api skill](.agents/skills/unifi-api/SKILL.md). It includes endpoint
references, credential setup, and controller log commands.

## Project Overview

Terrifi is a Terraform provider for managing Ubiquiti UniFi network infrastructure, built from scratch using the HashiCorp Terraform Plugin Framework (not the legacy SDK). It uses the [go-unifi](https://github.com/ubiquiti-community/go-unifi) SDK under the hood.

## Commands

This project uses [Task](https://taskfile.dev/) (not Make) as the build runner. Read `Taskfile.yml` for all available tasks.

Run a single test:
```sh
task test:unit -- -run TestDNSRecordModelToAPI
task test:acc -- -run TestAccDNSRecord_basic
```

The `-- -run <pattern>` syntax passes `-run` through to `go test` via `{{.CLI_ARGS}}`.

## Architecture

### Provider Structure

All provider code lives in `internal/provider/`. Each resource follows the Terraform Plugin Framework pattern:

1. **Model struct** (e.g., `dnsRecordModel`) — Go struct with `tfsdk:` tags mapping HCL attributes
2. **CRUD methods** — `Create`, `Read`, `Update`, `Delete`, `ImportState`
3. **Model-to-API conversion** — Functions converting between Terraform model types (`types.String`, `types.Bool`) and go-unifi API structs
4. **Schema** — Declares HCL attributes with validators, defaults, and plan modifiers

### Key Patterns

- **Null-aware field handling**: Terraform wrapper types (`types.String`, `types.Bool`, `types.Int64`) distinguish null/unknown/set. Optional fields use pointer types in go-unifi structs. Zero values are treated as null to avoid spurious diffs.
- **Full object updates via `applyPlanToState()`**: The UniFi API requires sending complete objects on PUT. Each resource has an `applyPlanToState()` method that merges the user's planned changes into the current state before sending, preventing accidental clearing of API-set fields.
- **Site fallback**: Resources have an optional `site` attribute that falls back to the provider's default site via `Client.SiteOrDefault()`.
- **Configuration cascading**: HCL attributes → environment variables (`UNIFI_API`, `UNIFI_USERNAME`, `UNIFI_PASSWORD`, `UNIFI_API_KEY`, `UNIFI_INSECURE`, `UNIFI_SITE`) → defaults.
- **Compile-time interface checks**: `var _ resource.Resource = &dnsRecordResource{}` pattern at the top of each resource file.

### Setting Ownership

For resources managed by Terraform, assume every controller setting exposed as a configurable Terraform attribute is exclusively managed through Terraform.
This applies even when an optional attribute is omitted from the configuration.
Implementations, tests, and code reviews should not assume someone concurrently edits these settings in the UniFi UI or through another API client between Terraform refresh and apply.
Preserving such concurrent edits is not a requirement and does not by itself justify a fix or additional update logic.

Controller settings the provider does not expose may be managed outside Terraform and must be preserved during resource updates.
Full-object API writes and SDK serialization defaults must not reset these settings.

Interpret omitted, null, and unknown values according to the attribute's schema and Terraform Plugin Framework semantics.
Omitting an optional/computed attribute does not by itself relinquish ownership or request a reset; it may retain its prior state or use a controller-computed value.
This ownership model does not remove the need for correct refresh and drift handling, imports, or consistency between configuration, plan, and resulting state.

### Testing

Tests are in the same package (`internal/provider/`), controlled by `TestMain`:
- **Unit tests** (no `TF_ACC`): Test model-to-API conversions and field mappings. Use `testify/assert` and `testify/require`.
- **Acceptance tests** (`TF_ACC=1`): Full Terraform lifecycle tests using `helper/resource.Test()`. Prefixed with `TestAcc`. Two modes:
  - `TERRIFI_ACC_TARGET=docker` (default): Spins up UniFi controller via Docker Compose with testcontainers-go
  - `TERRIFI_ACC_TARGET=hardware`: Uses real hardware configured via `.envrc.local`
- **Test helpers** in `provider_test.go`: `preCheck()` validates env vars, `randomSuffix()` generates unique resource names to avoid conflicts from leftover resources.

Each new feature should include extensive acceptance testing.
Think of interesting permutations of settings and sequences of changes.
Too much testing is better than too little - don't hold back.

### go-unifi SDK Workarounds

The go-unifi SDK has several bugs that require workarounds in this provider. All workarounds are tagged with `TODO(go-unifi)` comments so they can be found and removed when the SDK is fixed.

**Conventions for SDK workarounds:**
- Tag every workaround site with a `// TODO(go-unifi):` comment explaining the upstream bug, the symptom, and what SDK fix would allow removing the workaround.
- When possible, isolate workarounds into named helper functions (e.g., `applySDKSettingPreferenceWorkaround`) so they can be deleted as a unit.
- When the SDK lacks working methods for an API (e.g., v2 firewall zones), put all custom HTTP logic in a dedicated `*_api.go` file (e.g., `firewall_zone_api.go`) with a file-level TODO explaining which SDK methods it replaces.
- Never patch the SDK in the module cache. All workarounds live in this repo.

**Current workarounds (search `TODO(go-unifi)` for details):**
- `firewall_zone_api.go` — Bypasses SDK's firewall zone CRUD entirely due to three bugs: `default_zone` serialization (400), missing `_id` in PUT body (500), and DELETE returning 204 treated as error.
- `firewall_policy_api.go` — Custom HTTP methods for v2 firewall policy endpoints not supported by the SDK.
- `client_device_api.go` — Custom HTTP methods for v2 client device endpoints not supported by the SDK.
- `network_resource.go` — `applySDKSettingPreferenceWorkaround()` forces `setting_preference=manual` because the SDK defaults to `auto`, which causes the controller to auto-override settings like DHCP enable.
- `network_resource.go` — `IsUnknown()` guards on DHCP fields in `modelToAPI` prevent passing empty strings to the SDK, which would crash the controller via `marshalCorporate()`'s `valueOrDefault()` defaults.

### CLI and Import Generation

The `cmd/terrifi/` directory contains a Cobra-based CLI. Its main command is `generate-imports`, which connects to a live UniFi controller and outputs Terraform `import {}` + `resource {}` blocks to stdout.

The `internal/generate/` package provides the conversion logic: each resource type has a `<Name>Blocks()` function (e.g., `DNSRecordBlocks()`) that converts go-unifi API structs into `ResourceBlock` objects, which are then rendered as HCL. Shared helpers like `ToTerraformName()`, `DeduplicateNames()`, and HCL formatting functions (`HCLString`, `HCLBool`, etc.) live in `generate.go`.

### Client Architecture

`internal/provider/client.go` defines the `Client` struct wrapping `go-unifi`'s `ApiClient`. Key details:
- Uses `go-retryablehttp` for automatic retries with TLS configuration.
- On initialization, probes the controller to discover the API path (`/proxy/network` for UniFi OS vs. empty for legacy controllers).
- `ClientConfigFromEnv()` reads `UNIFI_*` env vars, shared between the provider and CLI.

### Adding a New Resource

1. Create `internal/provider/<name>_resource.go` with model struct, CRUD methods, and schema
2. Create `internal/provider/<name>_resource_test.go` with unit tests for model conversion and acceptance tests for CRUD lifecycle
3. Register the resource in `provider.go` → `Resources()` method
4. Create `internal/generate/<name>.go` with a `<Name>Blocks()` function for import generation
5. Add the resource type to `cmd/terrifi/generate_imports.go` (`validResourceTypes` slice and switch statement)
6. Add docs in `docs/resources/` and examples in `examples/`

## Development

### Documentation

#### Contents

1. Write documentation for a reader who has no knowledge of the agent, the conversation, or the implementation process. The reader does not know or care who you are or what has been chatted about in the session. Describe the tool and its behavior as they exist: supported attributes, defaults, constraints, examples, and limitations. Every statement must make sense without hidden context from an issue, PR, review, or agent session.
2. Carefully consider the long-term structure of the documentation. Just because we're working on something in a given PR, does not mean it now lands front-and-center in the docs.
3. As implementations change, keep the documentation updated.
4. Keep test validation coverage tables up to date in the same PR as the provider and its API change.

#### Format

1. Avoid the typical ~80-column newlines. In paragraphs, write one sentence per line. In numbered and un-numbered lists, each list item is a line with no breaks between sentences.

### Merging Pull Requests

PRs are squash-merged. Never use GitHub's default squash message, which concatenates every branch commit into a bullet list. Write the message explicitly:

- **Subject:** the PR title followed by the PR number, e.g. `Add switch port profiles and device port assignments (#183)`.
- **Body:** one short paragraph (1–3 sentences) summarizing what the PR changes and why, written for someone reading `git log` on `main`. Do not list the branch commits, review fixes, or intermediate steps.
- **Trailers:** keep a `Co-authored-by:` line for every co-author of the branch commits except the person merging (GitHub may add one for the merger's other email address). GitHub only adds them automatically to its default message.
- **No separator:** do not add the `---------` line that the GitHub web UI puts before the trailers. Separate the body and the trailers with one blank line.

```sh
gh pr merge 183 --squash \
  --subject "Add switch port profiles and device port assignments (#183)" \
  --body "Add terrifi_port_profile and terrifi_device_ports, including local UniFi API contracts, import generation, documentation, and lifecycle tests.

Co-authored-by: Claude Opus 5.5 <noreply@anthropic.com>"
```
