package core_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	tmplcore "github.com/compozy/compozy/engine/task2/core"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
)

// mockTemplateEngine for testing
type mockTemplateEngine struct{}

func (m *mockTemplateEngine) Process(template string, _ map[string]any) (string, error) {
	return template, nil
}

func (m *mockTemplateEngine) ProcessMap(data map[string]any, _ map[string]any) (map[string]any, error) {
	return data, nil
}

func (m *mockTemplateEngine) ProcessSlice(slice []any, _ map[string]any) ([]any, error) {
	return slice, nil
}

func (m *mockTemplateEngine) ProcessString(templateStr string, _ map[string]any) (*shared.ProcessResult, error) {
	return &shared.ProcessResult{Text: templateStr}, nil
}

func (m *mockTemplateEngine) ParseMapWithFilter(
	data map[string]any,
	_ map[string]any,
	_ func(string) bool,
) (map[string]any, error) {
	// Simple mock that just returns the data unchanged
	return data, nil
}

func (m *mockTemplateEngine) ParseMap(data map[string]any, _ map[string]any) (map[string]any, error) {
	return data, nil
}

func (m *mockTemplateEngine) ParseValue(value any, _ map[string]any) (any, error) {
	return value, nil
}

func TestToolNormalizer_NewToolNormalizer(t *testing.T) {
	t.Run("Should create tool normalizer with dependencies", func(t *testing.T) {
		// Arrange
		templateEngine := &mockTemplateEngine{}
		envMerger := tmplcore.NewEnvMerger()

		// Act
		normalizer := tmplcore.NewToolNormalizer(templateEngine, envMerger)

		// Assert
		assert.NotNil(t, normalizer)
	})
}

func TestToolNormalizer_NormalizeTool(t *testing.T) {
	// Setup
	templateEngine := &mockTemplateEngine{}
	envMerger := tmplcore.NewEnvMerger()
	normalizer := tmplcore.NewToolNormalizer(templateEngine, envMerger)

	t.Run("Should merge environment variables across three levels", func(t *testing.T) {
		// Arrange
		workflowConfig := &workflow.Config{
			ID: "test-workflow",
			Opts: workflow.Opts{
				Env: &core.EnvMap{
					"WORKFLOW_VAR": "workflow_value",
					"SHARED_VAR":   "workflow_shared",
				},
			},
		}

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
				Env: &core.EnvMap{
					"TASK_VAR":   "task_value",
					"SHARED_VAR": "task_shared", // Overrides workflow
				},
			},
		}

		toolConfig := &tool.Config{
			ID:          "test-tool",
			Description: "Test tool",
			Execute:     "echo hello",
			Env: &core.EnvMap{
				"TOOL_VAR":   "tool_value",
				"SHARED_VAR": "tool_shared", // Overrides task and workflow
			},
		}

		ctx := &shared.NormalizationContext{
			WorkflowConfig: workflowConfig,
			TaskConfig:     taskConfig,
			Variables:      make(map[string]any),
		}

		// Act
		err := normalizer.NormalizeTool(toolConfig, ctx)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, ctx.MergedEnv)

		// Check that tool environment has highest priority
		assert.Equal(t, "tool_shared", (*ctx.MergedEnv)["SHARED_VAR"])
		assert.Equal(t, "tool_value", (*ctx.MergedEnv)["TOOL_VAR"])
		assert.Equal(t, "task_value", (*ctx.MergedEnv)["TASK_VAR"])
		assert.Equal(t, "workflow_value", (*ctx.MergedEnv)["WORKFLOW_VAR"])
	})

	t.Run("Should handle nil tool config", func(t *testing.T) {
		// Arrange
		ctx := &shared.NormalizationContext{}

		// Act
		err := normalizer.NormalizeTool(nil, ctx)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("Should set current input from tool config", func(t *testing.T) {
		// Arrange
		toolConfig := &tool.Config{
			ID:          "test-tool",
			Description: "Test tool",
			Execute:     "echo hello",
			With: &core.Input{
				"param": "value",
			},
		}

		ctx := &shared.NormalizationContext{
			Variables: make(map[string]any),
		}

		// Act
		err := normalizer.NormalizeTool(toolConfig, ctx)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, toolConfig.With, ctx.CurrentInput)
	})

	t.Run("Should not override existing current input", func(t *testing.T) {
		// Arrange
		existingInput := &core.Input{"existing": "value"}
		toolConfig := &tool.Config{
			ID:          "test-tool",
			Description: "Test tool",
			Execute:     "echo hello",
			With: &core.Input{
				"new": "value",
			},
		}

		ctx := &shared.NormalizationContext{
			CurrentInput: existingInput,
			Variables:    make(map[string]any),
		}

		// Act
		err := normalizer.NormalizeTool(toolConfig, ctx)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, existingInput, ctx.CurrentInput)
	})

	t.Run("Should handle tool with nil environment", func(t *testing.T) {
		// Arrange
		workflowConfig := &workflow.Config{
			ID: "test-workflow",
			Opts: workflow.Opts{
				Env: &core.EnvMap{
					"WORKFLOW_VAR": "workflow_value",
				},
			},
		}

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
				Env: &core.EnvMap{
					"TASK_VAR": "task_value",
				},
			},
		}

		toolConfig := &tool.Config{
			ID:          "test-tool",
			Description: "Test tool",
			Execute:     "echo hello",
			Env:         nil, // No tool-specific environment
		}

		ctx := &shared.NormalizationContext{
			WorkflowConfig: workflowConfig,
			TaskConfig:     taskConfig,
			Variables:      make(map[string]any),
		}

		// Act
		err := normalizer.NormalizeTool(toolConfig, ctx)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, ctx.MergedEnv)

		// Should only have workflow and task environment
		assert.Equal(t, "workflow_value", (*ctx.MergedEnv)["WORKFLOW_VAR"])
		assert.Equal(t, "task_value", (*ctx.MergedEnv)["TASK_VAR"])
	})
}
