import {
  to = terrifi_network.default
  id = "69a3d29a3f7ccf63eb24abba"
}

resource "terrifi_network" "default" {
  name         = "Default"
  purpose      = "corporate"
  subnet       = "192.168.1.1/24"
  dhcp_enabled = true
  dhcp_start   = "192.168.1.6"
  dhcp_stop    = "192.168.1.254"
}
