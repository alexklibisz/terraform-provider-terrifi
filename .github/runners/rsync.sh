#!/bin/bash
set -e
cd $(dirname $0)
rsync -av --delete --exclude '.env' . terrifi-gh-runner:/home/terrifi/github-runners/
