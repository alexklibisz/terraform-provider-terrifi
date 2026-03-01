---
page_title: "terrifi_wlan Resource - Terrifi"
subcategory: ""
description: |-
  Manages a WLAN (WiFi network) on the UniFi controller.
---

# terrifi_wlan (Resource)

Manages a WLAN (WiFi network) on the UniFi controller. Supports WPA2/WPA3 security, hidden SSIDs, and per-band configuration.

## Example Usage

### Basic WPA2 WiFi network

```terraform
# Set via: export TF_VAR_wifi_passphrase="your-password"
variable "wifi_passphrase" {
  type      = string
  sensitive = true
}

resource "terrifi_network" "main" {
  name    = "Main"
  purpose = "corporate"
}

resource "terrifi_wlan" "home" {
  name       = "Home WiFi"
  passphrase = var.wifi_passphrase
  network_id = terrifi_network.main.id
}
```

### 5 GHz only with hidden SSID

```terraform
resource "terrifi_wlan" "private" {
  name       = "Private 5G"
  passphrase = var.wifi_passphrase
  network_id = terrifi_network.main.id
  wifi_band  = "5g"
  hide_ssid  = true
}
```

### WPA3 transition mode

```terraform
resource "terrifi_wlan" "secure" {
  name            = "Secure WiFi"
  passphrase      = var.wifi_passphrase
  network_id      = terrifi_network.main.id
  wpa3_support    = true
  wpa3_transition = true
}
```

### Open guest network

```terraform
resource "terrifi_wlan" "guest" {
  name       = "Guest"
  network_id = terrifi_network.guest.id
  security   = "open"
}
```

### Disabled WLAN

```terraform
resource "terrifi_wlan" "maintenance" {
  name       = "Maintenance WiFi"
  passphrase = var.wifi_passphrase
  network_id = terrifi_network.main.id
  enabled    = false
}
```

## Schema

### Required

- `name` (String) — The SSID (network name) of the WLAN. Must be 1-32 characters.
- `network_id` (String) — The ID of the network to associate with this WLAN.

### Optional

- `enabled` (Boolean) — Whether the WLAN is enabled. Defaults to `true`.
- `passphrase` (String, Sensitive) — The WPA passphrase. Must be 8-255 characters. Required when `security` is `wpapsk`.
- `wifi_band` (String) — The WiFi band. Must be `2g`, `5g`, or `both`. Defaults to `both`.
- `security` (String) — The security protocol. Must be `open` or `wpapsk`. Defaults to `wpapsk`.
- `hide_ssid` (Boolean) — Whether to hide the SSID from broadcast. Defaults to `false`.
- `wpa_mode` (String) — The WPA mode. Must be `auto` or `wpa2`. Defaults to `wpa2`.
- `wpa3_support` (Boolean) — Whether to enable WPA3 support. Defaults to `false`.
- `wpa3_transition` (Boolean) — Whether to enable WPA3 transition mode (WPA2/WPA3 mixed). Defaults to `false`.
- `site` (String) — The site to associate the WLAN with. Defaults to the provider site. Changing this forces a new resource.

### Read-Only

- `id` (String) — The ID of the WLAN.

## Import

WLANs can be imported using the WLAN ID:

```shell
terraform import terrifi_wlan.home <id>
```

To import a WLAN from a non-default site, use the `site:id` format:

```shell
terraform import terrifi_wlan.home <site>:<id>
```

You can also use the [Terrifi CLI](../cli.md) to generate import blocks for all WLANs automatically:

```shell
terrifi generate-imports terrifi_wlan
```

~> **Note:** The `passphrase` attribute cannot be imported because the UniFi API does not return it. After import, set the passphrase in your configuration and run `terraform apply` to update it.
