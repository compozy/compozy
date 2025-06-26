package core

// MemoryReference is used in Agent configuration to point to a MemoryResource
// and define how the agent interacts with it.
type MemoryReference struct {
	ID string `yaml:"id"             json:"id"             validate:"required"`
	// Mode defines access permissions (e.g., "read-write", "read-only").
	Mode string `yaml:"mode,omitempty" json:"mode,omitempty" validate:"omitempty,oneof=read-write read-only"`
	// Key is a template string that resolves to the actual memory instance key.
	// e.g., "support-{{ .workflow.input.conversationId }}"
	Key string `yaml:"key"            json:"key"            validate:"required"`
	// ResolvedKey is the key after template evaluation.
	ResolvedKey string `yaml:"-"              json:"-"`
}
