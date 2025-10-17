package schema

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/kaptinlin/jsonschema"
	"gopkg.in/yaml.v3"
)

// -----------------------------------------------------------------------------
// Schema
// -----------------------------------------------------------------------------

type Schema map[string]any
type Result = jsonschema.EvaluationResult

// refKey is an internal sentinel key used when a schema is provided
// in YAML as a scalar string referencing a schema ID. The loader rejects
// '$' keys from YAML sources, so we use a non-dollar-prefixed key that
// only ever appears in-memory after decoding.
const refKey = "__schema_ref__"

func (s *Schema) String() string {
	bytes, err := json.Marshal(s)
	if err != nil {
		return ""
	}
	return string(bytes)
}

// UnmarshalYAML supports two forms:
// 1) Mapping node -> decodes to a full JSON Schema object
// 2) Scalar string  -> treated as a schema ID reference to be linked at compile time
func (s *Schema) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		return nil
	}
	switch value.Kind {
	case yaml.ScalarNode:
		var id string
		if err := value.Decode(&id); err != nil {
			return err
		}
		m := map[string]any{refKey: id}
		*s = Schema(m)
		return nil
	case yaml.MappingNode, yaml.SequenceNode, yaml.DocumentNode:
		var m map[string]any
		if err := value.Decode(&m); err != nil {
			return err
		}
		*s = Schema(m)
		return nil
	default:
		// Treat other node kinds as empty schema
		*s = Schema(map[string]any{})
		return nil
	}
}

// IsRef reports whether this schema is a reference created from scalar form
// and returns the referenced ID when true.
func (s *Schema) IsRef() (bool, string) {
	if s == nil {
		return false, ""
	}
	if v, ok := (*s)[refKey]; ok {
		if id, ok2 := v.(string); ok2 && id != "" {
			return true, id
		}
	}
	return false, ""
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

// GetID extracts the id field from a schema when present, else returns empty string.
func GetID(s *Schema) string {
	if s == nil {
		return ""
	}
	if v, ok := (*s)["id"]; ok {
		if str, ok2 := v.(string); ok2 {
			return str
		}
	}
	return ""
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
	result := core.CopyMaps(defaults, input)
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

// FromMap constructs a Schema from a raw map.
func FromMap(input map[string]any) *Schema {
	if input == nil {
		return nil
	}
	s := Schema(input)
	return &s
}

// ValidateRawMessage validates a JSON payload against the provided schema.
func ValidateRawMessage(ctx context.Context, sch *Schema, raw json.RawMessage) error {
	if sch == nil {
		return nil
	}
	if len(raw) == 0 {
		raw = json.RawMessage("{}")
	}
	var data any
	if err := json.Unmarshal(raw, &data); err != nil {
		return fmt.Errorf("failed to unmarshal tool arguments: %w", err)
	}
	result, err := sch.Validate(ctx, data)
	if err != nil {
		return err
	}
	_ = result
	return nil
}
