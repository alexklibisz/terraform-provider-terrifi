---
page_title: "Validation Coverage - Terrifi"
subcategory: ""
description: |-
  Acceptance-test coverage for Simulated and Hardware-in-the-loop targets.
---

# Validation coverage

Each resource and data-source page includes a **Validation coverage** table immediately after its schema. Every row names one setting and one usage, with separate columns for the two acceptance-test targets.

| Target | Environment |
| --- | --- |
| Simulated | An ephemeral UniFi Network Application configured for simulation mode, run by `task test:acc`. |
| Hardware-in-the-loop | The HIL UniFi OS Server connected to real UniFi devices, run by `task test:acc:hardware`. |

A ✅ means the acceptance suite covers applying or reading that setting on the named target. Controller readback and Terraform lifecycle assertions are sufficient for this coverage; observing a physical effect is not required.

| Symbol | Meaning |
| :---: | --- |
| ✅ | Covered by acceptance tests on this target, or explicitly labeled manual verification. |
| ❌ | Not covered on this target: the usage has no acceptance test, or its test skips on this target. This does **not** mean the feature is unsupported or known to fail. Use ◻️ instead when a documented target limitation prevents testing. |
| ◻️ | Unavailable in the current target environment because of a documented capability limitation; not merely a missing test. |
| ⚠️ | Conditional coverage requiring additional opt-in configuration, described beneath the table. |

For example, setting PoE to Off and removing the setting to restore Auto are separate rows. Both are exercised by the profile configuration tests on the Simulated and Hardware-in-the-loop targets:

| Setting | Usage | Simulated | Hardware-in-the-loop |
| --- | --- | :---: | :---: |
| `poe_mode` | Set to `off` | ✅ | ✅ |
| `poe_mode` | Remove to restore `auto` | ✅ | ✅ |

## Scope and limitations

The tables describe the checked-in acceptance suites and their target restrictions. They are not a dashboard of the latest CI results or a guarantee for every UniFi model and firmware version. Follow each page's evidence links to inspect the tests and their skip conditions.

Tests check saved controller configuration and Terraform behavior such as updates, imports, defaults, and drift correction. A check applies only to the listed usage. Unit tests, schema validation, and mock HTTP tests alone do not earn a ✅ in either target column.

Zone-based firewall tests require the Hardware-in-the-loop target and skip on the Simulated target. WLAN and client-group configuration tests run on both targets. Device, device-port, and device data-source tests need adopted devices or suitable APs. The current Docker controller exposes none, despite its demo-device configuration, so those tests skip and are marked ◻️. Model limitations also matter: custom-profile assignment tests skip on the Hardware-in-the-loop target's USW Flex Mini, while profile CRUD and inline VLAN tests run there.

The [2026-10-30 Simulated CI run](https://github.com/alexklibisz/terraform-provider-terrifi/actions/runs/41088244055) records the missing-device skips with the pinned Network Application 10.6.109 image. Recheck these limitations when changing the image or simulator setup.

A skipped test provides no validation for that run. Optional tests that require additional environment variables are marked ⚠️. Each page explains relevant skips and conditions.

The tables list specific uses and known gaps rather than every possible value or operation. An unlisted usage has no coverage claim. Shared resource identity fields such as `id` and `site` are generally omitted.

See the [HIL setup](https://github.com/alexklibisz/terraform-provider-terrifi/blob/main/hardware-testing/README.md) for equipment details, and the [Docker CI](https://github.com/alexklibisz/terraform-provider-terrifi/actions/workflows/ci-local.yaml) and [HIL CI](https://github.com/alexklibisz/terraform-provider-terrifi/actions/workflows/ci-hil.yaml) runs for execution results.

## Simulation coverage audit

The simulation sweep covers all documented settings and usages with missing Simulated coverage. Client-group and WLAN lifecycle suites run on both targets. DNS AAAA, CNAME, MX, and TXT tests exercise creation, updates, and import; the network suite toggles internet access in both directions. PTR creation is rejected by both current acceptance controllers and is recorded by a rejection test. Device, radio, port-assignment, and device-lookup tests remain unavailable in the current simulator because it exposes no adopted devices. Zone-based firewall tests require an adopted gateway. These are limitations of the current target, not claims that future simulator versions cannot support the operations.

## Keeping coverage current

Place the Validation coverage section immediately after the complete Schema section, including nested schemas, and before Import when present. When adding or changing a setting, update its resource's coverage table and evidence links.

Use one attribute path and one concrete usage per row. Repeat the setting for distinct uses such as setting a value, changing it, removing it, or importing it; use full paths for nested attributes. Determine each column independently from the relevant tests and target-specific skips. Use ◻️ only with an explanation of the target limitation and supporting evidence; a hardware-only skip alone is insufficient. Describe gaps and conditions beneath the table.

For manual verification, link a recorded result and label it explicitly. Record the target, controller version, device model and firmware where relevant, verification date, and any limitations so readers can distinguish a one-time check from automated coverage.
