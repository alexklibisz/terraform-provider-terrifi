import {
  to = terrifi_client_device.proxmox
  id = "69a3d685ab7d119515e69f73"
}

resource "terrifi_client_device" "proxmox" {
  mac  = "7c:83:34:b8:ae:be"
  name = "terrifi-proxmox"
}

import {
  to = terrifi_client_device.unifi_os_server
  id = "69a3d685ab7d119515e69f74"
}

resource "terrifi_client_device" "unifi_os_server" {
  mac        = "bc:24:11:e5:77:9f"
  name       = "unifi-os-server"
  fixed_ip   = "192.168.1.3"
  network_id = terrifi_network.default.id
}

import {
  to = terrifi_client_device.gh_runner
  id = "6acbbda49b1a52571042cafb"
}

resource "terrifi_client_device" "gh_runner" {
  mac        = "bc:24:11:70:ec:fe"
  name       = "terrifi-gh-runner"
  fixed_ip   = "192.168.1.5"
  network_id = terrifi_network.default.id
}
