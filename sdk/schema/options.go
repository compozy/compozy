package schema

import (
	engschema "github.com/compozy/compozy/engine/schema"
)

// Option is a functional option for configuring Schema wrapper.
type Option func(*engschema.Schema)

// WithJSONSchema sets the complete JSON schema definition.
//
// This accepts a map[string]any representing the full JSON schema object,
// which can include properties like:
// - "type": schema type (object, string, etc.)
// - "title": human-readable title
// - "description": schema description
// - "properties": object properties (can be built with Builder or plain map)
// - "version": schema version
// - Any other valid JSON Schema fields
//
// Note: The "id" field is preserved from the constructor and will not be overwritten.
//
// Example:
//
//	schema.New(ctx, "user-schema",
//	    schema.WithJSONSchema(map[string]any{
//	        "type": "object",
//	        "title": "User Schema",
//	        "description": "Validates user data",
//	        "properties": propertySchema, // Built with Builder or plain map
//	        "version": "1.0.0",
//	    }),
//	)
func WithJSONSchema(jsonSchema map[string]any) Option {
	return func(s *engschema.Schema) {
		if jsonSchema == nil {
			return
		}
		id := (*s)["id"]
		for k, v := range jsonSchema {
			(*s)[k] = v
		}
		if id != nil {
			(*s)["id"] = id
		}
	}
}
