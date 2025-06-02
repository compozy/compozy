package schema

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
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
