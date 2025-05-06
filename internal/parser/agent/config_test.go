package agent

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/package_ref"
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
				assert.Equal(t, ProviderAnthropic, config.Config.Provider)
				assert.Equal(t, ModelClaude3Opus, config.Config.Model)
				assert.InDelta(t, float32(0.7), float32(*config.Config.Temperature), 0.0001)
				assert.Equal(t, uint32(4000), uint32(*config.Config.MaxTokens))

				require.Len(t, config.Actions, 1)
				action := config.Actions[0]
				assert.Equal(t, ActionID("review-code"), action.ID)

				require.NotNil(t, action.InputSchema)
				schema := action.InputSchema
				assert.Equal(t, "object", schema.Type)
				require.NotNil(t, schema.Properties)
				assert.Contains(t, schema.Properties, "code")
				assert.Contains(t, schema.Properties, "language")
				require.NotNil(t, schema.Required)
				assert.Contains(t, schema.Required, "code")

				require.NotNil(t, action.OutputSchema)
				outSchema := action.OutputSchema
				assert.Equal(t, "object", outSchema.Type)
				require.NotNil(t, outSchema.Properties)
				assert.Contains(t, outSchema.Properties, "feedback")
				feedback := outSchema.Properties["feedback"].(map[string]interface{})
				assert.Equal(t, "array", feedback["type"])
				items := feedback["items"].(map[string]interface{})
				assert.Equal(t, "object", items["type"])
				itemProps := items["properties"].(map[string]interface{})
				assert.Contains(t, itemProps, "category")
				assert.Contains(t, itemProps, "description")
				assert.Contains(t, itemProps, "suggestion")
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
			errMsg:  "Missing file path for agent: test-action",
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
				ID:  &agentID,
				cwd: common.NewCWD("/test/path"),
			},
			wantErr: false,
		},
		{
			name: "Missing CWD",
			config: &AgentConfig{
				ID: &agentID,
			},
			wantErr: true,
			errMsg:  "Missing file path for agent: test-agent",
		},
		{
			name: "Invalid Package Reference",
			config: &AgentConfig{
				ID:      &agentID,
				Use:     package_ref.NewPackageRefConfig("invalid"),
				Config:  &ProviderConfig{},
				Tools:   []*tool.ToolConfig{},
				Actions: []*AgentActionConfig{},
				cwd:     common.NewCWD("/test/path"),
			},
			wantErr: true,
			errMsg:  "Invalid package reference",
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
