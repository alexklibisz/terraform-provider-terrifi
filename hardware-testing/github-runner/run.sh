#!/bin/bash
set -euo pipefail
cd -- "$(dirname -- "$0")"
docker compose build --pull runner
docker compose up -d runner
