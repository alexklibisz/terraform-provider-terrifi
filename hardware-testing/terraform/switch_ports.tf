# TODO: Enable this configuration once switch port setup is released.
# Update the baseline provider pin and lock file, then uncomment and plan/apply.

# resource "terrifi_network" "switch_lab" {
#   name          = "Switch lab"
#   purpose       = "corporate"
#   vlan_id       = 20
#   subnet        = "192.168.20.1/24"
#   network_group = "LAN"
#   dhcp_enabled  = true
#   dhcp_start    = "192.168.20.10"
#   dhcp_stop     = "192.168.20.99"
# }
#
# import {
#   to = terrifi_device_ports.switch_flex_mini
#   id = "68:d7:9a:3a:79:ec"
# }
#
# resource "terrifi_device_ports" "switch_flex_mini" {
#   device_mac = terrifi_device.switch_flex_mini.mac
#
#   # Port 1 is the uplink. Ports 4 and 5 are reserved for acceptance tests.
#   ports = {
#     "2" = {
#       name              = "Default access"
#       native_network_id = terrifi_network.default.id
#       tagged_vlan_mode  = "block_all"
#     }
#     "3" = {
#       name              = "Lab VLAN 20 access"
#       native_network_id = terrifi_network.switch_lab.id
#       tagged_vlan_mode  = "block_all"
#     }
#   }
# }
