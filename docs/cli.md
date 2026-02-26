---
page_title: "Terrifi CLI"
subcategory: ""
description: |-
  CLI tool for generating Terraform import blocks from a live UniFi controller.
---

# Terrifi CLI

The Terrifi CLI connects to a live UniFi controller and generates Terraform `import {}` and `resource {}` blocks, making it easy to bring existing infrastructure under Terraform management.

## Install

```sh
go install github.com/alexklibisz/terrifi/cmd/terrifi@latest
```

## Configuration

The CLI uses the same `UNIFI_*` environment variables as the [Terrifi provider](index.md):

- `UNIFI_API` — Controller URL, including the port.
- `UNIFI_API_KEY` OR `UNIFI_USERNAME` and `UNIFI_PASSWORD` — Authentication credentials.
- `UNIFI_INSECURE` — Set to `true` for self-signed TLS certificates.
- `UNIFI_SITE` — Site name (defaults to `default`).

## Commands

### check-connection

Verify that your environment variables are configured correctly:

```sh
terrifi check-connection
```

### generate-imports

Generate Terraform import blocks for a resource type:

```sh
terrifi generate-imports <resource_type>
```

Supported resource types:

| Resource Type | Description | Docs |
|---|---|---|
| `terrifi_client_device` | Client devices (aliases, fixed IPs, etc.) | [client_device](resources/client_device.md) |
| `terrifi_client_group` | Client groups | [client_group](resources/client_group.md) |
| `terrifi_dns_record` | DNS records | [dns_record](resources/dns_record.md) |
| `terrifi_firewall_zone` | Firewall zones | [firewall_zone](resources/firewall_zone.md) |
| `terrifi_firewall_policy` | Firewall policies | [firewall_policy](resources/firewall_policy.md) |
| `terrifi_network` | Networks | [network](resources/network.md) |
| `terrifi_wlan` | Wireless networks | [wlan](resources/wlan.md) |

### Example

```sh
terrifi generate-imports terrifi_dns_record > imports.tf
```

This produces output like:

```terraform
import {
  id = "abc123"
  to = terrifi_dns_record.web_example_com
}

resource "terrifi_dns_record" "web_example_com" {
  name        = "web.example.com"
  value       = "192.168.1.100"
  record_type = "A"
}
```

You can then run `terraform plan` to verify and `terraform apply` to complete the import.
