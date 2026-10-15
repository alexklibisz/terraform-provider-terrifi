#!/bin/bash
set -euo pipefail
cd -- "$(dirname -- "$0")"
docker compose build --pull runner
# Remove the long-lived runner container from before the supervisor, if it
# is still around, giving any job it is running 5 minutes to finish.
docker compose stop --timeout 300 runner
docker compose rm --force runner
# Recreate the supervisor so it picks up changes to supervise.sh. It waits
# for a running job, and the next job's container uses the rebuilt image.
docker compose up -d --force-recreate supervisor
