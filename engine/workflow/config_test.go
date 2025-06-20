package workflow

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/pkg/ref"
	"github.com/compozy/compozy/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTest(t *testing.T, workflowFile string) (*core.PathCWD, string) {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	cwd, dstPath := utils.SetupTest(t, filename)
	dstPath = filepath.Join(dstPath, workflowFile)
	return cwd, dstPath
}

var globalScope = map[string]any{
	"models": []any{
		map[string]any{
			"id":       "gpt-4o",
			"provider": "openai",
			"model":    "gpt-4o",
			"params": map[string]any{
				"temperature": 0.7,
				"max_tokens":  4000,
			},
		},
	},
}

func Test_LoadWorkflow(t *testing.T) {
	t.Run("Should load basic workflow configuration correctly", func(t *testing.T) {
		cwd, dstPath := setupTest(t, "basic_workflow.yaml")
		ev := ref.NewEvaluator(ref.WithGlobalScope(globalScope))
		config, err := LoadAndEval(cwd, dstPath, ev)
		require.NoError(t, err)
		require.NotNil(t, config)
		require.NotNil(t, config.Opts)
		require.NotNil(t, config.ID)
		require.NotNil(t, config.Version)
		require.NotNil(t, config.Description)
		require.NotNil(t, config.Tasks)
		require.NotNil(t, config.Tools)
		require.NotNil(t, config.Agents)
		require.NotNil(t, config.Opts.Env)

		assert.Equal(t, "test-workflow", config.ID)
		assert.Equal(t, "1.0.0", config.Version)
		assert.Equal(t, "Test workflow for code formatting", config.Description)

		// Validate tasks
		require.Len(t, config.Tasks, 2)
		task := config.Tasks[0]
		assert.Equal(t, "format-code", task.ID)
		assert.Equal(t, "basic", string(task.Type))
		require.NotNil(t, task.Action)
		assert.Equal(t, "format-code", task.Action)

		// Validate tools
		require.Len(t, config.Tools, 1)
		tool := config.Tools[0]
		assert.Equal(t, "code-formatter", tool.ID)
		assert.Equal(t, "A tool for formatting code", tool.Description)
		assert.Equal(t, "./format.ts", tool.Execute)

		// Validate agents
		require.Len(t, config.Agents, 1)
		agentConfig := config.Agents[0]
		assert.Equal(t, "code-assistant", agentConfig.ID)
		require.NotNil(t, agentConfig.Config)
		assert.Equal(t, core.ProviderName("openai"), agentConfig.Config.Provider)
		assert.Equal(t, "gpt-4o", agentConfig.Config.Model)
		assert.InDelta(t, float64(0.7), agentConfig.Config.Params.Temperature, 0.0001)
		assert.Equal(t, int32(4000), agentConfig.Config.Params.MaxTokens)

		// Validate env
		assert.Equal(t, "1.0.0", config.GetEnv().Prop("WORKFLOW_VERSION"))
		assert.Equal(t, "3", config.GetEnv().Prop("MAX_RETRIES"))
	})

	t.Run("Should return error for invalid workflow configuration", func(t *testing.T) {
		cwd, dstPath := setupTest(t, "invalid_workflow.yaml")
		config, err := Load(cwd, dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		err = config.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "condition is required for router tasks")
	})
}

