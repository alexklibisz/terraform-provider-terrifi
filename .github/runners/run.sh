#!/bin/bash
set -e
cd $(dirname $0)
ssh terrifi-gh-runner 'cd /home/terrifi/github-runners && docker compose up -d --build'
