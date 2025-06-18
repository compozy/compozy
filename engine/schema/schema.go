package schema

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"

	"github.com/compozy/compozy/engine/core"
	"github.com/kaptinlin/jsonschema"
)

// -----------------------------------------------------------------------------
// Schema
// -----------------------------------------------------------------------------

type Schema map[string]any
type Result = jsonschema.EvaluationResult

func (s *Schema) String() string {
	bytes, err := json.Marshal(s)
	if err != nil {
		return ""
	}
	return string(bytes)
}

func (s *Schema) Compile() (*jsonschema.Schema, error) {
	if s == nil {
		return nil, nil
	}
	bytes, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("failed to compile schema: %w", err)
	}
	compiler := jsonschema.NewCompiler()
	schema, err := compiler.Compile(bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to compile schema: %w", err)
	}
	return schema, nil
}

func (s *Schema) Validate(_ context.Context, value any) (*Result, error) {
	schema, err := s.Compile()
	if err != nil {
		return nil, fmt.Errorf("failed to compile schema: %w", err)
	}
	if schema == nil {
		return nil, nil
	}
	result := schema.Validate(value)
	if result.Valid {
		return result, nil
	}
	return nil, fmt.Errorf("schema validation failed: %v", result.Errors)
}

func (s *Schema) Clone() (*Schema, error) {
	if s == nil {
		return nil, nil
	}
	return core.DeepCopy(s)
}

// ApplyDefaults merges default values from the schema with the provided input
func (s *Schema) ApplyDefaults(input map[string]any) (map[string]any, error) {
	if s == nil {
		return input, nil
	}
	if input == nil {
		input = make(map[string]any)
	}
	// Extract defaults from schema properties
	defaults := s.extractDefaults()
	// Create result by merging defaults with input (input takes precedence)
	result := make(map[string]any)
	maps.Copy(result, defaults)
	maps.Copy(result, input)
	return result, nil
}

// extractDefaults recursively extracts default values from schema properties
func (s *Schema) extractDefaults() map[string]any {
	defaults := make(map[string]any)
	schemaMap := map[string]any(*s)
	// Check if this is an object schema with properties
	if schemaType, exists := schemaMap["type"]; exists && schemaType == "object" {
		if properties, exists := schemaMap["properties"]; exists {
			var propsMap map[string]any
			// Handle both map[string]any and schema.Schema types
			switch v := properties.(type) {
			case map[string]any:
				propsMap = v
			case Schema:
				propsMap = map[string]any(v)
			default:
				return defaults
			}
			// Extract defaults from each property
			for propName, propSchema := range propsMap {
				var propMap map[string]any
				// Handle both map[string]any and schema.Schema types for individual properties
				switch v := propSchema.(type) {
				case map[string]any:
					propMap = v
				case Schema:
					propMap = map[string]any(v)
				default:
					continue
				}
				// Check if this property has a default value
				if defaultValue, hasDefault := propMap["default"]; hasDefault {
					defaults[propName] = defaultValue
				}
			}
		}
	}

	return defaults
}
