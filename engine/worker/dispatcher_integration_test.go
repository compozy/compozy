package worker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	wf "github.com/compozy/compozy/engine/workflow"
)

// TestDispatcherWorkflow_PayloadValidationLogic tests the validation logic without full workflow setup
func TestDispatcherWorkflow_PayloadValidationLogic(t *testing.T) {
	t.Run("Should create compiled triggers with schema", func(t *testing.T) {
		schemaDefinition := &schema.Schema{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
			"required": []string{"name"},
		}

		compiled, err := schemaDefinition.Compile()
		assert.NoError(t, err)
		assert.NotNil(t, compiled)

		trigger := &compiledTrigger{
			config: &wf.Config{ID: "test-workflow"},
			trigger: &wf.Trigger{
				Type:   wf.TriggerTypeSignal,
				Name:   "test-signal",
				Schema: schemaDefinition,
			},
			compiledSchema: compiled,
		}

		assert.Equal(t, "test-workflow", trigger.config.ID)
		assert.Equal(t, "test-signal", trigger.trigger.Name)
		assert.NotNil(t, trigger.compiledSchema)
	})

	t.Run("Should validate payload with compiled schema", func(t *testing.T) {
		schemaDefinition := &schema.Schema{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
			"required": []string{"name"},
		}

		compiled, err := schemaDefinition.Compile()
		assert.NoError(t, err)

		// Test valid payload
		validPayload := core.Input{"name": "John"}
		isValid, errors := validatePayloadAgainstCompiledSchema(validPayload, compiled)
		assert.True(t, isValid)
		assert.Nil(t, errors)

		// Test invalid payload
		invalidPayload := core.Input{"age": 30} // missing required "name"
		isValid, errors = validatePayloadAgainstCompiledSchema(invalidPayload, compiled)
		assert.False(t, isValid)
		assert.NotEmpty(t, errors)
	})
}
