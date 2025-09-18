package agent

import (
	"encoding/json"
	"fmt"

	enginecore "github.com/compozy/compozy/engine/core"
	"gopkg.in/yaml.v3"
)

// Model represents an agent model that can be specified either as:
// - a string reference to a model resource ID (to be resolved during compile/link), or
// - an inline ProviderConfig defining provider/model and parameters.
//
// YAML support mirrors schema.Schema semantics: scalar → ref, mapping → inline.
type Model struct {
	// Ref is the referenced model ID when provided as a scalar string in YAML/JSON.
	Ref string `json:"-"       yaml:"-"       mapstructure:"-"`
	// Config holds an inline provider configuration when specified as a mapping.
	// When Ref is set, Config may be merged with the resolved model during linking.
	Config enginecore.ProviderConfig `json:",inline" yaml:",inline" mapstructure:",squash"`
}

// HasRef reports whether this model is a reference to a stored model ID.
func (m *Model) HasRef() bool { return m != nil && m.Ref != "" }

// HasConfig reports whether this model has an inline provider configuration.
func (m *Model) HasConfig() bool {
	return m != nil && (m.Config.Provider != "" || m.Config.Model != "")
}

// IsEmpty reports whether no ref nor inline provider config has been provided.
func (m *Model) IsEmpty() bool {
	return m == nil || (!m.HasRef() && m.Config.Provider == "" && m.Config.Model == "")
}

// UnmarshalYAML supports both scalar refs and inline provider configs.
func (m *Model) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		return nil
	}
	switch value.Kind {
	case yaml.ScalarNode:
		var id string
		if err := value.Decode(&id); err != nil {
			return err
		}
		m.Ref = id
		// zero the inline config
		m.Config = enginecore.ProviderConfig{}
		return nil
	case yaml.MappingNode, yaml.SequenceNode, yaml.DocumentNode:
		var cfg enginecore.ProviderConfig
		if err := value.Decode(&cfg); err != nil {
			return err
		}
		m.Ref = ""
		m.Config = cfg
		return nil
	default:
		// treat other node kinds as empty
		*m = Model{}
		return nil
	}
}

// MarshalYAML encodes to inline config when present, otherwise a scalar ref when set.
func (m *Model) MarshalYAML() (any, error) {
	if m == nil {
		return nil, nil
	}
	if m.HasConfig() {
		return m.Config, nil
	}
	if m.HasRef() {
		return m.Ref, nil
	}
	return nil, nil
}

// UnmarshalJSON accepts either a JSON string (ref) or an object (inline config).
func (m *Model) UnmarshalJSON(b []byte) error {
	// Try string ref first
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		m.Ref = s
		m.Config = enginecore.ProviderConfig{}
		return nil
	}
	// Fallback to inline provider config
	var cfg enginecore.ProviderConfig
	if err := json.Unmarshal(b, &cfg); err == nil {
		m.Ref = ""
		m.Config = cfg
		return nil
	}
	return fmt.Errorf("invalid model JSON: expected string ref or provider config")
}

// MarshalJSON emits inline config when present, otherwise a string ref if set.
func (m *Model) MarshalJSON() ([]byte, error) {
	if m == nil {
		return []byte("null"), nil
	}
	if m.HasConfig() {
		return json.Marshal(m.Config)
	}
	if m.HasRef() {
		return json.Marshal(m.Ref)
	}
	// emit null when empty
	return []byte("null"), nil
}
