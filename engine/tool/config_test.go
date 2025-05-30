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

func setupTest(t *testing.T, toolFile string) (cwd *core.CWD, projectRoot, dstPath string) {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	_, tempFixturesDir := utils.SetupTest(t, filename)
	// Set CWD to the temporary fixtures directory where the files actually exist
	cwd, err := core.CWDFromPath(tempFixturesDir)
	require.NoError(t, err)
	projectRoot = tempFixturesDir
	dstPath = filepath.Join(tempFixturesDir, toolFile)
	return
}

func Test_LoadTool(t *testing.T) {
	t.Run("Should load basic tool configuration correctly", func(t *testing.T) {
		cwd, projectRoot, dstPath := setupTest(t, "basic_tool.yaml")

		// Run the test
		ctx := context.Background()
		config, err := Load(ctx, cwd, projectRoot, dstPath)
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
		schema := config.InputSchema.Schema
		assert.Equal(t, "object", schema.GetType())
		require.NotNil(t, schema.GetProperties())
		assert.Contains(t, schema.GetProperties(), "code")
		assert.Contains(t, schema.GetProperties(), "language")
		if required, ok := schema["required"].([]string); ok && len(required) > 0 {
			assert.Contains(t, required, "code")
		}

		// Validate output schema
		outSchema := config.OutputSchema.Schema
		assert.Equal(t, "object", outSchema.GetType())
		require.NotNil(t, outSchema.GetProperties())
		assert.Contains(t, outSchema.GetProperties(), "formatted_code")
		if required, ok := outSchema["required"].([]string); ok && len(required) > 0 {
			assert.Contains(t, required, "formatted_code")
		}

		// Validate env and with
		assert.Equal(t, "1.0.0", config.Env["FORMATTER_VERSION"])
		assert.Equal(t, 2, (*config.With)["indent_size"])
		assert.Equal(t, false, (*config.With)["use_tabs"])
	})

	t.Run("Should load package tool configuration correctly", func(t *testing.T) {
		cwd, projectRoot, dstPath := setupTest(t, "package_tool.yaml")

		// Run the test
		ctx := context.Background()
		config, err := Load(ctx, cwd, projectRoot, dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Validate the config
		err = config.Validate()
		require.NoError(t, err)

		require.NotNil(t, config.ID)
		require.NotNil(t, config.Description)
		require.NotNil(t, config.InputSchema)
		require.NotNil(t, config.OutputSchema)
		require.NotNil(t, config.Env)
		require.NotNil(t, config.With)

		assert.Equal(t, "code-linter", config.ID)
		assert.Equal(t, "A tool for linting code", config.Description)

		// Validate input schema
		schema := config.InputSchema.Schema
		assert.Equal(t, "object", schema.GetType())
		require.NotNil(t, schema.GetProperties())
		assert.Contains(t, schema.GetProperties(), "code")
		assert.Contains(t, schema.GetProperties(), "language")
		if required, ok := schema["required"].([]string); ok && len(required) > 0 {
			assert.Contains(t, required, "code")
		}

		// Validate output schema
		outSchema := config.OutputSchema.Schema
		assert.Equal(t, "object", outSchema.GetType())
		require.NotNil(t, outSchema.GetProperties())
		assert.Contains(t, outSchema.GetProperties(), "issues")
		issues := outSchema.GetProperties()["issues"]
		assert.Equal(t, "array", issues.GetType())

		// Get the items from the schema
		if items, ok := (*issues)["items"].(map[string]any); ok {
			// Check properties directly from the items map
			if itemType, ok := items["type"].(string); ok {
				assert.Equal(t, "object", itemType)
			}

			if itemProps, ok := items["properties"].(map[string]any); ok {
				assert.Contains(t, itemProps, "line")
				assert.Contains(t, itemProps, "message")
				assert.Contains(t, itemProps, "severity")
			} else {
				t.Error("Item properties not found or not a map")
			}
		} else {
			t.Error("Items not found or not a map")
		}

		if required, ok := outSchema["required"].([]string); ok && len(required) > 0 {
			assert.Contains(t, required, "issues")
		}

		// Validate env and with
		assert.Equal(t, "8.0.0", config.Env["ESLINT_VERSION"])
		assert.Equal(t, 10, (*config.With)["max_warnings"])
		assert.Equal(t, true, (*config.With)["fix"])
	})

	t.Run("Should return error for invalid tool configuration", func(t *testing.T) {
		cwd, projectRoot, dstPath := setupTest(t, "invalid_tool.yaml")

		// Run the test - Load should now fail directly due to internal validation
		ctx := context.Background()
		config, err := Load(ctx, cwd, projectRoot, dstPath)
		require.Error(t, err)
		assert.Nil(t, config)
		assert.Contains(t, err.Error(), "file not found or inaccessible")
	})

	t.Run("Should load tool configuration with external schema references", func(t *testing.T) {
		cwd, projectRoot, dstPath := setupTest(t, "ref_tool.yaml")

		// Run the test
		ctx := context.Background()
		config, err := Load(ctx, cwd, projectRoot, dstPath)
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

		assert.Equal(t, "code-processor", config.ID)
		assert.Equal(t, "A tool that processes code using external schema references with merging", config.Description)
		assert.Equal(t, "./format.ts", config.Execute)
		assert.True(t, IsTypeScript(config.Execute))

		// Validate input schema was resolved from external file and merged with inline properties
		inputSchema := config.InputSchema.Schema
		assert.Equal(t, "object", inputSchema.GetType())
		require.NotNil(t, inputSchema.GetProperties())

		// Properties from the referenced schema
		assert.Contains(t, inputSchema.GetProperties(), "code")
		assert.Contains(t, inputSchema.GetProperties(), "language")
		assert.Contains(t, inputSchema.GetProperties(), "options")

		// Additional inline properties that should be merged
		assert.Contains(t, inputSchema.GetProperties(), "format_style")
		assert.Contains(t, inputSchema.GetProperties(), "strict_mode")

		// Verify the language property has enum constraints from external schema
		languageProp := inputSchema.GetProperties()["language"]
		if languageSchema, ok := (*languageProp)["enum"]; ok {
			enumValues, ok := languageSchema.([]any)
			require.True(t, ok)
			assert.Contains(t, enumValues, "javascript")
			assert.Contains(t, enumValues, "typescript")
			assert.Contains(t, enumValues, "python")
		}

		// Verify the inline format_style property has its enum
		formatStyleProp := inputSchema.GetProperties()["format_style"]
		if formatStyleSchema, ok := (*formatStyleProp)["enum"]; ok {
			enumValues, ok := formatStyleSchema.([]any)
			require.True(t, ok)
			assert.Contains(t, enumValues, "standard")
			assert.Contains(t, enumValues, "compact")
			assert.Contains(t, enumValues, "verbose")
		}

		// Check required fields - arrays are merged in merge mode
		if required, ok := inputSchema["required"].([]any); ok && len(required) > 0 {
			// Debug: print what's actually in the required array
			t.Logf("Required array contains: %v", required)
			// Arrays are merged, so we should have values from both reference and inline
			assert.Contains(t, required, "format_style") // from inline
			assert.Contains(t, required, "code")         // from reference
			assert.Contains(t, required, "language")     // from reference
		} else {
			t.Logf("No required array found or it's empty")
		}

		// Validate output schema was resolved and merged with inline properties
		outputSchema := config.OutputSchema.Schema
		assert.Equal(t, "object", outputSchema.GetType())
		require.NotNil(t, outputSchema.GetProperties())

		// Properties from the referenced schema
		assert.Contains(t, outputSchema.GetProperties(), "processed_code")
		assert.Contains(t, outputSchema.GetProperties(), "metadata")

		// Additional inline properties that should be merged
		assert.Contains(t, outputSchema.GetProperties(), "execution_time")
		assert.Contains(t, outputSchema.GetProperties(), "tool_version")

		if required, ok := outputSchema["required"].([]any); ok && len(required) > 0 {
			assert.Contains(t, required, "processed_code")
		}

		// Validate env and with
		assert.Equal(t, "2.0.0", config.Env["PROCESSOR_VERSION"])
		assert.Equal(t, "info", config.Env["LOG_LEVEL"])
		assert.Equal(t, "console.log('Hello, World!');", (*config.With)["code"])
		assert.Equal(t, "javascript", (*config.With)["language"])
		assert.Equal(t, "standard", (*config.With)["format_style"])
		assert.Equal(t, true, (*config.With)["strict_mode"])

		// Validate nested options from with
		if options, ok := (*config.With)["options"].(map[string]any); ok {
			assert.Equal(t, 4, options["indent_size"])
			assert.Equal(t, false, options["use_tabs"])
		}
	})

	t.Run("Should handle replace merge mode correctly", func(t *testing.T) {
		cwd, projectRoot, dstPath := setupTest(t, "replace_mode_tool.yaml")

		// Run the test
		ctx := context.Background()
		config, err := Load(ctx, cwd, projectRoot, dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Validate the config
		err = config.Validate()
		require.NoError(t, err)

		assert.Equal(t, "replace-mode-processor", config.ID)

		// Validate input schema with replace mode - should only have referenced schema, no inline properties
		inputSchema := config.InputSchema.Schema
		assert.Equal(t, "object", inputSchema.GetType())
		properties := inputSchema.GetProperties()
		require.NotNil(t, properties)

		// Should have properties from referenced schema only
		assert.Contains(t, properties, "code")
		assert.Contains(t, properties, "language")
		assert.Contains(t, properties, "options")

		// Should NOT have inline properties due to replace mode
		assert.NotContains(t, properties, "ignored_field")

		// Required should come from referenced schema, not inline
		if required, ok := inputSchema["required"].([]any); ok && len(required) > 0 {
			assert.Contains(t, required, "code")
			assert.Contains(t, required, "language")
			assert.NotContains(t, required, "ignored_field")
		}
	})

	t.Run("Should validate parameter merging with referenced schemas", func(t *testing.T) {
		cwd, projectRoot, dstPath := setupTest(t, "ref_tool.yaml")

		// Run the test
		ctx := context.Background()
		config, err := Load(ctx, cwd, projectRoot, dstPath)
		require.NoError(t, err)

		// Test parameter validation with merged schema
		validParams := &core.Input{
			"code":         "console.log('test');",
			"language":     "javascript",
			"format_style": "standard",
			"strict_mode":  true,
			"options": map[string]any{
				"indent_size": 4,
				"use_tabs":    false,
			},
		}

		err = config.ValidateParams(validParams)
		assert.NoError(t, err)

		// Test with invalid parameters (missing required from inline)
		invalidParams := &core.Input{
			"code":     "console.log('test');",
			"language": "javascript",
			// Missing format_style which is required in inline schema
		}

		err = config.ValidateParams(invalidParams)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "with parameters invalid")

		// Test with invalid enum value for referenced property
		invalidEnumParams := &core.Input{
			"code":         "console.log('test');",
			"language":     "invalid_language", // Not in referenced enum
			"format_style": "standard",
		}

		err = config.ValidateParams(invalidEnumParams)
		assert.Error(t, err)
	})
}

func Test_ToolConfigValidation(t *testing.T) {
	toolID := "test-tool"
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	fixturesDir := filepath.Join(filepath.Dir(filename), "fixtures")
	toolCWD, err := core.CWDFromPath(fixturesDir)
	require.NoError(t, err)

	// Create tool metadata
	metadata := &core.ConfigMetadata{
		CWD:         toolCWD,
		FilePath:    filepath.Join(fixturesDir, "tool.yaml"),
		ProjectRoot: fixturesDir,
	}

	t.Run("Should validate valid tool configuration", func(t *testing.T) {
		config := &Config{
			ID:       toolID,
			metadata: metadata,
		}

		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should return error when CWD is missing", func(t *testing.T) {
		config := &Config{
			ID:       toolID,
			metadata: &core.ConfigMetadata{},
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "current working directory is required for test-tool")
	})

	t.Run("Should return error for invalid execute path", func(t *testing.T) {
		config := &Config{
			ID:       toolID,
			Execute:  "./nonexistent.ts",
			metadata: metadata,
		}

		err := config.Validate()
		assert.Error(t, err)
	})

	t.Run("Should validate tool with input and output schemas", func(t *testing.T) {
		config := &Config{
			ID:      toolID,
			Execute: "./format.ts",
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
			OutputSchema: &schema.OutputSchema{
				Schema: schema.Schema{
					"type": "object",
					"properties": map[string]any{
						"result": map[string]any{
							"type": "string",
						},
					},
					"required": []string{"result"},
				},
			},
			metadata: metadata,
		}

		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should validate parameters against input schema", func(t *testing.T) {
		config := &Config{
			ID:      toolID,
			Execute: "./test.ts",
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
			With: &core.Input{
				"name": "test",
			},
			metadata: metadata,
		}

		err := config.ValidateParams(config.With)
		assert.NoError(t, err)
	})

	t.Run("Should return error for tool with invalid parameters", func(t *testing.T) {
		config := &Config{
			ID:      toolID,
			Execute: "./test.ts",
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
			With: &core.Input{
				"age": 42,
			},
			metadata: metadata,
		}

		err := config.ValidateParams(config.With)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "with parameters invalid for test-tool")
	})

	t.Run("Should handle empty input schema gracefully", func(t *testing.T) {
		config := &Config{
			ID:      toolID,
			Execute: "./test.ts",
			With: &core.Input{
				"param": "value",
			},
			metadata: metadata,
		}

		err := config.ValidateParams(config.With)
		assert.NoError(t, err)
	})

	t.Run("Should handle nil input gracefully", func(t *testing.T) {
		config := &Config{
			ID:      toolID,
			Execute: "./test.ts",
			InputSchema: &schema.InputSchema{
				Schema: schema.Schema{
					"type": "object",
				},
			},
			metadata: metadata,
		}

		err := config.ValidateParams(nil)
		assert.NoError(t, err)
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
