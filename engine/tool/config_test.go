package tool

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

func setupTest(t *testing.T, toolFile string) (cwd *core.CWD, dstPath string) {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	cwd, dstPath = utils.SetupTest(t, filename)
	dstPath = filepath.Join(dstPath, toolFile)
	return
}

func Test_LoadTool(t *testing.T) {
	t.Run("Should load basic tool configuration correctly", func(t *testing.T) {
		cwd, dstPath := setupTest(t, "basic_tool.yaml")
		config, err := Load(cwd, dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Validate the config
		err = config.Validate()
		require.NoError(t, err)

		require.NotNil(t, config.ID)
		require.NotNil(t, config.Description)
		require.NotNil(t, config.Execute)
		require.NotNil(t, config.InputSchema)
		require.NotNil(t, config.OutputSchema)
		require.NotNil(t, config.Env)
		require.NotNil(t, config.With)

		assert.Equal(t, "code-formatter", config.ID)
		assert.Equal(t, "A tool for formatting code", config.Description)
		assert.Equal(t, "./format.ts", config.Execute)
		assert.True(t, IsTypeScript(config.Execute))

		// Validate input schema
		schema := config.InputSchema
		compiledSchema, err := schema.Compile()
		require.NoError(t, err)
		assert.Equal(t, []string{"object"}, []string(compiledSchema.Type))
		require.NotNil(t, compiledSchema.Properties)
		assert.Contains(t, (*compiledSchema.Properties), "code")
		assert.Contains(t, (*compiledSchema.Properties), "language")
		assert.Contains(t, compiledSchema.Required, "code")

		// Validate output schema
		outSchema := config.OutputSchema
		compiledOutSchema, err := outSchema.Compile()
		require.NoError(t, err)
		assert.Equal(t, []string{"object"}, []string(compiledOutSchema.Type))
		require.NotNil(t, compiledOutSchema.Properties)
		assert.Contains(t, (*compiledOutSchema.Properties), "formatted_code")
		assert.Contains(t, compiledOutSchema.Required, "formatted_code")

		// Validate env and with
		assert.Equal(t, "1.0.0", config.Env["FORMATTER_VERSION"])
		assert.Equal(t, 2, (*config.With)["indent_size"])
		assert.Equal(t, false, (*config.With)["use_tabs"])
	})

	t.Run("Should return error for invalid tool configuration", func(t *testing.T) {
		cwd, dstPath := setupTest(t, "invalid_tool.yaml")
		config, err := Load(cwd, dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Validate the config
		err = config.Validate()
		require.Error(t, err)
	})
}

func Test_ToolConfigValidation(t *testing.T) {
	toolID := "test-tool"
	toolPath := "/test/path"
	toolCWD, err := core.CWDFromPath(toolPath)
	require.NoError(t, err)

	t.Run("Should validate valid tool configuration", func(t *testing.T) {
		config := &Config{
			ID:  toolID,
			cwd: toolCWD,
		}

		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should return error when CWD is missing", func(t *testing.T) {
		config := &Config{
			ID: toolID,
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "current working directory is required for test-tool")
	})

	t.Run("Should return error for invalid execute path", func(t *testing.T) {
		config := &Config{
			ID:      toolID,
			Execute: "./nonexistent.ts",
			cwd:     toolCWD,
		}

		err := config.Validate()
		assert.Error(t, err)
	})

	t.Run("Should return error for tool with invalid parameters", func(t *testing.T) {
		config := &Config{
			ID:      toolID,
			Execute: "./test.ts",
			cwd:     toolCWD,
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

		err := config.ValidateParams(context.Background(), config.With)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "with parameters invalid for test-tool")
	})
}

func Test_ToolConfigCWD(t *testing.T) {
	t.Run("Should handle CWD operations correctly", func(t *testing.T) {
		config := &Config{}

		// Test setting CWD
		config.SetCWD("/test/path")
		assert.Equal(t, "/test/path", config.GetCWD().PathStr())

		// Test updating CWD
		config.SetCWD("/new/path")
		assert.Equal(t, "/new/path", config.GetCWD().PathStr())
	})
}

func Test_ToolConfigMerge(t *testing.T) {
	t.Run("Should merge configurations correctly", func(t *testing.T) {
		baseConfig := &Config{
			Env: core.EnvMap{
				"KEY1": "value1",
			},
			With: &core.Input{},
		}

		otherConfig := &Config{
			Env: core.EnvMap{
				"KEY2": "value2",
			},
			With: &core.Input{},
		}

		err := baseConfig.Merge(otherConfig)
		require.NoError(t, err)

		// Check that base config has both env variables
		assert.Equal(t, "value1", baseConfig.Env["KEY1"])
		assert.Equal(t, "value2", baseConfig.Env["KEY2"])
	})
}

func Test_ToolExecuteIsTypeScript(t *testing.T) {
	t.Run("Should identify TypeScript file correctly", func(t *testing.T) {
		assert.True(t, IsTypeScript("./script.ts"))
	})

	t.Run("Should identify JavaScript file correctly", func(t *testing.T) {
		assert.False(t, IsTypeScript("./script.js"))
	})

	t.Run("Should identify Python file correctly", func(t *testing.T) {
		assert.False(t, IsTypeScript("./script.py"))
	})

	t.Run("Should identify file without extension correctly", func(t *testing.T) {
		assert.False(t, IsTypeScript("./script"))
	})
}
