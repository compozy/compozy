package tool

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/pkg/ref"
	fixtures "github.com/compozy/compozy/test/fixtures"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTest(t *testing.T, toolFile string) (*core.PathCWD, string) {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	cwd, dstPath := fixtures.SetupConfigTest(t, filename)
	dstPath = filepath.Join(dstPath, toolFile)
	return cwd, dstPath
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
		// Execute field removed - tools resolved via entrypoint exports
		require.NotNil(t, config.InputSchema)
		require.NotNil(t, config.OutputSchema)
		require.NotNil(t, config.Env)
		require.NotNil(t, config.With)

		assert.Equal(t, "code-formatter", config.ID)
		assert.Equal(t, "A tool for formatting code", config.Description)
		// Execute field removed - tools resolved via entrypoint exports

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
		assert.Equal(t, "1.0.0", config.GetEnv().Prop("FORMATTER_VERSION"))
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
			CWD: toolCWD,
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

	// Test removed - Execute field no longer exists, tools resolved via entrypoint

	t.Run("Should return error for tool with invalid parameters", func(t *testing.T) {
		config := &Config{
			ID:  toolID,
			CWD: toolCWD,
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

func Test_Config_LLMDefinition_And_Maps(t *testing.T) {
	t.Parallel()
	t.Run("Should build LLM definition from config", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{ID: "file-reader", Description: "Reads files", InputSchema: &schema.Schema{"type": "object"}}
		def := cfg.GetLLMDefinition()
		require.NotNil(t, def.Function)
		assert.Equal(t, "function", def.Type)
		assert.Equal(t, "file-reader", def.Function.Name)
		assert.Equal(t, "Reads files", def.Function.Description)
		assert.NotNil(t, def.Function.Parameters)
	})
	t.Run("Should convert to map and back with FromMap", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			ID:          "x",
			Description: "desc",
			Timeout:     "30s",
			With:        &core.Input{"a": 1},
			Config:      &core.Input{"b": true},
		}
		m, err := cfg.AsMap()
		require.NoError(t, err)
		var dst Config
		require.NoError(t, dst.FromMap(m))
		assert.Equal(t, "x", dst.ID)
		assert.Equal(t, "desc", dst.Description)
		assert.Equal(t, "30s", dst.Timeout)
		assert.EqualValues(t, 1, (*dst.With)["a"])
		assert.Equal(t, true, (*dst.Config)["b"])
	})
	t.Run("Should round-trip complex fields including schema and env", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			ID:          "complex",
			Description: "with schema and env",
			InputSchema: &schema.Schema{
				"type":       "object",
				"properties": map[string]any{"k": map[string]any{"type": "string"}},
			},
			Env:    &core.EnvMap{"X": "1"},
			Config: &core.Input{"nested": map[string]any{"flag": true}},
		}
		m, err := cfg.AsMap()
		require.NoError(t, err)
		var dst Config
		require.NoError(t, dst.FromMap(m))
		assert.Equal(t, "complex", dst.ID)
		assert.Equal(t, "1", dst.GetEnv().Prop("X"))
		require.NotNil(t, dst.InputSchema)
		compiled, err := dst.InputSchema.Compile()
		require.NoError(t, err)
		require.NotNil(t, compiled.Properties)
		_, ok := (*compiled.Properties)["k"]
		assert.True(t, ok)
		nested := (*dst.Config)["nested"].(map[string]any)
		assert.Equal(t, true, nested["flag"])
	})
}

func Test_Config_LoadAndEval_EnvTemplate(t *testing.T) {
	t.Parallel()
	t.Run("Should load config with template unchanged (evaluated elsewhere)", func(t *testing.T) {
		t.Parallel()
		_, filename, _, ok := runtime.Caller(0)
		require.True(t, ok)
		cwd, dst := fixtures.SetupConfigTest(t, filename)
		path := dst + "/config_example.yaml"
		ev := ref.NewEvaluator(ref.WithGlobalScope(map[string]any{"env": map[string]any{"API_SECRET": "sekret"}}))
		cfg, err := LoadAndEval(cwd, path, ev)
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Equal(t, "api-client", cfg.ID)
		assert.Equal(t, "{{ .env.API_SECRET }}", cfg.GetEnv().Prop("API_KEY"))
	})
}

