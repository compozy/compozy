package workflow

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/webhook"
	"github.com/compozy/compozy/pkg/logger"
	fixtures "github.com/compozy/compozy/test/fixtures"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTest(t *testing.T, workflowFile string) (*core.PathCWD, string) {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	cwd, dstPath := fixtures.SetupConfigTest(t, filename)
	dstPath = filepath.Join(dstPath, workflowFile)
	return cwd, dstPath
}

func Test_LoadWorkflow(t *testing.T) {
	t.Run("Should load basic workflow configuration correctly", func(t *testing.T) {
		cwd, dstPath := setupTest(t, "basic_workflow.yaml")
		config, err := Load(cwd, dstPath)
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
		// Execute field removed - tools resolved via entrypoint exports

		// Validate agents
		require.Len(t, config.Agents, 1)
		agentConfig := config.Agents[0]
		assert.Equal(t, "code-assistant", agentConfig.ID)
		assert.Equal(t, core.ProviderName("openai"), agentConfig.Model.Config.Provider)
		assert.Equal(t, "gpt-4o", agentConfig.Model.Config.Model)
		assert.InDelta(t, float64(0.7), agentConfig.Model.Config.Params.Temperature, 0.0001)
		assert.Equal(t, int32(4000), agentConfig.Model.Config.Params.MaxTokens)

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
		os.Setenv("MCP_PROXY_URL", "http://localhost:6001")
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

func fixturesCWD(t *testing.T) *core.PathCWD {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	dir := filepath.Join(filepath.Dir(filename), "fixtures")
	CWD, err := core.CWDFromPath(dir)
	require.NoError(t, err)
	return CWD
}

func TestWorkflowsFromProject_AndHelpers(t *testing.T) {
	t.Run("Should load workflows from project and inject env", func(t *testing.T) {
		cwd := fixturesCWD(t)
		proj := &project.Config{
			Name:      "proj-wf-list",
			Workflows: []*project.WorkflowSourceConfig{{Source: "basic_workflow.yaml"}, {Source: "mcp_workflow.yaml"}},
		}
		require.NoError(t, proj.SetCWD(cwd.PathStr()))
		proj.SetEnv(core.EnvMap{"X": "Y"})
		wfs, err := WorkflowsFromProject(proj)
		require.NoError(t, err)
		require.Len(t, wfs, 2)
		for _, w := range wfs {
			require.NotNil(t, w.Opts.Env)
			assert.Equal(t, "Y", w.GetEnv().Prop("X"))
			assert.Equal(t, core.ConfigWorkflow, w.Component())
			m, err := w.AsMap()
			require.NoError(t, err)
			require.NotEmpty(t, m["id"])
			var w2 Config
			require.NoError(t, w2.FromMap(m))
			assert.Equal(t, w.GetID(), w2.GetID())
		}
	})
	t.Run("Should find agent config across workflows and handle not found", func(t *testing.T) {
		wf1 := &Config{ID: "a", Agents: []agent.Config{{ID: "writer"}}}
		wf2 := &Config{ID: "b"}
		list := []*Config{wf1, wf2}
		got, err := FindAgentConfig[*agent.Config](list, "writer")
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, "writer", got.ID)
		_, err = FindAgentConfig[*agent.Config](list, "nope")
		require.Error(t, err)
		_, err = FindAgentConfig[*tool.Config](list, "writer")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "agent config is not of type *tool.Config")
	})
	t.Run("Should collect webhook slugs from workflows", func(t *testing.T) {
		wf1 := &Config{
			ID:       "w1",
			Triggers: []Trigger{{Type: TriggerTypeWebhook, Name: "web", Webhook: &webhook.Config{Slug: "hello"}}},
		}
		wf2 := &Config{ID: "w2", Triggers: []Trigger{{Type: TriggerTypeSignal, Name: "sig"}}}
		slugs := SlugsFromList([]*Config{wf1, wf2})
		assert.ElementsMatch(t, []string{"hello"}, slugs)
	})
	t.Run("Should determine next task and find workflow config", func(t *testing.T) {
		n2 := "t2"
		wf := &Config{ID: "wfX", Tasks: []task.Config{}}
		t1 := taskConfig("t1", &n2, nil)
		t2 := taskConfig("t2", nil, nil)
		w := &Config{ID: "wfY", Tasks: []task.Config{t1, t2}}
		next := w.DetermineNextTask(&w.Tasks[0], true)
		require.NotNil(t, next)
		assert.Equal(t, "t2", next.ID)
		got, err := FindConfig([]*Config{wf, w}, "wfY")
		require.NoError(t, err)
		assert.Equal(t, "wfY", got.ID)
	})
}

func taskConfig(id string, onSuccessNext *string, onErrorNext *string) task.Config {
	var onSuccess *core.SuccessTransition
	if onSuccessNext != nil {
		onSuccess = &core.SuccessTransition{Next: onSuccessNext}
	}
	var onError *core.ErrorTransition
	if onErrorNext != nil {
		onError = &core.ErrorTransition{Next: onErrorNext}
	}
	return task.Config{
		BaseConfig: task.BaseConfig{ID: id, Type: task.TaskTypeBasic, OnSuccess: onSuccess, OnError: onError},
	}
}

func TestLinkWorkflowTriggersSchemas(t *testing.T) {
	t.Run("Should resolve schema refs for trigger and webhook events", func(t *testing.T) {
		ctx := logger.ContextWithLogger(ctxWithBG(), logger.NewForTests())
		store := resources.NewMemoryResourceStore()
		proj := &project.Config{
			Name:    "proj1",
			Schemas: []schema.Schema{{"id": "trig", "type": "object"}, {"id": "evt", "type": "object"}},
		}
		require.NoError(t, proj.IndexToResourceStore(ctx, store))
		wf := &Config{
			ID: "wf1",
			Triggers: []Trigger{
				{
					Type:   TriggerTypeSignal,
					Name:   "t1",
					Schema: &schema.Schema{"__schema_ref__": "trig"},
					Webhook: &webhook.Config{
						Slug:   "s",
						Events: []webhook.EventConfig{{Name: "e1", Schema: &schema.Schema{"__schema_ref__": "evt"}}},
					},
				},
			},
		}
		require.NoError(t, linkWorkflowSchemas(ctx, proj, store, wf))
		require.NotNil(t, wf.Triggers[0].Schema)
		isRef, _ := wf.Triggers[0].Schema.IsRef()
		assert.False(t, isRef)
		require.NotNil(t, wf.Triggers[0].Webhook)
		require.Len(t, wf.Triggers[0].Webhook.Events, 1)
		isRefEvt, _ := wf.Triggers[0].Webhook.Events[0].Schema.IsRef()
		assert.False(t, isRefEvt)
	})
}
