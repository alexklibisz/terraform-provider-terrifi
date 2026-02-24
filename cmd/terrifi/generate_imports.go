package main

import (
	"context"
	"fmt"
	"os"

	"github.com/alexklibisz/terrifi/internal/generate"
	"github.com/alexklibisz/terrifi/internal/provider"
	"github.com/spf13/cobra"
)

var validResourceTypes = []string{
	"terrifi_client_device",
	"terrifi_dns_record",
	"terrifi_firewall_zone",
	"terrifi_firewall_policy",
	"terrifi_network",
	"terrifi_wlan",
}

func generateImportsCmd() *cobra.Command {
	return &cobra.Command{
		Use:       "generate-imports <resource_type>",
		Short:     "Generate Terraform import blocks and resource definitions from live UniFi data",
		Long:      "Connects to a UniFi controller using UNIFI_* environment variables and generates Terraform import {} + resource {} blocks for all resources of the given type.",
		Args:      cobra.ExactArgs(1),
		ValidArgs: validResourceTypes,
		RunE:      runGenerateImports,
	}
}

func runGenerateImports(cmd *cobra.Command, args []string) error {
	resourceType := args[0]
	ctx := context.Background()

	cfg := provider.ClientConfigFromEnv()
	client, err := provider.NewClient(ctx, cfg)
	if err != nil {
		return fmt.Errorf("connecting to UniFi controller: %w", err)
	}

	site := cfg.Site

	var blocks []generate.ResourceBlock

	switch resourceType {
	case "terrifi_client_device":
		clients, err := client.ListClientDevices(ctx, site)
		if err != nil {
			return fmt.Errorf("listing client devices: %w", err)
		}
		blocks = generate.ClientDeviceBlocks(clients)

	case "terrifi_dns_record":
		records, err := client.ListDNSRecord(ctx, site)
		if err != nil {
			return fmt.Errorf("listing DNS records: %w", err)
		}
		blocks = generate.DNSRecordBlocks(records)

	case "terrifi_firewall_zone":
		zones, err := client.ListFirewallZone(ctx, site)
		if err != nil {
			return fmt.Errorf("listing firewall zones: %w", err)
		}
		blocks = generate.FirewallZoneBlocks(zones)

	case "terrifi_firewall_policy":
		policies, err := client.ListFirewallPolicies(ctx, site)
		if err != nil {
			return fmt.Errorf("listing firewall policies: %w", err)
		}
		blocks = generate.FirewallPolicyBlocks(policies)

	case "terrifi_network":
		networks, err := client.ListNetwork(ctx, site)
		if err != nil {
			return fmt.Errorf("listing networks: %w", err)
		}
		blocks = generate.NetworkBlocks(networks)

	case "terrifi_wlan":
		wlans, err := client.ListWLAN(ctx, site)
		if err != nil {
			return fmt.Errorf("listing WLANs: %w", err)
		}
		blocks = generate.WLANBlocks(wlans)

	default:
		return fmt.Errorf("unknown resource type: %s\nValid types: %v", resourceType, validResourceTypes)
	}

	if len(blocks) == 0 {
		fmt.Fprintf(os.Stderr, "No %s resources found.\n", resourceType)
		return nil
	}

	return generate.WriteBlocks(os.Stdout, blocks)
}
