#!/bin/bash
# Run on the runner VM, not on the workstation: ./prune.sh
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: ./prune.sh

Reclaim unused APT packages/cache, superseded runner images, other unused
Docker images and build cache older than 7 days, non-Compose stopped
containers/networks older than 7 days, archived journals (100 MB / 7 days),
rotated text logs older than 14 days, and expired temporary files according to
systemd-tmpfiles policy.

Preserves the current runner image, all Docker volumes (including the Go
module cache), Compose containers/networks, running containers, and active
text/container logs.
EOF
}

case "${1:-}" in
  '') ;;
  -h|--help) usage; exit 0 ;;
  *) usage >&2; exit 2 ;;
esac
if (( $# > 1 )); then
  usage >&2
  exit 2
fi

if [[ ! -r /etc/os-release ]]; then
  echo 'This script requires an Ubuntu VM.' >&2
  exit 1
fi
# shellcheck source=/dev/null
source /etc/os-release
if [[ "${ID:-}" != ubuntu ]]; then
  echo 'This script requires an Ubuntu VM.' >&2
  exit 1
fi
# Elevate once so package caches and system logs can be cleaned.
if (( EUID != 0 )); then
  exec sudo -- bash "$0" "$@"
fi

# Never accidentally prune a remote daemon selected by DOCKER_HOST or context.
docker_local() {
  docker --host unix:///var/run/docker.sock "$@"
}

docker_available=false
if command -v docker >/dev/null 2>&1; then
  docker_local info >/dev/null
  docker_available=true
fi

exec 9>/run/lock/terrifi-github-runner-prune.lock
flock -n 9 || { echo 'Another prune is already running.' >&2; exit 1; }

run() {
  printf '+ '
  printf '%q ' "$@"
  printf '\n'
  "$@"
}

report() {
  printf '\nDisk usage\n'
  df -h /
  df -i /
  du -xhd1 /var/lib /var/log /var/cache /usr /home /root
  journalctl --disk-usage
  if "$docker_available"; then
    docker_local system df
    # Read sizes only; Docker owns these files, so never truncate them directly.
    local id log_path
    while IFS= read -r id; do
      log_path=$(docker_local inspect --format '{{.LogPath}}' "$id")
      if [[ -n "$log_path" && -f "$log_path" ]]; then
        du -h -- "$log_path"
      fi
    done < <(docker_local ps -aq)
  fi
}

report
printf '\nPruning\n'

# Use APT's dependency bookkeeping, including its running-kernel protection.
# Do not remove arbitrary installed packages just because this is a thin VM.
run apt-get autoremove --purge -y
run apt-get clean

if "$docker_available"; then
  # Job containers are started with --rm, so this only catches leftovers.
  run docker_local container prune --force --filter until=168h --filter 'label!=com.docker.compose.project'
  # Each ./run.sh leaves the previous runner image untagged. Docker skips it
  # while a job container still uses it.
  run docker_local image prune --force
  # Keep the current runner image regardless of age: between jobs no container
  # uses it, and supervise.sh cannot pull it back from a registry.
  run docker_local image prune --all --force --filter until=168h \
    --filter 'label!=com.docker.compose.project=terrifi-github-runner'
  run docker_local builder prune --all --force --filter until=168h
  run docker_local network prune --force --filter until=168h --filter 'label!=com.docker.compose.project'
fi

# Rotate first so vacuuming can reclaim formerly active journal files.
run journalctl --rotate --vacuum-size=100M --vacuum-time=7d
if command -v logrotate >/dev/null 2>&1 && [[ -f /etc/logrotate.conf ]]; then
  # A broken host rotation config must not block unrelated cleanup below.
  if ! run logrotate /etc/logrotate.conf; then
    echo 'WARNING: logrotate failed; continuing cleanup of already-rotated logs and expired temporary files.' >&2
    echo 'Repair the host logrotate configuration; see the logrotate errors above.' >&2
  fi
fi
# Restrict deletion to rotated text logs; never delete active logs or journals.
run find /var/log -xdev -type f \( -name '*.log.[0-9]*' -o -name '*.gz' -o -name 'syslog.[0-9]*' \) -mtime +13 -print -delete
run systemd-tmpfiles --clean

report
