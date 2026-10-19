#!/bin/bash
set -euo pipefail
cd -- "$(dirname -- "$0")"

# Trust the HIL controller's self-signed certificate in the runner image, so
# jobs verify TLS instead of setting UNIFI_INSECURE. Fetch it over Tailscale,
# which authenticates the controller VM, not over the HIL LAN, where an
# impersonator is what the verification protects against. Rerun this script
# whenever the controller's certificate changes.
controller="${CONTROLLER_TAILNET_HOST:-terrifi-unifi-os-server}"
controller_ip="$(tailscale ip -4 "$controller")"
trap 'rm -f controller.crt.tmp' EXIT
openssl s_client -connect "$controller_ip:11443" -servername unifi.local </dev/null 2>/dev/null \
  | openssl x509 -outform PEM >controller.crt.tmp
# The workflows connect to https://unifi.local:11443, so the certificate must
# name it in a DNS SAN. Go ignores the Common Name, which -checkhost would
# accept.
if ! openssl x509 -in controller.crt.tmp -noout -ext subjectAltName \
  | grep -qE '(^|[ ,])DNS:unifi\.local(,|$)'; then
  echo "The certificate from $controller has no DNS SAN for unifi.local." >&2
  exit 1
fi
mv controller.crt.tmp controller.crt
echo "Trusting the controller certificate from $controller ($controller_ip):"
openssl x509 -in controller.crt -noout -subject -enddate -fingerprint -sha256

docker compose build --pull runner
# Remove the long-lived runner container from before the supervisor, if it
# is still around. This interrupts any job it is running.
docker compose stop --timeout 300 runner
docker compose rm --force runner
# Recreate the supervisor so it picks up changes to supervise.sh. This
# interrupts any running job. The next job's container uses the rebuilt image.
docker compose up -d --force-recreate supervisor
