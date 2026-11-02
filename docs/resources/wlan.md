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
- `application` (String) — The application type. Must be `standard`, `hotspot`, or `iot`. `hotspot` enables guest behavior (captive portal); `iot` enables IoT-optimized behavior. Defaults to `standard`.
- `optimize_iot_connectivity` (Boolean) — Enable IoT-specific radio optimizations that improve connection reliability for IoT devices. Only meaningful when `application = "iot"`. Defaults to `false`. See [IoT band selection](#iot-band-selection).
- `site` (String) — The site to associate the WLAN with. Defaults to the provider site. Changing this forces a new resource.

### Read-Only

- `id` (String) — The ID of the WLAN.

## Validation coverage

✅ Covered · ❌ Not covered · ◻️ Unavailable on this target · ⚠️ Conditional. See [validation coverage](../guides/validation-coverage.md) for definitions and limitations.

| Setting | Usage | Simulated | Hardware-in-the-loop |
| --- | --- | :---: | :---: |
| `name` | Create an SSID | ✅ | ✅ |
| `name` | Rename the SSID | ✅ | ✅ |
| `network_id` | Assign the WLAN network | ✅ | ✅ |
| `network_id` | Change the WLAN network | ✅ | ✅ |
| `enabled` | Set to `false` | ✅ | ✅ |
| `enabled` | Re-enable with `true` | ✅ | ✅ |
| `passphrase` | Set a WPA passphrase | ✅ | ✅ |
| `passphrase` | Change the passphrase | ✅ | ✅ |
| `wifi_band` | Set to `2g` | ✅ | ✅ |
| `wifi_band` | Set to `5g` | ✅ | ✅ |
| `wifi_band` | Set to `both` | ✅ | ✅ |
| `security` | Set to `open` | ✅ | ✅ |
| `security` | Change from `open` to `wpapsk` | ✅ | ✅ |
| `security` | Change from `wpapsk` to `open` | ✅ | ✅ |
| `hide_ssid` | Set to `true` | ✅ | ✅ |
| `wpa_mode` | Set to `auto` | ✅ | ✅ |
| `wpa_mode` | Set to `wpa2` | ✅ | ✅ |
| `wpa3_support` | Set to `true` | ✅ | ✅ |
| `wpa3_transition` | Set to `true` | ✅ | ✅ |
| `application` | Omit to use `standard` | ✅ | ✅ |
| `application` | Set to `hotspot` | ✅ | ✅ |
| `application` | Set to `iot` | ✅ | ✅ |
| `optimize_iot_connectivity` | Enable for an IoT WLAN | ✅ | ✅ |
| `optimize_iot_connectivity` | Disable for an IoT WLAN | ✅ | ✅ |

WLAN acceptance tests run on both controller targets. Simulated coverage verifies saved WLAN configuration; it does not validate broadcasting an SSID or connecting wireless clients.

Evidence: [wlan_resource_test.go](https://github.com/alexklibisz/terraform-provider-terrifi/blob/main/internal/provider/wlan_resource_test.go).

## Import

WLANs can be imported using the WLAN ID:

```shell
terraform import terrifi_wlan.home <id>
```

To import a WLAN from a non-default site, use the `site:id` format:

```shell
terraform import terrifi_wlan.home <site>:<id>
```

You can also use the [Terrifi CLI](../index.md#cli) to generate import blocks for all WLANs automatically:

```shell
terrifi generate-imports terrifi_wlan
```

~> **Note:** The `passphrase` attribute cannot be imported because the UniFi API does not return it. After import, set the passphrase in your configuration and run `terraform apply` to update it.

## Additional Notes

### IoT band selection

The UniFi controller coerces iot WLANs (especially with `optimize_iot_connectivity = true`) to the 2.4 GHz band. Set `wifi_band = "2g"` explicitly when using `application = "iot"` to avoid inconsistent-plan errors.
