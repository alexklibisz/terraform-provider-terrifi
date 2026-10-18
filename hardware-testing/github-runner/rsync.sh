#!/bin/bash
set -euo pipefail
cd -- "$(dirname -- "$0")"
# Copy into the runner user's home directory. No --delete: that would remove
# everything else in the home directory, including ~/.ssh and ~/.env.
rsync -av .dockerignore .env.example docker-compose.yaml Dockerfile goproxy.conf run.sh supervise.sh \
  terrifi-gh-runner:
