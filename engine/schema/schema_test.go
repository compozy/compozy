package schema

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchema_Validate(t *testing.T) {
	t.Run("Should validate complex workflow configuration with nested objects", func(t *testing.T) {
		schema := &Schema{
			"type": "object",
			"properties": map[string]any{
				"workflow": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{"type": "string"},
						"steps": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"id":   map[string]any{"type": "string"},
									"type": map[string]any{"type": "string"},
								},
								"required": []string{"id", "type"},
							},
						},
					},
					"required": []string{"name", "steps"},
				},
			},
			"required": []string{"workflow"},
		}
		value := map[string]any{
			"workflow": map[string]any{
				"name": "data-processing",
				"steps": []any{
					map[string]any{"id": "step1", "type": "basic"},
					map[string]any{"id": "step2", "type": "parallel"},
				},
			},
		}

		result, err := schema.Validate(context.Background(), value)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Valid)
	})

	t.Run("Should fail validation when required workflow fields are missing", func(t *testing.T) {
		schema := &Schema{
			"type": "object",
			"properties": map[string]any{
				"name":    map[string]any{"type": "string"},
				"version": map[string]any{"type": "string"},
			},
			"required": []string{"name", "version"},
		}
		value := map[string]any{
			"name": "test-workflow", // Missing required "version" field
		}

		result, err := schema.Validate(context.Background(), value)
		require.Error(t, err)
		assert.Nil(t, result)
		assert.ErrorContains(t, err, "schema validation failed")
	})

	t.Run("Should fail validation with specific error for incorrect data types", func(t *testing.T) {
		schema := &Schema{
			"type": "object",
			"properties": map[string]any{
				"timeout": map[string]any{
					"type":    "number",
					"minimum": 1,
				},
				"retries": map[string]any{
					"type":    "integer",
					"minimum": 0,
				},
			},
		}
		value := map[string]any{
			"timeout": "invalid", // Should be number
			"retries": -1,        // Should be >= 0
		}

		result, err := schema.Validate(context.Background(), value)
		require.Error(t, err)
		assert.Nil(t, result)
		assert.ErrorContains(t, err, "schema validation failed")
	})

	t.Run("Should allow validation to pass when schema is nil", func(t *testing.T) {
		var schema *Schema
		value := map[string]any{"any": "data"}

		result, err := schema.Validate(context.Background(), value)
		assert.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("Should handle schema compilation errors gracefully", func(t *testing.T) {
		// Create a schema that will cause validation errors
		schema := &Schema{
			"type":      "string",
			"maxLength": 5,
		}
		value := "this is a very long string that exceeds the limit"

		result, err := schema.Validate(context.Background(), value)
		require.Error(t, err)
		assert.Nil(t, result)
		assert.ErrorContains(t, err, "schema validation failed")
	})
}

func TestSchema_ApplyDefaults(t *testing.T) {
	t.Run("Should merge user input with schema defaults for workflow configuration", func(t *testing.T) {
		schema := &Schema{
			"type": "object",
			"properties": map[string]any{
				"timeout": map[string]any{
					"type":    "number",
					"default": 30,
				},
				"retries": map[string]any{
					"type":    "integer",
					"default": 3,
				},
				"enabled": map[string]any{
					"type":    "boolean",
					"default": true,
				},
				"tags": map[string]any{
					"type":    "array",
					"items":   map[string]any{"type": "string"},
					"default": []any{"workflow", "default"},
				},
			},
		}
		input := map[string]any{
			"timeout": 60,                // User override
			"name":    "custom-workflow", // Additional user field
		}

		result, err := schema.ApplyDefaults(input)
		require.NoError(t, err)

		// User values should take precedence
		assert.Equal(t, 60, result["timeout"])
		assert.Equal(t, "custom-workflow", result["name"])
		// Default values should be applied for missing fields
		assert.Equal(t, 3, result["retries"])
		assert.Equal(t, true, result["enabled"])
		assert.Equal(t, []any{"workflow", "default"}, result["tags"])
	})

	t.Run("Should create complete object from defaults when input is nil", func(t *testing.T) {
		schema := &Schema{
			"type": "object",
			"properties": map[string]any{
				"maxWorkers": map[string]any{
					"type":    "integer",
					"default": 5,
				},
				"queueName": map[string]any{
					"type":    "string",
					"default": "default-queue",
				},
			},
		}

		result, err := schema.ApplyDefaults(nil)
		require.NoError(t, err)

		assert.Equal(t, 5, result["maxWorkers"])
		assert.Equal(t, "default-queue", result["queueName"])
		assert.Len(t, result, 2)
	})

	t.Run("Should preserve input unchanged when schema is nil", func(t *testing.T) {
		var schema *Schema
		input := map[string]any{
			"customField": "customValue",
			"count":       42,
		}

		result, err := schema.ApplyDefaults(input)
		require.NoError(t, err)
		assert.Equal(t, input, result)
	})

	t.Run("Should handle schema with mixed properties with and without defaults", func(t *testing.T) {
		schema := &Schema{
			"type": "object",
			"properties": map[string]any{
				"requiredField": map[string]any{
					"type": "string",
					// No default - should not appear in result unless provided
				},
				"optionalWithDefault": map[string]any{
					"type":    "string",
					"default": "default-value",
				},
				"numericDefault": map[string]any{
					"type":    "number",
					"default": 100,
				},
			},
		}
		input := map[string]any{
			"requiredField": "user-provided",
		}

		result, err := schema.ApplyDefaults(input)
		require.NoError(t, err)

		assert.Equal(t, "user-provided", result["requiredField"])
		assert.Equal(t, "default-value", result["optionalWithDefault"])
		assert.Equal(t, 100, result["numericDefault"])
		assert.Len(t, result, 3)
	})
}

func TestSchema_Compile(t *testing.T) {
	t.Run("Should compile workflow schema with complex validation rules", func(t *testing.T) {
		schema := &Schema{
			"type": "object",
			"properties": map[string]any{
				"workflow": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{
							"type":      "string",
							"minLength": 1,
							"maxLength": 100,
						},
						"version": map[string]any{
							"type":    "string",
							"pattern": "^\\d+\\.\\d+\\.\\d+$",
						},
					},
					"required": []string{"name", "version"},
				},
			},
			"required": []string{"workflow"},
		}

		compiledSchema, err := schema.Compile()
		require.NoError(t, err)
		assert.NotNil(t, compiledSchema)
	})

	t.Run("Should return error for malformed schema", func(t *testing.T) {
		// Create a schema that cannot be marshaled to JSON (circular reference in Go map)
		schema := &Schema{}
		(*schema)["self"] = schema // Create circular reference in the map itself

		compiledSchema, err := schema.Compile()
		require.Error(t, err)
		assert.Nil(t, compiledSchema)
		assert.ErrorContains(t, err, "failed to compile schema")
	})

	t.Run("Should return nil for nil schema without error", func(t *testing.T) {
		var schema *Schema

		compiledSchema, err := schema.Compile()
		assert.NoError(t, err)
		assert.Nil(t, compiledSchema)
	})
}
