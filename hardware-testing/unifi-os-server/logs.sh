#!/usr/bin/env bash
#
# logs.sh — Tail UOS Server logs.
#
# Usage:
#   ./logs.sh              # tail server.log (UniFi Network Application)
#   ./logs.sh mongod       # tail mongod.log
#   ./logs.sh server       # tail server.log (explicit)
#
# Logs are stored inside the podman container at /usr/lib/unifi/logs/.
# This script runs `podman exec` as the uosserver user to access them.
# install.sh sets up a sudoers rule so no password prompt is needed.
#
# Remote usage:
#   ssh terrifi-unifi-os-server './logs.sh'

set -euo pipefail

LOG_NAME="${1:-server}"
LOG_PATH="/usr/lib/unifi/logs/${LOG_NAME}.log"

echo "Tailing ${LOG_PATH} inside uosserver container ..."
echo "(Ctrl+C to stop)"
echo ""

sudo su -s /bin/bash -l uosserver -c "podman exec uosserver tail -f ${LOG_PATH}"
