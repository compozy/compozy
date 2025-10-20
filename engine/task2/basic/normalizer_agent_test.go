package basic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/basic"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/pkg/tplengine"
)

// setupBasicNormalizer creates a new basic normalizer with a template engine for testing
func setupBasicNormalizer(t *testing.T) *basic.Normalizer {
	t.Helper()
	templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
	return basic.NewNormalizer(t.Context(), templateEngine)
}

// setupNormalizationContext creates a normalization context with the given variables
func setupNormalizationContext(variables map[string]any) *shared.NormalizationContext {
	return &shared.NormalizationContext{
		Variables: variables,
	}
}

func TestBasicNormalizer_AgentNormalization(t *testing.T) {
	t.Run("Should normalize agent config with API key template", func(t *testing.T) {
		// Arrange
		normalizer := setupBasicNormalizer(t)

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
				Agent: &agent.Config{
					ID: "tourist_guide",
					Model: agent.Model{Config: core.ProviderConfig{
						Provider: "groq",
						Model:    "llama3-70b-8192",
						APIKey:   "{{ .env.GROQ_API_KEY }}",
					}},
					Instructions: "You are a helpful weather assistant",
				},
			},
			BasicTask: task.BasicTask{
				Action: "get_weather",
			},
		}

		ctx := setupNormalizationContext(map[string]any{
			"env": map[string]any{
				"GROQ_API_KEY": "test-api-key-12345",
			},
		})

		// Act
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, taskConfig.Agent)

		// Check that the API key template was evaluated
		assert.Equal(
			t,
			"test-api-key-12345",
			taskConfig.Agent.Model.Config.APIKey,
			"API key template should be evaluated",
		)
	})

	t.Run("Should handle agent config with multiple template fields", func(t *testing.T) {
		// Arrange
		normalizer := setupBasicNormalizer(t)

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
				Agent: &agent.Config{
					ID: "assistant",
					Model: agent.Model{Config: core.ProviderConfig{
						Provider: "openai",
						Model:    "{{ .model_name }}",
						APIKey:   "{{ .env.OPENAI_API_KEY }}",
						Params: core.PromptParams{
							Temperature: 0.7,  // Will be overridden by template
							MaxTokens:   1000, // Will be overridden by template
						},
					}},
					Instructions: "{{ .instructions }}",
				},
			},
			BasicTask: task.BasicTask{
				Action: "process_data",
			},
		}

		ctx := setupNormalizationContext(map[string]any{
			"env": map[string]any{
				"OPENAI_API_KEY": "sk-test-key",
			},
			"model_name":   "gpt-4",
			"instructions": "You are a data processing assistant",
		})

		// Act
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, taskConfig.Agent)
		assert.Equal(t, "gpt-4", taskConfig.Agent.Model.Config.Model)
		assert.Equal(t, "sk-test-key", taskConfig.Agent.Model.Config.APIKey)
		assert.Equal(t, "You are a data processing assistant", taskConfig.Agent.Instructions)
	})

	t.Run("Should handle missing agent config gracefully", func(t *testing.T) {
		// Arrange
		normalizer := setupBasicNormalizer(t)

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
			},
			BasicTask: task.BasicTask{
				Action: "simple_action",
			},
			// No agent config
		}

		ctx := setupNormalizationContext(map[string]any{})

		// Act
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)

		// Assert
		assert.NoError(t, err)
		assert.Nil(t, taskConfig.Agent)
	})

	t.Run("Should preserve agent ID when using $use directive", func(t *testing.T) {
		// Arrange
		normalizer := setupBasicNormalizer(t)

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
				Agent: &agent.Config{
					ID: "tourist_guide", // This would be set by $use
					Model: agent.Model{Config: core.ProviderConfig{
						Provider: "groq",
						Model:    "llama3-70b-8192",
						APIKey:   "{{ .env.GROQ_API_KEY }}",
					}},
					Instructions: "You are a tourist guide",
				},
			},
			BasicTask: task.BasicTask{
				Action: "get_weather",
			},
		}

		ctx := setupNormalizationContext(map[string]any{
			"env": map[string]any{
				"GROQ_API_KEY": "test-key",
			},
		})

		// Act
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "tourist_guide", taskConfig.Agent.ID)
		assert.Equal(t, "test-key", taskConfig.Agent.Model.Config.APIKey)
	})
}
