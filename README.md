# terrifi

Yet another Terraform provider for UniFi.

## Introduction

This is my attempt at making a working Terraform provider to manage my home UniFi network.

Full disclosure, much of this code is written by and with help from various AI coding agents (Claude code in particular).

Compared to existing UniFi providers ([paultyng](https://github.com/paultyng/terraform-provider-unifi), [filipowm](https://github.com/filipowm/terraform-provider-unifi), [ubiquiti-community](https://github.com/ubiquiti-community/terraform-provider-unifi)), Terrifi is a nearly-from-scratch implementation with a particular focus on extensive testing, including hardware-in-the-loop testing.

## Docs

- [OpenTofu Registry](https://search.opentofu.org/provider/alexklibisz/terrifi/latest)
- [Provider](./docs/index.md)
- [CLI](./docs/index.md#cli)
- [Blog Post - _Terrifi: a vibe-coded Terraform provider to manage UniFi networks with hardware-in-the-loop testing_](https://alexklibisz.com/2026/03/07/terrifi)