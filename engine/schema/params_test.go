package schema

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParamsValidator_Validate(t *testing.T) {
	t.Run("Should return error when params are nil but schema is defined", func(t *testing.T) {
		schema := &Schema{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
			"required": []string{"name"},
		}
		validatorID := "test-task-params"

		v := NewParamsValidator[any](nil, schema, validatorID)
		err := v.Validate(context.Background())

		require.Error(t, err)
		assert.ErrorContains(t, err, "parameters are nil but a schema is defined")
		assert.ErrorContains(t, err, validatorID)
	})

	t.Run("Should validate successfully when params match schema", func(t *testing.T) {
		schema := &Schema{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
				"age":  map[string]any{"type": "number"},
			},
			"required": []string{"name"},
		}
		params := map[string]any{
			"name": "John Doe",
			"age":  30,
		}

		v := NewParamsValidator(params, schema, "test-task")
		err := v.Validate(context.Background())

		assert.NoError(t, err)
	})

	t.Run("Should return validation error when params violate schema constraints", func(t *testing.T) {
		schema := &Schema{
			"type": "object",
			"properties": map[string]any{
				"count": map[string]any{
					"type":    "integer",
					"minimum": 1,
				},
			},
			"required": []string{"count"},
		}
		params := map[string]any{
			"count": 0, // Violates minimum constraint
		}
		validatorID := "count-validation-task"

		v := NewParamsValidator(params, schema, validatorID)
		err := v.Validate(context.Background())

		require.Error(t, err)
		assert.ErrorContains(t, err, "validation error")
		assert.ErrorContains(t, err, validatorID)
	})

	t.Run("Should allow validation when schema is nil", func(t *testing.T) {
		params := map[string]any{"any": "data"}

		v := NewParamsValidator(params, nil, "test-task")
		err := v.Validate(context.Background())

		assert.NoError(t, err)
	})

	t.Run("Should handle missing required fields in params", func(t *testing.T) {
		schema := &Schema{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
				"age":  map[string]any{"type": "number"},
			},
			"required": []string{"name", "age"},
		}
		params := map[string]any{
			"name": "John Doe", // Missing required "age" field
		}
		validatorID := "required-fields-task"

		v := NewParamsValidator(params, schema, validatorID)
		err := v.Validate(context.Background())

		require.Error(t, err)
		assert.ErrorContains(t, err, "validation error")
		assert.ErrorContains(t, err, validatorID)
	})
}
