package agent

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTest(t *testing.T, agentFile string) (*core.PathCWD, string) {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	cwd, dstPath := utils.SetupTest(t, filename)
	dstPath = filepath.Join(dstPath, agentFile)
	return cwd, dstPath
}

func Test_LoadAgent(t *testing.T) {
	t.Run("Should load basic agent configuration correctly", func(t *testing.T) {
		CWD, dstPath := setupTest(t, "basic_agent.yaml")
		config, err := Load(CWD, dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		require.NotNil(t, config.ID)
		require.NotNil(t, config.Config)
		require.NotNil(t, config.Config.Params.Temperature)
		require.NotNil(t, config.Config.Params.MaxTokens)

		assert.Equal(t, "code-assistant", config.ID)
		assert.Equal(t, core.ProviderAnthropic, config.Config.Provider)
		assert.Equal(t, "claude-3-opus-20240229", config.Config.Model)
		assert.InDelta(t, float32(0.7), config.Config.Params.Temperature, 0.0001)
		assert.Equal(t, int32(4000), config.Config.Params.MaxTokens)

		require.Len(t, config.Actions, 1)
		action := config.Actions[0]
		assert.Equal(t, "review-code", action.ID)

		require.NotNil(t, action.InputSchema)
		schema := action.InputSchema
		compiledSchema, err := schema.Compile()
		require.NoError(t, err)
		assert.Equal(t, []string{"object"}, []string(compiledSchema.Type))
		require.NotNil(t, compiledSchema.Properties)
		assert.Contains(t, (*compiledSchema.Properties), "code")
		assert.Contains(t, (*compiledSchema.Properties), "language")
		assert.Contains(t, compiledSchema.Required, "code")

		require.NotNil(t, action.OutputSchema)
		outSchema := action.OutputSchema
		compiledOutSchema, err := outSchema.Compile()
		require.NoError(t, err)
		assert.Equal(t, []string{"object"}, []string(compiledOutSchema.Type))
		require.NotNil(t, compiledOutSchema.Properties)
		assert.Contains(t, (*compiledOutSchema.Properties), "feedback")
		assert.Contains(t, compiledOutSchema.Required, "feedback")

		// Get the feedback property from compiled schema
		feedbackProp := (*compiledOutSchema.Properties)["feedback"]
		require.NotNil(t, feedbackProp)
		assert.Equal(t, []string{"array"}, []string(feedbackProp.Type))

		// Check array items structure
		require.NotNil(t, feedbackProp.Items)
		itemSchema := feedbackProp.Items
		assert.Equal(t, []string{"object"}, []string(itemSchema.Type))
		require.NotNil(t, itemSchema.Properties)

		// Check that the required properties exist in items
		itemProps := (*itemSchema.Properties)
		assert.Contains(t, itemProps, "category")
		assert.Contains(t, itemProps, "description")
		assert.Contains(t, itemProps, "suggestion")
	})
}

func Test_AgentActionConfigValidation(t *testing.T) {
	actionCWD, err := core.CWDFromPath("/test/path")
	require.NoError(t, err)

	t.Run("Should validate action config with all required fields", func(t *testing.T) {
		config := &ActionConfig{
			ID:     "test-action",
			Prompt: "test prompt",
			CWD:    actionCWD,
		}
		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should return error when CWD is missing", func(t *testing.T) {
		config := &ActionConfig{
			ID:     "test-action",
			Prompt: "test prompt",
		}
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "current working directory is required for test-action")
	})

	t.Run("Should return error when parameters are invalid", func(t *testing.T) {
		config := &ActionConfig{
			ID:     "test-action",
			Prompt: "test prompt",
			CWD:    actionCWD,
			InputSchema: &schema.Schema{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type": "string",
					},
				},
				"required": []string{"name"},
			},
			With: &core.Input{
				"age": 42,
			},
		}
		err := config.ValidateInput(context.Background(), config.With)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Required property 'name' is missing")
	})
}

func Test_AgentConfigCWD(t *testing.T) {
	t.Run("Should set and get CWD correctly", func(t *testing.T) {
		config := &Config{}
		config.SetCWD("/test/path")
		assert.Equal(t, "/test/path", config.GetCWD().PathStr())
	})

	t.Run("Should set CWD for all actions", func(t *testing.T) {
		config := &Config{}
		action := &ActionConfig{
			ID:     "test-action",
			Prompt: "test prompt",
		}
		config.Actions = []*ActionConfig{action}
		config.SetCWD("/new/path")
		assert.Equal(t, "/new/path", action.GetCWD().PathStr())
	})
}

func Test_AgentConfigMerge(t *testing.T) {
	t.Run("Should merge configurations correctly", func(t *testing.T) {
		baseConfig := &Config{
			Env: &core.EnvMap{
				"KEY1": "value1",
			},
			With: &core.Input{},
		}

		otherConfig := &Config{
			Env: &core.EnvMap{
				"KEY2": "value2",
			},
			With: &core.Input{},
		}

		err := baseConfig.Merge(otherConfig)
		require.NoError(t, err)

		// Check that base config has both env variables
		assert.Equal(t, "value1", baseConfig.GetEnv().Prop("KEY1"))
		assert.Equal(t, "value2", baseConfig.GetEnv().Prop("KEY2"))
	})
}

func Test_AgentConfigValidation(t *testing.T) {
	agentID := "test-agent"
	agentCWD, err := core.CWDFromPath("/test/path")
	require.NoError(t, err)

	t.Run("Should validate config with all required fields", func(t *testing.T) {
		config := &Config{
			ID:           agentID,
			Config:       core.ProviderConfig{},
			Instructions: "test instructions",
			CWD:          agentCWD,
		}
		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should return error when CWD is missing", func(t *testing.T) {
		config := &Config{
			ID:           agentID,
			Config:       core.ProviderConfig{},
			Instructions: "test instructions",
		}
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "current working directory is required for test-agent")
	})
}
