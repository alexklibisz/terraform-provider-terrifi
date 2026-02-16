---
page_title: "terrifi_dns_record Resource - Terrifi"
subcategory: ""
description: |-
  Manages a DNS record on the UniFi controller.
---

# terrifi_dns_record (Resource)

Manages a DNS record on the UniFi controller.

## Example Usage

### A record

```terraform
resource "terrifi_dns_record" "web" {
  name        = "web.example.com"
  value       = "192.168.1.100"
  record_type = "A"
  ttl         = 300
}
```

### SRV record with optional fields

```terraform
resource "terrifi_dns_record" "minecraft" {
  name        = "_minecraft._tcp.example.com"
  value       = "mc.example.com"
  record_type = "SRV"
  port        = 25565
  priority    = 10
  weight      = 100
  ttl         = 3600
}
```

## Schema

### Required

- `name` (String) — The hostname for the DNS record. Changing this forces a new resource.
- `value` (String) — The value of the DNS record (IP address, hostname, etc.).

### Optional

- `enabled` (Boolean) — Whether the DNS record is enabled. Defaults to `true`.
- `port` (Number) — The port for SRV records. Must be between 0 and 65535.
- `priority` (Number) — The priority for MX/SRV records. Must be >= 0.
- `record_type` (String) — The DNS record type. One of: `A`, `AAAA`, `CNAME`, `MX`, `TXT`, `SRV`, `PTR`.
- `ttl` (Number) — The TTL in seconds. Must be <= 65535.
- `weight` (Number) — The weight for SRV records. Must be >= 0.
- `site` (String) — The site to associate the DNS record with. Defaults to the provider site. Changing this forces a new resource.

### Read-Only

- `id` (String) — The ID of the DNS record.

## Import

DNS records can be imported using the record ID:

```shell
terraform import terrifi_dns_record.web <id>
```

To import a record from a non-default site, use the `site:id` format:

```shell
terraform import terrifi_dns_record.web <site>:<id>
```
