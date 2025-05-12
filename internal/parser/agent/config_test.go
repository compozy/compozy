package agent

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/pkgref"
	"github.com/compozy/compozy/internal/parser/provider"
	"github.com/compozy/compozy/internal/parser/schema"
	"github.com/compozy/compozy/internal/parser/tool"
	"github.com/compozy/compozy/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_LoadAgent(t *testing.T) {
	t.Run("Should load basic agent configuration correctly", func(t *testing.T) {
		// Get the test directory path
		_, filename, _, ok := runtime.Caller(0)
		require.True(t, ok)
		testDir := filepath.Dir(filename)
		cwd, err := common.CWDFromPath(testDir)
		require.NoError(t, err)

		// Setup test fixture using utils
		dstPath := utils.SetupFixture(t, testDir, "basic_agent.yaml")

		// Run the test
		config, err := Load(cwd, dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		require.NotNil(t, config.ID)
		require.NotNil(t, config.Config)
		require.NotNil(t, config.Config.Temperature)
		require.NotNil(t, config.Config.MaxTokens)

		assert.Equal(t, "code-assistant", config.ID)
		assert.Equal(t, provider.ProviderAnthropic, config.Config.Provider)
		assert.Equal(t, provider.ModelClaude3Opus, config.Config.Model)
		assert.InDelta(t, float32(0.7), config.Config.Temperature, 0.0001)
		assert.Equal(t, int32(4000), config.Config.MaxTokens)

		require.Len(t, config.Actions, 1)
		action := config.Actions[0]
		assert.Equal(t, "review-code", action.ID)

		require.NotNil(t, action.InputSchema)
		schema := action.InputSchema.Schema
		assert.Equal(t, "object", schema.GetType())
		require.NotNil(t, schema.GetProperties())
		assert.Contains(t, schema.GetProperties(), "code")
		assert.Contains(t, schema.GetProperties(), "language")
		if required, ok := schema["required"].([]string); ok && len(required) > 0 {
			assert.Contains(t, required, "code")
		}

		require.NotNil(t, action.OutputSchema)
		outSchema := action.OutputSchema.Schema
		assert.Equal(t, "object", outSchema.GetType())
		require.NotNil(t, outSchema.GetProperties())
		assert.Contains(t, outSchema.GetProperties(), "feedback")

		feedback := outSchema.GetProperties()["feedback"]
		assert.NotNil(t, feedback)
		assert.Equal(t, "array", feedback.GetType())

		// Get items by accessing the items map directly
		if itemsMap, ok := (*feedback)["items"].(map[string]any); ok {
			// Check type directly
			if typ, ok := itemsMap["type"].(string); ok {
				assert.Equal(t, "object", typ)
			}

			// Check properties directly
			if props, ok := itemsMap["properties"].(map[string]any); ok {
				assert.Contains(t, props, "category")
				assert.Contains(t, props, "description")
				assert.Contains(t, props, "suggestion")
			}
		} else {
			t.Error("Items is not a map or not found")
		}
	})
}

func Test_AgentActionConfigValidation(t *testing.T) {
	actionCWD, err := common.CWDFromPath("/test/path")
	require.NoError(t, err)

	t.Run("Should validate action config with all required fields", func(t *testing.T) {
		config := &AgentActionConfig{
			ID:     "test-action",
			Prompt: "test prompt",
			cwd:    actionCWD,
		}
		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should return error when CWD is missing", func(t *testing.T) {
		config := &AgentActionConfig{
			ID:     "test-action",
			Prompt: "test prompt",
		}
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "current working directory is required for test-action")
	})

	t.Run("Should return error when parameters are invalid", func(t *testing.T) {
		config := &AgentActionConfig{
			ID:     "test-action",
			Prompt: "test prompt",
			cwd:    actionCWD,
			InputSchema: &schema.InputSchema{
				Schema: schema.Schema{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{
							"type": "string",
						},
					},
					"required": []string{"name"},
				},
			},
			With: &common.Input{
				"age": 42,
			},
		}
		err := config.ValidateParams(*config.With)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "with parameters invalid for test-action")
	})
}

