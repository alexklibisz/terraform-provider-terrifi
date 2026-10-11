#!/usr/bin/env bash
# testing/verify-devices.sh
#
# Asserts that the running uosserver has the expected number of adopted fake
# devices in simulation mode. Intended as a post-condition check after
# enable-simulation.sh — useful in CI to fail fast if the controller didn't
# fully synthesize the simulated UGW/USW/UAP.
#
# Reads UNIFI_API and UNIFI_API_KEY from the environment (set by bootstrap.sh
# or `eval "$(testing/bootstrap.sh)"`). Falls back to UOS_URL +
# admin/password if no key is set.
#
# Env overrides:
#   UNIFI_API / UOS_URL          (default https://127.0.0.1:11443)
#   UNIFI_API_KEY                 (preferred — use the bootstrap-issued key)
#   ADMIN_USER, ADMIN_PASS        (fallback when no API key)
#   EXPECTED_DEVICES              (default sum of NUM_UGW+NUM_USW+NUM_UAP, or 6
#                                   if those are unset — matches the default
#                                   1+2+3 split in enable-simulation.sh)
#
# Exit 0 with `UNIFI_FAKE_DEVICES=<n>` on stdout if all expected devices are
# present and adopted. Exit 1 with a diagnostic on stderr otherwise.

set -euo pipefail

UOS_URL="${UNIFI_API:-${UOS_URL:-https://127.0.0.1:11443}}"
API_KEY="${UNIFI_API_KEY:-}"
ADMIN_USER="${ADMIN_USER:-admin}"
ADMIN_PASS="${ADMIN_PASS:-TerrifiTest!2026}"

NUM_UGW="${NUM_UGW:-1}"
NUM_USW="${NUM_USW:-2}"
NUM_UAP="${NUM_UAP:-3}"
EXPECTED_DEVICES="${EXPECTED_DEVICES:-$(( NUM_UGW + NUM_USW + NUM_UAP ))}"

log() { printf >&2 '[verify-devices] %s\n' "$*"; }

if [ -n "$API_KEY" ]; then
  AUTH=(-H "X-API-KEY: $API_KEY")
else
  log "no UNIFI_API_KEY set — logging in as $ADMIN_USER"
  COOKIE=$(mktemp)
  trap 'rm -f "$COOKIE"' EXIT
  curl -fsSk -c "$COOKIE" -o /dev/null \
    -X POST -H 'Content-Type: application/json' \
    -d "$(jq -nc --arg u "$ADMIN_USER" --arg p "$ADMIN_PASS" '{username:$u,password:$p,rememberMe:false}')" \
    "$UOS_URL/api/auth/login"
  AUTH=(-b "$COOKIE")
fi

devices=$(curl -fsSk "${AUTH[@]}" "$UOS_URL/proxy/network/api/s/default/stat/device")
count=$(echo "$devices" | jq '.data | length')
adopted=$(echo "$devices" | jq '[.data[] | select(.adopted)] | length')

log "found $count device(s); $adopted adopted; expected $EXPECTED_DEVICES adopted"

if [ "$adopted" -lt "$EXPECTED_DEVICES" ]; then
  log "ERROR: only $adopted of $EXPECTED_DEVICES expected devices are adopted"
  echo "$devices" | jq -r '.data[] | "  \(.mac)  adopted=\(.adopted)  type=\(.type)  state=\(.state)"' >&2
  printf 'UNIFI_FAKE_DEVICES=%d\n' "$adopted"
  exit 1
fi

printf 'UNIFI_FAKE_DEVICES=%d\n' "$adopted"
