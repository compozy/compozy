package schema

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_SchemaValidate(t *testing.T) {
	t.Run("Should validate valid string value", func(t *testing.T) {
		schema := &Schema{
			"type": "string",
		}
		_, err := schema.Validate(context.Background(), "test")
		assert.NoError(t, err)
	})

	t.Run("Should return error for invalid string value", func(t *testing.T) {
		schema := &Schema{
			"type": "string",
		}
		_, err := schema.Validate(context.Background(), 123)
		assert.Error(t, err)
	})

	t.Run("Should validate valid number value", func(t *testing.T) {
		schema := &Schema{
			"type": "number",
		}
		_, err := schema.Validate(context.Background(), 123.45)
		assert.NoError(t, err)
	})

	t.Run("Should return error for invalid number value", func(t *testing.T) {
		schema := &Schema{
			"type": "number",
		}
		_, err := schema.Validate(context.Background(), "test")
		assert.Error(t, err)
	})

	t.Run("Should validate valid object value", func(t *testing.T) {
		schema := &Schema{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type": "string",
				},
				"age": map[string]any{
					"type": "number",
				},
			},
			"required": []string{"name"},
		}
		value := map[string]any{
			"name": "John",
			"age":  30,
		}
		_, err := schema.Validate(context.Background(), value)
		assert.NoError(t, err)
	})

	t.Run("Should return error for object missing required field", func(t *testing.T) {
		schema := &Schema{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type": "string",
				},
			},
			"required": []string{"name"},
		}
		_, err := schema.Validate(context.Background(), map[string]any{})
		assert.Error(t, err)
	})

	t.Run("Should return error for object with wrong field type", func(t *testing.T) {
		schema := &Schema{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type": "string",
				},
			},
		}
		value := map[string]any{
			"name": 123,
		}
		_, err := schema.Validate(context.Background(), value)
		assert.Error(t, err)
	})

	t.Run("Should validate valid array value", func(t *testing.T) {
		schema := &Schema{
			"type": "array",
			"items": map[string]any{
				"type": "string",
			},
		}
		value := []any{"a", "b", "c"}
		_, err := schema.Validate(context.Background(), value)
		assert.NoError(t, err)
	})

	t.Run("Should return error for array with invalid item type", func(t *testing.T) {
		schema := &Schema{
			"type": "array",
			"items": map[string]any{
				"type": "string",
			},
		}
		value := []any{"a", 2, "c"}
		_, err := schema.Validate(context.Background(), value)
		assert.Error(t, err)
	})

	t.Run("Should handle nil schema", func(t *testing.T) {
		var schema *Schema
		_, err := schema.Validate(context.Background(), "test")
		assert.NoError(t, err)
	})
}

func TestSchema_ApplyDefaults(t *testing.T) {
	t.Run("Should apply defaults for missing properties", func(t *testing.T) {
		schema := &Schema{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":    "string",
					"default": "John Doe",
				},
				"age": map[string]any{
					"type":    "number",
					"default": 30,
				},
				"active": map[string]any{
					"type":    "boolean",
					"default": true,
				},
				"tags": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "string",
					},
					"default": []any{"default", "tag"},
				},
			},
		}

		input := map[string]any{
			"name": "Jane Smith", // This should override the default
		}

		result, err := schema.ApplyDefaults(input)
		require.NoError(t, err)

		// Should have user-provided value (override)
		assert.Equal(t, "Jane Smith", result["name"])

		// Should have default values for missing properties
		assert.Equal(t, 30, result["age"])
		assert.Equal(t, true, result["active"])
		assert.Equal(t, []any{"default", "tag"}, result["tags"])
	})

	t.Run("Should handle nil input", func(t *testing.T) {
		schema := &Schema{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":    "string",
					"default": "Default Name",
				},
			},
		}

		result, err := schema.ApplyDefaults(nil)
		require.NoError(t, err)

		assert.Equal(t, "Default Name", result["name"])
	})

	t.Run("Should handle nil schema", func(t *testing.T) {
		var schema *Schema

		input := map[string]any{
			"test": "value",
		}

		result, err := schema.ApplyDefaults(input)
		require.NoError(t, err)
		assert.Equal(t, input, result)
	})

	t.Run("Should handle schema without properties", func(t *testing.T) {
		schema := &Schema{
			"type": "string",
		}

		input := map[string]any{
			"test": "value",
		}

		result, err := schema.ApplyDefaults(input)
		require.NoError(t, err)
		assert.Equal(t, input, result)
	})

	t.Run("Should handle properties without defaults", func(t *testing.T) {
		schema := &Schema{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type": "string",
					// no default
				},
				"age": map[string]any{
					"type":    "number",
					"default": 25,
				},
			},
		}

		input := map[string]any{
			"name": "Test User",
		}

		result, err := schema.ApplyDefaults(input)
		require.NoError(t, err)

		assert.Equal(t, "Test User", result["name"])
		assert.Equal(t, 25, result["age"])
		assert.NotContains(t, result, "undefined_property")
	})
}

func TestSchema_Compile(t *testing.T) {
	t.Run("Should successfully compile valid schema", func(t *testing.T) {
		s := &Schema{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type": "string",
				},
			},
		}

		schema, err := s.Compile()
		assert.NoError(t, err)
		assert.NotNil(t, schema)
	})

	t.Run("Should return nil for nil schema", func(t *testing.T) {
		var s *Schema

		schema, err := s.Compile()
		assert.NoError(t, err)
		assert.Nil(t, schema)
	})
}

func TestSchema_Validate(t *testing.T) {
	t.Run("Should validate correct data", func(t *testing.T) {
		s := &Schema{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type": "string",
				},
			},
			"required": []string{"name"},
		}

		data := map[string]any{
			"name": "John",
		}

		result, err := s.Validate(context.Background(), data)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Valid)
	})

	t.Run("Should return error for invalid data", func(t *testing.T) {
		s := &Schema{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type": "string",
				},
			},
			"required": []string{"name"},
		}

		data := map[string]any{
			"age": 30, // missing required "name"
		}

		result, err := s.Validate(context.Background(), data)
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}
