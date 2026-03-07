package main

import (
	"context"
	"fmt"

	"github.com/alexklibisz/terrifi/internal/provider"
	"github.com/spf13/cobra"
)

func checkConnectionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check-connection",
		Short: "Verify that the UNIFI_* environment variables are configured correctly",
		Args:  cobra.NoArgs,
		RunE:  runCheckConnection,
	}
}

func runCheckConnection(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	cfg := provider.ClientConfigFromEnv()
	client, err := provider.NewClient(ctx, cfg)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}

	sites, err := client.ListSites(ctx)
	if err != nil {
		return fmt.Errorf("connected but could not list sites: %w", err)
	}

	fmt.Printf("Connection successful (%s)\n", cfg.APIURL)
	fmt.Printf("Auth: ")
	if cfg.APIKey != "" {
		fmt.Println("API key")
	} else {
		fmt.Printf("username (%s)\n", cfg.Username)
	}
	fmt.Printf("Sites: ")
	for i, s := range sites {
		if i > 0 {
			fmt.Print(", ")
		}
		fmt.Print(s.Name)
	}
	fmt.Println()

	return nil
}
