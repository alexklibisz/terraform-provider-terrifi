#!/usr/bin/env bash
#
# install.sh — Install Ubiquiti's UniFi OS Server on a bare Ubuntu/Debian host.
#
# This downloads the official UOS Server binary and runs it in non-interactive
# mode. The installer creates a podman container managed by a systemd service.
#
# Usage:
#   sudo ./install.sh              # install default version (5.0.6)
#   sudo ./install.sh 4.3.6        # install a specific version

set -euo pipefail

VERSION="${1:-5.0.6}"
PLATFORM="x64"
DOWNLOAD_URL="https://fw-download.ubnt.com/data/uos-server/${VERSION}/uosserver-installer-${VERSION}-${PLATFORM}.bin"

# ── Preflight ────────────────────────────────────────────────────────

if [[ $EUID -ne 0 ]]; then
  echo "ERROR: This script must be run as root (or with sudo)." >&2
  exit 1
fi

# ── Install podman ───────────────────────────────────────────────────

if ! command -v podman &>/dev/null; then
  echo "Installing podman ..."
  apt-get update -qq
  apt-get install -y podman
else
  echo "podman is already installed."
fi

# ── Download UOS Server installer ────────────────────────────────────

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

INSTALLER="${TMPDIR}/uosserver-installer.bin"

echo "Downloading UOS Server v${VERSION} (${PLATFORM}) ..."
echo "  URL: ${DOWNLOAD_URL}"
curl -fSL -o "$INSTALLER" "$DOWNLOAD_URL"
chmod +x "$INSTALLER"

# ── Run installer ────────────────────────────────────────────────────

echo "Running UOS Server installer (non-interactive) ..."
"$INSTALLER" --non-interactive

# ── Allow passwordless podman exec as uosserver ──────────────────────
# Rootless podman stores container state per-user, so interacting with
# the uosserver container requires switching to the uosserver user.
# This sudoers rule lets any user do that without a password prompt.

SUDOERS_FILE="/etc/sudoers.d/uosserver"
echo "ALL ALL=(ALL) NOPASSWD: /usr/bin/su -s /bin/bash -l uosserver -c podman *" > "$SUDOERS_FILE"
chmod 0440 "$SUDOERS_FILE"
echo "Installed sudoers rule: ${SUDOERS_FILE}"

# ── Wait for service to become healthy ───────────────────────────────

echo "Waiting for UOS Server to start ..."

MAX_ATTEMPTS=60
ATTEMPT=0

while [[ $ATTEMPT -lt $MAX_ATTEMPTS ]]; do
  ATTEMPT=$((ATTEMPT + 1))

  # Check if the web UI is responding
  HTTP_CODE=$(curl -sk -o /dev/null -w "%{http_code}" "https://localhost:11443" 2>/dev/null || echo "000")

  if [[ "$HTTP_CODE" == "200" || "$HTTP_CODE" == "302" ]]; then
    echo "UOS Server is up! (HTTP ${HTTP_CODE} after ${ATTEMPT} attempts)"
    echo ""
    echo "Web UI:    https://$(hostname -I | awk '{print $1}'):11443"
    echo "Legacy:    https://$(hostname -I | awk '{print $1}'):8443"
    echo ""
    echo "Manage with: uosserver start|stop|status|shell|version"
    exit 0
  fi

  if [[ $((ATTEMPT % 10)) -eq 0 ]]; then
    echo "  ... still waiting (attempt ${ATTEMPT}/${MAX_ATTEMPTS}, last HTTP ${HTTP_CODE})"
  fi

  sleep 5
done

echo "WARNING: UOS Server did not respond within $((MAX_ATTEMPTS * 5)) seconds."
echo "Check status with: uosserver status"
echo "Check logs with:   sudo su -s /bin/bash -l uosserver -c 'podman logs -f uosserver'"
exit 1
