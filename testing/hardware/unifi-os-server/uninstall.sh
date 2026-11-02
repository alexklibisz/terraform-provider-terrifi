#!/usr/bin/env bash
#
# uninstall.sh — Completely remove UOS Server and all its data.
#
# This runs `uosserver-purge`, which stops the service, removes the podman
# container, deletes all data, and uninstalls the uosserver binary.

set -euo pipefail

if [[ $EUID -ne 0 ]]; then
  echo "ERROR: This script must be run as root (or with sudo)." >&2
  exit 1
fi

if ! command -v uosserver-purge &>/dev/null; then
  echo "ERROR: uosserver-purge not found. Is UOS Server installed?" >&2
  exit 1
fi

echo "This will completely remove UOS Server and ALL its data."
read -rp "Are you sure? [y/N] " confirm

if [[ "${confirm,,}" != "y" ]]; then
  echo "Aborted."
  exit 0
fi

echo "Purging UOS Server ..."
uosserver-purge

echo "UOS Server has been removed."
