package schema

// Schema represents a JSON schema
type Schema struct {
	Type        string         `json:"type" yaml:"type"`
	Properties  map[string]any `json:"properties,omitempty" yaml:"properties,omitempty"`
	Required    []string       `json:"required,omitempty" yaml:"required,omitempty"`
	Items       *Schema        `json:"items,omitempty" yaml:"items,omitempty"`
	Description string         `json:"description,omitempty" yaml:"description,omitempty"`
	Enum        []string       `json:"enum,omitempty" yaml:"enum,omitempty"`
}

// InputSchema represents the input schema for a component
type InputSchema struct {
	Schema `yaml:",inline"`
}

// OutputSchema represents the output schema for a component
type OutputSchema struct {
	Schema `yaml:",inline"`
}
