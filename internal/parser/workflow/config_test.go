package workflow

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/compozy/compozy/internal/parser/agent"
	"github.com/compozy/compozy/internal/parser/common"
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

func TestLoadWorkflow(t *testing.T) {
	// Set test mode at the beginning
	TestMode = true
	tool.TestMode = true
	defer func() {
		TestMode = false
		tool.TestMode = false
	}()

	tests := []struct {
		name     string
		fixture  string
		wantErr  bool
		validate func(*testing.T, *WorkflowConfig)
	}{
		{
			name:    "basic workflow",
			fixture: "basic_workflow.yaml",
			validate: func(t *testing.T, config *WorkflowConfig) {
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

				assert.Equal(t, WorkflowID("test-workflow"), config.ID)
				assert.Equal(t, WorkflowVersion("1.0.0"), *config.Version)
				assert.Equal(t, WorkflowDescription("Test workflow for code formatting"), *config.Description)

				// Validate tasks
				require.Len(t, config.Tasks, 2)
				task := config.Tasks[0]
				assert.Equal(t, "format-code", string(*task.ID))
				assert.Equal(t, "basic", string(task.Type))
				require.NotNil(t, task.Use)
				assert.Equal(t, "agent(id=code-assistant)", string(*task.Use))
				require.NotNil(t, task.Action)
				assert.Equal(t, "format-code", string(*task.Action))

				// Validate tools
				require.Len(t, config.Tools, 1)
				tool := config.Tools[0]
				assert.Equal(t, "code-formatter", string(*tool.ID))
				assert.Equal(t, "A tool for formatting code", string(*tool.Description))
				assert.Equal(t, "./format.ts", string(*tool.Execute))

				// Validate agents
				require.Len(t, config.Agents, 1)
				agent := config.Agents[0]
				assert.Equal(t, "code-assistant", string(*agent.ID))
				require.NotNil(t, agent.Config)
				assert.Equal(t, "anthropic", string(agent.Config.Provider))
				assert.Equal(t, "claude-3-opus", string(agent.Config.Model))
				assert.InDelta(t, float32(0.7), float32(*agent.Config.Temperature), 0.0001)
				assert.Equal(t, uint32(4000), uint32(*agent.Config.MaxTokens))

				// Validate trigger
				assert.Equal(t, "webhook", string(config.Trigger.Type))
				require.NotNil(t, config.Trigger.Webhook)
				assert.Equal(t, "/test-webhook", string(config.Trigger.Webhook.URL))

				// Validate env
				assert.Equal(t, "1.0.0", config.Env["WORKFLOW_VERSION"])
				assert.Equal(t, "3", config.Env["MAX_RETRIES"])
			},
		},
		{
			name:    "invalid workflow",
			fixture: "invalid_workflow.yaml",
			wantErr: true,
			validate: func(t *testing.T, config *WorkflowConfig) {
				TestMode = false // Enable file existence check for invalid test
				err := config.Validate()
				require.Error(t, err)
				assert.Contains(t, err.Error(), "Basic task configuration is required for basic task type")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get the test directory path
			_, filename, _, ok := runtime.Caller(0)
			require.True(t, ok)
			testDir := filepath.Dir(filename)

			// Setup test fixture using utils
			dstPath := utils.SetupFixture(t, testDir, tt.fixture)

			// Run the test
			config, err := Load(dstPath)
			if err != nil {
				if tt.wantErr {
					if tt.validate != nil {
						tt.validate(t, config)
					}
					return
				}
				require.NoError(t, err)
			}
			require.NotNil(t, config)

			// Set CWD for all tasks
			for i := range config.Tasks {
				config.Tasks[i].SetCWD(config.GetCWD())
			}

			// Validate the config
			err = config.Validate()
			if err != nil {
				if tt.wantErr {
					if tt.validate != nil {
						tt.validate(t, config)
					}
					return
				}
				require.NoError(t, err)
			}

			if tt.validate != nil {
				tt.validate(t, config)
			}
		})
	}
}

func TestWorkflowConfigValidation(t *testing.T) {
	workflowID := WorkflowID("test-workflow")
	tests := []struct {
		name    string
		config  *WorkflowConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid Config",
			config: &WorkflowConfig{
				ID: workflowID,
				Trigger: trigger.TriggerConfig{
					Type: trigger.TriggerTypeWebhook,
					Webhook: &trigger.WebhookConfig{
						URL: "/test",
					},
				},
				cwd: common.NewCWD("/test/path"),
			},
			wantErr: false,
		},
		{
			name: "Missing CWD",
			config: &WorkflowConfig{
				ID: workflowID,
			},
			wantErr: true,
			errMsg:  "Current working directory is required for test-workflow",
		},
		{
			name: "Invalid Task With Params",
			config: &WorkflowConfig{
				ID: workflowID,
				Tasks: []task.TaskConfig{
					{
						ID:   func() *task.TaskID { id := task.TaskID("test-task"); return &id }(),
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
				Trigger: trigger.TriggerConfig{
					Type: trigger.TriggerTypeWebhook,
					Webhook: &trigger.WebhookConfig{
						URL: "/test",
					},
				},
				cwd: common.NewCWD("/test/path"),
			},
			wantErr: true,
			errMsg:  "With parameters invalid for test-task",
		},
		{
			name: "Invalid Tool With Params",
			config: &WorkflowConfig{
				ID: workflowID,
				Tools: []tool.ToolConfig{
					{
						ID:      func() *tool.ToolID { id := tool.ToolID("test-tool"); return &id }(),
						Execute: func() *tool.ToolExecute { e := tool.ToolExecute("./test.ts"); return &e }(),
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
				Trigger: trigger.TriggerConfig{
					Type: trigger.TriggerTypeWebhook,
					Webhook: &trigger.WebhookConfig{
						URL: "/test",
					},
				},
				cwd: common.NewCWD("/test/path"),
			},
			wantErr: true,
			errMsg:  "With parameters invalid for test-tool",
		},
		{
			name: "Invalid Agent With Params",
			config: &WorkflowConfig{
				ID: workflowID,
				Agents: []agent.AgentConfig{
					{
						ID: func() *agent.AgentID { id := agent.AgentID("test-agent"); return &id }(),
						Config: &provider.ProviderConfig{
							Provider:    "anthropic",
							Model:       "claude-3-opus",
							Temperature: func() *provider.Temperature { t := provider.Temperature(0.7); return &t }(),
							MaxTokens:   func() *provider.MaxTokens { t := provider.MaxTokens(4000); return &t }(),
						},
						Instructions: func() *agent.Instructions { i := agent.Instructions("Test instructions"); return &i }(),
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
				Trigger: trigger.TriggerConfig{
					Type: trigger.TriggerTypeWebhook,
					Webhook: &trigger.WebhookConfig{
						URL: "/test",
					},
				},
				cwd: common.NewCWD("/test/path"),
			},
			wantErr: true,
			errMsg:  "With parameters invalid for test-agent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestWorkflowConfigCWD(t *testing.T) {
	config := &WorkflowConfig{}

	// Test setting CWD
	config.SetCWD("/test/path")
	assert.Equal(t, "/test/path", config.GetCWD())

	// Test updating CWD
	config.SetCWD("/new/path")
	assert.Equal(t, "/new/path", config.GetCWD())
}

func TestWorkflowConfigMerge(t *testing.T) {
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
}
