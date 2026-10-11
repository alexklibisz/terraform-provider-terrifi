#!/usr/bin/env bash
# testing/bootstrap.sh
#
# Take a freshly-installed UniFi OS Server (uosserver.service active,
# /api/system reachable on https://localhost:11443) from "isSetup=false" to a
# working integration-API X-API-Key suitable for terrifi acceptance tests.
#
# Sequence:
#   1. Wait for /api/system to respond.
#   2. POST /api/setup with admin credentials (idempotent: tolerates "Device is
#      already setup" / "Setup already in progress" by polling deviceState).
#   3. Log in as that admin (cookie jar + CSRF token extracted from the JWT).
#   4. Create an integration API key on the admin account.
#   5. Verify the key works by calling /proxy/network/integration/v1/sites.
#   6. (Optional, ENABLE_FAKE_DEVICES=1) call enable-simulation.sh to adopt
#      simulated UGW/USW/UAP devices — unblocks tests that depend on adopted
#      hardware (firewall zones/policies, AP groups, port profiles, etc.).
#
# Output: prints `UNIFI_API=...`, `UNIFI_API_KEY=...`, `UNIFI_INSECURE=true` on
# stdout, ready for `eval "$(testing/bootstrap.sh)"` or `>> "$GITHUB_ENV"`. All
# diagnostics go to stderr.
#
# Env overrides:
#   UOS_URL       (default https://127.0.0.1:11443)
#   ADMIN_USER    (default admin)
#   ADMIN_PASS    (default TerrifiTest!2026)
#   DEVICE_NAME   (default terrifi-test)
#   TIMEZONE      (default UTC)
#   KEY_NAME      (default terrifi-test)
#   ENABLE_FAKE_DEVICES=1 — chain into enable-simulation.sh after key creation.

set -euo pipefail

UOS_URL="${UOS_URL:-https://127.0.0.1:11443}"
ADMIN_USER="${ADMIN_USER:-admin}"
ADMIN_PASS="${ADMIN_PASS:-TerrifiTest!2026}"
DEVICE_NAME="${DEVICE_NAME:-terrifi-test}"
TIMEZONE="${TIMEZONE:-UTC}"
KEY_NAME="${KEY_NAME:-terrifi-test}"

log() { printf >&2 '[bootstrap] %s\n' "$*"; }

# 1. Wait until /api/system responds.
log "Waiting for $UOS_URL/api/system..."
deadline=$(( $(date +%s) + 300 ))
until curl -fsSk -o /dev/null "$UOS_URL/api/system"; do
  [ "$(date +%s)" -lt "$deadline" ] || { log "ERROR: /api/system unreachable after 5 min"; exit 1; }
  sleep 1
done

IS_SETUP=$(curl -fsSk "$UOS_URL/api/system" | jq -r .isSetup)
log "isSetup=$IS_SETUP"

# 2. First-run setup. /api/setup is eventually-consistent: a fresh container
# can return HTTP 500 "Setup already in progress" or "Device is already setup"
# while it is still applying the credentials we just sent. Treat those as
# benign and poll deviceState until it reaches "setup". The login step (#3) is
# the real success check.
device_state=$(curl -fsSk "$UOS_URL/api/system" | jq -r .deviceState)
log "deviceState=$device_state"

if [ "$device_state" != "setup" ]; then
  log "Running first-run setup as $ADMIN_USER..."
  setup_body=$(jq -nc \
    --arg name "$DEVICE_NAME" \
    --arg tz   "$TIMEZONE" \
    --arg u    "$ADMIN_USER" \
    --arg p    "$ADMIN_PASS" \
    '{name:$name, timezone:$tz, optimizeNetwork:false, sendDiagnostics:false,
      newAccount:true, advancedSetup:{mode:"dhcp"},
      username:$u, password:$p}')
  setup_resp=$(mktemp)
  http=$(curl -sk -o "$setup_resp" -w '%{http_code}' \
    -X POST -H 'Content-Type: application/json' \
    -d "$setup_body" "$UOS_URL/api/setup")
  body=$(cat "$setup_resp"); rm -f "$setup_resp"
  case "$http" in
    200) ;;
    500)
      case "$body" in
        *"Setup already in progress"*|*"Device is already setup"*)
          log "Setup POST returned $http (transient: $body) — polling for completion"
          ;;
        *)
          log "Setup POST returned $http; body: $body"; exit 1 ;;
      esac ;;
    *) log "Setup POST returned $http; body: $body"; exit 1 ;;
  esac

  log "Waiting for deviceState=setup..."
  deadline=$(( $(date +%s) + 120 ))
  while :; do
    device_state=$(curl -fsSk "$UOS_URL/api/system" | jq -r .deviceState)
    [ "$device_state" = "setup" ] && break
    [ "$(date +%s)" -lt "$deadline" ] || { log "ERROR: deviceState stuck at '$device_state'"; exit 1; }
    sleep 2
  done
  log "deviceState=setup reached"
