# terrifi

Yet another Terraform provider for UniFi

## Introduction

This is my attempt at making a working Terraform provider to manage my home UniFi network.

Full disclosure, much of this code is written by and with help from various AI coding agents (Claude code in particular).

Compared to existing UniFi providers ([paultyng](https://github.com/paultyng/terraform-provider-unifi), [filipowm](https://github.com/filipowm/terraform-provider-unifi), [ubiquiti-community](https://github.com/ubiquiti-community/terraform-provider-unifi)), Terrifi is a nearly-from-scratch implementation with a particular focus on extensive testing, including hardware-in-the-loop testing.

## Docs

- Usage docs: [docs/index.md](./docs/index.md)

## TODO

This project is very much a work-in-progress.
Here's a rough list of features that I'm looking to implement:

- [x] `terrifi_dns_record` resource working on real hardware.
- [ ] `terrifi_network` resource working on real hardware.
- [ ] `terrifi_client_device` resource working on real hardware.
- [ ] `terrifi_wifi` resource working on real hardware.
- [ ] `terrifi_zone` resource working on real hardware.
- [ ] `terrifi_policy` resource working on real hardware.
- [ ] End-to-end working with my home UniFi network.
- [ ] Publish to Terraform and Opentofu registries.

## Background

### Starting from scratch

There is some prior-art when it comes to unofficial Terraform providers for UniFi: [paultyng](https://github.com/paultyng/terraform-provider-unifi), [filipowm](https://github.com/filipowm/terraform-provider-unifi), [ubiquiti-community](https://github.com/ubiquiti-community/terraform-provider-unifi).
Rather than forking and fixing one of the existing providers, I decided to start from scratch.
Notably, I'm still using [go-unifi](https://github.com/ubiquiti-community/go-unifi) under the hood.

Maybe starting over is a dumb idea, but here is my reasoning:

- I tried to import my home network into each of the existing providers and found errors and issues with each of them.
- It seems the existing providers are either un-maintained or very sparsely maintained at this point. That's not to disparage the maintainers; we all have busy lives and other things to do. I just wanted to avoid the overhead of maintaining a fork with no real feedback on when it might get merged in.
- I want to place a particular focus on hardware-in-the-loop testing. So I've spun up a hardware-in-the-loop test environment with a UniFi Gateway Lite, a UniFi AC Pro, and a mini PC running the UniFi OS Server control plane.
- I just wanted to learn how to implement a Terraform provider. I've used Terraform for years, but have never had the opportunity to implement a provider.
