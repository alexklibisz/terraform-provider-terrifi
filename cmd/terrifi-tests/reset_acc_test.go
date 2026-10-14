package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alexklibisz/terrifi/internal/provider"
	"github.com/stretchr/testify/require"
)

// This test resets the whole HIL site, not just its fixture. Run it separately
// from the provider acceptance suite so their controller mutations cannot race.
func TestAccResetHIL(t *testing.T) {
	if os.Getenv("TF_ACC") != "1" || os.Getenv("TERRIFI_ACC_TARGET") != "hardware" {
		t.Skip("requires TF_ACC=1 and TERRIFI_ACC_TARGET=hardware")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	root, err := filepath.Abs("../..")
	require.NoError(t, err)
	dir := filepath.Join(root, "hardware-testing/terraform")
	cli := filepath.Join(t.TempDir(), "terrifi")
	build := exec.CommandContext(ctx, "go", "build", "-o", cli, "./cmd/terrifi")
	build.Dir = root
	out, err := build.CombinedOutput()
	require.NoError(t, err, string(out))
	tf := os.Getenv("TF_ACC_TERRAFORM_PATH")
	if tf == "" {
		tf = "tofu"
	}
	require.NoError(t, runReset(ctx, dir, tf, cli))
	baselineRunner := resetRunner{ctx: ctx, dir: dir, tf: tf}
	before, err := baselineRunner.command(tf, "show", "-json")
	require.NoError(t, err)
	var baseline resetState
	require.NoError(t, json.Unmarshal(before, &baseline))

	client, err := provider.NewClient(ctx, provider.ClientConfigFromEnv())
	require.NoError(t, err)
	zones, err := client.ListFirewallZone(ctx, client.Site)
	require.NoError(t, err)
	internalID := ""
	for _, zone := range zones {
		if zone.DefaultZone && zone.Name == "Internal" {
			internalID = zone.ID
		}
	}
	require.NotEmpty(t, internalID)
	t.Setenv("TF_VAR_internal_zone_id", internalID)
	fixtureDir := t.TempDir()
	for _, file := range []string{"providers.tf", "terraform.rc"} {
		data, err := os.ReadFile(filepath.Join(dir, file))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(fixtureDir, file), data, 0600))
	}
	fixture, err := os.ReadFile(filepath.Join(dir, "testdata/reset_fixture.tf"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(fixtureDir, "main.tf"), fixture, 0600))
	fixtureRunner := resetRunner{ctx: ctx, dir: fixtureDir, tf: tf}
	_, err = fixtureRunner.command(tf, "init", "-input=false")
	require.NoError(t, err)
	// Also clean up partial fixture creation if the create or assertions fail.
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cleanupCancel()
		require.NoError(t, runReset(cleanupCtx, dir, tf, cli))
	}()
	_, err = fixtureRunner.command(tf, "apply", "-input=false", "-auto-approve")
	require.NoError(t, err)
	seedJSON, err := fixtureRunner.command(tf, "show", "-json")
	require.NoError(t, err)
	var seed resetState
	require.NoError(t, json.Unmarshal(seedJSON, &seed))
	require.Len(t, seed.Values.RootModule.Resources, 10)
	require.NoError(t, runReset(ctx, dir, tf, cli))
	generated, err := baselineRunner.command(cli, "generate-imports", "all", "--exclude-default-zones")
	require.NoError(t, err)
	for _, resource := range seed.Values.RootModule.Resources {
		var id string
		require.NoError(t, json.Unmarshal(resource.Values["id"], &id))
		require.False(t, strings.Contains(string(generated), id), "fixture %s still exists", resource.Address)
	}
	after, err := baselineRunner.command(tf, "show", "-json")
	require.NoError(t, err)
	var restored resetState
	require.NoError(t, json.Unmarshal(after, &restored))
	require.Len(t, restored.Values.RootModule.Resources, len(baseline.Values.RootModule.Resources))
	for i, original := range baseline.Values.RootModule.Resources {
		require.Equal(t, original.Address, restored.Values.RootModule.Resources[i].Address)
		require.JSONEq(t, string(original.Values["id"]), string(restored.Values.RootModule.Resources[i].Values["id"]))
	}
}
