package schema

import (
	"context"
	"encoding/json"
	"fmt"

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
