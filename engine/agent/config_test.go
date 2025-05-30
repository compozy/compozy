package agent

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTest(t *testing.T, agentFile string) (cwd *core.CWD, projectRoot, dstPath string) {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	cwd, dstPath = utils.SetupTest(t, filename)
	projectRoot = cwd.PathStr()
	dstPath = filepath.Join(dstPath, agentFile)
	return
}

func Test_LoadAgent(t *testing.T) {
	t.Run("Should load basic agent configuration correctly", func(t *testing.T) {
		cwd, projectRoot, dstPath := setupTest(t, "basic_agent.yaml")

		// Run the test
		ctx := context.Background()
		config, err := Load(ctx, cwd, projectRoot, dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Validate the config
		err = config.Validate()
		require.NoError(t, err)

		require.NotNil(t, config.ID)
		require.NotNil(t, config.Config)
		require.NotNil(t, config.Config.Temperature)
		require.NotNil(t, config.Config.MaxTokens)

		assert.Equal(t, "code-assistant", config.ID)
		assert.Equal(t, ProviderAnthropic, config.Config.Provider)
		assert.Equal(t, ModelClaude3Opus, config.Config.Model)
		assert.InDelta(t, float32(0.7), config.Config.Temperature, 0.0001)
		assert.Equal(t, int32(4000), config.Config.MaxTokens)

		require.Len(t, config.Actions, 3)

		// Find the review-code action specifically
		var reviewAction *ActionConfig
		for _, action := range config.Actions {
			if action.ID == "review-code" {
				reviewAction = action
				break
			}
		}
		require.NotNil(t, reviewAction, "review-code action should be present")

		t.Logf("Review action found: %+v", reviewAction)
		t.Logf("Review action InputSchema: %+v", reviewAction.InputSchema)
		if reviewAction.InputSchema != nil {
			t.Logf("Review action InputSchema.Schema: %+v", reviewAction.InputSchema.Schema)
		}

		require.NotNil(t, reviewAction.InputSchema)
		schema := reviewAction.InputSchema.Schema
		assert.Equal(t, "object", schema.GetType())
		require.NotNil(t, schema.GetProperties())
		assert.Contains(t, schema.GetProperties(), "code")
		assert.Contains(t, schema.GetProperties(), "language")
		if required, ok := schema["required"].([]string); ok && len(required) > 0 {
			assert.Contains(t, required, "code")
		}

		require.NotNil(t, reviewAction.OutputSchema)
		outSchema := reviewAction.OutputSchema.Schema
		assert.Equal(t, "object", outSchema.GetType())
		require.NotNil(t, outSchema.GetProperties())
		assert.Contains(t, outSchema.GetProperties(), "feedback")

		feedback := outSchema.GetProperties()["feedback"]
		assert.NotNil(t, feedback)
		assert.Equal(t, "array", feedback.GetType())

		if itemsMap, ok := (*feedback)["items"].(map[string]any); ok {
			if typ, ok := itemsMap["type"].(string); ok {
				assert.Equal(t, "object", typ)
			}

			if props, ok := itemsMap["properties"].(map[string]any); ok {
				assert.Contains(t, props, "category")
				assert.Contains(t, props, "message")
				assert.Contains(t, props, "suggestion")
			}
		} else {
			t.Error("Items is not a map or not found")
		}
	})
}

func Test_AgentActionConfigValidation(t *testing.T) {
	agentID := "test-action"
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	fixturesDir := filepath.Join(filepath.Dir(filename), "fixtures")
	cwd, err := core.CWDFromPath(fixturesDir)
	require.NoError(t, err)

	// Create action metadata
	metadata := &core.ConfigMetadata{
		CWD:         cwd,
		FilePath:    filepath.Join(fixturesDir, "action.yaml"),
		ProjectRoot: fixturesDir,
	}

	t.Run("Should validate action config with all required fields", func(t *testing.T) {
		config := &ActionConfig{
			ID:     agentID,
			Prompt: "test prompt",
		}
		config.SetMetadata(metadata)
		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should return error when CWD is missing", func(t *testing.T) {
		config := &ActionConfig{
			ID:     agentID,
			Prompt: "test prompt",
		}
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "current working directory is required for test-action")
	})

	t.Run("Should return error when parameters are invalid", func(t *testing.T) {
		config := &ActionConfig{
			ID:     agentID,
			Prompt: "test prompt",
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
		}
		config.SetMetadata(metadata)
		err := config.ValidateParams(*config.With)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "with parameters invalid for test-action")
	})
}

