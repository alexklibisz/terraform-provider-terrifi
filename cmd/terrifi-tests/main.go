// terrifi-tests is a developer-only CLI for managing test infrastructure.
// It is not shipped to end users.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/alexklibisz/terrifi/internal/provider"
	"github.com/spf13/cobra"
	"github.com/ubiquiti-community/go-unifi/unifi"
)

// testResourcePrefix is the name prefix used by all acceptance tests when
// creating resources. Anything matching this prefix is assumed to be leftover
// from a failed/aborted test run and safe to delete.
const testResourcePrefix = "tfacc-"

func main() {
	rootCmd := &cobra.Command{
		Use:   "terrifi-tests",
		Short: "Developer tools for managing terrifi acceptance-test infrastructure",
	}
	rootCmd.AddCommand(cleanupCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func cleanupCmd() *cobra.Command {
	var dryRun bool
	var site string
	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Delete leftover acceptance-test resources (name prefix \"tfacc-\") from a controller",
		Long: "Scans the configured UniFi controller for resources whose names start with " +
			"\"tfacc-\" and deletes them. Recovers from prior test runs that left orphans " +
			"(e.g. blocked client devices that hit the controller's max_blocked_user limit).",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCleanup(cmd.Context(), site, dryRun)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "List candidates without deleting")
	cmd.Flags().StringVar(&site, "site", "", "Site name (defaults to UNIFI_SITE or 'default')")
	return cmd
}

func runCleanup(ctx context.Context, siteFlag string, dryRun bool) error {
	if ctx == nil {
		ctx = context.Background()
	}

	cfg := provider.ClientConfigFromEnv()
	client, err := provider.NewClient(ctx, cfg)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}

	site := siteFlag
	if site == "" {
		site = cfg.Site
	}
	if site == "" {
		site = "default"
	}

	fmt.Printf("Scanning site %q for resources with prefix %q (dry-run=%v)\n",
		site, testResourcePrefix, dryRun)

	deleted, errs := cleanupClientDevices(ctx, client, site, dryRun)

	fmt.Printf("\nDone. Deleted: %d, errors: %d\n", deleted, errs)
	if errs > 0 {
		return fmt.Errorf("%d errors during cleanup", errs)
	}
	return nil
}

func cleanupClientDevices(ctx context.Context, client *provider.Client, site string, dryRun bool) (deleted, errs int) {
	clients, err := client.ListClientDevices(ctx, site)
	if err != nil {
		fmt.Printf("client_devices: list failed: %v\n", err)
		return 0, 1
	}

	var matches []unifi.Client
	for _, c := range clients {
		if strings.HasPrefix(c.Name, testResourcePrefix) {
			matches = append(matches, c)
		}
	}

	fmt.Printf("client_devices: %d candidates\n", len(matches))
	for _, c := range matches {
		blocked := c.Blocked != nil && *c.Blocked
		fmt.Printf("  %s  mac=%s  blocked=%v  name=%q\n", c.ID, c.MAC, blocked, c.Name)
		if dryRun {
			continue
		}
		if err := client.DeleteClientDevice(ctx, site, c.ID); err != nil {
			var notFound *unifi.NotFoundError
			if errors.As(err, &notFound) {
				continue
			}
			fmt.Printf("    delete failed: %v\n", err)
			errs++
			continue
		}
		deleted++
	}
	return deleted, errs
}
