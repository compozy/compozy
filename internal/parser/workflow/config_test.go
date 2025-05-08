package workflow

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/compozy/compozy/internal/parser/agent"
	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/pkgref"
	"github.com/compozy/compozy/internal/parser/provider"
	"github.com/compozy/compozy/internal/parser/schema"
	"github.com/compozy/compozy/internal/parser/task"
	"github.com/compozy/compozy/internal/parser/tool"
	"github.com/compozy/compozy/internal/parser/trigger"
	"github.com/compozy/compozy/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	// Set test mode for all packages
	TestMode = true
	tool.TestMode = true
	// Run tests
	m.Run()
}

func Test_LoadWorkflow(t *testing.T) {
	// Set test mode at the beginning
	TestMode = true
	tool.TestMode = true
	defer func() {
		TestMode = false
		tool.TestMode = false
	}()

	t.Run("Should load basic workflow configuration correctly", func(t *testing.T) {
		// Get the test directory path
		_, filename, _, ok := runtime.Caller(0)
		require.True(t, ok)
		testDir := filepath.Dir(filename)

		// Setup test fixture using utils
		dstPath := utils.SetupFixture(t, testDir, "basic_workflow.yaml")

		// Run the test
		config, err := Load(dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Set CWD for all tasks
		for i := range config.Tasks {
			config.Tasks[i].SetCWD(config.GetCWD())
		}

		// Validate the config
		err = config.Validate()
		require.NoError(t, err)

		TestMode = true // Skip file existence check for valid test
		defer func() { TestMode = false }()

		require.NotNil(t, config.ID)
		require.NotNil(t, config.Version)
		require.NotNil(t, config.Description)
		require.NotNil(t, config.Tasks)
		require.NotNil(t, config.Tools)
		require.NotNil(t, config.Agents)
		require.NotNil(t, config.Trigger)
		require.NotNil(t, config.Env)

		assert.Equal(t, "test-workflow", config.ID)
		assert.Equal(t, "1.0.0", config.Version)
		assert.Equal(t, "Test workflow for code formatting", config.Description)

		// Validate tasks
		require.Len(t, config.Tasks, 2)
		task := config.Tasks[0]
		assert.Equal(t, "format-code", task.ID)
		assert.Equal(t, "basic", string(task.Type))
		require.NotNil(t, task.Use)
		assert.Equal(t, pkgref.NewPackageRefConfig("agent(id=code-assistant)"), task.Use)
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
		agent := config.Agents[0]
		assert.Equal(t, "code-assistant", agent.ID)
		require.NotNil(t, agent.Config)
		assert.Equal(t, provider.ProviderName("anthropic"), agent.Config.Provider)
		assert.Equal(t, provider.ModelName("claude-3-opus"), agent.Config.Model)
		assert.InDelta(t, float32(0.7), agent.Config.Temperature, 0.0001)
		assert.Equal(t, int32(4000), agent.Config.MaxTokens)

		// Validate trigger
		assert.Equal(t, trigger.TriggerType("webhook"), config.Trigger.Type)
		require.NotNil(t, config.Trigger.Config)
		assert.Equal(t, "/test-webhook", config.Trigger.Config.URL)

		// Validate env
		assert.Equal(t, "1.0.0", config.Env["WORKFLOW_VERSION"])
		assert.Equal(t, "3", config.Env["MAX_RETRIES"])
	})

	t.Run("Should return error for invalid workflow configuration", func(t *testing.T) {
		// Get the test directory path
		_, filename, _, ok := runtime.Caller(0)
		require.True(t, ok)
		testDir := filepath.Dir(filename)

		// Setup test fixture using utils
		dstPath := utils.SetupFixture(t, testDir, "invalid_workflow.yaml")

		// Run the test
		config, err := Load(dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Set CWD for all tasks
		for i := range config.Tasks {
			config.Tasks[i].SetCWD(config.GetCWD())
		}

		// Validate the config
		err = config.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Basic task configuration is required for basic task type")
	})
}

func Test_WorkflowConfigValidation(t *testing.T) {
	workflowID := "test-workflow"

	t.Run("Should validate valid workflow configuration", func(t *testing.T) {
		config := &WorkflowConfig{
			ID: workflowID,
			Trigger: trigger.TriggerConfig{
				Type: trigger.TriggerTypeWebhook,
				Config: &trigger.WebhookConfig{
					URL: "/test",
				},
			},
			cwd: common.NewCWD("/test/path"),
		}

		err := config.Validate()
		require.NoError(t, err)
	})

	t.Run("Should return error when CWD is missing", func(t *testing.T) {
		config := &WorkflowConfig{
			ID: "test-workflow",
		}

		err := config.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "current working directory is required for test-workflow")
	})

	t.Run("Should return error for task with invalid parameters", func(t *testing.T) {
		config := &WorkflowConfig{
			ID:  "test-workflow",
			cwd: common.NewCWD("/test/path"),
			Tasks: []task.TaskConfig{
				{
					ID:   "test-task",
					Type: task.TaskTypeBasic,
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
					With: &common.WithParams{
						"age": 42,
					},
				},
			},
		}

		err := config.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "with parameters invalid for test-task")
	})

	t.Run("Should return error for tool with invalid parameters", func(t *testing.T) {
		config := &WorkflowConfig{
			ID:  "test-workflow",
			cwd: common.NewCWD("/test/path"),
			Tools: []tool.ToolConfig{
				{
					ID:      "test-tool",
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
					With: &common.WithParams{
						"age": 42,
					},
				},
			},
		}

		err := config.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "with parameters invalid for test-tool")
	})

	t.Run("Should return error for agent with invalid parameters", func(t *testing.T) {
		config := &WorkflowConfig{
			ID:  "test-workflow",
			cwd: common.NewCWD("/test/path"),
			Agents: []agent.AgentConfig{
				{
					ID: "test-agent",
					Actions: []*agent.AgentActionConfig{
						{
							ID:     "test-action",
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
							With: &common.WithParams{
								"age": 42,
							},
						},
					},
				},
			},
		}

		err := config.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "with parameters invalid for test-action")
	})
}

func Test_WorkflowConfigCWD(t *testing.T) {
	t.Run("Should handle CWD operations correctly", func(t *testing.T) {
		config := &WorkflowConfig{}

		// Test setting CWD
		config.SetCWD("/test/path")
		assert.Equal(t, "/test/path", config.GetCWD())

		// Test updating CWD
		config.SetCWD("/new/path")
		assert.Equal(t, "/new/path", config.GetCWD())
	})
}

func Test_WorkflowConfigMerge(t *testing.T) {
	t.Run("Should merge configurations correctly", func(t *testing.T) {
		baseConfig := &WorkflowConfig{
			Env: common.EnvMap{
				"KEY1": "value1",
			},
		}

		otherConfig := &WorkflowConfig{
			Env: common.EnvMap{
				"KEY2": "value2",
			},
		}

		err := baseConfig.Merge(otherConfig)
		require.NoError(t, err)

		// Check that base config has both env variables
		assert.Equal(t, "value1", baseConfig.Env["KEY1"])
		assert.Equal(t, "value2", baseConfig.Env["KEY2"])
	})
}