func Test_AgentConfigCWD(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	fixturesDir := filepath.Join(filepath.Dir(filename), "fixtures")
	cwd, err := core.CWDFromPath(fixturesDir)
	require.NoError(t, err)

	metadata := &core.ConfigMetadata{
		CWD:         cwd,
		FilePath:    filepath.Join(fixturesDir, "agent.yaml"),
		ProjectRoot: fixturesDir,
	}

	t.Run("Should set and get CWD correctly", func(t *testing.T) {
		config := &Config{}
		config.SetMetadata(metadata)
		assert.Equal(t, filepath.Join(fixturesDir, "agent.yaml"), config.GetMetadata().FilePath)
	})

	t.Run("Should set metadata for all actions", func(t *testing.T) {
		config := &Config{}
		action := &ActionConfig{
			ID:     "test-action",
			Prompt: "test prompt",
		}
		config.Actions = []*ActionConfig{action}
		config.SetMetadata(metadata)
		assert.Equal(t, filepath.Join(fixturesDir, "agent.yaml"), action.GetMetadata().FilePath)
	})
}

func Test_AgentConfigMerge(t *testing.T) {
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

func Test_AgentConfigValidation(t *testing.T) {
	agentID := "test-agent"
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	fixturesDir := filepath.Join(filepath.Dir(filename), "fixtures")
	cwd, err := core.CWDFromPath(fixturesDir)
	require.NoError(t, err)

	// Create agent metadata
	metadata := &core.ConfigMetadata{
		CWD:         cwd,
		FilePath:    filepath.Join(fixturesDir, "agent.yaml"),
		ProjectRoot: fixturesDir,
	}

	t.Run("Should validate config with all required fields", func(t *testing.T) {
		config := &Config{
			ID:           agentID,
			Config:       ProviderConfig{},
			Instructions: "test instructions",
		}
		config.SetMetadata(metadata)
		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should return error when CWD is missing", func(t *testing.T) {
		config := &Config{
			ID:           agentID,
			Config:       ProviderConfig{},
			Instructions: "test instructions",
		}
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "current working directory is required for test-agent")
	})

	t.Run("Should return error when parameters are invalid", func(t *testing.T) {
		config := &Config{
			ID:           agentID,
			Config:       ProviderConfig{},
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
			With: &core.Input{
				"age": 42,
			},
		}
		config.SetMetadata(metadata)
		err := config.ValidateParams(config.With)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "with parameters invalid for test-agent")
	})
}

