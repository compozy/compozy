package core_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	tmplcore "github.com/compozy/compozy/engine/task2/core"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
)

func TestAgentNormalizer_NewAgentNormalizer(t *testing.T) {
	t.Run("Should create agent normalizer with dependencies", func(t *testing.T) {
		// Arrange
		templateEngine := &mockTemplateEngine{}
		envMerger := tmplcore.NewEnvMerger()

		// Act
		normalizer := tmplcore.NewAgentNormalizer(templateEngine, envMerger)

		// Assert
		assert.NotNil(t, normalizer)
	})
}

func TestAgentNormalizer_NormalizeAgent(t *testing.T) {
	// Setup
	templateEngine := &mockTemplateEngine{}
	envMerger := tmplcore.NewEnvMerger()
	normalizer := tmplcore.NewAgentNormalizer(templateEngine, envMerger)

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

		agentConfig := &agent.Config{
			ID:           "test-agent",
			Instructions: "Test instructions",
			Config:       core.ProviderConfig{},
			Env: &core.EnvMap{
				"AGENT_VAR":  "agent_value",
				"SHARED_VAR": "agent_shared", // Overrides task and workflow
			},
		}

		ctx := &shared.NormalizationContext{
			WorkflowConfig: workflowConfig,
			TaskConfig:     taskConfig,
			Variables:      make(map[string]any),
		}

		// Act
		err := normalizer.NormalizeAgent(agentConfig, ctx, "")

		// Assert
		require.NoError(t, err)
		require.NotNil(t, ctx.MergedEnv)

		// Check that agent environment has highest priority
		assert.Equal(t, "agent_shared", (*ctx.MergedEnv)["SHARED_VAR"])
		assert.Equal(t, "agent_value", (*ctx.MergedEnv)["AGENT_VAR"])
		assert.Equal(t, "task_value", (*ctx.MergedEnv)["TASK_VAR"])
		assert.Equal(t, "workflow_value", (*ctx.MergedEnv)["WORKFLOW_VAR"])
	})

	t.Run("Should handle nil agent config", func(t *testing.T) {
		// Arrange
		ctx := &shared.NormalizationContext{}

		// Act
		err := normalizer.NormalizeAgent(nil, ctx, "")

		// Assert
		assert.NoError(t, err)
	})

	t.Run("Should set current input from agent config", func(t *testing.T) {
		// Arrange
		agentConfig := &agent.Config{
			ID:           "test-agent",
			Instructions: "Test instructions",
			Config:       core.ProviderConfig{},
			With: &core.Input{
				"param": "value",
			},
		}

		ctx := &shared.NormalizationContext{
			Variables: make(map[string]any),
		}

		// Act
		err := normalizer.NormalizeAgent(agentConfig, ctx, "")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, agentConfig.With, ctx.CurrentInput)
	})

	t.Run("Should not override existing current input", func(t *testing.T) {
		// Arrange
		existingInput := &core.Input{"existing": "value"}
		agentConfig := &agent.Config{
			ID:           "test-agent",
			Instructions: "Test instructions",
			Config:       core.ProviderConfig{},
			With: &core.Input{
				"new": "value",
			},
		}

		ctx := &shared.NormalizationContext{
			CurrentInput: existingInput,
			Variables:    make(map[string]any),
		}

		// Act
		err := normalizer.NormalizeAgent(agentConfig, ctx, "")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, existingInput, ctx.CurrentInput)
	})

	t.Run("Should handle agent with nil environment", func(t *testing.T) {
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

		agentConfig := &agent.Config{
			ID:           "test-agent",
			Instructions: "Test instructions",
			Config:       core.ProviderConfig{},
			Env:          nil, // No agent-specific environment
		}

		ctx := &shared.NormalizationContext{
			WorkflowConfig: workflowConfig,
			TaskConfig:     taskConfig,
			Variables:      make(map[string]any),
		}

		// Act
		err := normalizer.NormalizeAgent(agentConfig, ctx, "")

		// Assert
		require.NoError(t, err)
		require.NotNil(t, ctx.MergedEnv)

		// Should only have workflow and task environment
		assert.Equal(t, "workflow_value", (*ctx.MergedEnv)["WORKFLOW_VAR"])
		assert.Equal(t, "task_value", (*ctx.MergedEnv)["TASK_VAR"])
	})

	t.Run("Should handle agent actions normalization", func(t *testing.T) {
		// Arrange
		agentConfig := &agent.Config{
			ID:           "test-agent",
			Instructions: "Test instructions",
			Config:       core.ProviderConfig{},
			Actions: []*agent.ActionConfig{
				{
					ID:     "action1",
					Prompt: "Test action prompt",
					With: &core.Input{
						"action_param": "action_value",
					},
				},
			},
		}

		ctx := &shared.NormalizationContext{
			Variables: make(map[string]any),
		}

		// Act
		err := normalizer.NormalizeAgent(agentConfig, ctx, "action1")

		// Assert
		require.NoError(t, err)
		// The action should have been processed
		assert.Len(t, agentConfig.Actions, 1)
	})
}
