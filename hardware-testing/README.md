# Hardware Testing

![Image of the hardware-in-the-loop testing setup](hardware.png)

## Overview

Terrifi makes use of hardware testing (aka hardware-in-the-loop testing or HIL testing).

The continuous integration suite has a series of acceptance tests which run againt two targets:

1. Simulation: the [self-hosted UniFi OS Server](https://help.ui.com/hc/en-us/articles/34210126298775-Self-Hosting-UniFi) running in an ephemeral Docker container ([jacobalberty/unifi](https://hub.docker.com/r/jacobalberty/unifi)) with _no_ UniFi hardware attached.
2. Hardware-in-the-loop (HIL): the same self-hosted UniFi OS Server running in the same Docker container, but it's running on a VM that's attached to a real UniFi network with real UniFi hardware.

The simulation mode exposes some but not all functionality of a real UniFi network.
The HIL mode is a literal UniFi network with literal UniFi hardware.
So we can test (almost) all the funcitonality of the connected hardware.
The only functionality we can't test is some aspects of the initial setup (e.g., adopting devices), as I haven't found a way to script or automate this yet.

## Background

To run a UniFi network, you need to run the UniFi server (essentially a controlplane for the network).
As far as I know, there are three options for running the server:

1. Some of the higher-end UniFi hardware includes the server. This is similar to many general-purpose routers.
2. UniFi has a hosted offering, i.e., they run it as a subscription.
3. UniFi publishes the server as a self-hostable application. Some nice community members have packaged this up in a way that's easy to deploy, e.g., [the jacobalberty/unifi Docker image](https://hub.docker.com/r/jacobalberty/unifi) and [the UniFi OS Server Installation scripts](https://community.ui.com/questions/UniFi-OS-Server-Installation-Scripts-or-UniFi-Network-Application-Installation-Scripts-or-UniFi-Eas/ccbc7530-dd61-40a7-82ec-22b17f027776).

## Setup

This section describes the hardware and software that I've deployed to support hardware testing.

### Hardware

1. A Gl.iNet Opal travel router. I use this to connect the HIL setup to my home WiFi. It's analogous to an ISP modem in a typical home network.
2. A UniFi Gateway Lite, purchased for the purpose of this project. I also happen to use a Gateway Lite for my home network.
3. A generic gigabit 5-port switch. This is analogous to an unmanaged switch in a typical network.
4. A UniFi AC Pro access point, purchased for the purpose of this project. I use some newer access points in my actual network, but this is good enough for testing.
5. A Beelink Mini PC running Proxmox. I run two VMs here: one for the self-hosted UniFi OS Server and one for the Github Actions runner that runs the HIL test suite.

### Software