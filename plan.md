# Terrifi Provider Scaffold Plan

## Goal
Scaffold a working Terraform provider named `terrifi` with a single `terrifi_dns_record` resource, buildable and runnable against the local UniFi test harness (Gateway Lite + AC Pro + mini PC running UniFi OS Server).

## File Structure

```
terrifi/
├── main.go                          # Entry point, provider server
├── go.mod / go.sum                  # Module: github.com/terrifi/terraform-provider-terrifi
├── Taskfile.yml                     # build, test, fmt, vet
├── .envrc                           # Env vars for real test harness
├── internal/
│   └── provider/
│       ├── provider.go              # Provider config, auth, client wrapper
│       ├── logger.go                # tflog adapter for go-retryablehttp
│       └── dns_record_resource.go   # terrifi_dns_record full CRUD
└── examples/
    └── dns_record/
        └── main.tf                  # Manual test config with dev_overrides
```

## Steps

### 1. Initialize Go module and fetch dependencies
- `go mod init github.com/terrifi/terraform-provider-terrifi`
- Dependencies: terraform-plugin-framework v1.17+, terraform-plugin-framework-validators, go-retryablehttp, terraform-plugin-log, go-unifi v1.33.29

### 2. main.go — provider entry point
- Provider server address: `registry.terraform.io/terrifi/terrifi`
- Debug flag for delve attachment
- Calls `providerserver.Serve` with our provider constructor

### 3. internal/provider/provider.go — provider configuration
- Type name: `terrifi`
- Schema (6 attributes): `api_url`, `username`, `password`, `api_key`, `site`, `allow_insecure`
- Env var fallbacks: `UNIFI_API`, `UNIFI_USERNAME`, `UNIFI_PASSWORD`, `UNIFI_API_KEY`, `UNIFI_SITE`, `UNIFI_INSECURE`
- Configure method: retryablehttp client → TLS config → cookie jar → go-unifi ApiClient → auth (API key or login) → Client wrapper
- Registers `terrifi_dns_record` resource

### 4. internal/provider/logger.go — retryablehttp logger adapter
- Bridges go-retryablehttp's LeveledLogger interface to terraform-plugin-log/tflog

### 5. internal/provider/dns_record_resource.go — first resource
- Full CRUD + ImportState (import format: `site:id` or `id`)
- Model: ID, Site, Name, Enabled, Port, Priority, RecordType, TTL, Value, Weight
- Schema with validators (port 0-65535, priority >= 0, record_type enum, ttl <= 65535)
- Conversion helpers: `modelToDNSRecord()`, `dnsRecordToModel()`
- `applyPlanToState()` for merge-on-update
- Site fallback to provider default
- Uses go-unifi SDK: CreateDNSRecord, GetDNSRecord, UpdateDNSRecord, DeleteDNSRecord

### 6. Taskfile.yml
- `task build` — `go install`
- `task test` — `TF_ACC=1 go test ./... -v -count 1 -timeout 10m`
- `task fmt` — `go fmt ./...`
- `task vet` — `go vet ./...`

### 7. .envrc — test environment
- Template pointing at real hardware (192.168.1.12:8443 or whatever the actual address is)

### 8. examples/dns_record/main.tf — smoke test config
- Terraform block requiring `terrifi/terrifi` provider
- Creates an A record like `test.home` -> `192.168.1.100`
- Instructions for setting up `~/.terraformrc` dev_overrides

### 9. Build and verify
- `go mod tidy && go build ./...`
- `task build` to install
- Configure dev_overrides in `~/.terraformrc`
- `terraform plan` + `terraform apply` in examples/dns_record/ against real hardware

## Design Decisions
- **go-unifi SDK** for all UniFi API calls — typed structs, CRUD methods, auth handling
- **internal/provider/ package** — standard Terraform provider convention
- **No util/merge package yet** — DNS record is simple; we'll add generic merge when we tackle Network
- **No acceptance tests yet** — validate against real hardware first, add test framework next
- **Same env var names as community provider** — easy to switch, reuse existing .envrc
- **Taskfile over Makefile** — cleaner YAML syntax, already installed
