package provider

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/testcontainers/testcontainers-go/modules/compose"
	"github.com/ubiquiti-community/go-unifi/unifi"
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

// runDockerTests starts a UniFi controller in Docker, waits for it to be ready,
// sets the env vars that the provider reads, runs all tests, then tears down.
func runDockerTests(m *testing.M) int {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the Docker Compose stack (docker-compose.yaml in project root).
	dc, err := compose.NewDockerCompose("../../docker-compose.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create docker compose: %s\n", err)
		return 1
	}

	// Tear down containers when we're done, regardless of test results.
	defer func() {
		fmt.Println("Tearing down Docker containers...")
		if err := dc.Down(context.Background(), compose.RemoveOrphans(true)); err != nil {
			fmt.Fprintf(os.Stderr, "failed to tear down: %s\n", err)
		}
	}()

	fmt.Println("Starting UniFi controller in Docker...")
	if err := dc.Up(ctx, compose.Wait(true)); err != nil {
		fmt.Fprintf(os.Stderr, "failed to start docker compose: %s\n", err)
		return 1
	}

	// Get the container's mapped port. Docker maps the container's 8443 to a
	// random host port to avoid conflicts with other services.
	container, err := dc.ServiceContainer(ctx, "unifi")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get container: %s\n", err)
		return 1
	}

	host, err := container.Host(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get host: %s\n", err)
		return 1
	}

	mappedPort, err := container.MappedPort(ctx, "8443/tcp")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get mapped port: %s\n", err)
		return 1
	}

	endpoint := fmt.Sprintf("https://%s:%s", host, mappedPort.Port())

	// Set the env vars that our provider reads in Configure().
	// These override anything from .envrc since we want tests to hit Docker.
	os.Setenv("UNIFI_USERNAME", "admin")
	os.Setenv("UNIFI_PASSWORD", "admin")
	os.Setenv("UNIFI_API", endpoint)
	os.Setenv("UNIFI_INSECURE", "true")
	os.Setenv("UNIFI_SITE", "default")
	os.Setenv("UNIFI_API_KEY", "") // Clear any API key from env

	// Wait for the UniFi API to be fully operational. The container may be
	// "healthy" per Docker but the API might not be ready yet.
	fmt.Printf("Waiting for UniFi API at %s...\n", endpoint)
	if err := waitForAPI(ctx, endpoint, "admin", "admin"); err != nil {
		fmt.Fprintf(os.Stderr, "API never became ready: %s\n", err)
		return 1
	}

	fmt.Println("UniFi API ready, running tests...")
	return m.Run()
}

// waitForAPI polls the UniFi API until login succeeds and basic endpoints respond.
// The controller can take 60-120 seconds to initialize after Docker reports healthy.
func waitForAPI(ctx context.Context, endpoint, user, pass string) error {
	client := &unifi.ApiClient{}

	httpClient := retryablehttp.NewClient()
	httpClient.HTTPClient.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}
	jar, _ := cookiejar.New(nil)
	httpClient.HTTPClient.Jar = jar
	httpClient.Logger = nil // Suppress retry logs during startup

	if err := client.SetHTTPClient(httpClient); err != nil {
		return fmt.Errorf("setting HTTP client: %w", err)
	}
	if err := client.SetBaseURL(endpoint); err != nil {
		return fmt.Errorf("setting base URL: %w", err)
	}

	maxRetries := 60
	retryDelay := 3 * time.Second

	for i := range maxRetries {
		// Try to login.
		if err := client.Login(ctx, user, pass); err != nil {
			if i%10 == 0 {
				fmt.Printf("  attempt %d/%d: login failed (%s), retrying...\n", i+1, maxRetries, err)
			}
			time.Sleep(retryDelay)
			continue
		}

		// Verify the sites endpoint responds (confirms the network application is ready).
		if _, err := client.ListSites(ctx); err != nil {
			fmt.Printf("  attempt %d/%d: login OK but sites not ready (%s)\n", i+1, maxRetries, err)
			time.Sleep(retryDelay)
			continue
		}

		fmt.Printf("  API ready after %d attempts\n", i+1)
		return nil
	}

	return fmt.Errorf("API not ready after %d attempts (%v total)", maxRetries, time.Duration(maxRetries)*retryDelay)
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