func Test_AgentConfigCWD(t *testing.T) {
	t.Run("Should set and get CWD correctly", func(t *testing.T) {
		config := &AgentConfig{}
		config.SetCWD("/test/path")
		assert.Equal(t, "/test/path", config.GetCWD())
	})

	t.Run("Should set CWD for all actions", func(t *testing.T) {
		config := &AgentConfig{}
		action := &AgentActionConfig{
			ID:     "test-action",
			Prompt: "test prompt",
		}
		config.Actions = []*AgentActionConfig{action}
		config.SetCWD("/new/path")
		assert.Equal(t, "/new/path", action.GetCWD())
	})
}

func Test_AgentConfigMerge(t *testing.T) {
	t.Run("Should merge configurations correctly", func(t *testing.T) {
		baseConfig := &AgentConfig{
			Env: common.EnvMap{
				"KEY1": "value1",
			},
			With: &common.Input{},
		}

		otherConfig := &AgentConfig{
			Env: common.EnvMap{
				"KEY2": "value2",
			},
			With: &common.Input{},
		}

		err := baseConfig.Merge(otherConfig)
		require.NoError(t, err)

		// Check that base config has both env variables
		assert.Equal(t, "value1", baseConfig.Env["KEY1"])
		assert.Equal(t, "value2", baseConfig.Env["KEY2"])
	})
}

func Test_AgentConfigValidation(t *testing.T) {
	agentID := "test-agent"
	agentCWD, err := common.CWDFromPath("/test/path")
	require.NoError(t, err)

	t.Run("Should validate config with all required fields", func(t *testing.T) {
		config := &AgentConfig{
			ID:           agentID,
			Config:       provider.ProviderConfig{},
			Instructions: "test instructions",
			cwd:          agentCWD,
		}
		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should return error when CWD is missing", func(t *testing.T) {
		config := &AgentConfig{
			ID:           agentID,
			Config:       provider.ProviderConfig{},
			Instructions: "test instructions",
		}
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "current working directory is required for test-agent")
	})

	t.Run("Should return error for invalid package reference", func(t *testing.T) {
		config := &AgentConfig{
			ID:      agentID,
			Use:     pkgref.NewPackageRefConfig("invalid"),
			Config:  provider.ProviderConfig{},
			Tools:   []tool.ToolConfig{},
			Actions: []*AgentActionConfig{},
			cwd:     agentCWD,
		}
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid package reference")
	})

	t.Run("Should return error when input schema is used with ID reference", func(t *testing.T) {
		config := &AgentConfig{
			ID:           agentID,
			Use:          pkgref.NewPackageRefConfig("agent(id=test-agent)"),
			Config:       provider.ProviderConfig{},
			Instructions: "test instructions",
			InputSchema: &schema.InputSchema{
				Schema: schema.Schema{
					"type": "object",
				},
			},
			cwd: agentCWD,
		}
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "input schema not allowed for reference type id")
	})

	t.Run("Should return error when output schema is used with file reference", func(t *testing.T) {
		config := &AgentConfig{
			ID:           agentID,
			Use:          pkgref.NewPackageRefConfig("agent(file=basic_agent.yaml)"),
			Config:       provider.ProviderConfig{},
			Instructions: "test instructions",
			OutputSchema: &schema.OutputSchema{
				Schema: schema.Schema{
					"type": "object",
				},
			},
			cwd: agentCWD,
		}
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "output schema not allowed for reference type file")
	})

	t.Run("Should return error when schemas are used with dep reference", func(t *testing.T) {
		config := &AgentConfig{
			ID:           agentID,
			Use:          pkgref.NewPackageRefConfig("agent(dep=compozy/agents:test-agent)"),
			Config:       provider.ProviderConfig{},
			Instructions: "test instructions",
			InputSchema: &schema.InputSchema{
				Schema: schema.Schema{
					"type": "object",
				},
			},
			OutputSchema: &schema.OutputSchema{
				Schema: schema.Schema{
					"type": "object",
				},
			},
			cwd: agentCWD,
		}
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "input schema not allowed for reference type dep")
	})

	t.Run("Should return error when parameters are invalid", func(t *testing.T) {
		config := &AgentConfig{
			ID:           agentID,
			Config:       provider.ProviderConfig{},
			Instructions: "test instructions",
			InputSchema: &schema.InputSchema{
				Schema: schema.Schema{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{
							"type": "string",
						},
					},
					"required": []string{"name"},
				},
			},
			With: &common.Input{
				"age": 42,
			},
			cwd: agentCWD,
		}
		err := config.ValidateParams(*config.With)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "with parameters invalid for test-agent")
	})
}
