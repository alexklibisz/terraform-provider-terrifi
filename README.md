# terrifi

Yet another Terraform provider for UniFi

## Overview

This is my attempt at making a working Terraform provider to manage my home UniFi network.

Full disclosure, much of this code is written with the help of various coding agents.

**Why start from scratch?**

Rather than forking one of the existing providers ([paultyng](https://github.com/paultyng/terraform-provider-unifi), [filipowm](https://github.com/filipowm/terraform-provider-unifi), [ubiquiti-community](https://github.com/ubiquiti-community/terraform-provider-unifi)), I decided to start from scratch, though I'm still using [go-unifi](https://github.com/ubiquiti-community/go-unifi) under the hood.
I tried to import my home network into each of the existing providers and found errors and issues with each of them.
It also seems that they are all either un-maintained or very sparsely maintained at this point.
Plus, I just wanted to learn how to implement a Terraform provider.
I've used Terraform for years, but have never had the opportunity to implement a provider.
