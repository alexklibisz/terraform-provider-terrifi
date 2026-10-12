// Package hilreset prepares temporary Terraform imports for resetting a HIL
// controller to the checked-in baseline. It never stores credentials in config.
package hilreset

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

const CaptureFilename = "hil-reset.generated.tf"
const Prefix = "hil_reset_"

type State struct {
	Values struct {
		RootModule struct {
			Resources []StateResource `json:"resources"`
		} `json:"root_module"`
	} `json:"values"`
}

type StateResource struct {
	Address string                     `json:"address"`
	Type    string                     `json:"type"`
	Values  map[string]json.RawMessage `json:"values"`
}

// BaselineAddresses reads root resource declarations. Refuse modules/count/
// for_each rather than silently misidentifying protected baseline instances.
func BaselineAddresses(dir string) (map[string]bool, error) {
	jsonPaths, err := filepath.Glob(filepath.Join(dir, "*.tf.json"))
	if err != nil {
		return nil, err
	}
	if len(jsonPaths) > 0 {
		return nil, fmt.Errorf("HIL baseline .tf.json files are not supported")
	}
	paths, err := filepath.Glob(filepath.Join(dir, "*.tf"))
	if err != nil {
		return nil, err
	}
	addresses := map[string]bool{}
	for _, path := range paths {
		if filepath.Base(path) == CaptureFilename {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		file, diags := hclwrite.ParseConfig(data, path, hcl.InitialPos)
		if diags.HasErrors() {
			return nil, fmt.Errorf("%s", diags.Error())
		}
		for _, block := range file.Body().Blocks() {
			if block.Type() == "module" {
				return nil, fmt.Errorf("HIL baseline modules are not supported")
			}
			if block.Type() != "resource" {
				continue
			}
			labels := block.Labels()
			if len(labels) != 2 || strings.HasPrefix(labels[1], Prefix) {
				return nil, fmt.Errorf("invalid or reserved baseline resource address")
			}
			if block.Body().GetAttribute("count") != nil || block.Body().GetAttribute("for_each") != nil {
				return nil, fmt.Errorf("HIL baseline count/for_each are not supported")
			}
			addresses[labels[0]+"."+labels[1]] = true
		}
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("refusing to reset with an empty baseline")
	}
	return addresses, nil
}

// CaptureConfig excludes objects already represented by baseline state, gives
// extra objects stable addresses, and links literal IDs to resource references
// so Terraform deletes dependents before their networks/zones/groups.
func CaptureConfig(generated []byte, state State, baseline map[string]bool) ([]byte, int, error) {
	protected := map[string]bool{}
	for _, resource := range state.Values.RootModule.Resources {
		if !baseline[resource.Address] {
			continue
		}
		for _, key := range []string{"id", "mac"} {
			var value string
			if json.Unmarshal(resource.Values[key], &value) == nil && value != "" {
				protected[resource.Type+":"+strings.ToLower(value)] = true
			}
		}
	}
	file, diags := hclwrite.ParseConfig(generated, "generated.tf", hcl.InitialPos)
	if diags.HasErrors() {
		return nil, 0, fmt.Errorf("%s", diags.Error())
	}
	ids := map[string]string{}
	for _, block := range file.Body().Blocks() {
		if block.Type() != "import" {
			continue
		}
		to := block.Body().GetAttribute("to")
		id := block.Body().GetAttribute("id")
		if to == nil || id == nil {
			return nil, 0, fmt.Errorf("generated import lacks to/id")
		}
		value, err := literalValue(id)
		if err != nil || value.Type() != cty.String {
			return nil, 0, fmt.Errorf("generated import ID is not a literal string")
		}
		ids[strings.TrimSpace(string(to.Expr().BuildTokens(nil).Bytes()))] = value.AsString()
	}
	resources := map[string]*hclwrite.Block{}
	seenAddresses := map[string]bool{}
	addresses := map[string]string{}
	refs := map[string]string{}
	var ordered []string
	for _, block := range file.Body().Blocks() {
		if block.Type() != "resource" {
			continue
		}
		labels := block.Labels()
		if len(labels) != 2 {
			return nil, 0, fmt.Errorf("invalid generated resource")
		}
		address := strings.Join(labels, ".")
		id := ids[address]
		if id == "" {
			return nil, 0, fmt.Errorf("missing import for %s", address)
		}
		// Adopted devices cannot be deleted, so only the baseline manages them.
		if labels[0] == "terrifi_device" {
			continue
		}
		if protected[labels[0]+":"+strings.ToLower(id)] {
			continue
		}
		if mac := block.Body().GetAttribute("mac"); mac != nil {
			value, err := literalValue(mac)
			if err != nil {
				return nil, 0, err
			}
			if value.Type() == cty.String && protected[labels[0]+":"+strings.ToLower(value.AsString())] {
				continue
			}
		}
		hash := sha256.Sum256([]byte(id))
		name := fmt.Sprintf("%s%x", Prefix, hash[:8])
		newAddress := labels[0] + "." + name
		if seenAddresses[newAddress] {
			return nil, 0, fmt.Errorf("duplicate generated object %s", newAddress)
		}
		seenAddresses[newAddress] = true
		addresses[address] = newAddress
		resources[address] = block
		ordered = append(ordered, address)
		if labels[0] != "terrifi_firewall_policy_order" {
			refs[id] = addresses[address] + ".id"
		}
	}
	out := hclwrite.NewEmptyFile()
	for _, address := range ordered {
		block := resources[address]
		newAddress := addresses[address]
		block.SetLabels(strings.Split(newAddress, "."))
		if err := linkBody(block.Body(), refs, newAddress); err != nil {
			return nil, 0, err
		}
		lifecycle := block.Body().AppendNewBlock("lifecycle", nil)
		lifecycle.Body().SetAttributeTraversal("ignore_changes", traversal("all"))
		imp := out.Body().AppendNewBlock("import", nil)
		imp.Body().SetAttributeTraversal("to", traversal(newAddress))
		imp.Body().SetAttributeValue("id", cty.StringVal(ids[address]))
		out.Body().AppendNewline()
		out.Body().AppendBlock(block)
		out.Body().AppendNewline()
	}
	return out.Bytes(), len(ordered), nil
}

func literalValue(attr *hclwrite.Attribute) (cty.Value, error) {
	expr, diags := hclsyntax.ParseExpression(attr.Expr().BuildTokens(nil).Bytes(), "generated.tf", hcl.InitialPos)
	if diags.HasErrors() {
		return cty.NilVal, fmt.Errorf("%s", diags.Error())
	}
	value, diags := expr.Value(nil)
	if diags.HasErrors() {
		return cty.NilVal, fmt.Errorf("%s", diags.Error())
	}
	return value, nil
}

func traversal(address string) hcl.Traversal {
	parts := strings.Split(address, ".")
	result := hcl.Traversal{hcl.TraverseRoot{Name: parts[0]}}
	for _, part := range parts[1:] {
		result = append(result, hcl.TraverseAttr{Name: part})
	}
	return result
}

func linkBody(body *hclwrite.Body, refs map[string]string, self string) error {
	for name, attr := range body.Attributes() {
		value, err := literalValue(attr)
		if err != nil {
			return err
		}
		body.SetAttributeRaw(name, referenceTokens(value, refs, self))
	}
	for _, block := range body.Blocks() {
		if err := linkBody(block.Body(), refs, self); err != nil {
			return err
		}
	}
	return nil
}

func referenceTokens(value cty.Value, refs map[string]string, self string) hclwrite.Tokens {
	if value.IsNull() {
		return hclwrite.TokensForValue(value)
	}
	if value.Type() == cty.String {
		if ref := refs[value.AsString()]; ref != "" && ref != self+".id" {
			return hclwrite.TokensForTraversal(traversal(ref))
		}
	}
	if value.Type().IsTupleType() || value.Type().IsListType() || value.Type().IsSetType() {
		var tokens []hclwrite.Tokens
		for _, item := range value.AsValueSlice() {
			tokens = append(tokens, referenceTokens(item, refs, self))
		}
		return hclwrite.TokensForTuple(tokens)
	}
	return hclwrite.TokensForValue(value)
}
