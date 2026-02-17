# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Terrifi is a Terraform provider for managing Ubiquiti UniFi network infrastructure, built from scratch using the HashiCorp Terraform Plugin Framework (not the legacy SDK). It uses the [go-unifi](https://github.com/ubiquiti-community/go-unifi) SDK under the hood.

## Commands

This project uses [Task](https://taskfile.dev/) (not Make) as the build runner.

- `task build` — Build the provider binary and generate `.terraformrc` for local dev
- `task lint` — Run `go fmt` + `go vet`
- `task test:unit` — Fast unit tests (no network, no Docker)
- `task test:acc` — Acceptance tests against a Docker UniFi controller (starts/stops automatically)
- `task test:acc:hardware` — Acceptance tests against real hardware (requires `.envrc.local`)
- `task deps` — Download Go module dependencies

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
- **Site fallback**: Resources have an optional `site` attribute that falls back to the provider's default site via `Client.SiteOrDefault()`.
- **Configuration cascading**: HCL attributes → environment variables (`UNIFI_API`, `UNIFI_USERNAME`, `UNIFI_PASSWORD`, `UNIFI_API_KEY`, `UNIFI_INSECURE`, `UNIFI_SITE`) → defaults.
- **Compile-time interface checks**: `var _ resource.Resource = &dnsRecordResource{}` pattern at the top of each resource file.

### Testing

Tests are in the same package (`internal/provider/`), controlled by `TestMain`:
- **Unit tests** (no `TF_ACC`): Test model-to-API conversions and field mappings. Use `testify/assert` and `testify/require`.
- **Acceptance tests** (`TF_ACC=1`): Full Terraform lifecycle tests using `helper/resource.Test()`. Prefixed with `TestAcc`. Two modes:
  - `TERRIFI_ACC_TARGET=docker` (default): Spins up UniFi controller via Docker Compose with testcontainers-go
  - `TERRIFI_ACC_TARGET=hardware`: Uses real hardware configured via `.envrc.local`

Each new feature should include extensive acceptance testing.
Think of interesting permutations of settings and sequences of changes.
Too much testing is better than too little - don't hold back.

### Adding a New Resource

1. Create `internal/provider/<name>_resource.go` with model struct, CRUD methods, and schema
2. Create `internal/provider/<name>_resource_test.go` with unit tests for model conversion and acceptance tests for CRUD lifecycle
3. Register the resource in `provider.go` → `Resources()` method
4. Add docs in `docs/resources/` and examples in `examples/`
