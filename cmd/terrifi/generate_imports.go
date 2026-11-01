package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/alexklibisz/terrifi/internal/generate"
	"github.com/alexklibisz/terrifi/internal/provider"
	"github.com/spf13/cobra"
	"github.com/ubiquiti-community/go-unifi/unifi"
)

var validResourceTypes = []string{
	"terrifi_client_device",
	"terrifi_client_group",
	"terrifi_device",
	"terrifi_device_ports",
	"terrifi_dns_record",
	"terrifi_firewall_group",
	"terrifi_firewall_zone",
	"terrifi_firewall_policy",
	"terrifi_firewall_policy_order",
	"terrifi_network",
	"terrifi_port_profile",
	"terrifi_wlan",
}

func generateImportsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:       "generate-imports <resource_type|all>",
		Short:     "Generate Terraform import blocks and resource definitions from live UniFi data",
		Long:      "Connects to a UniFi controller using UNIFI_* environment variables and generates Terraform import {} + resource {} blocks for the given type, or all supported types with 'all'.",
		Args:      cobra.ExactArgs(1),
		ValidArgs: append([]string{"all"}, validResourceTypes...),
		RunE:      runGenerateImports,
	}
	cmd.Flags().Bool("exclude-default-zones", false, "Exclude built-in firewall zones that cannot be deleted")
	cmd.Flags().String("exclude-state", "", "Exclude objects in a terraform/tofu show -json state file")
	cmd.Flags().String("format", "hcl", "Output format: hcl or json (resource block descriptions with HCL attribute expressions)")
	return cmd
}

func runGenerateImports(cmd *cobra.Command, args []string) error {
	resourceType := args[0]
	ctx := cmd.Context()
	format, _ := cmd.Flags().GetString("format")
	if format != "hcl" && format != "json" {
		return fmt.Errorf("unsupported format %q: use hcl or json", format)
	}
	statePath, _ := cmd.Flags().GetString("exclude-state")
	var state generate.StateIndex
	if statePath != "" {
		file, err := os.Open(statePath)
		if err != nil {
			return fmt.Errorf("open exclusion state: %w", err)
		}
		defer file.Close()
		state, err = generate.ReadStateIndex(file)
		if err != nil {
			return err
		}
	}

	cfg := provider.ClientConfigFromEnv()
	client, err := provider.NewClient(ctx, cfg)
	if err != nil {
		return fmt.Errorf("connecting to UniFi controller: %w", err)
	}

	site := cfg.Site

	excludeDefaultZones, _ := cmd.Flags().GetBool("exclude-default-zones")
	resourceTypes := []string{resourceType}
	if resourceType == "all" {
		resourceTypes = validResourceTypes
	}
	var blocks []generate.ResourceBlock
	for _, resourceType := range resourceTypes {
		generated, err := generateResourceBlocks(ctx, client, site, resourceType, excludeDefaultZones)
		if err != nil {
			return err
		}
		blocks = append(blocks, generated...)
	}
	if state != nil {
		blocks = state.ExcludeManaged(blocks, site)
	}
	if format == "json" {
		if blocks == nil {
			blocks = []generate.ResourceBlock{}
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(blocks)
	}
	if len(blocks) == 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "No unmanaged %s resources found.\n", resourceType)
		return nil
	}
	return generate.WriteBlocks(cmd.OutOrStdout(), blocks)
}

