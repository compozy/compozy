package llm_test

import (
	"context"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRuntimeWithConfig tracks config parameter usage
type mockRuntimeWithConfig struct {
	executedConfig *core.Input
}

func (m *mockRuntimeWithConfig) ExecuteTool(
	_ context.Context,
	_ string,
	_ core.ID,
	_ *core.Input,
	config *core.Input,
	_ core.EnvMap,
) (*core.Output, error) {
	m.executedConfig = config
	return &core.Output{"result": "success"}, nil
}

func (m *mockRuntimeWithConfig) ExecuteToolWithTimeout(
	_ context.Context,
	_ string,
	_ core.ID,
	_ *core.Input,
	config *core.Input,
	_ core.EnvMap,
	_ time.Duration,
) (*core.Output, error) {
	m.executedConfig = config
	return &core.Output{"result": "success with timeout"}, nil
}

func (m *mockRuntimeWithConfig) GetGlobalTimeout() time.Duration {
	return 60 * time.Second
}

func TestInternalTool_ConfigParameter(t *testing.T) {
	t.Run("Should pass config from tool configuration", func(t *testing.T) {
		ctx := context.Background()
		runtime := &mockRuntimeWithConfig{}

		// Create tool configuration with config parameter
		toolConfig := &tool.Config{
			ID:          "test-tool",
			Description: "A test tool with config",
			Config: &core.Input{
				"default_timeout": 30,
				"retry_count":     3,
				"api_endpoint":    "https://api.example.com",
			},
		}

		// Create internal tool
		env := &core.EnvMap{}
		internalTool := llm.NewTool(toolConfig, env, runtime)
		require.NotNil(t, internalTool)

		// Call the tool
		input := &core.Input{
			"query": "test query",
		}
		result, err := internalTool.Call(ctx, input)
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Verify that config was passed to runtime
		assert.NotNil(t, runtime.executedConfig)
		assert.Equal(t, 30, (*runtime.executedConfig)["default_timeout"])
		assert.Equal(t, 3, (*runtime.executedConfig)["retry_count"])
		assert.Equal(t, "https://api.example.com", (*runtime.executedConfig)["api_endpoint"])
	})

	t.Run("Should handle nil config", func(t *testing.T) {
		ctx := context.Background()
		runtime := &mockRuntimeWithConfig{}

		// Create tool configuration without config
		toolConfig := &tool.Config{
			ID:          "test-tool",
			Description: "A test tool without config",
			Config:      nil,
		}

		// Create internal tool
		env := &core.EnvMap{}
		internalTool := llm.NewTool(toolConfig, env, runtime)
		require.NotNil(t, internalTool)

		// Call the tool
		input := &core.Input{
			"query": "test query",
		}
		result, err := internalTool.Call(ctx, input)
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Verify that empty config was passed (not nil, due to GetConfig method)
		assert.NotNil(t, runtime.executedConfig)
		assert.Equal(t, &core.Input{}, runtime.executedConfig)
	})

	t.Run("Should handle empty config", func(t *testing.T) {
		ctx := context.Background()
		runtime := &mockRuntimeWithConfig{}

		// Create tool configuration with empty config
		emptyConfig := &core.Input{}
		toolConfig := &tool.Config{
			ID:          "test-tool",
			Description: "A test tool with empty config",
			Config:      emptyConfig,
		}

		// Create internal tool
		env := &core.EnvMap{}
		internalTool := llm.NewTool(toolConfig, env, runtime)
		require.NotNil(t, internalTool)

		// Call the tool
		input := &core.Input{
			"query": "test query",
		}
		result, err := internalTool.Call(ctx, input)
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Verify that empty config was passed
		assert.NotNil(t, runtime.executedConfig)
		assert.Equal(t, emptyConfig, runtime.executedConfig)
	})
}