func Test_WorkflowConfigCWD(t *testing.T) {
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

func Test_WorkflowConfigMerge(t *testing.T) {
	t.Run("Should merge configurations correctly", func(t *testing.T) {
		baseConfig := &Config{
			Opts: Opts{
				Env: &core.EnvMap{
					"KEY1": "value1",
				},
			},
		}

		otherConfig := &Config{
			Opts: Opts{
				Env: &core.EnvMap{
					"KEY2": "value2",
				},
			},
		}

		err := baseConfig.Merge(otherConfig)
		require.NoError(t, err)

		// Check that base config has both env variables
		assert.Equal(t, "value1", baseConfig.GetEnv().Prop("KEY1"))
		assert.Equal(t, "value2", baseConfig.GetEnv().Prop("KEY2"))
	})
}

func TestLoadMCPWorkflow(t *testing.T) {
	t.Run("Should load MCP workflow configuration successfully", func(t *testing.T) {
		CWD, err := core.CWDFromPath("./fixtures")
		require.NoError(t, err)

		config, err := Load(CWD, "mcp_workflow.yaml")
		require.NoError(t, err)
		require.NotNil(t, config)

		// Verify basic workflow properties
		assert.Equal(t, "mcp-test-workflow", config.ID)
		assert.Equal(t, "1.0.0", config.Version)
		assert.Equal(t, "Test workflow with MCP server integration", config.Description)
	})

	t.Run("Should parse MCP server configurations correctly", func(t *testing.T) {
		CWD, err := core.CWDFromPath("./fixtures")
		require.NoError(t, err)

		config, err := Load(CWD, "mcp_workflow.yaml")
		require.NoError(t, err)

		// Verify MCP configurations
		assert.Len(t, config.MCPs, 2)

		// Check primary MCP server
		primaryMCP := config.MCPs[0]
		assert.Equal(t, "primary-mcp-server", primaryMCP.ID)
		assert.Equal(t, "http://localhost:4000/mcp", primaryMCP.URL)
		assert.Equal(t, "{{ .env.MCP_API_KEY }}", primaryMCP.Env["API_KEY"])

		// Check secondary MCP server
		secondaryMCP := config.MCPs[1]
		assert.Equal(t, "secondary-mcp-server", secondaryMCP.ID)
		assert.Equal(t, "https://api.example.com/mcp", secondaryMCP.URL)
		assert.Equal(t, "{{ .env.EXTERNAL_MCP_TOKEN }}", secondaryMCP.Env["AUTH_TOKEN"])
	})

	t.Run("Should pass validation for valid MCP configuration", func(t *testing.T) {
		// Set required environment variable for MCP validation
		os.Setenv("MCP_PROXY_URL", "http://localhost:8081")
		defer os.Unsetenv("MCP_PROXY_URL")

		CWD, err := core.CWDFromPath("./fixtures")
		require.NoError(t, err)

		config, err := Load(CWD, "mcp_workflow.yaml")
		require.NoError(t, err)

		err = config.Validate()
		assert.NoError(t, err)
	})
}

func TestConfig_ApplyInputDefaults(t *testing.T) {
	t.Run("Should apply defaults from input schema", func(t *testing.T) {
		config := &Config{
			ID: "test-workflow",
			Opts: Opts{
				InputSchema: &schema.Schema{
					"type": "object",
					"properties": map[string]any{
						"folder_path": map[string]any{
							"type": "string",
						},
						"include_extensions": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "string",
							},
							"default": []any{".go", ".yaml", ".yml"},
						},
						"exclude_patterns": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "string",
							},
							"default": []any{"*_test.go", "*.bak", "*.tmp"},
						},
						"report_format": map[string]any{
							"type":    "string",
							"default": "markdown",
						},
					},
					"required": []string{"folder_path"},
				},
			},
		}

		input := &core.Input{
			"folder_path": "/path/to/code",
			// Not providing exclude_patterns, should get default
		}

		result, err := config.ApplyInputDefaults(input)
		require.NoError(t, err)

		// Should have user-provided value
		assert.Equal(t, "/path/to/code", (*result)["folder_path"])

		// Should have default values
		assert.Equal(t, []any{".go", ".yaml", ".yml"}, (*result)["include_extensions"])
		assert.Equal(t, []any{"*_test.go", "*.bak", "*.tmp"}, (*result)["exclude_patterns"])
		assert.Equal(t, "markdown", (*result)["report_format"])
	})

	t.Run("Should handle nil input schema", func(t *testing.T) {
		config := &Config{
			ID: "test-workflow",
			// No input schema
		}

		input := &core.Input{
			"test": "value",
		}

		result, err := config.ApplyInputDefaults(input)
		require.NoError(t, err)
		assert.Equal(t, input, result)
	})

	t.Run("Should handle nil input", func(t *testing.T) {
		config := &Config{
			ID: "test-workflow",
			Opts: Opts{
				InputSchema: &schema.Schema{
					"type": "object",
					"properties": map[string]any{
						"default_prop": map[string]any{
							"type":    "string",
							"default": "default_value",
						},
					},
				},
			},
		}

		result, err := config.ApplyInputDefaults(nil)
		require.NoError(t, err)

		assert.Equal(t, "default_value", (*result)["default_prop"])
	})

	t.Run("Should override defaults with user values", func(t *testing.T) {
		config := &Config{
			ID: "test-workflow",
			Opts: Opts{
				InputSchema: &schema.Schema{
					"type": "object",
					"properties": map[string]any{
						"mode": map[string]any{
							"type":    "string",
							"default": "production",
						},
						"debug": map[string]any{
							"type":    "boolean",
							"default": false,
						},
					},
				},
			},
		}

		input := &core.Input{
			"mode":  "development", // Override default
			"debug": true,          // Override default
		}

		result, err := config.ApplyInputDefaults(input)
		require.NoError(t, err)

		// Should use user-provided values, not defaults
		assert.Equal(t, "development", (*result)["mode"])
		assert.Equal(t, true, (*result)["debug"])
	})
}

func TestWorkflowConfig_Outputs(t *testing.T) {
	t.Run("Should get outputs when defined", func(t *testing.T) {
		outputs := &core.Output{
			"result": "{{ .tasks.final.output }}",
		}
		config := &Config{
			ID:      "test-workflow",
			Outputs: outputs,
		}

		assert.Equal(t, outputs, config.GetOutputs())
	})

	t.Run("Should return nil when outputs not defined", func(t *testing.T) {
		config := &Config{
			ID: "test-workflow",
		}

		assert.Nil(t, config.GetOutputs())
	})
}

func TestWorkflowConfig_ScheduleDefaults(t *testing.T) {
	t.Run("Should set schedule defaults when schedule exists", func(t *testing.T) {
		config := &Config{
			Schedule: &Schedule{
				Cron: "0 9 * * *",
				// Enabled and OverlapPolicy not set
			},
		}
		config.SetDefaults()
		// Check defaults were applied
		require.NotNil(t, config.Schedule.Enabled)
		assert.True(t, *config.Schedule.Enabled)
		assert.Equal(t, OverlapSkip, config.Schedule.OverlapPolicy)
	})
	t.Run("Should not override existing schedule values", func(t *testing.T) {
		enabled := false
		config := &Config{
			Schedule: &Schedule{
				Cron:          "0 9 * * *",
				Enabled:       &enabled,
				OverlapPolicy: OverlapAllow,
			},
		}
		config.SetDefaults()
		// Check existing values were preserved
		require.NotNil(t, config.Schedule.Enabled)
		assert.False(t, *config.Schedule.Enabled)
		assert.Equal(t, OverlapAllow, config.Schedule.OverlapPolicy)
	})
	t.Run("Should not panic when schedule is nil", func(t *testing.T) {
		config := &Config{
			Schedule: nil,
		}
		assert.NotPanics(t, func() {
			config.SetDefaults()
		})
	})
}