func generateResourceBlocks(ctx context.Context, client *provider.Client, site, resourceType string, excludeDefaultZones bool) ([]generate.ResourceBlock, error) {
	var blocks []generate.ResourceBlock

	switch resourceType {
	case "terrifi_client_device":
		clients, err := client.ListClientDevices(ctx, site)
		if err != nil {
			return nil, fmt.Errorf("listing client devices: %w", err)
		}
		// Enrich with fingerprint overrides from the v2 API.
		overrides := map[string]int64{}
		for _, c := range clients {
			if c.MAC == "" {
				continue
			}
			devTypeID, err := client.GetFingerprintOverride(ctx, site, c.MAC)
			if err != nil || devTypeID == 0 {
				continue
			}
			overrides[c.MAC] = devTypeID
		}
		blocks = generate.ClientDeviceBlocks(clients, overrides)

	case "terrifi_client_group":
		groups, err := client.ListNetworkMembersGroups(ctx, site)
		if err != nil {
			return nil, fmt.Errorf("listing client groups: %w", err)
		}
		// Only include CLIENTS-type groups. The network-members-group endpoint
		// also returns USERS-type groups (legacy QoS user groups), which don't
		// correspond to terrifi_client_group.
		filtered := make([]unifi.NetworkMembersGroup, 0, len(groups))
		for _, g := range groups {
			if g.Type == "CLIENTS" {
				filtered = append(filtered, g)
			}
		}
		blocks = generate.ClientGroupBlocks(filtered)

	case "terrifi_device":
		devices, err := client.ListDevice(ctx, site)
		if err != nil {
			return nil, fmt.Errorf("listing devices: %w", err)
		}
		blocks = generate.DeviceBlocks(devices)

	case "terrifi_dns_record":
		records, err := client.ListDNSRecord(ctx, site)
		if err != nil {
			return nil, fmt.Errorf("listing DNS records: %w", err)
		}
		blocks = generate.DNSRecordBlocks(records)

	case "terrifi_firewall_group":
		groups, err := client.ListFirewallGroup(ctx, site)
		if err != nil {
			return nil, fmt.Errorf("listing firewall groups: %w", err)
		}
		blocks = generate.FirewallGroupBlocks(groups)

	case "terrifi_firewall_zone":
		zones, err := client.ListFirewallZone(ctx, site)
		if err != nil {
			return nil, fmt.Errorf("listing firewall zones: %w", err)
		}
		if excludeDefaultZones {
			customZones := zones[:0]
			for _, zone := range zones {
				if !zone.DefaultZone {
					customZones = append(customZones, zone)
				}
			}
			zones = customZones
		}
		blocks = generate.FirewallZoneBlocks(zones)

	case "terrifi_firewall_policy":
		policies, err := client.ListFirewallPolicies(ctx, site)
		if err != nil {
			return nil, fmt.Errorf("listing firewall policies: %w", err)
		}
		blocks = generate.FirewallPolicyBlocks(policies)

	case "terrifi_firewall_policy_order":
		policies, err := client.ListFirewallPolicies(ctx, site)
		if err != nil {
			return nil, fmt.Errorf("listing firewall policies: %w", err)
		}
		blocks = generate.FirewallPolicyOrderBlocks(policies)

	case "terrifi_network":
		networks, err := client.ListNetwork(ctx, site)
		if err != nil {
			return nil, fmt.Errorf("listing networks: %w", err)
		}
		blocks = generate.NetworkBlocks(networks)

	case "terrifi_device_ports":
		devices, err := client.ListDevicePorts(ctx, site, "")
		if err != nil {
			return nil, fmt.Errorf("listing devices: %w", err)
		}
		blocks = generate.DevicePortsBlocks(devices)

	case "terrifi_port_profile":
		profiles, err := client.ListPortProfiles(ctx, site)
		if err != nil {
			return nil, fmt.Errorf("listing port profiles: %w", err)
		}
		lan, err := client.ListLANNetworkIDs(ctx, site)
		if err != nil {
			return nil, fmt.Errorf("listing networks: %w", err)
		}
		blocks = generate.PortProfileBlocks(profiles, lan)

	case "terrifi_wlan":
		wlans, err := client.ListWLAN(ctx, site)
		if err != nil {
			return nil, fmt.Errorf("listing WLANs: %w", err)
		}
		blocks = generate.WLANBlocks(wlans)

	default:
		return nil, fmt.Errorf("unknown resource type: %s\nValid types: %v", resourceType, validResourceTypes)
	}

	return blocks, nil
}
