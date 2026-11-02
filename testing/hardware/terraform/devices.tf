import {
  to = terrifi_device.gateway_uxg_lite
  id = "9c:05:d6:36:b5:9c"
}

resource "terrifi_device" "gateway_uxg_lite" {
  mac  = "9c:05:d6:36:b5:9c"
  name = "Gateway (UXG Lite)"
}

import {
  to = terrifi_device.ap_ac_pro
  id = "74:83:c2:89:cd:39"
}

resource "terrifi_device" "ap_ac_pro" {
  mac  = "74:83:c2:89:cd:39"
  name = "AP (AC Pro)"

  config_network = {
    type    = "static"
    ip      = "192.168.1.2"
    netmask = "255.255.255.0"
    gateway = "192.168.1.1"
    dns1    = "192.168.1.1"
  }
}

import {
  to = terrifi_device.switch_flex_mini
  id = "68:d7:9a:3a:79:ec"
}

resource "terrifi_device" "switch_flex_mini" {
  mac  = "68:d7:9a:3a:79:ec"
  name = "Switch (Flex Mini)"

  config_network = {
    type    = "static"
    ip      = "192.168.1.4"
    netmask = "255.255.255.0"
    gateway = "192.168.1.1"
    dns1    = "192.168.1.1"
  }
}
