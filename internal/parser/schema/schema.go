package schema

import (
	"encoding/json"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

// -----------------------------------------------------------------------------
// Schema
// -----------------------------------------------------------------------------

type Schema map[string]any

type InputSchema struct {
	Schema Schema `yaml:",inline"`
}

type OutputSchema struct {
	Schema Schema `yaml:",inline"`
}

func (s *Schema) Validate(value any) error {
	if s == nil {
		return nil
	}

	// Convert schema to JSON for jsonschema
	schemaJSON, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("failed to marshal schema: %w", err)
	}

	// Compile the schema
	schema, err := jsonschema.CompileString("schema.json", string(schemaJSON))
	if err != nil {
		return fmt.Errorf("invalid schema: %w", err)
	}

	// Perform validation
	if err := schema.Validate(value); err != nil {
		return err
	}

	return nil
}

func (s *Schema) GetType() string {
	if typ, ok := (*s)["type"].(string); ok {
		return typ
	}
	return ""
}

func (s *Schema) GetProperties() map[string]*Schema {
	props, ok := (*s)["properties"].(map[string]any)
	if !ok {
		return nil
	}
	result := make(map[string]*Schema)
	for k, v := range props {
		if schema, ok := v.(map[string]any); ok {
			s := Schema(schema)
			result[k] = &s
		}
	}
	return result
}
