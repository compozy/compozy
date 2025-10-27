package schema

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	engschema "github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/internal/validate"
)

// New creates a new schema configuration with metadata wrapper.
//
// This is a simple wrapper for schema metadata and configuration.
// For dynamic schema construction (runtime-dependent schemas), use the
// PropertyBuilder pattern from sdk/schema package directly.
//
// Parameters:
//   - ctx: Context for logging and cancellation
//   - id: Schema identifier (required, non-empty)
//   - opts: Functional options for schema configuration
//
// Returns a deep-copied schema ready for use.
//
// Example:
//
//	// Static schema with metadata
//	schema, err := schema.New(ctx, "user-schema",
//	    schema.WithJSONSchema(map[string]any{
//	        "type": "object",
//	        "title": "User Schema",
//	        "description": "Validates user data",
//	        "properties": map[string]any{
//	            "name": map[string]any{"type": "string"},
//	            "age": map[string]any{"type": "integer"},
//	        },
//	        "required": []string{"name"},
//	    }),
//	)
func New(ctx context.Context, id string, opts ...Option) (*engschema.Schema, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	log := logger.FromContext(ctx)
	log.Debug("creating schema configuration", "id", id)
	collected := make([]error, 0)
	if err := validate.ID(ctx, id); err != nil {
		collected = append(collected, fmt.Errorf("id is invalid: %w", err))
	}
	schema := engschema.Schema{
		"id": id,
	}
	for _, opt := range opts {
		opt(&schema)
	}
	if err := validateSchema(&schema); err != nil {
		collected = append(collected, err)
	}
	if len(collected) > 0 {
		return nil, &sdkerrors.BuildError{Errors: collected}
	}
	cloned, err := core.DeepCopy(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to clone schema: %w", err)
	}
	return &cloned, nil
}

func validateSchema(s *engschema.Schema) error {
	if s == nil {
		return fmt.Errorf("schema cannot be nil")
	}
	schemaMap := map[string]any(*s)
	if schemaType, ok := schemaMap["type"]; ok {
		typeStr, isString := schemaType.(string)
		if !isString {
			return fmt.Errorf("schema type must be a string")
		}
		validTypes := map[string]bool{
			"object":  true,
			"string":  true,
			"number":  true,
			"integer": true,
			"boolean": true,
			"array":   true,
			"null":    true,
		}
		if !validTypes[typeStr] {
			return fmt.Errorf("invalid schema type: %s", typeStr)
		}
	}
	if properties, ok := schemaMap["properties"]; ok {
		if _, isMap := properties.(map[string]any); !isMap {
			return fmt.Errorf("properties must be a map[string]any")
		}
	}
	if required, ok := schemaMap["required"]; ok {
		switch v := required.(type) {
		case []string:
		case []any:
			for i, item := range v {
				if _, isString := item.(string); !isString {
					return fmt.Errorf("required[%d] must be a string", i)
				}
			}
		default:
			return fmt.Errorf("required must be a string array")
		}
	}
	return nil
}
