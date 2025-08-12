package core_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/compozy/compozy/engine/agent"
	enginecore "github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task2/core"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
)

func TestAgentNormalizer_NormalizeAgent(t *testing.T) {
	t.Run("Should return nil for nil config", func(t *testing.T) {
		// Arrange
		envMerger := core.NewEnvMerger()
		normalizer := core.NewAgentNormalizer(envMerger)
		ctx := &shared.NormalizationContext{}
		// Act
		err := normalizer.NormalizeAgent(nil, ctx, "")
		// Assert
		assert.NoError(t, err)
	})

	t.Run("Should process agent config without errors", func(t *testing.T) {
		// Arrange
		envMerger := core.NewEnvMerger()
		normalizer := core.NewAgentNormalizer(envMerger)
		config := &agent.Config{
			ID:           "test-agent",
			Instructions: "Simple instructions without templates",
			Config: enginecore.ProviderConfig{
				Provider: "openai",
				Model:    "gpt-4",
			},
		}
		ctx := &shared.NormalizationContext{
			WorkflowConfig: &workflow.Config{ID: "test"},
		}
		// Act
		err := normalizer.NormalizeAgent(config, ctx, "")
		// Assert
		assert.NoError(t, err)
		assert.Equal(t, "test-agent", config.ID)
	})

	t.Run("Should set current input from config when nil", func(t *testing.T) {
		// Arrange
		envMerger := core.NewEnvMerger()
		normalizer := core.NewAgentNormalizer(envMerger)
		input := enginecore.NewInput(map[string]any{"key": "value"})
		config := &agent.Config{
			ID:           "test-agent",
			Instructions: "Test",
			Config:       enginecore.ProviderConfig{},
			With:         &input,
		}
		ctx := &shared.NormalizationContext{
			WorkflowConfig: &workflow.Config{ID: "test"},
			CurrentInput:   nil,
		}
		// Act
		err := normalizer.NormalizeAgent(config, ctx, "")
		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, ctx.CurrentInput)
		assert.Equal(t, &input, ctx.CurrentInput)
	})

	t.Run("Should handle agent without actions", func(t *testing.T) {
		// Arrange
		envMerger := core.NewEnvMerger()
		normalizer := core.NewAgentNormalizer(envMerger)
		config := &agent.Config{
			ID:           "test-agent",
			Instructions: "Simple agent",
			Config:       enginecore.ProviderConfig{},
			Actions:      nil,
		}
		ctx := &shared.NormalizationContext{
			WorkflowConfig: &workflow.Config{ID: "test"},
		}
		// Act
		err := normalizer.NormalizeAgent(config, ctx, "action-id")
		// Assert
		assert.NoError(t, err)
	})
}

func TestAgentNormalizer_TemplateProcessing(t *testing.T) {
	t.Run("Should process template placeholders in agent instructions", func(t *testing.T) {
		// Arrange
		envMerger := core.NewEnvMerger()
		normalizer := core.NewAgentNormalizer(envMerger)
		config := &agent.Config{
			ID:           "test-agent",
			Instructions: "Process {{.input.name}} with value {{.input.value}}",
			Config: enginecore.ProviderConfig{
				Provider: "openai",
				Model:    "gpt-4",
			},
		}
		input := enginecore.NewInput(map[string]any{
			"name":  "testItem",
			"value": "testValue",
		})
		ctx := &shared.NormalizationContext{
			WorkflowConfig: &workflow.Config{ID: "test"},
			CurrentInput:   &input,
			Variables: map[string]any{
				"input": map[string]any{
					"name":  "testItem",
					"value": "testValue",
				},
			},
		}
		// Act
		err := normalizer.NormalizeAgent(config, ctx, "")
		// Assert
		assert.NoError(t, err)
		assert.Contains(t, config.Instructions, "testItem")
		assert.Contains(t, config.Instructions, "testValue")
		assert.NotContains(t, config.Instructions, "{{.input.name}}")
		assert.NotContains(t, config.Instructions, "{{.input.value}}")
	})

	t.Run("Should handle template processing with missing fields gracefully", func(t *testing.T) {
		// Arrange
		envMerger := core.NewEnvMerger()
		normalizer := core.NewAgentNormalizer(envMerger)
		config := &agent.Config{
			ID:           "test-agent",
			Instructions: "Template with {{.input.name}} and missing {{.input.missing}}",
			Config:       enginecore.ProviderConfig{},
		}
		input := enginecore.NewInput(map[string]any{
			"name": "testItem",
		})
		ctx := &shared.NormalizationContext{
			WorkflowConfig: &workflow.Config{ID: "test"},
			CurrentInput:   &input,
			Variables: map[string]any{
				"input": map[string]any{
					"name": "testItem",
				},
			},
		}
		// Act
		err := normalizer.NormalizeAgent(config, ctx, "")
		// Assert
		// Template processing with missing fields should cause an error
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "template execution error")
	})
}
