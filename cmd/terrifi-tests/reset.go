package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/alexklibisz/terrifi/internal/generate"
	"github.com/spf13/cobra"
)

func resetCmd() *cobra.Command {
	var dir, tf, cli string
	cmd := &cobra.Command{
		Use: "reset", Short: "Restore the HIL baseline and delete extra controller resources through Terraform",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runReset(cmd.Context(), dir, tf, cli)
		},
	}
	cmd.Flags().StringVar(&dir, "terraform-dir", "hardware-testing/terraform", "Baseline Terraform directory")
	cmd.Flags().StringVar(&tf, "terraform", "tofu", "Terraform/OpenTofu executable")
	cmd.Flags().StringVar(&cli, "terrifi", "terrifi", "Terrifi CLI executable")
	return cmd
}

type resetRunner struct {
	ctx                context.Context
	dir, tf, cli, temp string
	baseline           map[string]bool
}

func runReset(ctx context.Context, dir, tf, cli string) error {
	for _, key := range []string{"TERRIFI_STATE_BUCKET", "AWS_ENDPOINT_URL_S3", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "UNIFI_API", "UNIFI_API_KEY"} {
		if os.Getenv(key) == "" {
			return fmt.Errorf("set %s before resetting HIL", key)
		}
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	temp, err := os.MkdirTemp(dir, ".hil-reset-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	r := resetRunner{ctx: ctx, dir: dir, tf: tf, cli: cli, temp: temp, baseline: nil}
	capture := filepath.Join(dir, captureFilename)
	// A failed capture is recoverable from remote state; never reuse stale config.
	if err := os.Remove(capture); err != nil && !os.IsNotExist(err) {
		return err
	}
	defer os.Remove(capture)
	if _, err := r.command(tf, "init", "-input=false", "-lockfile=readonly", "-backend-config=bucket="+os.Getenv("TERRIFI_STATE_BUCKET")); err != nil {
		return err
	}
	if err := r.applyPlan("baseline"); err != nil {
		return err
	}
	stateJSON, err := r.command(tf, "show", "-json")
	if err != nil {
		return err
	}
	var state resetState
	if err := json.Unmarshal(stateJSON, &state); err != nil {
		return err
	}
	tracked := map[string]bool{}
	for _, resource := range state.Values.RootModule.Resources {
		tracked[resource.Address] = true
	}
	for address := range r.baseline {
		if !tracked[address] {
			return fmt.Errorf("baseline resource %s is missing from state", address)
		}
	}
	statePath := filepath.Join(temp, "state.json")
	if err := os.WriteFile(statePath, stateJSON, 0600); err != nil {
		return err
	}
	generated, err := r.command(cli, "generate-imports", "all", "--exclude-state", statePath, "--exclude-default-zones", "--format=json")
	if err != nil {
		return err
	}
	var blocks []generate.ResourceBlock
	if err := json.Unmarshal(generated, &blocks); err != nil {
		return err
	}
	config, count, err := captureConfig(blocks)
	if err != nil {
		return err
	}

	fmt.Printf("HIL reset: importing %d extra resources without changing them\n", count)
	if count > 0 {
		if err := os.WriteFile(capture, config, 0600); err != nil {
			return err
		}
		if err := r.applyPlan("capture"); err != nil {
			return err
		}
		if err := os.Remove(capture); err != nil {
			return err
		}
		if err := r.applyPlan("delete"); err != nil {
			return err
		}
	}
	// A final plan must be empty before acceptance tests can start.
	if err := r.applyPlan("verify"); err != nil {
		return err
	}
	fmt.Println("HIL reset complete: baseline restored and extra managed resources removed")
	return nil
}

func (r *resetRunner) command(binary string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(r.ctx, binary, args...)
	cmd.Dir = r.dir
	// Scope the release provider configuration to subprocesses, not other tasks.
	cmd.Env = append(os.Environ(), "TF_CLI_CONFIG_FILE="+filepath.Join(r.dir, "terraform.rc"))
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s %s failed: %w\n%s", filepath.Base(binary), args[0], err, stderr.String())
	}
	return out, nil
}

const captureFilename = "hil-reset.generated.tf"
const capturePrefix = "hil_reset_"

type resetState struct {
	Values struct {
		RootModule struct {
			Resources []struct {
				Address string                     `json:"address"`
				Values  map[string]json.RawMessage `json:"values"`
			} `json:"resources"`
		} `json:"root_module"`
	} `json:"values"`
}

type resetPlan struct {
	Configuration struct {
		RootModule struct {
			Resources []struct {
				Address string `json:"address"`
				Mode    string `json:"mode"`
			} `json:"resources"`
			Modules map[string]json.RawMessage `json:"module_calls"`
		} `json:"root_module"`
	} `json:"configuration"`
	ResourceChanges []struct {
		Address string `json:"address"`
		Change  struct {
			Actions   []string `json:"actions"`
			Importing *struct {
				ID string `json:"id"`
			} `json:"importing"`
		} `json:"change"`
	} `json:"resource_changes"`
}

func (r *resetRunner) applyPlan(phase string) error {
	planPath := filepath.Join(r.temp, phase+".tfplan")
	if _, err := r.command(r.tf, "plan", "-input=false", "-lock-timeout=5m", "-out="+planPath); err != nil {
		return err
	}
	data, err := r.command(r.tf, "show", "-json", planPath)
	if err != nil {
		return err
	}
	var plan resetPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return err
	}
	if phase == "baseline" {
		r.baseline, err = baselineAddresses(plan)
		if err != nil {
			return err
		}
	}
	if err := validateResetPlan(plan, phase, r.baseline); err != nil {
		return err
	}
	changes, imports := 0, 0
	for _, resource := range plan.ResourceChanges {
		if resource.Change.Importing != nil {
			imports++
		}
		if strings.Join(resource.Change.Actions, ",") != "no-op" {
			changes++
		}
	}
	fmt.Printf("HIL reset %s: %d imports, %d changes\n", phase, imports, changes)
	if changes == 0 && imports == 0 {
		return nil
	}
	_, err = r.command(r.tf, "apply", "-input=false", planPath)
	return err
}

// Apply only reviewed action classes. Capture may only import; deletion may
// only remove reserved cleanup addresses. Never replace/delete baseline objects.
func validateResetPlan(plan resetPlan, phase string, baseline map[string]bool) error {
	for _, resource := range plan.ResourceChanges {
		if resource.Change.Importing != nil && (phase == "verify" || phase == "delete") {
			return fmt.Errorf("refusing %s plan: %s would import", phase, resource.Address)
		}
		action := strings.Join(resource.Change.Actions, ",")
		if action == "no-op" {
			continue
		}
		parts := strings.Split(resource.Address, ".")
		cleanup := len(parts) == 2 && strings.HasPrefix(parts[1], capturePrefix) && !baseline[resource.Address]
		allowed := phase == "baseline" && ((baseline[resource.Address] && (action == "update" || action == "create")) || (cleanup && action == "delete"))
		allowed = allowed || phase == "delete" && cleanup && action == "delete"
		if !allowed {
			return fmt.Errorf("refusing %s plan: %s would %s", phase, resource.Address, action)
		}
	}
	return nil
}
