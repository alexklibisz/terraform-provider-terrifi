# HIL Terraform

This directory contains a Terraform project used to provision the HIL-testing hardware.

## Environment variables

Set these before running the Task commands:

| Variable | Value |
| --- | --- |
| `TERRIFI_STATE_BUCKET` | Private R2 bucket containing the Terraform state. |
| `AWS_ENDPOINT_URL_S3` | `https://<account-id>.r2.cloudflarestorage.com` |
| `AWS_ACCESS_KEY_ID` | R2 access key ID, scoped to the state bucket. |
| `AWS_SECRET_ACCESS_KEY` | Corresponding R2 secret access key. |
| `UNIFI_API` | Controller URL, e.g. `https://<hil-controller>:11443`. |
| `UNIFI_API_KEY` | UniFi controller API key. |

For a self-signed controller certificate, also set `UNIFI_INSECURE=true`.
`UNIFI_SITE` is optional and defaults to `default`.

You can copy `.env.example` to the ignored `.env`, fill in the values, and load
them with `source .env` from this directory. Task does not load `.env` automatically.
