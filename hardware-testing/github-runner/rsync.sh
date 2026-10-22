#!/bin/bash
# Run on the workstation: ./rsync.sh
set -euo pipefail
cd -- "$(dirname -- "$0")"

# Fetch the HIL controller's self-signed certificate, which run.sh builds into
# the runner image so jobs verify TLS instead of setting UNIFI_INSECURE. Fetch
# it over Tailscale, which authenticates the controller VM, not over the HIL
# LAN, where an impersonator is what the verification protects against. The
# runner VM is not on the tailnet, so this runs here. Rerun this script and
# run.sh whenever the controller's certificate changes.
controller="${CONTROLLER_TAILNET_HOST:-terrifi-unifi-os-server}"
controller_ip="$(tailscale ip -4 "$controller")"
# openssl s_client has no connect timeout, so fail fast if the controller is
# unreachable instead of hanging. This only checks reachability; --insecure
# sends nothing secret. Bypass any proxy to test the route s_client takes.
if ! curl --silent --insecure --noproxy '*' --connect-timeout 5 --max-time 10 --output /dev/null \
  "https://$controller_ip:11443/"; then
  echo "Cannot reach $controller ($controller_ip) on port 11443 over Tailscale." >&2
  exit 1
fi
trap 'rm -f controller.crt.tmp' EXIT
# LibreSSL's s_client exits 1 after a successful handshake when stdin closes,
# so ignore its status. openssl x509 fails if no certificate arrived.
{ openssl s_client -connect "$controller_ip:11443" -servername unifi.local </dev/null 2>/dev/null || true; } \
  | openssl x509 -outform PEM >controller.crt.tmp
# The workflows connect to https://unifi.local:11443, so the certificate must
# name it in a DNS SAN. Go ignores the Common Name, which -checkhost would
# accept. Parse -text rather than using -ext, which LibreSSL (macOS's
# /usr/bin/openssl) lacks.
if ! openssl x509 -in controller.crt.tmp -noout -text \
  | awk '/X509v3 Subject Alternative Name/ { getline; print }' \
  | grep -qE '(^|[ ,])DNS:unifi\.local(,|$)'; then
  echo "The certificate from $controller has no DNS SAN for unifi.local." >&2
  exit 1
fi
mv controller.crt.tmp controller.crt
echo "Fetched the controller certificate from $controller ($controller_ip):"
openssl x509 -in controller.crt -noout -subject -enddate -fingerprint -sha256

rsync -av --delete --exclude '.env' --exclude 'controller.crt.tmp' ./ terrifi-gh-runner:github-runner/
