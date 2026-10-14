import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

from reset import baseline_addresses, capture_config, reset, validate_plan


class ResetTests(unittest.TestCase):
    def test_capture_links_dependencies_and_ignores_changes(self):
        blocks = [
            {"resource_type": "terrifi_device", "import_id": "aa:bb:cc:dd:ee:ff"},
            {"resource_type": "terrifi_network", "import_id": "network", "attributes": [
                {"key": "name", "value": '"Temporary"'}, {"key": "purpose", "value": '"corporate"'}]},
            {"resource_type": "terrifi_firewall_zone", "import_id": "zone", "attributes": [
                {"key": "name", "value": '"Temporary"'}, {"key": "network_ids", "value": '["network"]'}]},
            {"resource_type": "terrifi_firewall_policy", "import_id": "policy", "blocks": [
                {"name": "source", "attributes": [{"key": "zone_id", "value": '"zone"'}]},
                {"name": "destination", "attributes": [{"key": "zone_id", "value": '"builtin-zone"'}]}]},
        ]
        config = capture_config(blocks)
        self.assertNotIn("terrifi_device", config)
        self.assertIn("network_ids = [terrifi_network.hil_reset_", config)
        self.assertIn("zone_id = terrifi_firewall_zone.hil_reset_", config)
        self.assertIn('zone_id = "builtin-zone"', config)
        self.assertEqual(3, config.count("ignore_changes = all"))
        self.assertEqual(config, capture_config(blocks))
        with self.assertRaises(ValueError):
            capture_config([blocks[1], blocks[1]])

    def test_plan_guards(self):
        baseline = {"terrifi_network.default"}
        for phase, address, actions, allowed in [
            ("baseline", "terrifi_network.default", ["update"], True),
            ("baseline", "terrifi_network.default", ["delete"], False),
            ("baseline", "terrifi_network.default", ["delete", "create"], False),
            ("baseline", "terrifi_dns_record.hil_reset_retry", ["delete"], True),
            ("capture", "terrifi_dns_record.hil_reset_new", ["no-op"], True),
            ("capture", "terrifi_dns_record.hil_reset_new", ["update"], False),
            ("capture", "terrifi_dns_record.hil_reset_new", ["create"], False),
            ("delete", "terrifi_dns_record.hil_reset_new", ["delete"], True),
            ("delete", "terrifi_network.default", ["delete"], False),
            ("delete", "terrifi_firewall_zone.internal", ["delete"], False),
            ("verify", "terrifi_network.default", ["update"], False),
        ]:
            with self.subTest(phase=phase, address=address, actions=actions):
                plan = {"resource_changes": [{"address": address, "change": {"actions": actions}}]}
                if allowed:
                    validate_plan(plan, phase, baseline)
                else:
                    with self.assertRaises(ValueError):
                        validate_plan(plan, phase, baseline)
        with self.assertRaises(ValueError):
            validate_plan({"resource_changes": [{"address": "terrifi_network.default", "change": {
                "actions": ["no-op"], "importing": {"id": "lan"}}}]}, "verify", baseline)

    def test_refuse_empty_or_reserved_baseline(self):
        for root in ({}, {"module_calls": {"lab": {}}}, {"resources": [
                {"mode": "managed", "address": "terrifi_network.hil_reset_custom"}]}):
            with self.assertRaises(ValueError):
                baseline_addresses({"configuration": {"root_module": root}})


@unittest.skipUnless(os.environ.get("TF_ACC") == "1" and os.environ.get("TERRIFI_ACC_TARGET") == "hardware",
                     "requires exclusive HIL access, TF_ACC=1, and TERRIFI_ACC_TARGET=hardware")
class HardwareResetTests(unittest.TestCase):
    def test_reset_fixture_and_preserve_baseline(self):
        directory = Path(__file__).resolve().parent
        tf = os.environ.get("TF_ACC_TERRAFORM_PATH", "tofu")
        with tempfile.TemporaryDirectory() as temp:
            temp = Path(temp)
            cli = str(temp / "terrifi")
            subprocess.run(["go", "build", "-o", cli, "./cmd/terrifi"], cwd=directory.parent.parent, check=True)
            reset(directory, tf, cli)

            def command(cwd, binary, *args, **extra_env):
                env = dict(os.environ, TF_CLI_CONFIG_FILE=str(cwd / "terraform.rc"), **extra_env)
                return subprocess.check_output([binary, *args], cwd=cwd, env=env, text=True)

            def state(cwd):
                return json.loads(command(cwd, tf, "show", "-json"))["values"]["root_module"]["resources"]

            before = {r["address"]: r["values"]["id"] for r in state(directory)}
            zones = json.loads(command(directory, cli, "generate-imports", "terrifi_firewall_zone", "--format=json"))
            internal = next(z["import_id"] for z in zones if z["resource_name"] == "internal")
            fixture = temp / "fixture"
            fixture.mkdir()
            for file in ("providers.tf", "terraform.rc"):
                shutil.copyfile(directory / file, fixture / file)
            shutil.copyfile(directory / "testdata/reset_fixture.tf", fixture / "main.tf")
            command(fixture, tf, "init", "-input=false")
            try:
                command(fixture, tf, "apply", "-input=false", "-auto-approve", TF_VAR_internal_zone_id=internal)
                seeds = state(fixture)
                self.assertEqual(10, len(seeds))
                reset(directory, tf, cli)
                generated = json.loads(command(directory, cli, "generate-imports", "all", "--format=json"))
                remaining = {r["import_id"] for r in generated}
                for resource in seeds:
                    self.assertNotIn(resource["values"]["id"], remaining)
                self.assertEqual(before, {r["address"]: r["values"]["id"] for r in state(directory)})
            finally:
                reset(directory, tf, cli)  # Also handles partial fixture creation; verifies repeatability.
