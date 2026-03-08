# Hardware Testing

![Image of the hardware-in-the-loop testing setup](hardware.jpg)

## Overview

Terrifi uses hardware testing (aka hardware-in-the-loop testing or HIL testing) to ensure all features work with real UniFi hardware.

The continuous integration suite has a series of acceptance tests (aka integration tests) which run against two targets:

1. Simulation Target: the [self-hosted UniFi Network Application](https://ui.com/download/releases/network-server), running in an ephemeral Docker container ([linuxserver/unifi-network-application](https://docs.linuxserver.io/images/docker-unifi-network-application/)), using a simulation mode setting with _no_ UniFi hardware attached.
2. Hardware-in-the-loop (HIL) Target: the [self-hosted UniFi OS Server](https://help.ui.com/hc/en-us/articles/220066768-Updating-and-Installing-Self-Hosted-UniFi-Network-Servers-Linux), running on an Ubuntu virtual machine, connected to a real UniFi network with real UniFi hardware.

The simulation target exposes some but not all functionality of a real UniFi network.
For example, firewall zones, firewall policies, and WLANs all require real hardware to test.

The HIL mode is a real UniFi OS Server with real UniFi hardware.
So we can test almost all the functionality of the connected hardware.
The only functionality we can't test is some aspects of the initial setup, e.g., reseting and adopting devices.

## Background

To run a UniFi network, you need to run the UniFi server (essentially a controlplane for the network).

As far as I know, there are three options for running the server:

1. Some of the higher-end UniFi hardware includes the server. This is similar to many general-purpose routers.
2. UniFi has a hosted offering, i.e., they run it as a subscription.
3. UniFi offers a couple options for self-hosting the server, covered below.

The self-hosted server comes in two variants: UniFi Network Application and UniFi OS Server.

UniFi Network Application seems to be an older variant and seems to be slated for deprecation.
On startup it shows a notification about upgrading to UniFi OS Server.
As far as I can tell, this app can be Dockerized; that's what the [Linux Server image](https://docs.linuxserver.io/images/docker-unifi-network-application/) is doing.

UniFi OS Server is the newer variant.
Compared to Network Application, it adds some functionality, and also seems to be harder to Dockerize.
For example, I wasn't able to find a way to generate API Keys in the UniFi Network Application, but I can in UniFi OS Server.
I also didn't see an option to enable Zone-based firewalls in the Network Application, but I can in UniFi OS Server.
The way it's packaged is a bit atypical.
It seems that the installer contains an embedded Podman container (~800MB size), extracts the Podman container, and runs it on the host.

## Setup

This section describes the hardware and software that I've deployed to support hardware testing.

### Hardware

1. [A Gl.iNet Opal travel router](https://www.amazon.com/GL-iNet-GL-SFT1200-Secure-Travel-Router/dp/B09N72FMH5?th=1). I use this to connect the HIL setup to my home WiFi. It's analogous to an ISP modem in a typical home network. I did it this way so that the test harness is fully isolated from my actual UniFi newtwork, and so I can place the test harness in the corner of my office where I don't have an Ethernet connection.
2. [A UniFi Gateway Lite](https://www.amazon.com/Ubiquiti-Networks-Gateway-Lite-UXG-Lite/dp/B0CW2DZZ3Z). I purchased this specifically for this project. I also happen to use a Gateway Lite for my home network.
3. A generic gigabit 5-port switch. This is analogous to an unmanaged switch in a typical network.
4. [A UniFi AC Pro access point](https://store.ui.com/us/en/products/uap-ac-pro). I purchased it used on eBay specifically for this project. I use some newer access points in my actual network, but this is good enough for testing.
5. A Beelink Mini PC running Proxmox. I run two VMs here: one for the self-hosted UniFi OS Server and one for the Github Actions runner that runs the HIL test suite.

```mermaid
graph TD
    HomeWiFi["Home WiFi"]
    Opal["Gl.iNet Opal\n(travel router / ISP modem analog)"]
    GW["UniFi Gateway Lite\n(UXG-Lite)"]
    SW["Generic Gigabit\n5-Port Switch"]
    AP["UniFi AC Pro\n(access point)"]
    Beelink["Beelink Mini PC\n(Proxmox)"]
    VM1["VM: UniFi OS Server\n(self-hosted controller)"]
    VM2["VM: GitHub Actions Runner\n(HIL test suite)"]

    HomeWiFi -->|WiFi uplink| Opal
    Opal -->|WAN| GW
    GW -->|LAN| SW
    SW --> AP
    SW --> Beelink
    Beelink --> VM1
    Beelink --> VM2
```

### Software

The Beelink Mini PC runs [Proxmox](https://www.proxmox.com/) as the hypervisor, hosting two Ubuntu VMs:

1. **UniFi OS Server VM** — runs [UniFi OS Server](https://ui.com/download/releases/unifi-os-server), installed via [`unifi-os-server/install.sh`](./unifi-os-server/install.sh). UniFi OS Server is a single binary that downloads an embedded Podman container and registers it as a `systemd` service. It exposes the full UniFi API (including zone-based firewall) at `https://<host>:11443`.

2. **GitHub Actions Runner VM** — runs a self-hosted GitHub Actions runner via Docker Compose ([`github-runner/`](./github-runner/)). The runner uses the [myoung34/github-runner](https://github.com/myoung34/docker-github-actions-runner) base image and is ephemeral (clean workspace per job). It carries the labels `self-hosted` and `terrifi-hardware-test`, which the HIL CI workflow uses to target it specifically.

```mermaid
graph TD
    GH["GitHub Actions\n(CI workflow)"]
    Runner["GitHub Actions Runner VM\n(Ubuntu + Docker)"]
    UOS["UniFi OS Server VM\n(Ubuntu + Podman)"]
    Container["uosserver container\n(MongoDB + UniFi + RabbitMQ)"]
    GW["UniFi Gateway Lite"]
    AP["UniFi AC Pro"]

    GH -->|"dispatches job\n(terrifi-hardware-test label)"| Runner
    Runner -->|"runs acceptance tests\n(TERRIFI_ACC_TARGET=hardware)"| UOS
    UOS --> Container
    Container -->|"manages via UniFi API"| GW
    Container -->|"manages via UniFi API"| AP
```

See the subdirectory READMEs for setup instructions:
- [`unifi-os-server/`](./unifi-os-server/) — install and manage UOS Server
- [`github-runner/`](./github-runner/) — set up the self-hosted Actions runner