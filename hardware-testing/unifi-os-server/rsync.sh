#!/bin/bash
set -e
cd $(dirname $0)
rsync -av --delete --exclude '.env' docker-compose/ terrifi-unifi-os-server:/home/terrifi/docker-compose/
