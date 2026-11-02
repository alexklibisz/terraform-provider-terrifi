package generate

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// StateIndex identifies controller objects in terraform/tofu show -json output.
// Addresses are deliberately ignored: modules, count, and for_each do not change
// an object's identity. Site names keep identical IDs in other sites separate.
type StateIndex map[stateIdentity]bool

type stateIdentity struct{ resourceType, site, kind, value string }

type stateModule struct {
	Resources []struct {
		Mode   string                     `json:"mode"`
		Type   string                     `json:"type"`
		Values map[string]json.RawMessage `json:"values"`
	} `json:"resources"`
	Children []stateModule `json:"child_modules"`
}

// ReadStateIndex accepts the public JSON state representation, not raw .tfstate
// or plan JSON. Read it before connecting to the controller, failing closed on
// malformed input instead of accidentally emitting already-managed objects.
func ReadStateIndex(r io.Reader) (StateIndex, error) {
	var state struct {
		Version string `json:"format_version"`
		Values  *struct {
			Root stateModule `json:"root_module"`
		} `json:"values"`
		Planned json.RawMessage `json:"planned_values"`
	}
	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&state); err != nil {
		return nil, fmt.Errorf("decode state JSON: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("expected one state JSON object")
	}
	if !strings.HasPrefix(state.Version, "1.") || state.Planned != nil {
		return nil, fmt.Errorf("expected terraform/tofu show -json state output (not raw state or a plan)")
	}
	index := StateIndex{}
	if state.Values == nil {
		return index, nil
	} // An empty state has only format_version.
	var visit func(stateModule) error
	visit = func(module stateModule) error {
		for _, resource := range module.Resources {
			if resource.Mode != "managed" || !strings.HasPrefix(resource.Type, "terrifi_") {
				continue
			}
			value := func(key string) string {
				var s string
				_ = json.Unmarshal(resource.Values[key], &s)
				return s
			}
			site := value("site")
			if site == "" {
				site = "default"
			}
			id, mac := value("id"), value("mac")
			if resource.Type == "terrifi_device_ports" {
				mac = value("device_mac")
			}
			if (resource.Type == "terrifi_device" || resource.Type == "terrifi_device_ports") && mac == "" {
				return fmt.Errorf("managed %s has no MAC in state", resource.Type)
			}
			if id == "" && mac == "" {
				return fmt.Errorf("managed %s has no identity in state", resource.Type)
			}
			if id != "" {
				index[stateIdentity{resource.Type, site, "id", id}] = true
			}
			if mac != "" {
				index[stateIdentity{resource.Type, site, "mac", strings.ToLower(mac)}] = true
			}
		}
		for _, child := range module.Children {
			if err := visit(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(state.Values.Root); err != nil {
		return nil, err
	}
	return index, nil
}

// ExcludeManaged retains original names and order, matching IDs and (for device
// resources) MACs. Client MACs also protect a rediscovered client with a new ID.
func (index StateIndex) ExcludeManaged(blocks []ResourceBlock, site string) []ResourceBlock {
	if site == "" {
		site = "default"
	}
	result := make([]ResourceBlock, 0, len(blocks))
	for _, block := range blocks {
		managed := index[stateIdentity{block.ResourceType, site, "id", block.ImportID}]
		if block.ResourceType == "terrifi_device" || block.ResourceType == "terrifi_client_device" || block.ResourceType == "terrifi_device_ports" {
			for _, attr := range block.Attributes {
				if attr.Key != "mac" && (block.ResourceType != "terrifi_device_ports" || attr.Key != "device_mac") {
					continue
				}
				mac, err := strconv.Unquote(attr.Value)
				if err == nil {
					managed = managed || index[stateIdentity{block.ResourceType, site, "mac", strings.ToLower(mac)}]
				}
			}
		}
		if !managed {
			result = append(result, block)
		}
	}
	return result
}
