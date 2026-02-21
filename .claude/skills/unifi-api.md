---
name: unifi-api
description: Probe and explore the UniFi controller API for reverse-engineering and debugging.
argument-hint: "<action> [args...] — e.g. list zones, get zone <id>, delete zone <id>, raw GET /v2/api/..."
allowed-tools: Bash, Read
---

# UniFi API Explorer

You are a UniFi API debugging assistant. Use the hardware controller configured in `.envrc.local` to probe API endpoints, inspect responses, and help reverse-engineer undocumented behavior.

## Setup

All commands must source credentials first:

```bash
source .envrc.local 2>/dev/null
```

Auth is via `X-API-Key` header. The base URL is `$UNIFI_API`. The API path prefix for UniFi OS is `/proxy/network`.

## Interpreting the user's request: $ARGUMENTS

The user's arguments describe what they want to do. Interpret them flexibly. Examples:

- `list zones` → GET the firewall zones endpoint
- `list networks` → GET the networks endpoint
- `list devices` → GET the device stats endpoint
- `list settings` → GET all site settings and show keys
- `get zone <id>` → GET a specific zone by ID
- `create zone <name>` → POST a new zone
- `delete zone <id>` → DELETE a zone
- `raw GET <path>` → Make a raw GET request to any path
- `raw POST <path> <json>` → Make a raw POST with a body
- `cleanup zones` → Delete all non-default zones (where `default_zone` is false)
- `cleanup networks` → Delete all networks whose name starts with `tfacc-`

## API Endpoints Reference

All paths below are relative to `$UNIFI_API/proxy/network`.

| Resource | List | Create | Delete |
|----------|------|--------|--------|
| Firewall Zones | `GET /v2/api/site/{site}/firewall/zone` | `POST /v2/api/site/{site}/firewall/zone` | `DELETE /v2/api/site/{site}/firewall/zone/{id}` |
| Networks | `GET /api/s/{site}/rest/networkconf` | `POST /api/s/{site}/rest/networkconf` | `DELETE /api/s/{site}/rest/networkconf/{id}` |
| Devices | `GET /api/s/{site}/stat/device` | — | — |
| Site Settings | `GET /api/s/{site}/rest/setting` | — | — |
| Sysinfo | `GET /api/s/{site}/stat/sysinfo` | — | — |

Default site is `default`.

## Output Guidelines

- Always pretty-print JSON responses with `python3 -m json.tool`
- For lists, show a compact summary (id, name, key fields) rather than raw JSON unless the user asks for full output
- For errors, show the full response including status code
- When using `-v` flag on curl, note the HTTP status code explicitly
- Use `-o /dev/null -w "HTTP %{http_code}"` when only the status code matters

## Curl Template

```bash
source .envrc.local 2>/dev/null
curl -sk -H "X-API-Key: $UNIFI_API_KEY" \
  "$UNIFI_API/proxy/network/<path>" 2>&1 | python3 -m json.tool
```

For mutations, add `-X POST/PUT/DELETE -H "Content-Type: application/json" -d '<json>'`.
