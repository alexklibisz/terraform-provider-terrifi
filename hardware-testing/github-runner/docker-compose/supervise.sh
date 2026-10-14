#!/bin/bash
# Runs the runner as one fresh container per job.
#
# EPHEMERAL makes the runner exit after a single job, but Docker's restart
# policies restart that same container, so files a job writes (Go caches, the
# tool cache, ~/.config) would still be there for the next job. Approved fork
# PRs run code here, and later jobs run with the HIL state credentials, so
# instead this loop removes the container after every job and creates a new
# one from the image.
#
# Run it through terrifi-gh-runner.service, not by hand.
set -euo pipefail
cd -- "$(dirname -- "$0")"

# systemd sends SIGTERM to this script only (KillMode=mixed). Let a running
# job finish within the compose stop_grace_period, then exit.
stop() {
  docker compose down --timeout 300
  exit 0
}
trap stop TERM INT

while true; do
  # Start detached from this shell so `wait` returns when the trap fires.
  docker compose up --force-recreate --renew-anon-volumes \
    --abort-on-container-exit runner &
  wait $! || true
  docker compose rm --force --stop --volumes runner
  # Avoid a tight loop if the runner fails at startup, e.g. a bad token.
  sleep 5
done
