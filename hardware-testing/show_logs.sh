#!/usr/bin/env bash
set -euo pipefail

TIME=${1:-60s}

ssh terrifi-unifi-os-server "cd docker-compose && docker compose logs --timestamps --since ${TIME}"
