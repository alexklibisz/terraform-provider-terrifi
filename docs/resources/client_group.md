---
page_title: "terrifi_client_group Resource - Terrifi"
subcategory: ""
description: |-
  Manages a client group on the UniFi controller.
---

# terrifi_client_group (Resource)

Manages a client group on the UniFi controller. Client groups can be referenced when assigning client devices.

## Example Usage

### Basic group

```terraform
resource "terrifi_client_group" "iot" {
  name = "IoT Devices"
}
```

### Group alongside client devices

```terraform
resource "terrifi_client_group" "iot" {
  name = "IoT Devices"
}

resource "terrifi_client_device" "thermostat" {
  mac  = "aa:bb:cc:dd:ee:01"
  name = "Thermostat"
}

resource "terrifi_client_device" "camera" {
  mac  = "aa:bb:cc:dd:ee:02"
  name = "Security Camera"
}
```

## Schema

### Required

- `name` (String) — The name of the client group. Must be 1-128 characters.

### Optional

- `site` (String) — The site to associate the client group with. Defaults to the provider site. Changing this forces a new resource.

### Read-Only

- `id` (String) — The ID of the client group.

## Import

Client groups can be imported using the group ID:

```shell
terraform import terrifi_client_group.iot <id>
```

To import a client group from a non-default site, use the `site:id` format:

```shell
terraform import terrifi_client_group.iot <site>:<id>
```
