package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/alexklibisz/terrifi/internal/generate"
)

func baselineAddresses(plan resetPlan) (map[string]bool, error) {
	root := plan.Configuration.RootModule
	if len(root.Modules) != 0 {
		return nil, fmt.Errorf("HIL baseline must use root resources")
	}
	addresses := map[string]bool{}
	for _, resource := range root.Resources {
		if resource.Mode != "managed" {
			continue
		}
		parts := strings.Split(resource.Address, ".")
		if len(parts) != 2 || strings.HasPrefix(parts[1], capturePrefix) {
			return nil, fmt.Errorf("invalid or reserved baseline resource address")
		}
		addresses[resource.Address] = true
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("refusing to reset with an empty baseline")
	}
	return addresses, nil
}

// State exclusion belongs to generate-imports. This helper only prepares its
// remaining block descriptions for temporary import and dependency-safe delete.
func captureConfig(blocks []generate.ResourceBlock) ([]byte, int, error) {
	var capture []generate.ResourceBlock
	refs := map[string]string{}
	seen := map[string]bool{}
	for _, block := range blocks {
		// Adopted device deletion is a no-op; only the baseline manages devices.
		if block.ResourceType == "terrifi_device" {
			continue
		}
		block.Attributes = append([]generate.Attr(nil), block.Attributes...)
		block.Blocks = append([]generate.NestedBlock(nil), block.Blocks...)
		for i := range block.Blocks {
			block.Blocks[i].Attributes = append([]generate.Attr(nil), block.Blocks[i].Attributes...)
		}
		if block.ImportID == "" {
			return nil, 0, fmt.Errorf("generated object has no import ID")
		}
		hash := sha256.Sum256([]byte(block.ImportID))
		block.ResourceName = fmt.Sprintf("%s%x", capturePrefix, hash[:8])
		address := block.ResourceType + "." + block.ResourceName
		if seen[address] {
			return nil, 0, fmt.Errorf("duplicate generated object %s", address)
		}
		seen[address] = true
		block.Comment = ""
		if block.ResourceType != "terrifi_firewall_policy_order" {
			if refs[block.ImportID] != "" {
				return nil, 0, fmt.Errorf("ambiguous controller ID %s", block.ImportID)
			}
			refs[block.ImportID] = address + ".id"
		}
		capture = append(capture, block)
	}
	for i := range capture {
		if err := linkAttributes(capture[i].Attributes, refs); err != nil {
			return nil, 0, err
		}
		for _, block := range capture[i].Blocks {
			if err := linkAttributes(block.Attributes, refs); err != nil {
				return nil, 0, err
			}
		}
		capture[i].Blocks = append(capture[i].Blocks, generate.NestedBlock{
			Name: "lifecycle", Attributes: []generate.Attr{{Key: "ignore_changes", Value: "all"}},
		})
	}
	var out bytes.Buffer
	if err := generate.WriteBlocks(&out, capture); err != nil {
		return nil, 0, err
	}
	return out.Bytes(), len(capture), nil
}

func linkAttributes(attrs []generate.Attr, refs map[string]string) error {
	for i, attr := range attrs {
		switch attr.Key {
		case "network_id", "network_override_id", "network_ids", "client_group_ids",
			"zone_id", "source_zone_id", "destination_zone_id", "policy_ids", "port_group_id", "device_ids":
		default:
			continue
		}
		link := func(id string) string {
			if ref := refs[id]; ref != "" {
				return ref
			}
			return generate.HCLString(id)
		}
		var id string
		if json.Unmarshal([]byte(attr.Value), &id) == nil {
			attrs[i].Value = link(id)
		} else {
			var ids []string
			if err := json.Unmarshal([]byte(attr.Value), &ids); err != nil {
				return fmt.Errorf("decode %s references: %w", attr.Key, err)
			}
			values := make([]string, len(ids))
			for j, id := range ids {
				values[j] = link(id)
			}
			attrs[i].Value = "[" + strings.Join(values, ", ") + "]"
		}
		attrs[i].Comment = ""
	}
	return nil
}
