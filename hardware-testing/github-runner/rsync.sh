#!/bin/bash
set -euo pipefail
cd -- "$(dirname -- "$0")"
rsync -av --delete --exclude '.env' --exclude 'controller.crt*' ./ terrifi-gh-runner:github-runner/
