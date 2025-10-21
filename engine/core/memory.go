package core

import (
	"bytes"
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

// Standard memory access mode constants to avoid string drift across packages.
const (
	MemoryModeReadWrite = "read-write"
	MemoryModeReadOnly  = "read-only"
)

// MemoryReference is used in Agent configuration to point to a MemoryResource
// and define how the agent interacts with it.
type MemoryReference struct {
	ID string `yaml:"id"             json:"id"             validate:"required"`
	// Mode defines access permissions (e.g., "read-write", "read-only").
	Mode string `yaml:"mode,omitempty" json:"mode,omitempty" validate:"omitempty,oneof=read-write read-only"`
	// Key is a template string that resolves to the actual memory instance key.
	// e.g., "support-{{ .workflow.input.conversationId }}"
	Key string `yaml:"key,omitempty"  json:"key,omitempty"  validate:"omitempty"`
	// ResolvedKey is the key after template evaluation.
	ResolvedKey string `yaml:"-"              json:"-"`
}

// UnmarshalYAML supports both scalar string and full object forms.
// When a scalar string is provided, it is interpreted as an ID-only
// selector (e.g., memory: ["conversation"]). Object form follows normal decoding.
func (m *MemoryReference) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		return nil
	}
	if value.Tag == "!!null" {
		*m = MemoryReference{}
		return nil
	}
	if value.Kind == yaml.ScalarNode {
		var id string
		if err := value.Decode(&id); err != nil {
			return fmt.Errorf("memoryref: decode scalar id: %w", err)
		}
		m.ID = id
		m.Key = ""
		m.Mode = ""
		m.ResolvedKey = ""
		return nil
	}
	type alias MemoryReference
	var tmp alias
	if err := value.Decode(&tmp); err != nil {
		return fmt.Errorf("memoryref: decode object: %w", err)
	}
	*m = MemoryReference(tmp)
	return nil
}

// UnmarshalJSON accepts either a JSON string (ID ref) or a full object.
func (m *MemoryReference) UnmarshalJSON(b []byte) error {
	switch string(bytes.TrimSpace(b)) {
	case "null", "":
		*m = MemoryReference{}
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		m.ID = s
		m.Key = ""
		m.Mode = ""
		m.ResolvedKey = ""
		return nil
	}
	type alias MemoryReference
	var tmp alias
	if err := json.Unmarshal(b, &tmp); err != nil {
		return fmt.Errorf("memoryref: decode object: %w", err)
	}
	*m = MemoryReference(tmp)
	return nil
}
