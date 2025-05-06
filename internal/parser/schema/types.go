package schema

// Schema represents a JSON schema as a generic map
type Schema map[string]any

// InputSchema represents the input schema for a component
type InputSchema struct {
	Schema Schema `yaml:",inline"`
}

// OutputSchema represents the output schema for a component
type OutputSchema struct {
	Schema Schema `yaml:",inline"`
}
