# UniFi client

This package provides the controller API contracts and HTTP client used by the Terraform provider and the Terrifi CLI.
It has no Terraform dependencies or imports from other Terrifi packages.
Its only nonstandard runtime dependency is `github.com/hashicorp/go-retryablehttp`.

Create a client with `New(ctx, Config{...})` or read connection settings with `ConfigFromEnv()`.
The client probes the controller root to select the UniFi OS `/proxy/network` prefix or the legacy controller layout.
API keys take precedence over username/password credentials.
Session authentication performs one login and shares its cookies and CSRF token across all endpoints.
TLS verification is enabled unless `AllowInsecure` is set.
Requests honor context cancellation and use a 30-second HTTP timeout with retryable transport errors and statuses.

## Boundaries

- API contracts contain ordinary Go values, pointers, slices, and JSON tags.
- Endpoint files own URL construction, request serialization, response envelopes, and controller-specific response decoding.
- The provider owns Terraform schemas, null/unknown handling, plan-to-request conversion, default-site resolution, diagnostics, and Terraform-specific settling after writes.
- The CLI uses this package directly and passes returned contracts to `internal/generate` for HCL generation.

The package remains inside the repository's Go module so provider and client changes can be reviewed and validated together.
Extracting it into a separate module requires moving the package, changing consumer import paths, and declaring the retryable HTTP dependency; it does not require extracting Terraform types or provider implementation code.
The API covers Terrifi's supported operations rather than every controller endpoint.

## Endpoints

| Contract | Controller API |
| --- | --- |
| DNS records | v2 `static-dns` |
| Firewall groups | v1 `rest/firewallgroup` |
| Firewall zones | v2 `firewall/zone` |
| Firewall policies and ordering | v2 `firewall-policies` and `batch-reorder` |
| Client groups | v2 `network-members-group` and list `network-members-groups` |
| Client devices | v1 `rest/user`, station commands, and v2 fingerprint overrides |
| Networks | v1 `rest/networkconf` |
| WLANs | v1 `rest/wlanconf`, with WLAN, user, and AP group lookups |
| Devices and ports | v1 `stat/device` and `rest/device` |
| Port profiles | v1 `rest/portconf` |
| Controller and fingerprint metadata | v1 system information and v2 fingerprint database |

## Writes and response handling

HTTP success includes all 2xx statuses, including 201 on creation and 204 on deletion.
V1 REST operations also inspect the controller's `meta` envelope.
Reads return `*NotFoundError` when an object is absent; `*HTTPError` reports failed HTTP requests without including request payloads that may contain secrets.
Empty successful v1 update envelopes trigger a readback when an updated object is required.
Numeric fields known to vary across controller versions accept JSON numbers and numeric strings.
Radio channels and transmit power accept either numeric or string responses.

Network, WLAN, firewall-group, and port-profile updates overlay their write contracts on the stored raw JSON object.
Device updates send a narrow payload, radio entries retain unknown JSON fields, and port overrides preserve settings outside the provider's managed ports.
These paths preserve controller settings outside Terrifi's exposed attributes without adding those settings to the typed contracts.
Nil member lists are normalized to empty arrays where an authoritative empty list is required.

GET response caching is opt-in and scoped to a client instance.
Writes invalidate cached reads, and verification of asynchronous controller changes uses uncached requests.
Configure caching before sharing a client across goroutines.

## Validation

`task test:unit` runs client and provider unit tests, including mock HTTP tests for authentication, response caching, raw-field preservation, and controller response decoding.
`task test:cli` checks HCL generation.
`task test:acc` and `task test:acc:hardware` exercise the provider lifecycle against simulated and hardware controllers using the same contracts and transport.
The resource [validation coverage guide](../../docs/guides/validation-coverage.md) describes target limitations and the settings covered by acceptance tests.
