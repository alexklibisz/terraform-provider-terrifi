---
page_title: "Validation Coverage - Terrifi"
subcategory: ""
description: |-
  Acceptance-test coverage on Docker and real UniFi controllers.
---

# Validation coverage

Each resource and data-source page includes a **Validation coverage** table immediately after its schema. Every row names one setting and one usage, with separate columns for the two acceptance-test targets.

| Target | Environment |
| --- | --- |
| Docker controller | An ephemeral UniFi Network Application in simulation mode with virtual devices, run by `task test:acc`. |
| Real controller | The HIL UniFi OS Server connected to real UniFi devices, run by `task test:acc:hardware`. |

A ✅ means the acceptance suite covers applying or reading that setting on the named target. Controller readback and Terraform lifecycle assertions are sufficient for this coverage; observing a physical effect is not required.

| Symbol | Meaning |
| :---: | --- |
| ✅ | Covered by acceptance tests on this target, or explicitly labeled manual verification. |
| ❌ | Not covered on this target: the usage has no acceptance test, or its test skips on this target. This does **not** mean the feature is unsupported or known to fail. |
| ⚠️ | Conditional coverage requiring additional opt-in configuration, described beneath the table. |

For example, setting PoE to Off and removing the setting to restore Auto are separate rows. Both are exercised by the profile configuration tests on Docker and real controllers:

| Setting | Usage | Docker controller | Real controller |
| --- | --- | :---: | :---: |
| `poe_mode` | Set to `off` | ✅ | ✅ |
| `poe_mode` | Remove to restore `auto` | ✅ | ✅ |

## Scope and limitations

The tables describe the checked-in acceptance suites and their target restrictions. They are not a dashboard of the latest CI results or a guarantee for every UniFi model and firmware version. Follow each page's evidence links to inspect the tests and their skip conditions.

Tests check saved controller configuration and Terraform behavior such as updates, imports, defaults, and drift correction. A check applies only to the listed usage. Unit tests, schema validation, and mock HTTP tests alone do not earn a ✅ in either target column.

Some suites, including WLAN and zone-based firewall tests, explicitly require the real-controller target and skip on Docker. Others need an adopted device or a suitable AP; Docker supplies simulated devices. Model limitations also matter: custom-profile assignment tests skip on the real-controller target's USW Flex Mini, while profile CRUD and inline VLAN tests run there.

A skipped test provides no validation for that run. Optional tests that require additional environment variables are marked ⚠️. Each page explains relevant skips and conditions.

The tables list specific uses and known gaps rather than every possible value or operation. An unlisted usage has no coverage claim. Shared resource identity fields such as `id` and `site` are generally omitted.

See the [HIL setup](https://github.com/alexklibisz/terraform-provider-terrifi/blob/main/hardware-testing/README.md) for equipment details, and the [Docker CI](https://github.com/alexklibisz/terraform-provider-terrifi/actions/workflows/ci-local.yaml) and [HIL CI](https://github.com/alexklibisz/terraform-provider-terrifi/actions/workflows/ci-hil.yaml) runs for execution results.

## Keeping coverage current

Place the Validation coverage section immediately after the complete Schema section, including nested schemas, and before Import when present. When adding or changing a setting, update its resource's coverage table and evidence links.

Use one attribute path and one concrete usage per row. Repeat the setting for distinct uses such as setting a value, changing it, removing it, or importing it; use full paths for nested attributes. Determine each column independently from the relevant tests and target-specific skips. Describe gaps and conditions beneath the table.

For manual verification, link a recorded result and label it explicitly. Record the target, controller version, device model and firmware where relevant, verification date, and any limitations so readers can distinguish a one-time check from automated coverage.
