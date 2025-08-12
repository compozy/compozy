package worker

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
		require.NoError(t, err)

		payload := core.Input{
			"age": 30,
		}
		isValid, validationErrors := ValidatePayloadAgainstCompiledSchema(payload, compiledSchema)
		assert.False(t, isValid)
		require.NotEmpty(t, validationErrors)
		assert.Contains(t, validationErrors[0], "name")
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
		require.NoError(t, err)

		payload := core.Input{
			"name": "John Doe",
			"age":  "thirty", // Wrong type - should be number
		}
		isValid, validationErrors := ValidatePayloadAgainstCompiledSchema(payload, compiledSchema)
		assert.False(t, isValid)
		require.NotEmpty(t, validationErrors)
		assert.Contains(t, validationErrors[0], "age")
	})

	t.Run("Should handle empty payload with complex schema", func(t *testing.T) {
		schemaDefinition := &schema.Schema{
			"type": "object",
			"properties": map[string]any{
				"user": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id": map[string]any{"type": "string"},
						"profile": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"email": map[string]any{"type": "string"},
								"preferences": map[string]any{
									"type":  "array",
									"items": map[string]any{"type": "string"},
								},
							},
							"required": []string{"email"},
						},
					},
					"required": []string{"id", "profile"},
				},
			},
			"required": []string{"user"},
		}
		compiledSchema, err := schemaDefinition.Compile()
		require.NoError(t, err)

		payload := core.Input{} // Empty payload
		isValid, validationErrors := ValidatePayloadAgainstCompiledSchema(payload, compiledSchema)

		assert.False(t, isValid)
		require.NotEmpty(t, validationErrors)
		assert.Contains(t, validationErrors[0], "user")
		// Accept either error message format depending on jsonschema version
		userErrorContains := strings.Contains(validationErrors[0], "Required property 'user' is missing") ||
			strings.Contains(validationErrors[0], "Property 'user' does not match the schema")
		assert.True(t, userErrorContains, "Expected error to mention user property validation failure")
	})

	t.Run("Should validate enum constraints in schema", func(t *testing.T) {
		// Test that enum validation works correctly for both valid and invalid values
		validSchema := &schema.Schema{
			"type": "object",
			"properties": map[string]any{
				"status": map[string]any{"type": "string", "enum": []string{"active", "inactive"}},
			},
			"required": []string{"status"},
		}
		compiledSchema, err := validSchema.Compile()
		require.NoError(t, err)

		// Test valid enum value
		validPayload := core.Input{"status": "active"}
		isValid, validationErrors := ValidatePayloadAgainstCompiledSchema(validPayload, compiledSchema)
		assert.True(t, isValid)
		assert.Nil(t, validationErrors)

		// Test invalid enum value
		invalidPayload := core.Input{"status": "pending"}
		isValid, validationErrors = ValidatePayloadAgainstCompiledSchema(invalidPayload, compiledSchema)
		assert.False(t, isValid)
		require.NotEmpty(t, validationErrors)
		assert.Contains(t, validationErrors[0], "status")
		assert.Contains(t, validationErrors[0], "Property 'status' does not match the schema")
	})
}
