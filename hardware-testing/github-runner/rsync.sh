#!/bin/bash
set -euo pipefail
cd -- "$(dirname -- "$0")"
rsync -av --delete --exclude '.env' docker-compose/ run.sh terrifi-gh-runner:/home/terrifi/docker-compose/
