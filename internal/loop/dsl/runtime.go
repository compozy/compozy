package dsl

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// RuntimeSpec selects one provider runtime without owning provider policy.
type RuntimeSpec struct {
	Provider  string         `json:"provider,omitempty"  yaml:"provider,omitempty"  toml:"provider"`
	Model     string         `json:"model,omitempty"     yaml:"model,omitempty"     toml:"model"`
	Reasoning string         `json:"reasoning,omitempty" yaml:"reasoning,omitempty" toml:"reasoning"`
	Extra     map[string]any `json:"-"                   yaml:",inline"`
}

// UnmarshalYAML keeps runtime authoring closed and type-safe at every YAML ingress.
func (r *RuntimeSpec) UnmarshalYAML(value *yaml.Node) error {
	if value == nil || value.Kind != yaml.MappingNode {
		return fmt.Errorf("runtime must be an object")
	}
	*r = RuntimeSpec{}
	seen := make(map[string]struct{}, len(value.Content)/2)
	for index := 0; index < len(value.Content); index += 2 {
		keyNode := value.Content[index]
		valueNode := value.Content[index+1]
		key := strings.TrimSpace(keyNode.Value)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("runtime.%s is duplicated", key)
		}
		seen[key] = struct{}{}
		if valueNode.Kind != yaml.ScalarNode || valueNode.Tag != "!!str" {
			return fmt.Errorf("runtime.%s must be a string", key)
		}
		text := strings.TrimSpace(valueNode.Value)
		switch key {
		case "provider":
			r.Provider = text
		case "model":
			r.Model = text
		case "reasoning":
			r.Reasoning = text
		default:
			return fmt.Errorf(
				"runtime.%s is unknown; see MIGRATION_GUIDE.md#per-task-runtime-selection",
				key,
			)
		}
	}
	return nil
}

// RuntimeDefaults defines worker and judge runtime defaults.
type RuntimeDefaults struct {
	Worker RuntimeSpec `json:"worker,omitzero" yaml:"worker,omitempty" toml:"worker"`
	Judge  RuntimeSpec `json:"judge,omitzero"  yaml:"judge,omitempty"  toml:"judge"`
}

// RuntimeMatch selects exactly one task identity dimension.
type RuntimeMatch struct {
	ID         string         `json:"id,omitempty"         yaml:"id,omitempty"         toml:"id"`
	Type       string         `json:"type,omitempty"       yaml:"type,omitempty"       toml:"type"`
	Complexity string         `json:"complexity,omitempty" yaml:"complexity,omitempty" toml:"complexity"`
	Extra      map[string]any `json:"-"                    yaml:",inline"`
}

// RuntimeRule applies one field-merged runtime override to matching tasks.
type RuntimeRule struct {
	Match   RuntimeMatch `json:"match"   yaml:"match"   toml:"match"`
	Runtime RuntimeSpec  `json:"runtime" yaml:"runtime" toml:"runtime"`
}