func Test_LoadAdvancedAgentWithReferences(t *testing.T) {
	t.Run("Should load advanced agent configuration with nested references correctly", func(t *testing.T) {
		cwd, projectRoot, dstPath := setupTest(t, "advanced_agent.yaml")

		// Run the test
		ctx := context.Background()
		config, err := Load(ctx, cwd, projectRoot, dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Validate the config
		err = config.Validate()
		require.NoError(t, err)

		// Verify basic agent properties
		assert.Equal(t, "advanced-code-assistant", config.ID)
		assert.Equal(t, ProviderAnthropic, config.Config.Provider)
		assert.Equal(t, ModelClaude3Opus, config.Config.Model)
		assert.InDelta(t, float32(0.3), config.Config.Temperature, 0.0001)
		assert.Equal(t, int32(8000), config.Config.MaxTokens)

		// Verify actions are loaded and resolved from references
		require.GreaterOrEqual(t, len(config.Actions), 4, "Expected at least 4 actions (3 from refs + inline actions)")

		// Find and verify referenced actions
		var formatAction, reviewAction, testAction *ActionConfig
		var architecturalAction, securityAction *ActionConfig

		for _, action := range config.Actions {
			switch action.ID {
			case "format-code":
				formatAction = action
			case "review-code":
				reviewAction = action
			case "generate-tests":
				testAction = action
			case "architectural-review":
				architecturalAction = action
			case "security-audit":
				securityAction = action
			}
		}

		// Verify format-code action was resolved from reference
		require.NotNil(t, formatAction, "format-code action should be loaded from reference")
		assert.Contains(t, formatAction.Prompt, "Format the provided code according to best practices")
		require.NotNil(t, formatAction.InputSchema, "Input schema should be resolved")
		assert.Equal(t, "object", formatAction.InputSchema.Schema.GetType())

		// Verify the input schema was resolved from schemas.yaml reference
		inputProps := formatAction.InputSchema.Schema.GetProperties()
		require.NotNil(t, inputProps)
		assert.Contains(t, inputProps, "code")
		assert.Contains(t, inputProps, "language")
		assert.Contains(t, inputProps, "options")

		// Verify review-code action was resolved from reference
		require.NotNil(t, reviewAction, "review-code action should be loaded from reference")
		assert.Contains(t, reviewAction.Prompt, "Review the provided code and provide comprehensive feedback")
		require.NotNil(t, reviewAction.OutputSchema, "Output schema should be resolved")

		// Verify the output schema was resolved from schemas.yaml reference
		outputProps := reviewAction.OutputSchema.Schema.GetProperties()
		require.NotNil(t, outputProps)
		assert.Contains(t, outputProps, "score")
		assert.Contains(t, outputProps, "feedback")
		assert.Contains(t, outputProps, "summary")

		// Verify generate-tests action
		require.NotNil(t, testAction, "generate-tests action should be loaded from reference")
		assert.Contains(t, testAction.Prompt, "Generate comprehensive unit tests")

		// Verify inline actions are present and properly configured
		require.NotNil(t, architecturalAction, "architectural-review action should be defined inline")
		assert.Contains(t, architecturalAction.Prompt, "architectural review")
		require.NotNil(t, architecturalAction.InputSchema)
		require.NotNil(t, architecturalAction.OutputSchema)

		require.NotNil(t, securityAction, "security-audit action should be defined inline")
		assert.Contains(t, securityAction.Prompt, "security audit")
		// Security action should reference schemas.yaml for input
		require.NotNil(t, securityAction.InputSchema)

		// Verify tools are loaded and resolved from references
		require.GreaterOrEqual(t, len(config.Tools), 4, "Expected at least 4 tools from references")

		// Find and verify referenced tools
		var formatterTool, linterTool, testGenTool, analyzerTool *tool.Config
		for i := range config.Tools {
			tool := &config.Tools[i]
			switch tool.ID {
			case "code-formatter":
				formatterTool = tool
			case "code-linter":
				linterTool = tool
			case "test-generator":
				testGenTool = tool
			case "code-analyzer":
				analyzerTool = tool
			}
		}

		// Verify tools were resolved from references
		require.NotNil(t, formatterTool, "code-formatter tool should be loaded from reference")
		assert.Equal(t, "./format.ts", formatterTool.Execute)
		require.NotNil(t, formatterTool.InputSchema)

		require.NotNil(t, linterTool, "code-linter tool should be loaded from reference")
		assert.Equal(t, "./lint.ts", linterTool.Execute)

		require.NotNil(t, testGenTool, "test-generator tool should be loaded from reference")
		assert.Equal(t, "./generate-tests.ts", testGenTool.Execute)

		require.NotNil(t, analyzerTool, "code-analyzer tool should be loaded from reference")
		assert.Equal(t, "./analyze.ts", analyzerTool.Execute)

		// Verify agent-level input schema was resolved from reference
		require.NotNil(t, config.InputSchema, "Agent input schema should be resolved from reference")
		agentInputProps := config.InputSchema.Schema.GetProperties()
		require.NotNil(t, agentInputProps)
		assert.Contains(t, agentInputProps, "code")
		assert.Contains(t, agentInputProps, "language")
		assert.Contains(t, agentInputProps, "focus_areas")

		// Verify agent-level output schema structure
		require.NotNil(t, config.OutputSchema, "Agent output schema should be defined")
		agentOutputProps := config.OutputSchema.Schema.GetProperties()
		require.NotNil(t, agentOutputProps)
		assert.Contains(t, agentOutputProps, "analysis_summary")
		assert.Contains(t, agentOutputProps, "detailed_results")
		assert.Contains(t, agentOutputProps, "action_plan")

		// Verify nested reference in output schema
		detailedResults := agentOutputProps["detailed_results"]
		require.NotNil(t, detailedResults)
		detailedProps := detailedResults.GetProperties()
		require.NotNil(t, detailedProps)
		assert.Contains(t, detailedProps, "code_quality")

		// Verify environment variables
		require.NotEmpty(t, config.Env)
		assert.Equal(t, "3.0.0", config.Env["AGENT_VERSION"])
		assert.Equal(t, "comprehensive", config.Env["ANALYSIS_DEPTH"])
		assert.Equal(t, "enabled", config.Env["SECURITY_SCANNING"])

		// Verify default parameters
		require.NotNil(t, config.With)
		with := *config.With
		assert.Equal(t, "comprehensive", with["analysis_depth"])
		assert.Equal(t, true, with["include_architectural_review"])
		assert.Equal(t, true, with["include_security_audit"])

		// Verify focus_areas is properly resolved as array
		focusAreas, ok := with["focus_areas"].([]any)
		require.True(t, ok, "focus_areas should be an array")
		assert.Contains(t, focusAreas, "performance")
		assert.Contains(t, focusAreas, "security")
		assert.Contains(t, focusAreas, "maintainability")
	})
}
