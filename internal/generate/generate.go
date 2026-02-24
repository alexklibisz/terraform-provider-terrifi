// Package generate produces Terraform import blocks and resource definitions
// from live UniFi API data. It is used by the terrifi CLI's generate-imports
// command to bootstrap Terraform state from an existing controller.
package generate

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"text/template"
)

// Attr represents a single key = value pair inside a resource block.
type Attr struct {
	Key     string
	Value   string // HCL-formatted (quoted strings, bare bools, etc.)
	Comment string // optional inline comment
}

// NestedBlock represents a nested block (like source {}) inside a resource block.
type NestedBlock struct {
	Name       string
	Attributes []Attr
}

// ResourceBlock represents one import {} + resource {} pair.
type ResourceBlock struct {
	Comment      string
	ResourceType string
	ResourceName string
	ImportID     string
	Attributes   []Attr
	Blocks       []NestedBlock
}

// HCL formatting helpers

func HCLString(s string) string {
	return fmt.Sprintf("%q", s)
}

func HCLBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func HCLInt64(v int64) string {
	return fmt.Sprintf("%d", v)
}

func HCLStringList(vals []string) string {
	if len(vals) == 0 {
		return "[]"
	}
	quoted := make([]string, len(vals))
	for i, v := range vals {
		quoted[i] = fmt.Sprintf("%q", v)
	}
	return fmt.Sprintf("[%s]", strings.Join(quoted, ", "))
}

// ToTerraformName converts a display name (e.g. "My Device") to a valid
// Terraform resource name (e.g. "my_device"). Returns "SET_NAME" for empty input.
func ToTerraformName(name string) string {
	if name == "" {
		return "SET_NAME"
	}
	// Lowercase
	s := strings.ToLower(name)
	// Replace non-alphanumeric runs with underscores
	re := regexp.MustCompile(`[^a-z0-9]+`)
	s = re.ReplaceAllString(s, "_")
	// Trim leading/trailing underscores
	s = strings.Trim(s, "_")
	// Must start with a letter or underscore
	if len(s) > 0 && s[0] >= '0' && s[0] <= '9' {
		s = "_" + s
	}
	if s == "" {
		return "SET_NAME"
	}
	return s
}

// DeduplicateNames takes a slice of ResourceBlocks and appends _2, _3, etc.
// to any duplicate ResourceNames.
func DeduplicateNames(blocks []ResourceBlock) {
	seen := make(map[string]int)
	for i := range blocks {
		name := blocks[i].ResourceName
		seen[name]++
		if seen[name] > 1 {
			blocks[i].ResourceName = fmt.Sprintf("%s_%d", name, seen[name])
		}
	}
}

var blockTemplate = template.Must(template.New("blocks").Parse(`{{- range . }}
{{- if .Comment }}
# {{ .Comment }}
{{ end -}}
import {
  to = {{ .ResourceType }}.{{ .ResourceName }}
  id = "{{ .ImportID }}"
}

resource "{{ .ResourceType }}" "{{ .ResourceName }}" {
{{- range .Attributes }}
{{- if .Comment }}
  {{ .Key }} = {{ .Value }} # {{ .Comment }}
{{- else }}
  {{ .Key }} = {{ .Value }}
{{- end }}
{{- end }}
{{- range .Blocks }}

  {{ .Name }} {
{{- range .Attributes }}
{{- if .Comment }}
    {{ .Key }} = {{ .Value }} # {{ .Comment }}
{{- else }}
    {{ .Key }} = {{ .Value }}
{{- end }}
{{- end }}
  }
{{- end }}
}

{{ end }}`))

// WriteBlocks renders the given ResourceBlocks as HCL to the writer.
func WriteBlocks(w io.Writer, blocks []ResourceBlock) error {
	return blockTemplate.Execute(w, blocks)
}
