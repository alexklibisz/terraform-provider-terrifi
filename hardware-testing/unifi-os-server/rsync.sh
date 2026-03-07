#!/usr/bin/env bash
#
# rsync.sh — Copy scripts to the UOS Server host.
#
# Usage:
#   ./rsync.sh                              # default target: terrifi-unifi-os-server
#   ./rsync.sh user@192.168.1.3             # custom target

set -euo pipefail

cd "$(dirname "$0")"

TARGET="${1:-terrifi-unifi-os-server}"

rsync -av install.sh uninstall.sh logs.sh "${TARGET}:~/"
