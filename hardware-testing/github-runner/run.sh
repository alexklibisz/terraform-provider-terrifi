#!/bin/bash
set -euo pipefail
cd -- "$(dirname -- "$0")"
docker compose build --pull runner
# supervise.sh creates each new container from the rebuilt image. Restarting
# also replaces the current container, waiting up to 5 minutes for a job.
sudo systemctl restart terrifi-gh-runner
