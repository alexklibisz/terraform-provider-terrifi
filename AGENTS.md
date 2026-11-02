# AGENTS.md

This file provides guidance to coding agents working in this repository.

## Repository Skills

For UniFi controller API exploration, reverse-engineering, and debugging, read
[the unifi-api skill](.agents/skills/unifi-api/SKILL.md). It includes endpoint
references, credential setup, and controller log commands.

## Project Overview

Terrifi is a Terraform provider for managing Ubiquiti UniFi network infrastructure, built from scratch using the HashiCorp Terraform Plugin Framework (not the legacy SDK). It uses local API contracts and an independent HTTP client in `internal/unifi/`.

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
3. **Model-to-API conversion** — Functions converting between Terraform model types (`types.String`, `types.Bool`) and local UniFi API structs
4. **Schema** — Declares HCL attributes with validators, defaults, and plan modifiers

### Key Patterns

- **Null-aware field handling**: Terraform wrapper types (`types.String`, `types.Bool`, `types.Int64`) distinguish null/unknown/set. Optional fields use pointer types in local UniFi contracts. Zero values are treated as null to avoid spurious diffs.
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
Full-object API writes and serialization defaults must not reset these settings.

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

### UniFi API Contracts

All controller contracts, authentication, transport, response caching, and endpoint methods live in `internal/unifi/`.
This package must remain independent of Terraform and other Terrifi packages so it can be extracted into a separate module.
See [the client package documentation](internal/unifi/README.md) for endpoints, serialization rules, and validation.

Keep controller quirks in endpoint implementations and local JSON decoders rather than provider resource methods.
Use narrow write payloads or merge managed fields into raw controller JSON so settings outside Terraform's exposed attributes survive updates.
Provider code owns Terraform types, schemas, plan-to-request conversion, diagnostics, and settling of asynchronous controller changes.

### CLI and Import Generation

The `cmd/terrifi/` directory contains a Cobra-based CLI. Its main command is `generate-imports`, which connects to a live UniFi controller and outputs Terraform `import {}` + `resource {}` blocks to stdout.

The `internal/generate/` package provides the conversion logic: each resource type has a `<Name>Blocks()` function (e.g., `DNSRecordBlocks()`) that converts local UniFi API structs into `ResourceBlock` objects, which are then rendered as HCL. Shared helpers like `ToTerraformName()`, `DeduplicateNames()`, and HCL formatting functions (`HCLString`, `HCLBool`, etc.) live in `generate.go`.

### Client Architecture

`internal/unifi/api_client.go` defines the independent controller `APIClient` and connection `Config`.
Key details:
- Uses `go-retryablehttp` with TLS configuration, cookies, and one authentication session for all endpoints.
- On initialization, probes the controller to discover the API path (`/proxy/network` for UniFi OS vs. empty for legacy controllers).
- `unifi.ConfigFromEnv()` reads `UNIFI_*` environment variables for both the provider and CLI.
- `internal/provider/client.go` embeds the independent client and adds Terraform-specific default-site resolution.
- HTTP endpoint methods belong in `internal/unifi/*_api.go`, never in the provider package.

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

### Release Notes

Use [Scala Steward's release notes](https://github.com/scala-steward-org/scala-steward/releases) as a formatting guide and [v0.11.0](https://github.com/alexklibisz/terraform-provider-terrifi/releases/tag/v0.11.0) as a repository example.
Start with a short paragraph highlighting the release's main user-facing changes.
Under `## What's Changed`, group entries in this order, omitting empty categories:

- `### 🚀 Features and Enhancements`
- `### 🐛 Bug Fixes`
- `### 📚 Documentation`
- `### 🧪 Testing`
- `### 🏗️ Build and Infrastructure`
- `### 🌱 Dependency Updates`

Lead with features and fixes that matter to provider users; put internal testing, CI, security, and contributor-workflow changes in the later categories.
Describe the final released behavior and its benefit rather than copying PR titles or recounting implementation history.
For major user-facing entries, start the bullet with a short bold summary, name the affected resources or attributes, and link to documentation when useful.
Combine related PRs into one item when they implement or refine the same behavior, including fixes to code introduced earlier in the same release; link every contributing PR from that item.
Group related internal changes and dependency bumps to keep them concise, and show the previous and final versions for dependency updates.
Distinguish product fixes from test-only changes; a fix to import verification tests, for example, belongs under Testing unless provider behavior also changed.
Account for every PR in the release range, remove duplicate generated entries, and avoid claims broader than the implementation and validation support.
Credit contributors in the relevant entries, retain `## New Contributors` when applicable, and finish with a `Full Changelog` link comparing the previous and current release tags.
Follow the documentation line-formatting rules above.

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
