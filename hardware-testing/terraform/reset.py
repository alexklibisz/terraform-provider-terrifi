#!/usr/bin/env python3
"""Reset supported HIL resources through the state-aware Terrifi CLI."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile

PREFIX = "hil_reset_"
CAPTURE = "hil-reset.generated.tf"
REFERENCE_FIELDS = {
    "network_id", "network_override_id", "network_ids", "client_group_ids",
    "zone_id", "source_zone_id", "destination_zone_id", "policy_ids",
    "port_group_id", "device_ids",
}


def capture_config(blocks):
    # Device deletion is a no-op: adopted devices are managed by the baseline.
    blocks = [b for b in blocks if b["resource_type"] != "terrifi_device"]
    addresses = {}
    for block in blocks:
        key = (block["resource_type"], block["import_id"])
        if key in addresses:
            raise ValueError(f"duplicate controller object: {key}")
        name = PREFIX + hashlib.sha256(block["import_id"].encode()).hexdigest()[:16]
        addresses[key] = f'{block["resource_type"]}.{name}'
    refs = {identity: address + ".id" for (kind, identity), address in addresses.items()
            if kind != "terrifi_firewall_policy_order"}

    def expression(attr):
        # Only identity attributes need interpretation. Other values remain
        # the CLI's HCL expressions (no HCL parsing dependency in this script).
        if attr["key"] not in REFERENCE_FIELDS:
            return attr["value"]
        value = json.loads(attr["value"])
        def link(item):
            return refs.get(item, json.dumps(item))
        return "[" + ", ".join(map(link, value)) + "]" if isinstance(value, list) else link(value)

    result = []
    for block in blocks:
        address = addresses[(block["resource_type"], block["import_id"])]
        kind, name = address.split(".")
        result += ["import {", f"  to = {address}",
                   f'  id = {json.dumps(block["import_id"])}', "}", "",
                   f'resource "{kind}" "{name}" {{']
        for attr in block.get("attributes") or []:
            result.append(f'  {attr["key"]} = {expression(attr)}')
        for nested in block.get("blocks") or []:
            result.append(f'  {nested["name"]} {{')
            for attr in nested["attributes"]:
                result.append(f'    {attr["key"]} = {expression(attr)}')
            result.append("  }")
        result += ["  lifecycle {", "    ignore_changes = all", "  }", "}", ""]
    return "\n".join(result)


def baseline_addresses(plan):
    root = plan["configuration"]["root_module"]
    if root.get("module_calls"):
        raise ValueError("HIL baseline must use root resources")
    addresses = {r["address"] for r in root.get("resources", []) if r["mode"] == "managed"}
    if not addresses or any(a.split(".")[-1].startswith(PREFIX) for a in addresses):
        raise ValueError("HIL baseline is empty or uses the reserved cleanup prefix")
    return addresses


def validate_plan(plan, phase, baseline):
    for resource in plan.get("resource_changes", []):
        address, change = resource["address"], resource["change"]
        actions = change["actions"]
        if change.get("importing") and phase in ("delete", "verify"):
            raise ValueError(f"refusing {phase}: {address} would import")
        if actions == ["no-op"]:
            continue
        parts = address.split(".")
        cleanup = len(parts) == 2 and parts[1].startswith(PREFIX) and address not in baseline
        if phase == "baseline" and address in baseline and actions in (["create"], ["update"]):
            continue
        if phase in ("baseline", "delete") and cleanup and actions == ["delete"]:
            continue
        raise ValueError(f"refusing {phase}: {address} would {actions}")


def reset(directory, tf, cli):
    for key in ("TERRIFI_STATE_BUCKET", "AWS_ENDPOINT_URL_S3", "AWS_ACCESS_KEY_ID",
                "AWS_SECRET_ACCESS_KEY", "UNIFI_API", "UNIFI_API_KEY"):
        if not os.environ.get(key):
            raise ValueError(f"set {key} before resetting HIL")
    directory = Path(directory).resolve()
    env = dict(os.environ, TF_CLI_CONFIG_FILE=str(directory / "terraform.rc"))
    def command(binary, *args):
        return subprocess.run([binary, *args], cwd=directory, env=env,
                              check=True, stdout=subprocess.PIPE, text=True).stdout
    capture = directory / CAPTURE
    capture.unlink(missing_ok=True)  # Interrupted imports remain in remote state.
    try:
        with tempfile.TemporaryDirectory(prefix=".hil-reset-", dir=directory) as temp:
            def apply(phase, baseline=None):
                path = str(Path(temp) / (phase + ".tfplan"))
                command(tf, "plan", "-input=false", "-lock-timeout=5m", "-out=" + path)
                plan = json.loads(command(tf, "show", "-json", path))
                if baseline is None:
                    baseline = baseline_addresses(plan)
                validate_plan(plan, phase, baseline)
                changes = plan.get("resource_changes", [])
                imports = sum(bool(r["change"].get("importing")) for r in changes)
                updates = sum(r["change"]["actions"] != ["no-op"] for r in changes)
                print(f"HIL reset {phase}: {imports} imports, {updates} changes", flush=True)
                if imports or updates:
                    command(tf, "apply", "-input=false", path)
                return baseline

            command(tf, "init", "-input=false", "-lockfile=readonly",
                    "-backend-config=bucket=" + os.environ["TERRIFI_STATE_BUCKET"])
            baseline = apply("baseline")
            state = Path(temp) / "state.json"
            snapshot = command(tf, "show", "-json")
            tracked = {r["address"] for r in json.loads(snapshot)["values"]["root_module"]["resources"]}
            if not baseline <= tracked:
                raise ValueError("baseline resources are missing from state; refusing to capture")
            state.write_text(snapshot)
            blocks = json.loads(command(cli, "generate-imports", "all", "--exclude-state", str(state),
                                        "--exclude-default-zones", "--format=json"))
            config = capture_config(blocks)
            if config:
                # The temp directory and generated file may contain state secrets.
                with open(capture, "x", opener=lambda p, flags: os.open(p, flags, 0o600)) as file:
                    file.write(config)
                apply("capture", baseline)
                capture.unlink()
                apply("delete", baseline)
            apply("verify", baseline)
            print("HIL reset complete", flush=True)
    finally:
        capture.unlink(missing_ok=True)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--terraform", default="tofu")
    parser.add_argument("--terrifi", default="terrifi")
    args = parser.parse_args()
    reset(Path(__file__).parent, args.terraform, args.terrifi)
