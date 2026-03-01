# terrifi

Yet another Terraform provider for UniFi.

## Introduction

This is my attempt at making a working Terraform provider to manage my home UniFi network.

Full disclosure, much of this code is written by and with help from various AI coding agents (Claude code in particular).

Compared to existing UniFi providers ([paultyng](https://github.com/paultyng/terraform-provider-unifi), [filipowm](https://github.com/filipowm/terraform-provider-unifi), [ubiquiti-community](https://github.com/ubiquiti-community/terraform-provider-unifi)), Terrifi is a nearly-from-scratch implementation with a particular focus on extensive testing, including hardware-in-the-loop testing.

## Docs

- [OpenTofu Registry](https://search.opentofu.org/provider/alexklibisz/terrifi/latest)
- [Provider](./docs/index.md)
- [CLI](./docs/cli.md)
- Resources: [client_device](./docs/resources/client_device.md), [dns_record](./docs/resources/dns_record.md), [firewall_zone](./docs/resources/firewall_zone.md), [firewall_policy](./docs/resources/firewall_policy.md), [network](./docs/resources/network.md), [wlan](./docs/resources/wlan.md)

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

## Releasing

1. Go to the [Tag workflow](../../actions/workflows/tag.yml) in GitHub Actions.
2. Click "Run workflow", enter the version tag (e.g., `v0.1.0`), and run it.
3. The tag workflow creates and pushes the tag, which triggers the [Release workflow](../../actions/workflows/release.yml).
4. The release workflow builds binaries for linux/darwin (amd64/arm64) and publishes them as a GitHub Release.
