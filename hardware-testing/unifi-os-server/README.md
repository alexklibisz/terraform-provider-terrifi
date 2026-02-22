# UniFi OS Server (Docker)

This directory contains a Docker Compose file for running a real instance of Unifi OS Server (i.e., the self-hosted Unifi control plane) for hardware-in-the-loop testing.

This is only really necessary if you intend to do hardware-in-the-loop testing.

## Setup

1. Create a server with Docker, Docker Compose, and rsync installed. I've only tested this setup on an Ubuntu Server VM running in Proxmox.
1. Setup ssh access with an alias `terrifi-unifi-os-server`, i.e., you should be able to ssh from your host (where the Terrifi source code resides) to the server via `ssh terrifi-unifi-os-server`.
1. Rsync the Docker Compose files over to the server: `./rsync.sh`.
1. SSH into the server.
1. Copy the `.env.example` file: `cp .env.example .env` and update the relevant properties for your setup.
1. Start the Docker Compose services: `docker compose up -d`

The UI will be running at `https://<host-ip-or-host-name>:8443`.

## Architecture

- **`unifi`** — [jacobalberty/unifi](https://hub.docker.com/r/jacobalberty/unifi) container with host networking so UniFi devices on the LAN can reach the controller directly.
- **`ulp-stub`** — tiny nginx that stubs `127.0.0.1:9080/api/ucore/manifest` to silence ULP log spam (UCore isn't present outside real UniFi OS hardware).

Data is persisted in Docker named volumes (`unifi-data`, `unifi-log`, `unifi-cert`).
