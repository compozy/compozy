package schema

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_MinimalConfig(t *testing.T) {
	t.Run("Should create minimal schema with just ID", func(t *testing.T) {
		schema, err := New(t.Context(), "test-schema")
		require.NoError(t, err)
		require.NotNil(t, schema)
		schemaMap := map[string]any(*schema)
		assert.Equal(t, "test-schema", schemaMap["id"])
	})
}

func TestNew_WithJSONSchema(t *testing.T) {
	t.Run("Should create schema with JSON schema definition", func(t *testing.T) {
		schema, err := New(t.Context(), "user-schema",
			WithJSONSchema(map[string]any{
				"type":        "object",
				"title":       "User Schema",
				"description": "Validates user data",
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
					"age":  map[string]any{"type": "integer"},
				},
				"required": []string{"name"},
			}),
		)
		require.NoError(t, err)
		require.NotNil(t, schema)
		schemaMap := map[string]any(*schema)
		assert.Equal(t, "user-schema", schemaMap["id"])
		assert.Equal(t, "object", schemaMap["type"])
		assert.Equal(t, "User Schema", schemaMap["title"])
		assert.Equal(t, "Validates user data", schemaMap["description"])
		properties, ok := schemaMap["properties"].(map[string]any)
		require.True(t, ok)
		assert.Contains(t, properties, "name")
		assert.Contains(t, properties, "age")
		required, ok := schemaMap["required"].([]string)
		require.True(t, ok)
		assert.Equal(t, []string{"name"}, required)
	})
	t.Run("Should handle string schema type", func(t *testing.T) {
		schema, err := New(t.Context(), "string-schema",
			WithJSONSchema(map[string]any{
				"type":      "string",
				"minLength": 1,
				"maxLength": 100,
			}),
		)
		require.NoError(t, err)
		require.NotNil(t, schema)
		schemaMap := map[string]any(*schema)
		assert.Equal(t, "string", schemaMap["type"])
		assert.Equal(t, 1, schemaMap["minLength"])
		assert.Equal(t, 100, schemaMap["maxLength"])
	})
	t.Run("Should handle array schema type", func(t *testing.T) {
		schema, err := New(t.Context(), "array-schema",
			WithJSONSchema(map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "string",
				},
			}),
		)
		require.NoError(t, err)
		require.NotNil(t, schema)
		schemaMap := map[string]any(*schema)
		assert.Equal(t, "array", schemaMap["type"])
		items, ok := schemaMap["items"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "string", items["type"])
	})
	t.Run("Should handle nil JSON schema gracefully", func(t *testing.T) {
		schema, err := New(t.Context(), "nil-schema",
			WithJSONSchema(nil),
		)
		require.NoError(t, err)
		require.NotNil(t, schema)
		schemaMap := map[string]any(*schema)
		assert.Equal(t, "nil-schema", schemaMap["id"])
	})
}