func Test_Config_Clone_And_Accessors(t *testing.T) {
	t.Parallel()
	t.Run("Should clone config deeply without affecting original", func(t *testing.T) {
		t.Parallel()
		c := &Config{ID: "t1", Description: "a", With: &core.Input{"key": "original"}}
		clone, err := c.Clone()
		require.NoError(t, err)
		require.NotNil(t, clone)
		clone.Description = "b"
		assert.Equal(t, "a", c.Description)
		assert.Equal(t, "b", clone.Description)
		(*clone.With)["key"] = "modified"
		assert.Equal(t, "original", (*c.With)["key"])
	})
	t.Run("Should expose defaults and detect schema presence", func(t *testing.T) {
		t.Parallel()
		c := &Config{}
		in := c.GetInput()
		cfg := c.GetConfig()
		assert.NotNil(t, in)
		assert.NotNil(t, cfg)
		assert.False(t, c.HasSchema())
		c.InputSchema = &schema.Schema{"type": "object"}
		assert.True(t, c.HasSchema())
	})
}

func Test_ToolConfigMerge_InvalidType(t *testing.T) {
	t.Parallel()
	t.Run("Should return error when merging with wrong type", func(t *testing.T) {
		t.Parallel()
		var a Config
		err := a.Merge(&struct{}{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid type for merge")
	})
	t.Run("Should return error when merging into nil receiver", func(t *testing.T) {
		t.Parallel()
		var p *Config
		err := p.Merge(&Config{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "nil config")
	})
}

func Test_ToolConfigOutputValidation(t *testing.T) {
	t.Parallel()
	t.Run("Should validate output against schema", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			ID:           "fmt",
			OutputSchema: &schema.Schema{"type": "object", "required": []string{"formatted_code"}},
		}
		out := core.Output{"formatted_code": "ok"}
		err := cfg.ValidateOutput(context.Background(), &out)
		require.NoError(t, err)
	})
	t.Run("Should return error when required output field is missing", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			ID:           "fmt",
			OutputSchema: &schema.Schema{"type": "object", "required": []string{"formatted_code"}},
		}
		out := core.Output{"other": true}
		err := cfg.ValidateOutput(context.Background(), &out)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "formatted_code")
	})
	t.Run("Should return error when field has wrong type", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{ID: "fmt", OutputSchema: &schema.Schema{
			"type":       "object",
			"properties": map[string]any{"formatted_code": map[string]any{"type": "string"}},
			"required":   []string{"formatted_code"},
		}}
		out := core.Output{"formatted_code": 123}
		err := cfg.ValidateOutput(context.Background(), &out)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "formatted_code")
	})
}

func Test_ToolConfig_ValidateNilPaths(t *testing.T) {
	t.Parallel()
	t.Run("Should skip input validation when schema is nil", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{ID: "fmt"}
		err := cfg.ValidateInput(context.Background(), &core.Input{"x": 1})
		require.NoError(t, err)
	})
	t.Run("Should skip input validation when input is nil", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{ID: "fmt", InputSchema: &schema.Schema{"type": "object"}}
		err := cfg.ValidateInput(context.Background(), nil)
		require.NoError(t, err)
	})
	t.Run("Should skip output validation when schema is nil", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{ID: "fmt"}
		err := cfg.ValidateOutput(context.Background(), &core.Output{"y": true})
		require.NoError(t, err)
	})
	t.Run("Should skip output validation when output is nil", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{ID: "fmt", OutputSchema: &schema.Schema{"type": "object"}}
		err := cfg.ValidateOutput(context.Background(), nil)
		require.NoError(t, err)
	})
}
