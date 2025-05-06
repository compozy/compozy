package agent

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/pkgref"
	"github.com/compozy/compozy/internal/parser/provider"
	"github.com/compozy/compozy/internal/parser/schema"
	"github.com/compozy/compozy/internal/parser/tool"
	"github.com/compozy/compozy/internal/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadAgent(t *testing.T) {
	tests := []struct {
		name     string
		fixture  string
		wantErr  bool
		validate func(*testing.T, *AgentConfig)
	}{
		{
			name:    "basic agent",
			fixture: "basic_agent.yaml",
			validate: func(t *testing.T, config *AgentConfig) {
				require.NotNil(t, config.ID)
				require.NotNil(t, config.Config)
				require.NotNil(t, config.Config.Temperature)
				require.NotNil(t, config.Config.MaxTokens)

				assert.Equal(t, AgentID("code-assistant"), *config.ID)
				assert.Equal(t, provider.ProviderAnthropic, config.Config.Provider)
				assert.Equal(t, provider.ModelClaude3Opus, config.Config.Model)
				assert.InDelta(t, float32(0.7), float32(*config.Config.Temperature), 0.0001)
				assert.Equal(t, uint32(4000), uint32(*config.Config.MaxTokens))

				require.Len(t, config.Actions, 1)
				action := config.Actions[0]
				assert.Equal(t, ActionID("review-code"), action.ID)

				require.NotNil(t, action.InputSchema)
				schema := action.InputSchema.Schema
				assert.Equal(t, "object", schema.GetType())
				require.NotNil(t, schema.GetProperties())
				assert.Contains(t, schema.GetProperties(), "code")
				assert.Contains(t, schema.GetProperties(), "language")
				if required, ok := schema["required"].([]string); ok && len(required) > 0 {
					assert.Contains(t, required, "code")
				}

				require.NotNil(t, action.OutputSchema)
				outSchema := action.OutputSchema.Schema
				assert.Equal(t, "object", outSchema.GetType())
				require.NotNil(t, outSchema.GetProperties())
				assert.Contains(t, outSchema.GetProperties(), "feedback")

				feedback := outSchema.GetProperties()["feedback"]
				assert.NotNil(t, feedback)
				assert.Equal(t, "array", feedback.GetType())

				// Get items by accessing the items map directly
				if itemsMap, ok := (*feedback)["items"].(map[string]any); ok {
					// Check type directly
					if typ, ok := itemsMap["type"].(string); ok {
						assert.Equal(t, "object", typ)
					}

					// Check properties directly
					if props, ok := itemsMap["properties"].(map[string]any); ok {
						assert.Contains(t, props, "category")
						assert.Contains(t, props, "description")
						assert.Contains(t, props, "suggestion")
					}
				} else {
					t.Error("Items is not a map or not found")
				}
			},
		},
		// Add more test cases here as needed
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get the test directory path
			_, filename, _, ok := runtime.Caller(0)
			require.True(t, ok)
			testDir := filepath.Dir(filename)

			// Setup test fixture using testutils
			dstPath := testutils.SetupFixture(t, testDir, tt.fixture)

			// Run the test
			config, err := Load(dstPath)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, config)
			if tt.validate != nil {
				tt.validate(t, config)
			}
		})
	}
}

func TestAgentActionConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *AgentActionConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid Action Config",
			config: &AgentActionConfig{
				ID:     "test-action",
				Prompt: "test prompt",
				cwd:    common.NewCWD("/test/path"),
			},
			wantErr: false,
		},
		{
			name: "Missing CWD",
			config: &AgentActionConfig{
				ID:     "test-action",
				Prompt: "test prompt",
			},
			wantErr: true,
			errMsg:  "Current working directory is required for test-action",
		},
		{
			name: "Valid With Params",
			config: &AgentActionConfig{
				ID:     "test-action",
				Prompt: "test prompt",
				cwd:    common.NewCWD("/test/path"),
				InputSchema: &schema.InputSchema{
					Schema: schema.Schema{
						"type": "object",
						"properties": map[string]any{
							"name": map[string]any{
								"type": "string",
							},
						},
					},
				},
				With: &common.WithParams{
					"name": "test",
				},
			},
			wantErr: false,
		},
		{
			name: "Invalid With Params",
			config: &AgentActionConfig{
				ID:     "test-action",
				Prompt: "test prompt",
				cwd:    common.NewCWD("/test/path"),
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
			wantErr: true,
			errMsg:  "With parameters invalid for test-action",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAgentConfigCWD(t *testing.T) {
	config := &AgentConfig{}

	// Test setting CWD
	config.SetCWD("/test/path")
	assert.Equal(t, "/test/path", config.GetCWD())

	// Test setting CWD for actions
	action := &AgentActionConfig{
		ID:     "test-action",
		Prompt: "test prompt",
	}
	config.Actions = []*AgentActionConfig{action}
	config.SetCWD("/new/path")
	assert.Equal(t, "/new/path", action.GetCWD())
}

func TestAgentConfigMerge(t *testing.T) {
	baseConfig := &AgentConfig{
		Env: common.EnvMap{
			"KEY1": "value1",
		},
		With: &common.WithParams{},
	}

	otherConfig := &AgentConfig{
		Env: common.EnvMap{
			"KEY2": "value2",
		},
		With: &common.WithParams{},
	}

	err := baseConfig.Merge(otherConfig)
	require.NoError(t, err)

	// Check that base config has both env variables
	assert.Equal(t, "value1", baseConfig.Env["KEY1"])
	assert.Equal(t, "value2", baseConfig.Env["KEY2"])
}

func TestAgentConfigValidation(t *testing.T) {
	agentID := AgentID("test-agent")
	tests := []struct {
		name    string
		config  *AgentConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid Config",
			config: &AgentConfig{
				ID:           &agentID,
				Config:       &provider.ProviderConfig{},
				Instructions: func() *Instructions { i := Instructions("test instructions"); return &i }(),
				cwd:          common.NewCWD("/test/path"),
			},
			wantErr: false,
		},
		{
			name: "Missing CWD",
			config: &AgentConfig{
				ID:           &agentID,
				Config:       &provider.ProviderConfig{},
				Instructions: func() *Instructions { i := Instructions("test instructions"); return &i }(),
			},
			wantErr: true,
			errMsg:  "Current working directory is required",
		},
		{
			name: "Invalid Package Reference",
			config: &AgentConfig{
				ID:      &agentID,
				Use:     pkgref.NewPackageRefConfig("invalid"),
				Config:  &provider.ProviderConfig{},
				Tools:   []*tool.ToolConfig{},
				Actions: []*AgentActionConfig{},
				cwd:     common.NewCWD("/test/path"),
			},
			wantErr: true,
			errMsg:  "Invalid package reference",
		},
		{
			name: "Input Schema Not Allowed with ID Reference",
			config: &AgentConfig{
				ID:           &agentID,
				Use:          pkgref.NewPackageRefConfig("agent(id=test-agent)"),
				Config:       &provider.ProviderConfig{},
				Instructions: func() *Instructions { i := Instructions("test instructions"); return &i }(),
				InputSchema: &schema.InputSchema{
					Schema: schema.Schema{
						"type": "object",
					},
				},
				cwd: common.NewCWD("/test/path"),
			},
			wantErr: true,
			errMsg:  "Input schema not allowed for reference type id",
		},
		{
			name: "Output Schema Not Allowed with File Reference",
			config: &AgentConfig{
				ID:           &agentID,
				Use:          pkgref.NewPackageRefConfig("agent(file=basic_agent.yaml)"),
				Config:       &provider.ProviderConfig{},
				Instructions: func() *Instructions { i := Instructions("test instructions"); return &i }(),
				OutputSchema: &schema.OutputSchema{
					Schema: schema.Schema{
						"type": "object",
					},
				},
				cwd: common.NewCWD("/test/data"),
			},
			wantErr: true,
			errMsg:  "Output schema not allowed for reference type file",
		},
		{
			name: "Both Schemas Not Allowed with Dep Reference",
			config: &AgentConfig{
				ID:           &agentID,
				Use:          pkgref.NewPackageRefConfig("agent(dep=compozy/agents:test-agent)"),
				Config:       &provider.ProviderConfig{},
				Instructions: func() *Instructions { i := Instructions("test instructions"); return &i }(),
				InputSchema: &schema.InputSchema{
					Schema: schema.Schema{
						"type": "object",
					},
				},
				OutputSchema: &schema.OutputSchema{
					Schema: schema.Schema{
						"type": "object",
					},
				},
				cwd: common.NewCWD("/test/path"),
			},
			wantErr: true,
			errMsg:  "Input schema not allowed for reference type dep",
		},
		{
			name: "Valid With Params",
			config: &AgentConfig{
				ID:           &agentID,
				Config:       &provider.ProviderConfig{},
				Instructions: func() *Instructions { i := Instructions("test instructions"); return &i }(),
				InputSchema: &schema.InputSchema{
					Schema: schema.Schema{
						"type": "object",
						"properties": map[string]any{
							"name": map[string]any{
								"type": "string",
							},
						},
					},
				},
				With: &common.WithParams{
					"name": "test",
				},
				cwd: common.NewCWD("/test/path"),
			},
			wantErr: false,
		},
		{
			name: "Invalid With Params",
			config: &AgentConfig{
				ID:           &agentID,
				Config:       &provider.ProviderConfig{},
				Instructions: func() *Instructions { i := Instructions("test instructions"); return &i }(),
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
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
