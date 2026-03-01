# UniFi OS Server

This directory contains scripts for installing [UniFi OS Server (UOS)](https://community.ui.com/) on a bare Ubuntu/Debian host for hardware-in-the-loop testing.

UOS Server is Ubiquiti's official self-hosted UniFi platform. It runs as a single podman container managed by a systemd service, bundling MongoDB, the UniFi Network Application, and RabbitMQ. Unlike the standalone Network Application (used by the LinuxServer Docker image), UOS Server supports the full UniFi feature set including zone-based firewall.

## Prerequisites

- Ubuntu or Debian server with systemd
- SSH access configured with alias `terrifi-unifi-os-server` (i.e., `ssh terrifi-unifi-os-server` works from your dev machine)
- Root/sudo access on the server

## Setup

1. Copy `install.sh` to the server:
   ```sh
   scp install.sh terrifi-unifi-os-server:~/
   ```

2. SSH in and run the installer:
   ```sh
   ssh terrifi-unifi-os-server
   sudo ./install.sh           # installs default version (5.0.6)
   sudo ./install.sh 4.3.6     # or specify a version
   ```

3. Complete the initial setup wizard in the web UI. When prompted to create an account, just create a local account.

4. Create an API Key. Click the Settings gear icon on the left. Click Control Plane. Go to the Integrations tab. Use the form to create an API key.

## Access

| Port  | Description |
|-------|-------------|
| 11443 | UOS web UI (primary) |
| 8443  | Legacy UniFi Network Application port |

The UI is available at `https://<host>:11443`. The legacy port `8443` also works.

## Architecture

The UOS Server installer:
- Installs podman (if not already present)
- Downloads and runs the `uosserver` binary
- Creates a `uosserver` system user
- Starts a podman container (`uosserver`) running as that user
- Registers a `uosserver` systemd service for automatic start on boot

Inside the container: MongoDB, Java/UniFi Network Application, and RabbitMQ all run together.

## Management Commands

```sh
uosserver status          # check if running
uosserver start           # start the service
uosserver stop            # stop the service
uosserver version         # show installed version
uosserver shell           # open a shell inside the container
```

## Logs

Use the included `logs.sh` script:
```sh
./logs.sh                 # tail server.log (UniFi Network Application)
./logs.sh mongod          # tail mongod.log
```

Or access logs directly:
```sh
# Podman container logs (startup, systemd)
sudo su -s /bin/bash -l uosserver -c 'podman logs -f uosserver'

# Application logs inside the container
sudo su -s /bin/bash -l uosserver -c 'podman exec uosserver tail -f /usr/lib/unifi/logs/server.log'
```

Remote log viewing:
```sh
ssh terrifi-unifi-os-server './logs.sh'
```

## Reset / Uninstall

To completely remove UOS Server and all its data:
```sh
sudo ./uninstall.sh
```

This runs `uosserver-purge`, which stops the service, removes the container, deletes all data, and uninstalls the binaries. You can then re-run `install.sh` for a fresh start.

## Why not Docker Compose?

This directory previously used a Docker Compose setup with the [LinuxServer unifi-network-application](https://github.com/linuxserver/docker-unifi-network-application) image. We switched to UOS Server because:

1. The LinuxServer image runs the **standalone Network Application**, which lacks features present in full UniFi OS (notably zone-based firewall policies).
2. Ubiquiti is deprecating the standalone Network Application in favor of UOS. The LinuxServer maintainers plan to retire their image once standalone releases stop.
3. UOS Server is a single binary install with no Docker Compose orchestration needed — simpler to set up and maintain.
