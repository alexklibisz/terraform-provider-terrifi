terraform {
  required_providers {
    terrifi = {
      source = "alexklibisz/terrifi"
    }
  }
}

provider "terrifi" {}

# Set via: export TF_VAR_wifi_passphrase="your-password"
variable "wifi_passphrase" {
  type      = string
  sensitive = true
}

# Create a network to associate with the WLAN.
resource "terrifi_network" "home" {
  name    = "Home"
  purpose = "corporate"
  vlan_id = 10
  subnet  = "192.168.10.1/24"
}

# Create a basic WPA2 WiFi network.
resource "terrifi_wlan" "home" {
  name       = "Home WiFi"
  passphrase = var.wifi_passphrase
  network_id = terrifi_network.home.id
}

# Create a hidden 5 GHz network with WPA3 transition mode.
resource "terrifi_wlan" "private" {
  name            = "Private 5G"
  passphrase      = var.wifi_passphrase
  network_id      = terrifi_network.home.id
  wifi_band       = "5g"
  hide_ssid       = true
  wpa3_support    = true
  wpa3_transition = true
}

output "wlan_id" {
  value = terrifi_wlan.home.id
}
