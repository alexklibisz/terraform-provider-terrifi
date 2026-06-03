#!/usr/bin/env bash
# testing/install-uos-server.sh
#
# Downloads and installs a pinned UniFi OS Server release on a Debian-based
# Linux host (LXC, VM, or bare metal). Run as root. Idempotent: re-running
# when the requested version is already installed is a no-op.
#
# After install completes:
#   - systemd unit `uosserver.service` is active
#   - https://127.0.0.1:11443/api/system responds (after ~30 s warm-up)
#   - Run testing/bootstrap.sh next to provision admin + API key
#
# Updating to a newer release:
#   1. Hit https://fw-update.ubnt.com/api/firmware-latest?filter=eq~~product~~unifi-os-server
#      to get the latest Linux x64 entry — the JSON includes the download URL,
#      version string, and SHA256.
#   2. Update UOS_VERSION, UOS_URL, and UOS_SHA256 below.
#   3. Commit the change with the upstream changelog reference.

set -euo pipefail

# Pinned release — see "Updating" comment block above.
UOS_VERSION="${UOS_VERSION:-5.1.15}"
UOS_URL="${UOS_URL:-https://fw-download.ubnt.com/data/unifi-os-server/24e0-linux-x64-5.1.15-926621de-c9d7-48cd-8921-a0ff3eebd3f4.15-x64}"
UOS_SHA256="${UOS_SHA256:-04c8e401eb34330fe99d94f35aa351e0e0e97895f0d9a4b459ff34fb50cad2bb}"

INSTALLER=/tmp/uos-server-installer-${UOS_VERSION}
LOG=/tmp/uos-install-${UOS_VERSION}.log

log() { printf >&2 '[install-uos-server] %s\n' "$*"; }

[ "$(id -u)" = "0" ] || { log "ERROR: must run as root"; exit 1; }

# Normalize the environment to "running directly as root". The Ubiquiti
# installer keys its rootless-podman config path off the invoking user,
# which it derives from SUDO_USER and (as a fallback) LOGNAME/USER. When
# invoked via `sudo` from a non-root login (e.g. `sudo bash
# install-uos-server.sh` on a GitHub runner as the `runner` user), the
# installer's image-load step reads
# /home/<that-user>/.config/containers/storage.conf and fails with
# "permission denied" / "Broken pipe" (the uosserver user can't read
# another user's 0700 home). Pinning every "who am I" signal to root makes
# the install behave like a bare-root invocation, which is how it works on
# the dev host (pct exec runs as root with no login session). Confirmed via
# `grep -a` on the installer ELF that it references SUDO_USER and LOGNAME.
unset SUDO_USER SUDO_UID SUDO_GID
export HOME=/root
export USER=root
export LOGNAME=root
export XDG_CONFIG_HOME=/root/.config

# Skip if the target version is already installed.
if command -v uosserver >/dev/null 2>&1; then
  current=$(uosserver version 2>/dev/null | head -1 || echo "unknown")
  if [ "$current" = "$UOS_VERSION" ]; then
    log "UOS Server $UOS_VERSION already installed; nothing to do"
    exit 0
  fi
  log "found existing UOS Server $current; will replace with $UOS_VERSION"
  log "running uosserver-purge to remove the existing install"
  # uosserver-purge prompts y/N and has no flag to bypass; feed it `y` on stdin.
  echo y | uosserver-purge 2>&1 | tee -a "$LOG" || {
    log "WARN: uosserver-purge returned non-zero; continuing"
  }
fi

# Dependencies the installer assumes are present.
log "ensuring podman, slirp4netns, curl, jq are installed"
apt-get update -qq
apt-get install -y --no-install-recommends podman slirp4netns curl jq ca-certificates >/dev/null

# Download (with resume).
log "downloading $UOS_URL -> $INSTALLER"
curl -fsSL -o "$INSTALLER" -C - "$UOS_URL"

# Verify checksum.
log "verifying SHA256"
actual=$(sha256sum "$INSTALLER" | awk '{print $1}')
if [ "$actual" != "$UOS_SHA256" ]; then
  log "ERROR: SHA256 mismatch"
  log "  expected: $UOS_SHA256"
  log "  actual:   $actual"
  exit 1
fi

# Run the installer. Ubiquiti distributes a native ELF binary, not a shell
# script — invoke it directly with chmod +x. Feed `y` on stdin to accept
# any interactive prompts the installer might surface.
log "running installer (this takes 2-3 minutes; output goes to $LOG)"
chmod +x "$INSTALLER"
yes y | "$INSTALLER" 2>&1 | tee -a "$LOG"

# Confirm the service is up.
log "waiting for uosserver.service to be active"
deadline=$(( $(date +%s) + 120 ))
while ! systemctl is-active uosserver >/dev/null 2>&1; do
  [ "$(date +%s)" -lt "$deadline" ] || { log "ERROR: uosserver did not become active in 2 min"; exit 1; }
  sleep 2
done

log "waiting for https://127.0.0.1:11443/api/system to respond"
deadline=$(( $(date +%s) + 180 ))
until curl -fsSk -o /dev/null https://127.0.0.1:11443/api/system; do
  [ "$(date +%s)" -lt "$deadline" ] || { log "ERROR: API never responded"; exit 1; }
  sleep 2
done

installed=$(uosserver version 2>/dev/null | head -1)
log "UOS Server $installed installed and running"
log "next step: testing/bootstrap.sh to provision admin + API key"
