#!/bin/bash
set -e
cd $(dirname $0)
rsync -av --delete --exclude '.env' docker-compose/ terrifi-gh-runner:/home/terrifi/docker-compose/
