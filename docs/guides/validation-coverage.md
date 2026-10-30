---
page_title: "Validation Coverage - Terrifi"
subcategory: ""
description: |-
  How to interpret controller/API and hardware validation coverage.
---

# Validation coverage

Each resource and data-source page includes a **Validation coverage** table. The tables describe the scope of the checked-in acceptance tests, not the result of the latest CI run or a guarantee for every UniFi model and firmware version. Follow each page's evidence links to see the assertions and skip conditions.

| Symbol | Meaning |
| :---: | --- |
| ✅ | Covered by acceptance-test assertions for the listed settings or behavior, or by explicitly labeled manual verification. |
| ❌ | Not verified at this level. This does **not** mean unsupported or known to fail. |
| — | Not applicable: the row describes controller metadata or provider behavior with no direct physical effect to verify. |

## Two kinds of validation

**Controller/API** coverage checks configuration saved by the controller and read back through the provider or API. Depending on the linked tests, this includes creation, updates, imports, deletion, defaults, and drift correction. A check applies to the settings and cases listed; it does not imply that every allowed value or combination has been exercised. Unit tests, schema validation, and mock HTTP tests alone do not earn a ✅ in this column.

**Hardware behavior** coverage checks the effect on an actual device or connected client. Examples include querying gateway DNS, obtaining a DHCP lease, associating with an SSID, checking traffic isolation, and measuring PoE power delivery. Saving a firewall rule on a controller connected to a gateway does not establish that the gateway filters packets as intended.

The hardware-in-the-loop (HIL) target runs acceptance tests against a controller connected to real UniFi devices. Some APIs require adopted hardware even for configuration tests. A passing HIL run therefore establishes only what the tests assert; it does not automatically earn a hardware-behavior ✅.

The current documented suites assert controller/API behavior. Hardware-behavior checks remain ❌ where a physical effect is applicable. A skipped test provides no validation for that run; hardware availability, model capabilities, and optional environment variables can all cause skips.

See the [HIL setup](https://github.com/alexklibisz/terraform-provider-terrifi/blob/main/hardware-testing/README.md) for equipment and test-target details, and the [HIL workflow runs](https://github.com/alexklibisz/terraform-provider-terrifi/actions/workflows/ci-hil.yaml) for execution results.

## Keeping coverage current

Place the Validation coverage section immediately after the complete Schema section, including nested schemas, and before Import when present. When adding or changing a setting, update its resource's coverage table and evidence links. Group settings with the same coverage; split a row when only some settings are tested. Describe skip conditions and gaps beneath the table. Keep status symbols in these sections so attribute descriptions remain easy to scan.

To mark hardware behavior ✅, link the automated assertion or a recorded manual verification that observes the effect. Record the device model, firmware and controller versions, verification date, and any limitations. For manual verification, label it explicitly so readers can distinguish a one-time observation from ongoing automated coverage. A report of controller persistence alone belongs in the Controller/API column.
