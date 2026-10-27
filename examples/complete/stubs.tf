# examples/complete — coverage stubs.
#
# The reference network this example is modelled on describes resources
# terrifi does not implement yet. They are recorded here as commented blocks
# so the gap is visible alongside the working config and reviewers can see
# exactly what the reference network needs next. Each block names the upstream
# UniFi concept, the reference section it comes from, and the §5 resource-
# it. When terrifi grows one of these resources, lift its block out of here
# into main.tf (or firewall-zbf.tf).
#
# None of these are valid HCL yet — they are intentionally commented out.

# --- WAN (dual ISP) ----------------------------------------------- §5 R08 --
# Reference §WAN. No terrifi_wan resource (R08). The reference's tfacc-wan-isp1
# (primary) / tfacc-wan-isp2 (failover) cannot be expressed today.
#
# resource "terrifi_wan" "isp1" { ... }   # primary, DHCP dual-stack
# resource "terrifi_wan" "isp2" { ... }   # failover

# --- purpose=guest network ------------------------------------- §6 (schema) --
# Reference §Networks. terrifi_network.purpose only accepts corporate|vlan-only.
# tfacc-net-guest is modeled as corporate in main.tf; the controller's native
# guest-policy branch (captive portal, voucher hooks) is unmanaged. This is a
# schema gap on an existing resource (§6 / finding F-001), not a new resource.
#
# resource "terrifi_network" "guest_native" { purpose = "guest" ... }

# --- Port profiles ------------------------------------------------ §5 R06 --
# Reference §Port profiles (6 profiles). No terrifi_port_profile resource (R06).
#
# resource "terrifi_port_profile" "trunk_all"      { ... }
# resource "terrifi_port_profile" "ap_uplink"      { ... }
# resource "terrifi_port_profile" "access_trusted" { ... }
# resource "terrifi_port_profile" "access_iot"     { ... }
# resource "terrifi_port_profile" "access_guest"   { ... }
# resource "terrifi_port_profile" "dot1x"          { ... }   # 802.1X / RADIUS

# --- Port forwards / DNAT capture --------------------------------- §5 R01 --
# Reference §Firewall (DNAT capture) + §Inbound. No terrifi_port_forward
# resource (R01). Blocks the 4 tfacc-pf-dnat-* DNS/NTP captures and
# tfacc-pf-https. The reference also flags wan.interface="lan" as a likely
# provider gap (finding F-008).
#
# resource "terrifi_port_forward" "dnat_dns_iot"  { ... }
# resource "terrifi_port_forward" "https"         { ... }

# --- Static / traffic routes ------------------------------ §5 R02 / R07 --
# Reference §Routing. No terrifi_static_route (R02) / terrifi_traffic_route (R07).
#
# resource "terrifi_static_route"  "test_net"          { ... }
# resource "terrifi_traffic_route" "trusted_via_wan1"  { ... }

# --- RADIUS ------------------------------------------------ §5 R03 / R05 --
# Reference §RADIUS. No terrifi_radius_user (R03) / terrifi_radius_profile (R05).
# The 802.1X port profile above would bind to the profile.
#
# resource "terrifi_radius_profile" "radius"      { ... }
# resource "terrifi_radius_user"    "radius_user" { ... }

# --- Dynamic DNS -------------------------------------------------- §5 R04 --
# Reference §Dynamic DNS. No terrifi_dynamic_dns resource (R04).
#
# resource "terrifi_dynamic_dns" "cloudflare" { ... }
