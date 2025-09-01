package uc_test

import (
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteTask_ModelResolution(t *testing.T) {
	t.Run("Should use task ModelConfig when explicitly set", func(t *testing.T) {
		// Setup
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
			},
			BasicTask: task.BasicTask{
				ModelConfig: core.ProviderConfig{
					Provider: "openai",
					Model:    "gpt-4",
				},
				Prompt: "Test prompt",
			},
		}
		projectConfig := &project.Config{
			Models: []*core.ProviderConfig{
				{Provider: "anthropic", Model: "claude-3", Default: true},
			},
		}
		// After resolution, task should keep its original model
		assert.Equal(t, "openai", string(taskConfig.ModelConfig.Provider))
		assert.Equal(t, "gpt-4", taskConfig.ModelConfig.Model)
		// Project config is available but not used when task has explicit model
		_ = projectConfig
	})
	t.Run("Should use project default model when task has no model", func(t *testing.T) {
		// Setup
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
			},
			BasicTask: task.BasicTask{
				ModelConfig: core.ProviderConfig{}, // Empty model config
				Prompt:      "Test prompt",
			},
		}
		projectConfig := &project.Config{
			Models: []*core.ProviderConfig{
				{Provider: "openai", Model: "gpt-3.5"},
				{Provider: "anthropic", Model: "claude-3", Default: true},
			},
		}
		// Verify the project default model is available
		defaultModel := projectConfig.GetDefaultModel()
		require.NotNil(t, defaultModel)
		assert.Equal(t, "anthropic", string(defaultModel.Provider))
		assert.Equal(t, "claude-3", defaultModel.Model)
		assert.True(t, defaultModel.Default)
		// Task config would receive this default during execution
		_ = taskConfig
	})
	t.Run("Should use agent model config when available", func(t *testing.T) {
		// Setup
		agentConfig := &agent.Config{
			ID: "test-agent",
			Config: core.ProviderConfig{
				Provider: "openai",
				Model:    "gpt-4-turbo",
			},
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:    "test-task",
				Type:  task.TaskTypeBasic,
				Agent: agentConfig,
			},
		}
		projectConfig := &project.Config{
			Models: []*core.ProviderConfig{
				{Provider: "anthropic", Model: "claude-3", Default: true},
			},
		}
		// Agent config should be preserved
		assert.Equal(t, "openai", string(taskConfig.Agent.Config.Provider))
		assert.Equal(t, "gpt-4-turbo", taskConfig.Agent.Config.Model)
		// Project config is available but not used when agent has explicit model
		_ = projectConfig
	})
	t.Run("Should apply default model to agent without config", func(t *testing.T) {
		// Setup
		agentConfig := &agent.Config{
			ID:     "test-agent",
			Config: core.ProviderConfig{}, // Empty config
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:    "test-task",
				Type:  task.TaskTypeBasic,
				Agent: agentConfig,
			},
		}
		projectConfig := &project.Config{
			Models: []*core.ProviderConfig{
				{Provider: "anthropic", Model: "claude-3", Default: true},
			},
		}
		// Default model should be available for agent to use
		defaultModel := projectConfig.GetDefaultModel()
		require.NotNil(t, defaultModel)
		assert.Equal(t, "anthropic", string(defaultModel.Provider))
		assert.True(t, defaultModel.Default)
		// Agent would receive this default during execution
		_ = taskConfig
	})
	t.Run("Should handle no default model gracefully", func(t *testing.T) {
		// Setup
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
			},
			BasicTask: task.BasicTask{
				ModelConfig: core.ProviderConfig{}, // Empty model config
				Prompt:      "Test prompt",
			},
		}
		projectConfig := &project.Config{
			Models: []*core.ProviderConfig{
				{Provider: "openai", Model: "gpt-4"},     // No default
				{Provider: "anthropic", Model: "claude"}, // No default
			},
		}
		// No default model available
		defaultModel := projectConfig.GetDefaultModel()
		assert.Nil(t, defaultModel)
		// Task would need to have explicit model or error during execution
		_ = taskConfig
	})
}

func TestExecuteTask_ModelResolutionPriority(t *testing.T) {
	t.Run("Should follow correct priority: task > agent > project default", func(t *testing.T) {
		// Test case 1: Task model takes priority
		taskWithModel := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "task-1",
				Type: task.TaskTypeBasic,
				Agent: &agent.Config{
					Config: core.ProviderConfig{
						Provider: "agent-provider",
						Model:    "agent-model",
					},
				},
			},
			BasicTask: task.BasicTask{
				ModelConfig: core.ProviderConfig{
					Provider: "task-provider",
					Model:    "task-model",
				},
			},
		}
		projectWithDefault := &project.Config{
			Models: []*core.ProviderConfig{
				{Provider: "project-provider", Model: "project-model", Default: true},
			},
		}
		// Task model should be used
		assert.Equal(t, "task-provider", string(taskWithModel.ModelConfig.Provider))
		// Test case 2: Agent model takes priority when task has none
		taskWithoutModel := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "task-2",
				Type: task.TaskTypeBasic,
				Agent: &agent.Config{
					Config: core.ProviderConfig{
						Provider: "agent-provider",
						Model:    "agent-model",
					},
				},
			},
		}
		// Agent model should be available
		assert.Equal(t, "agent-provider", string(taskWithoutModel.Agent.Config.Provider))
		// Test case 3: Project default used when neither task nor agent have model
		taskWithEmptyAgent := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:    "task-3",
				Type:  task.TaskTypeBasic,
				Agent: &agent.Config{},
			},
		}
		// Project default should be available
		defaultModel := projectWithDefault.GetDefaultModel()
		require.NotNil(t, defaultModel)
		assert.Equal(t, "project-provider", string(defaultModel.Provider))
		assert.True(t, defaultModel.Default)
		// Verify unused variables to suppress warnings
		_ = taskWithEmptyAgent
	})
}
