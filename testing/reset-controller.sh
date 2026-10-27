#!/usr/bin/env bash
# testing/reset-controller.sh
#
# Resets the UniFi Network application state on a running uosserver host —
# wipes the Network app's mongo database and configuration so the next start
# is equivalent to a fresh install. The UOS-level state (admin account, API
# keys, system settings) is preserved.
#
# Intended for between acceptance-test runs in CI: each TestAcc* lifecycle
# creates resources (firewall zones, policies, networks, …) and is supposed
# to destroy them, but a panic, timeout, or controller-side limit can leave
# orphans behind. Running reset-controller.sh restores a known-clean state
# without re-installing UOS.
#
# Sequence:
#   1. Stop uosserver; wait for the container to exit cleanly.
#   2. Wipe /home/uosserver/.local/share/containers/storage/volumes/
#      uosserver_var_lib_unifi/_data — the Network app's data volume
#      (mongo db, system.properties, keystore, model_lifecycles, etc.).
#   3. Start uosserver.
#   4. Wait for /api/system to respond, then for the Network app's legacy API.
#
# After this script returns, run testing/bootstrap.sh to obtain a fresh
# integration API key (the previous key references the wiped Network site).
#
# Requirements:
#   - Run as root on the host where uosserver is installed.
#
# Env overrides:
#   UOS_URL          (default https://127.0.0.1:11443)
#   UNIFI_DATA_DIR   (default the path above; override only if your container
#                      storage root is elsewhere)

set -euo pipefail

UOS_URL="${UOS_URL:-https://127.0.0.1:11443}"
UNIFI_DATA_DIR="${UNIFI_DATA_DIR:-/home/uosserver/.local/share/containers/storage/volumes/uosserver_var_lib_unifi/_data}"

log() { printf >&2 '[reset-controller] %s\n' "$*"; }

[ -d "$UNIFI_DATA_DIR" ] || { log "ERROR: $UNIFI_DATA_DIR not found — has uosserver been installed?"; exit 1; }

log "stopping uosserver"
systemctl stop uosserver

# Wait until the container is fully exited; podman state transitions through "stopping".
for i in $(seq 1 30); do
  state=$(sudo -iu uosserver podman ps -a --format '{{.State}}' --filter name=uosserver 2>/dev/null || echo "")
  case "$state" in
    exited|created|"") break ;;
  esac
  sleep 1
done

log "wiping Network app data volume ($UNIFI_DATA_DIR)"
# Use find -delete instead of rm -rf so we keep the volume directory itself
# (podman won't recreate it on restart if missing — only the contents are
# the volatile bit).
find "$UNIFI_DATA_DIR" -mindepth 1 -delete

log "starting uosserver"
systemctl start uosserver

log "waiting for /api/system"
deadline=$(( $(date +%s) + 300 ))
until curl -fsSk -o /dev/null "$UOS_URL/api/system"; do
  [ "$(date +%s)" -lt "$deadline" ] || { log "ERROR: /api/system unreachable after 5 min"; exit 1; }
  sleep 1
done

# The Network app behind /proxy/network takes a few extra seconds to warm up
# after /api/system returns 200. Wait until its legacy API is up too so
# bootstrap.sh's subsequent calls don't race.
log "waiting for Network app legacy API"
deadline=$(( $(date +%s) + 120 ))
while :; do
  http=$(curl -sk -o /dev/null -w '%{http_code}' "$UOS_URL/proxy/network/api/s/default/stat/device" || echo 0)
  case "$http" in
    200|401) break ;;
  esac
  [ "$(date +%s)" -lt "$deadline" ] || { log "ERROR: Network app legacy API didn't come up in 2 min (last: $http)"; exit 1; }
  sleep 2
done

log "ready — run testing/bootstrap.sh to obtain a fresh API key"
