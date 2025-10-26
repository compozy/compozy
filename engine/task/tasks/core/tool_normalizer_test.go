package core_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	enginecore "github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/tasks/core"
	"github.com/compozy/compozy/engine/task/tasks/shared"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
)

func TestToolNormalizer_NormalizeTool(t *testing.T) {
	t.Run("Should return nil for nil config", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		envMerger := core.NewEnvMerger()
		normalizer := core.NewToolNormalizer(templateEngine, envMerger)
		ctx := &shared.NormalizationContext{}
		// Act
		err := normalizer.NormalizeTool(nil, ctx)
		// Assert
		assert.NoError(t, err)
	})

	t.Run("Should normalize tool config successfully", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		envMerger := core.NewEnvMerger()
		normalizer := core.NewToolNormalizer(templateEngine, envMerger)
		config := &tool.Config{
			ID:          "test-tool-id",
			Description: "Test tool",
		}
		ctx := &shared.NormalizationContext{
			WorkflowState: &workflow.State{
				WorkflowID:     "test-workflow",
				WorkflowExecID: enginecore.MustNewID(),
			},
			WorkflowConfig: &workflow.Config{
				ID: "test-workflow",
			},
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID: "test-task",
				},
			},
			Variables: map[string]any{
				"name": "world",
			},
		}
		// Act
		err := normalizer.NormalizeTool(config, ctx)
		// Assert
		assert.NoError(t, err)
		// Verify config was processed
		assert.Equal(t, "test-tool-id", config.ID)
	})

	t.Run("Should merge environment variables", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		envMerger := core.NewEnvMerger()
		normalizer := core.NewToolNormalizer(templateEngine, envMerger)
		config := &tool.Config{
			ID: "test-tool-id",
			Env: &enginecore.EnvMap{
				"TOOL_VAR": "tool_value",
			},
		}
		workflowEnv := &enginecore.EnvMap{
			"WORKFLOW_VAR": "workflow_value",
		}
		taskEnv := &enginecore.EnvMap{
			"TASK_VAR": "task_value",
		}
		workflowConfig := &workflow.Config{
			ID: "test-workflow",
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:  "dummy-task",
						Env: workflowEnv,
					},
				},
			},
		}
		ctx := &shared.NormalizationContext{
			WorkflowConfig: workflowConfig,
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:  "test-task",
					Env: taskEnv,
				},
			},
		}
		// Act
		err := normalizer.NormalizeTool(config, ctx)
		// Assert
		assert.NoError(t, err)
		// Verify merged env
		assert.NotNil(t, ctx.MergedEnv)
		assert.Equal(t, "tool_value", (*ctx.MergedEnv)["TOOL_VAR"])
	})

	t.Run("Should set current input from config when nil", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		envMerger := core.NewEnvMerger()
		normalizer := core.NewToolNormalizer(templateEngine, envMerger)
		input := enginecore.NewInput(map[string]any{"key": "value"})
		config := &tool.Config{
			ID:   "test-tool-id",
			With: &input,
		}
		ctx := &shared.NormalizationContext{
			WorkflowConfig: &workflow.Config{ID: "test"},
			TaskConfig:     &task.Config{BaseConfig: task.BaseConfig{ID: "test"}},
			CurrentInput:   nil,
		}
		// Act
		err := normalizer.NormalizeTool(config, ctx)
		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, ctx.CurrentInput)
		assert.Equal(t, &input, ctx.CurrentInput)
	})

	t.Run("Should not override existing current input", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		envMerger := core.NewEnvMerger()
		normalizer := core.NewToolNormalizer(templateEngine, envMerger)
		existingInput := enginecore.NewInput(map[string]any{"existing": "input"})
		configInput := enginecore.NewInput(map[string]any{"config": "input"})
		config := &tool.Config{
			ID:   "test-tool-id",
			With: &configInput,
		}
		ctx := &shared.NormalizationContext{
			WorkflowConfig: &workflow.Config{ID: "test"},
			TaskConfig:     &task.Config{BaseConfig: task.BaseConfig{ID: "test"}},
			CurrentInput:   &existingInput,
		}
		// Act
		err := normalizer.NormalizeTool(config, ctx)
		// Assert
		assert.NoError(t, err)
		// Should keep existing input
		assert.Equal(t, &existingInput, ctx.CurrentInput)
	})

	t.Run("Should handle tool without spec", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		envMerger := core.NewEnvMerger()
		normalizer := core.NewToolNormalizer(templateEngine, envMerger)
		config := &tool.Config{
			ID: "test-tool-id",
		}
		ctx := &shared.NormalizationContext{
			WorkflowConfig: &workflow.Config{ID: "test"},
			TaskConfig:     &task.Config{BaseConfig: task.BaseConfig{ID: "test"}},
		}
		// Act
		err := normalizer.NormalizeTool(config, ctx)
		// Assert
		assert.NoError(t, err)
	})
}
