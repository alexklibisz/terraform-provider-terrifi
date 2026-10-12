package hilreset

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCaptureConfigPreservesBaselineAndLinksDependencies(t *testing.T) {
	var state State
	require.NoError(t, json.Unmarshal([]byte(`{"values":{"root_module":{"resources":[
		{"address":"terrifi_network.default","type":"terrifi_network","values":{"id":"lan"}},
		{"address":"terrifi_client_device.server","type":"terrifi_client_device","values":{"id":"old-server-id","mac":"AA:BB:CC:DD:EE:FF"}}
	]}}}`), &state))
	generated := []byte(`
import {
  to = terrifi_network.default
id = "lan"
}
resource "terrifi_network" "default" {
  name = "Default"
purpose = "corporate"
}
import {
  to = terrifi_client_device.server
id = "new-server-id"
}
resource "terrifi_client_device" "server" {
  mac = "aa:bb:cc:dd:ee:ff"
}
import {
  to = terrifi_device.ap
id = "11:22:33:44:55:66"
}
resource "terrifi_device" "ap" {
  mac = "11:22:33:44:55:66"
}
import {
  to = terrifi_network.extra
id = "extra-network"
}
resource "terrifi_network" "extra" {
  name = "Temporary"
purpose = "corporate"
}
import {
  to = terrifi_firewall_zone.extra
id = "extra-zone"
}
resource "terrifi_firewall_zone" "extra" {
  name = "Temporary"
network_ids = ["extra-network"]
}
import {
  to = terrifi_firewall_policy.extra
id = "extra-policy"
}
resource "terrifi_firewall_policy" "extra" {
  name = "Temporary"
action = "ALLOW"
source { zone_id = "extra-zone" }
destination { zone_id = "builtin-zone" }
}
`)
	baseline := map[string]bool{"terrifi_network.default": true, "terrifi_client_device.server": true}
	config, count, err := CaptureConfig(generated, state, baseline)
	require.NoError(t, err)
	require.Equal(t, 3, count)
	text := string(config)
	require.NotContains(t, text, "old-server-id")
	require.NotContains(t, text, "new-server-id")
	require.NotContains(t, text, "terrifi_device")
	require.Contains(t, text, "ignore_changes = all")
	require.Contains(t, text, "network_ids = [terrifi_network.hil_reset_")
	require.Contains(t, text, "zone_id = terrifi_firewall_zone.hil_reset_")
	require.Contains(t, text, `zone_id = "builtin-zone"`)
	// Stable addresses allow a capture interrupted after some imports to resume.
	again, _, err := CaptureConfig(generated, state, baseline)
	require.NoError(t, err)
	require.Equal(t, config, again)
}

func TestBaselineAddressesRefusesEmptyAndUnsupportedConfigurations(t *testing.T) {
	for _, config := range []string{"", `module "other" { source = "./other" }`, `resource "terrifi_network" "hil_reset_custom" {}`, `resource "terrifi_network" "lan" { count = 1 }`} {
		t.Run(config, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, "main.tf"), []byte(config), 0600))
			_, err := BaselineAddresses(dir)
			require.Error(t, err)
		})
	}
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.tf"), []byte(`resource "terrifi_network" "lan" {}`), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "other.tf.json"), []byte(`{}`), 0600))
	_, err := BaselineAddresses(dir)
	require.ErrorContains(t, err, ".tf.json")
}

func TestCaptureConfigRejectsMissingAndDuplicateImports(t *testing.T) {
	for _, generated := range []string{
		`resource "terrifi_dns_record" "missing" { name = "missing.invalid" }`,
		`import {
  to = terrifi_dns_record.first
  id = "same-id"
}
resource "terrifi_dns_record" "first" { name = "first.invalid" }
import {
  to = terrifi_dns_record.second
  id = "same-id"
}
resource "terrifi_dns_record" "second" { name = "second.invalid" }`,
	} {
		_, _, err := CaptureConfig([]byte(generated), State{}, map[string]bool{})
		require.Error(t, err)
	}
}
