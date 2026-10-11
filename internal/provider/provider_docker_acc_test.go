//go:build acc

// The Docker acceptance bootstrap lives behind the `acc` build tag so that
// testcontainers-go/modules/compose — and the containerd/docker/compose-go tree
// it pulls in — is only compiled when it is actually going to be used. Without
// the tag, `go test ./...` builds the whole Docker stack just to run unit tests.
//
// Run the Docker acceptance target with:
//
//	go test -tags=acc ./internal/provider/ -run TestAcc
//
// The Taskfile's test:acc* targets pass the tag for you.

package provider

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go/modules/compose"
	"github.com/ubiquiti-community/go-unifi/unifi"
)

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
	maxRetries := 60
	retryDelay := 3 * time.Second

	for i := range maxRetries {
		client, err := unifi.New(ctx, &unifi.Config{
			BaseURL:       endpoint,
			Username:      user,
			Password:      pass,
			AllowInsecure: true,
		})
		if err != nil {
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