func TestNew_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		opts    []Option
		wantErr string
	}{
		{
			name:    "empty id",
			id:      "",
			wantErr: "id is invalid",
		},
		{
			name:    "whitespace id",
			id:      "   ",
			wantErr: "id is invalid",
		},
		{
			name: "invalid schema type",
			id:   "test-schema",
			opts: []Option{
				WithJSONSchema(map[string]any{
					"type": "invalid-type",
				}),
			},
			wantErr: "invalid schema type",
		},
		{
			name: "non-string schema type",
			id:   "test-schema",
			opts: []Option{
				WithJSONSchema(map[string]any{
					"type": 123,
				}),
			},
			wantErr: "schema type must be a string",
		},
		{
			name: "invalid properties type",
			id:   "test-schema",
			opts: []Option{
				WithJSONSchema(map[string]any{
					"type":       "object",
					"properties": "invalid",
				}),
			},
			wantErr: "properties must be a map",
		},
		{
			name: "invalid required type",
			id:   "test-schema",
			opts: []Option{
				WithJSONSchema(map[string]any{
					"type":     "object",
					"required": "invalid",
				}),
			},
			wantErr: "required must be a string array",
		},
		{
			name: "invalid required array element",
			id:   "test-schema",
			opts: []Option{
				WithJSONSchema(map[string]any{
					"type":     "object",
					"required": []any{123, "valid"},
				}),
			},
			wantErr: "required[0] must be a string",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(t.Context(), tt.id, tt.opts...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestNew_NilContext(t *testing.T) {
	t.Run("Should return error for nil context", func(t *testing.T) {
		var nilCtx context.Context
		_, err := New(nilCtx, "test-schema")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "context is required")
	})
}

func TestNew_DeepCopy(t *testing.T) {
	t.Run("Should return deep copied schema", func(t *testing.T) {
		properties := map[string]any{
			"name": map[string]any{"type": "string"},
		}
		schema1, err := New(t.Context(), "test-schema",
			WithJSONSchema(map[string]any{
				"type":       "object",
				"properties": properties,
			}),
		)
		require.NoError(t, err)
		schema1Map := map[string]any(*schema1)
		props1, ok := schema1Map["properties"].(map[string]any)
		require.True(t, ok)
		props1["modified"] = map[string]any{"type": "boolean"}
		schema2, err := New(t.Context(), "test-schema",
			WithJSONSchema(map[string]any{
				"type":       "object",
				"properties": properties,
			}),
		)
		require.NoError(t, err)
		schema2Map := map[string]any(*schema2)
		props2, ok := schema2Map["properties"].(map[string]any)
		require.True(t, ok)
		assert.NotContains(t, props2, "modified")
		assert.Contains(t, props1, "modified")
	})
}

func TestNew_AllSchemaTypes(t *testing.T) {
	validTypes := []string{"object", "string", "number", "integer", "boolean", "array", "null"}
	for _, schemaType := range validTypes {
		t.Run("Should accept "+schemaType+" type", func(t *testing.T) {
			schema, err := New(t.Context(), "test-schema",
				WithJSONSchema(map[string]any{
					"type": schemaType,
				}),
			)
			require.NoError(t, err)
			require.NotNil(t, schema)
			schemaMap := map[string]any(*schema)
			assert.Equal(t, schemaType, schemaMap["type"])
		})
	}
}

func TestNew_ComplexSchema(t *testing.T) {
	t.Run("Should handle complex nested schema", func(t *testing.T) {
		schema, err := New(t.Context(), "complex-schema",
			WithJSONSchema(map[string]any{
				"type":        "object",
				"title":       "Complex Schema",
				"description": "A complex nested schema",
				"properties": map[string]any{
					"user": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"name": map[string]any{
								"type":      "string",
								"minLength": 1,
								"maxLength": 100,
							},
							"email": map[string]any{
								"type":    "string",
								"pattern": "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$",
							},
							"age": map[string]any{
								"type":    "integer",
								"minimum": 0,
								"maximum": 150,
							},
						},
						"required": []string{"name", "email"},
					},
					"tags": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "string",
						},
						"minItems": 1,
					},
				},
				"required": []string{"user"},
			}),
		)
		require.NoError(t, err)
		require.NotNil(t, schema)
		schemaMap := map[string]any(*schema)
		assert.Equal(t, "complex-schema", schemaMap["id"])
		assert.Equal(t, "object", schemaMap["type"])
		assert.Equal(t, "Complex Schema", schemaMap["title"])
		properties, ok := schemaMap["properties"].(map[string]any)
		require.True(t, ok)
		assert.Contains(t, properties, "user")
		assert.Contains(t, properties, "tags")
	})
}

func TestNew_WithVersion(t *testing.T) {
	t.Run("Should preserve custom fields like version", func(t *testing.T) {
		schema, err := New(t.Context(), "versioned-schema",
			WithJSONSchema(map[string]any{
				"type":    "object",
				"version": "1.0.0",
				"properties": map[string]any{
					"data": map[string]any{"type": "string"},
				},
			}),
		)
		require.NoError(t, err)
		require.NotNil(t, schema)
		schemaMap := map[string]any(*schema)
		assert.Equal(t, "versioned-schema", schemaMap["id"])
		assert.Equal(t, "1.0.0", schemaMap["version"])
	})
}
