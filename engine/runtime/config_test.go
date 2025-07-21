package runtime_test

import (
	"context"
	"os"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBunManager_ConfigParameter(t *testing.T) {
	if !isBunAvailable() {
		t.Skip("Bun is not available, skipping test")
	}

	t.Run("Should pass config parameter to tool", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "bun-config-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		// Create a tool that uses both input and config
		entrypointContent := `
export async function config_tool(input: any, config?: any) {
    return {
        input_message: input.message || "no input message",
        config_value: config?.default_value || "no config value",
        combined: (input.message || "") + " " + (config?.suffix || "")
    };
}
`
		err = os.WriteFile(tmpDir+"/tools.ts", []byte(entrypointContent), 0644)
		require.NoError(t, err)

		ctx := context.Background()
		config := &runtime.Config{
			EntrypointPath: "./tools.ts",
		}
		bm, err := runtime.NewBunManager(ctx, tmpDir, runtime.WithConfig(config))
		require.NoError(t, err)

		toolExecID, _ := core.NewID()
		input := &core.Input{"message": "Hello"}
		configParam := &core.Input{
			"default_value": "Default Config",
			"suffix":        "World",
		}

		result, err := bm.ExecuteTool(ctx, "config_tool", toolExecID, input, configParam, core.EnvMap{})
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Verify the response includes both input and config values
		assert.Equal(t, "Hello", (*result)["input_message"])
		assert.Equal(t, "Default Config", (*result)["config_value"])
		assert.Equal(t, "Hello World", (*result)["combined"])
	})

	t.Run("Should handle nil config parameter", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "bun-config-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		// Create a tool that handles missing config
		entrypointContent := `
export async function optional_config_tool(input: any, config?: any) {
    return {
        has_config: config !== undefined && config !== null,
        config_keys: config ? Object.keys(config) : [],
        fallback_value: config?.value || "fallback"
    };
}
`
		err = os.WriteFile(tmpDir+"/tools.ts", []byte(entrypointContent), 0644)
		require.NoError(t, err)

		ctx := context.Background()
		config := &runtime.Config{
			EntrypointPath: "./tools.ts",
		}
		bm, err := runtime.NewBunManager(ctx, tmpDir, runtime.WithConfig(config))
		require.NoError(t, err)

		toolExecID, _ := core.NewID()
		input := &core.Input{"test": "data"}

		// Execute with nil config
		result, err := bm.ExecuteTool(ctx, "optional_config_tool", toolExecID, input, nil, core.EnvMap{})
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Verify the tool handled nil config gracefully
		assert.Equal(t, false, (*result)["has_config"])
		assert.Equal(t, []any{}, (*result)["config_keys"])
		assert.Equal(t, "fallback", (*result)["fallback_value"])
	})

	t.Run("Should handle empty config parameter", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "bun-config-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		// Reuse the optional_config_tool
		entrypointContent := `
export async function optional_config_tool(input: any, config?: any) {
    return {
        has_config: config !== undefined && config !== null,
        config_keys: config ? Object.keys(config) : [],
        fallback_value: config?.value || "fallback"
    };
}
`
		err = os.WriteFile(tmpDir+"/tools.ts", []byte(entrypointContent), 0644)
		require.NoError(t, err)

		ctx := context.Background()
		config := &runtime.Config{
			EntrypointPath: "./tools.ts",
		}
		bm, err := runtime.NewBunManager(ctx, tmpDir, runtime.WithConfig(config))
		require.NoError(t, err)

		toolExecID, _ := core.NewID()
		input := &core.Input{"test": "data"}
		emptyConfig := &core.Input{}

		// Execute with empty config
		result, err := bm.ExecuteTool(ctx, "optional_config_tool", toolExecID, input, emptyConfig, core.EnvMap{})
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Verify the tool received an empty config object
		assert.Equal(t, true, (*result)["has_config"])
		assert.Equal(t, []any{}, (*result)["config_keys"])
		assert.Equal(t, "fallback", (*result)["fallback_value"])
	})
}
