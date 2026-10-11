package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// testAccProtoV6ProviderFactories creates a provider factory for acceptance tests.
// Every acceptance test references this to tell the test framework how to create
// our provider. It's the same provider we use in production — no mocks.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"terrifi": providerserver.NewProtocol6WithError(New()),
}

// TestMain is the entry point for all tests in this package. Go's testing package
// calls this instead of running tests directly, giving us a chance to set up
// infrastructure (like Docker containers) before any test runs.
//
// The flow:
//   - No TF_ACC env var → run unit tests only (fast, no network)
//   - TF_ACC=1 + TERRIFI_ACC_TARGET=docker → spin up Docker, run acceptance tests
//   - TF_ACC=1 + TERRIFI_ACC_TARGET=hardware → use existing env vars, run acceptance tests
func TestMain(m *testing.M) {
	if os.Getenv("TF_ACC") == "" {
		// Unit tests only — no Docker, no env vars needed.
		os.Exit(m.Run())
	}

	target := os.Getenv("TERRIFI_ACC_TARGET")
	if target == "" {
		target = "docker"
	}

	switch target {
	case "docker":
		os.Exit(runDockerTests(m))
	case "hardware":
		// Hardware mode: env vars already set by direnv/.envrc.local.
		// Just run the tests directly.
		os.Exit(m.Run())
	default:
		fmt.Fprintf(os.Stderr, "unknown TERRIFI_ACC_TARGET: %s (expected 'docker' or 'hardware')\n", target)
		os.Exit(1)
	}
}

// preCheck validates that the required env vars are set before running an
// acceptance test. Called at the start of every TestAcc* function.
func preCheck(t *testing.T) {
	t.Helper()

	if os.Getenv("UNIFI_API") == "" {
		t.Fatal("UNIFI_API must be set for acceptance tests")
	}

	hasAPIKey := os.Getenv("UNIFI_API_KEY") != ""
	hasCredentials := os.Getenv("UNIFI_USERNAME") != "" && os.Getenv("UNIFI_PASSWORD") != ""
	if !hasAPIKey && !hasCredentials {
		t.Fatal("either UNIFI_API_KEY or both UNIFI_USERNAME and UNIFI_PASSWORD must be set for acceptance tests")
	}
}
