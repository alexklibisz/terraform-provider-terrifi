# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Terrifi is a Terraform provider for managing Ubiquiti UniFi network infrastructure, built from scratch using the HashiCorp Terraform Plugin Framework (not the legacy SDK). The UniFi controller is reached directly via HTTP — there is no upstream Go SDK in the dependency graph. All API types live in `internal/unifi/` and CRUD methods live alongside each resource in `internal/provider/<name>_api.go`.

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
3. **Model-to-API conversion** — Functions converting between Terraform model types (`types.String`, `types.Bool`) and the local `internal/unifi` API structs
4. **Schema** — Declares HCL attributes with validators, defaults, and plan modifiers

### Key Patterns

- **Null-aware field handling**: Terraform wrapper types (`types.String`, `types.Bool`, `types.Int64`) distinguish null/unknown/set. Optional fields in `internal/unifi` structs use pointer types. Zero values are treated as null to avoid spurious diffs.
- **Full object updates via `applyPlanToState()`**: The UniFi API requires sending complete objects on PUT. Each resource has an `applyPlanToState()` method that merges the user's planned changes into the current state before sending, preventing accidental clearing of API-set fields.
- **Site fallback**: Resources have an optional `site` attribute that falls back to the provider's default site via `Client.SiteOrDefault()`.
- **Configuration cascading**: HCL attributes → environment variables (`UNIFI_API`, `UNIFI_USERNAME`, `UNIFI_PASSWORD`, `UNIFI_API_KEY`, `UNIFI_INSECURE`, `UNIFI_SITE`) → defaults.
- **Compile-time interface checks**: `var _ resource.Resource = &dnsRecordResource{}` pattern at the top of each resource file.

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

### UniFi Wire-Format Notes

The controller has several quirks worth knowing about when adding or changing resources. They are not bugs in any client we use; they are observed controller behavior.

- **v2 endpoints emit numeric fields as strings on some sites**: `DNSRecord.Port/Priority/Ttl/Weight` (handled via `unifi.Number` in `internal/unifi/number.go`), `FirewallPolicy*.Port` (handled via `firewallPolicyEndpointResponse.parsePort`), `Network.IPV6RaPreferredLifetime` (the field is intentionally not declared in our local `Network`, so json silently drops it), `DeviceRadioTable.TxPower/Channel` (regex-coerced in `device_api.go`'s `fixRadioTableBytes`).
- **v2 firewall-zone POST/PUT rejects `default_zone: false`**: the local `unifi.FirewallZone` does not declare `DefaultZone`, so it is never serialized.
- **v2 firewall-zone and firewall-policy PUT require `_id` in the body**: each local struct declares `json:"_id,omitempty"`, so a non-empty ID rides along automatically.
- **v2 endpoints return 201/204 statuses on POST/DELETE**: `doV2Request` in `internal/provider/http.go` accepts any 2xx as success.
- **v2 network-members-group**: LIST uses the plural URL (`/network-members-groups`); every other verb uses the singular URL (`/network-members-group[/id]`).
- **v1 user (client device) booleans**: `use_fixedip`, `local_dns_record_enabled`, and `fixed_ap_enabled` must not be sent as bare `false` when the provider is not authoritative over them — `client_device_api.go` builds a bespoke request struct that uses `*bool` + `omitempty` and derives the toggles from the presence of the related values.
- **Network `setting_preference=manual` is required on corporate networks**: without it the controller auto-overrides DHCP and subnet settings. `forceManualSettingPreference` in `network_resource.go` does this on every write.
- **Network DHCP range fields crash the controller on `""`**: pointer-to-empty-string serializes as `"dhcpd_start":""` (omitempty does not save you on non-nil pointers). The `IsUnknown()` guards in `network_resource.go`'s `modelToAPI` prevent that from happening during Terraform create.
- **v1 `/rest/networkconf` DELETE wants the name in the body**: not just in the URL. `DeleteNetwork(ctx, site, id, name)` mirrors that contract.
- **UpdateDevice fragility**: full-object PUTs round-trip stat counters that change between reads and confuse the controller. `device_api.go`'s `deviceUpdatePayload` sends only the fields the provider manages.

When adding a new resource, treat all of these as the kinds of quirks the controller might throw at you and write a focused test for any new one.

### CLI and Import Generation

The `cmd/terrifi/` directory contains a Cobra-based CLI. Its main command is `generate-imports`, which connects to a live UniFi controller and outputs Terraform `import {}` + `resource {}` blocks to stdout.

The `internal/generate/` package provides the conversion logic: each resource type has a `<Name>Blocks()` function (e.g., `DNSRecordBlocks()`) that converts `internal/unifi` structs into `ResourceBlock` objects, which are then rendered as HCL. Shared helpers like `ToTerraformName()`, `DeduplicateNames()`, and HCL formatting functions (`HCLString`, `HCLBool`, etc.) live in `generate.go`.

### Client Architecture

`internal/provider/client.go` defines the `Client` struct that every resource holds. Key details:
- Uses `go-retryablehttp` for automatic retries with TLS configuration.
- On initialization, probes the controller to discover the API path (`/proxy/network` for UniFi OS vs. empty for legacy controllers).
- One login flow: when authenticating with username/password, `loginForCustomRequests` posts to `/api/login` (or `/api/auth/login` on UniFi OS) and stashes the CSRF token from the response headers. API-key auth skips login — the key is sent as a header on every request.
- `ClientConfigFromEnv()` reads `UNIFI_*` env vars, shared between the provider and CLI.
- All HTTP work flows through `doV2Request` in `internal/provider/http.go` (despite the name, it is endpoint-agnostic — v1 callers wrap it via a `doXRequest` helper that translates 404 to `*unifi.NotFoundError`).

### Adding a New Resource

1. Add the API type to `internal/unifi/<name>.go`. Declare only the fields the provider reads or writes; the controller's other fields are dropped on decode.
2. Create `internal/provider/<name>_resource.go` with model struct, CRUD methods, and schema.
3. Create `internal/provider/<name>_api.go` with `Create/Get/Update/Delete[/List]` methods on `*Client` that use `doV2Request` for v2 endpoints (or a 404-translating wrapper for v1).
4. Create `internal/provider/<name>_resource_test.go` with unit tests for model conversion and acceptance tests for CRUD lifecycle.
5. Register the resource in `provider.go` → `Resources()` method.
6. Create `internal/generate/<name>.go` with a `<Name>Blocks()` function for import generation.
7. Add the resource type to `cmd/terrifi/generate_imports.go` (`validResourceTypes` slice and switch statement).
8. Add docs in `docs/resources/` and examples in `examples/`.