fi

# 3. Login → cookie jar + CSRF token (extracted from the JWT TOKEN payload).
COOKIE_JAR=$(mktemp)
trap 'rm -f "$COOKIE_JAR"' EXIT
log "Logging in as $ADMIN_USER..."
login_resp=$(curl -sk -c "$COOKIE_JAR" \
  -X POST -H 'Content-Type: application/json' \
  -d "$(jq -nc --arg u "$ADMIN_USER" --arg p "$ADMIN_PASS" \
        '{username:$u, password:$p, rememberMe:false}')" \
  "$UOS_URL/api/auth/login")
USER_ID=$(echo "$login_resp" | jq -r .unique_id)
[ -n "$USER_ID" ] && [ "$USER_ID" != "null" ] || { log "ERROR: login failed: $login_resp"; exit 1; }

TOKEN=$(awk '/^#HttpOnly_.*TOKEN/ {print $NF}' "$COOKIE_JAR")
PAYLOAD=$(printf '%s' "$TOKEN" | cut -d. -f2 | tr '_-' '/+')
while [ $((${#PAYLOAD} % 4)) -ne 0 ]; do PAYLOAD="${PAYLOAD}="; done
CSRF=$(printf '%s' "$PAYLOAD" | base64 -d 2>/dev/null | jq -r .csrfToken)
[ -n "$CSRF" ] && [ "$CSRF" != "null" ] || { log "ERROR: could not extract CSRF token"; exit 1; }

# 4. Create an integration API key on the owner account.
log "Creating API key '$KEY_NAME' on user $USER_ID..."
key_resp=$(curl -sk -b "$COOKIE_JAR" \
  -X POST -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $CSRF" \
  -d "$(jq -nc --arg n "$KEY_NAME" '{name:$n}')" \
  "$UOS_URL/proxy/users/api/v2/user/$USER_ID/keys")
API_KEY=$(echo "$key_resp" | jq -r '.data.full_api_key // empty')
[ -n "$API_KEY" ] || { log "ERROR: key creation failed: $key_resp"; exit 1; }

# 5. Verify the key works. The integration API endpoint can return non-JSON
# error pages for several seconds after a fresh setup or a state reset, so
# retry until we get a parseable response (or give up after ~30s).
log "Verifying integration API with new key..."
deadline=$(( $(date +%s) + 30 ))
sites=""
while :; do
  resp=$(curl -fsSk -H "X-API-KEY: $API_KEY" "$UOS_URL/proxy/network/integration/v1/sites" 2>/dev/null || echo "")
  sites=$(printf '%s' "$resp" | jq -r '.totalCount // empty' 2>/dev/null || true)
  [ -n "$sites" ] && break
  [ "$(date +%s)" -lt "$deadline" ] || { log "WARN: integration API never returned parseable JSON in 30s; key may still be valid"; sites="?"; break; }
  sleep 2
done
log "Integration API reachable with new key; site count=$sites"

# 6. (Optional) Synthesize fake devices via the Network app's simulation mode.
# Gated by ENABLE_FAKE_DEVICES=1 so the default zero-device behaviour is
# preserved. Without it, tests that don't need an adopted gateway still work
# (DNS records, network configs, firewall groups, etc.). With it, the
# firewall_zone, firewall_policy, and AP-group tests become runnable.
if [ "${ENABLE_FAKE_DEVICES:-0}" = "1" ]; then
  here=$(dirname "$0")
  log "ENABLE_FAKE_DEVICES=1 — running enable-simulation.sh"
  "$here/enable-simulation.sh" >&2
fi

# 7. Machine-readable output on stdout.
printf 'UNIFI_API=%s\n'      "$UOS_URL"
printf 'UNIFI_API_KEY=%s\n'  "$API_KEY"
printf 'UNIFI_INSECURE=%s\n' "true"
