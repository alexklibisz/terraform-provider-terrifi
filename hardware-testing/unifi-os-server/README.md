# UniFi OS Server

This directory contains scripts for installing [UniFi OS Server (UOS)](https://community.ui.com/) on a bare Ubuntu/Debian host for hardware-in-the-loop testing.

UOS Server is Ubiquiti's official self-hosted UniFi platform. It runs as a single podman container managed by a systemd service, bundling MongoDB, the UniFi Network Application, and RabbitMQ. Unlike the standalone Network Application (used by the LinuxServer Docker image), UOS Server supports the full UniFi feature set including zone-based firewall.

## Prerequisites

- Ubuntu or Debian server with systemd
- SSH access configured with alias `terrifi-unifi-os-server` (i.e., `ssh terrifi-unifi-os-server` works from your dev machine)
- Root/sudo access on the server

## Setup

1. Copy scripts to the server:
   ```sh
   ./rsync.sh
   ```

2. SSH in and run the installer:
   ```sh
   ssh terrifi-unifi-os-server
   sudo ./install.sh           # installs default version (5.0.6)
   sudo ./install.sh 4.3.6     # or specify a version
   ```

3. Once installed, go to `https://<hostname or IP>:<port>`. Unless you've provisioned a certificate with Lets Encrypt or similar, you will see a warning that the site is not secure. This is because it's using a self-signed certificate. If you're sure you're at the right hostname/IP, click through the warnings.

4. Complete the initial setup wizard in the web UI. When prompted to create an account, just create a local account.

5. Upgrade to the latest version. Go to `https://<hostname or IP>:<port>/network/default/settings/control-plane`. There's a table with columns Application, Status, etc. If an update is available it will show up under the Status tab.

6. Adopt any connected UniFi devices (gateways, access points, etc.). This typically requires resetting the devices.

7. Enable Firewall Zones. Go to `https://<hostname or IP>:<port>/network/default/settings/traffic-and-firewall-rules`. You'll see something like "Upgrade to the New Zone-Based Firewall". Click "Click to upgrade" and go through the upgrade steps.

8. Create an API Key. This is the key we'll use in the test suite. The exact steps vary by version:
    1. On version 9.x: Click the Settings gear icon on the left. Click Control Plane. Go to the Integrations tab. Use the form to create an API key.
    2. On version 10.1.x: Click the Integrations icon on the left (looks like an electrical plug). Click "Create new API key".

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

## Approaches Considered

We evaluated three approaches for running a UniFi controller for hardware-in-the-loop testing:

### 1. LinuxServer Docker Compose (previous approach)

A Docker Compose stack with the [LinuxServer unifi-network-application](https://github.com/linuxserver/docker-unifi-network-application) image, a separate MongoDB container, and an nginx stub to silence ULP log spam.

| Pros | Cons |
|------|------|
| Familiar Docker Compose workflow | Runs the **standalone Network Application**, not full UniFi OS |
| Easy to version-pin and reproduce | Missing features like zone-based firewall policies |
| Dependabot tracks image updates | Three containers to orchestrate (UniFi, MongoDB, ULP stub) |
| | Ubiquiti is deprecating the standalone app; LinuxServer will [retire the image](https://github.com/linuxserver/docker-unifi-network-application/issues/171) |

### 2. UniFi OS Server (current approach)

Ubiquiti's official [UOS Server](https://community.ui.com/) installer — a single binary that sets up a podman container managed by systemd.

| Pros | Cons |
|------|------|
| Full UniFi OS feature set (zone-based firewall, etc.) | Not containerized in the Docker Compose sense — installs system-wide |
| Single binary install, no orchestration needed | Opaque: Ubiquiti controls what's inside the podman container |
| Officially supported by Ubiquiti | Requires a dedicated VM or bare-metal host |
| Automatic systemd service management | Updates are manual (through the UI or re-running the installer) |
| Same API surface as real UniFi hardware | |

### 3. Real UniFi Hardware (e.g., Cloud Key, Dream Machine)

Run tests against an actual UniFi appliance on the network.

| Pros | Cons |
|------|------|
| Most realistic test environment | Expensive hardware required |
| Zero setup beyond initial device config | Can't easily reset/rebuild for clean test runs |
| Guaranteed API parity | Single point of failure if the device dies |
| | Harder to version-control the setup |

We chose **UOS Server** as the best balance: it provides the full UniFi OS API surface (unlike the LinuxServer image) while being easy to set up and tear down on a VM (unlike dedicated hardware).
