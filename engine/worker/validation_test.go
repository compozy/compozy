package worker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
)

func TestValidatePayloadAgainstCompiledSchema(t *testing.T) {
	t.Run("Should pass validation with no schema", func(t *testing.T) {
		payload := core.Input{"test": "value"}
		isValid, validationErrors := ValidatePayloadAgainstCompiledSchema(payload, nil)
		assert.True(t, isValid)
		assert.Nil(t, validationErrors)
	})

	t.Run("Should pass validation with valid payload", func(t *testing.T) {
		schemaDefinition := &schema.Schema{
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
		compiledSchema, err := schemaDefinition.Compile()
		assert.NoError(t, err)

		payload := core.Input{
			"name": "John Doe",
			"age":  30,
		}
		isValid, validationErrors := ValidatePayloadAgainstCompiledSchema(payload, compiledSchema)
		assert.True(t, isValid)
		assert.Nil(t, validationErrors)
	})

	t.Run("Should fail validation with missing required field", func(t *testing.T) {
		schemaDefinition := &schema.Schema{
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
		compiledSchema, err := schemaDefinition.Compile()
		assert.NoError(t, err)

		payload := core.Input{
			"age": 30,
		}
		isValid, validationErrors := ValidatePayloadAgainstCompiledSchema(payload, compiledSchema)
		assert.False(t, isValid)
		assert.NotEmpty(t, validationErrors)
	})

	t.Run("Should fail validation with wrong type", func(t *testing.T) {
		schemaDefinition := &schema.Schema{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type": "string",
				},
				"age": map[string]any{
					"type": "number",
				},
			},
		}
		compiledSchema, err := schemaDefinition.Compile()
		assert.NoError(t, err)

		payload := core.Input{
			"name": "John Doe",
			"age":  "thirty", // Wrong type - should be number
		}
		isValid, validationErrors := ValidatePayloadAgainstCompiledSchema(payload, compiledSchema)
		assert.False(t, isValid)
		assert.NotEmpty(t, validationErrors)
	})
}
