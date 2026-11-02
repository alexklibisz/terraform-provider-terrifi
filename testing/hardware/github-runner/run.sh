#!/bin/bash
set -euo pipefail
cd -- "$(dirname -- "$0")"

# rsync.sh fetches the controller's certificate over Tailscale and syncs it
# here; the Dockerfile builds it into the image.
if [[ ! -f controller.crt ]]; then
  echo "controller.crt is missing. Run ./rsync.sh on the workstation first." >&2
  exit 1
fi

docker compose build --pull runner supervisor
# Remove the long-lived runner container from before the supervisor, if it
# is still around. This interrupts any job it is running.
docker compose stop --timeout 300 runner
docker compose rm --force runner
# Recreate the supervisor so it picks up changes to supervise.sh. This
# interrupts any running job. The next job's container uses the rebuilt image.
docker compose up -d --force-recreate supervisor
