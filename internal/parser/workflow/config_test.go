package workflow

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/pkgref"
	"github.com/compozy/compozy/internal/parser/provider"
	"github.com/compozy/compozy/internal/parser/trigger"
	"github.com/compozy/compozy/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTest(t *testing.T, workflowFile string) (cwd *common.CWD, dstPath string) {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	cwd, dstPath = utils.SetupTest(t, filename)
	dstPath = filepath.Join(dstPath, workflowFile)
	return
}

func Test_LoadWorkflow(t *testing.T) {
	t.Run("Should load basic workflow configuration correctly", func(t *testing.T) {
		cwd, dstPath := setupTest(t, "basic_workflow.yaml")
		config, err := Load(cwd, dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)
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
		assert.Equal(t, provider.Name("anthropic"), agent.Config.Provider)
		assert.Equal(t, provider.ModelName("claude-3-opus"), agent.Config.Model)
		assert.InDelta(t, float32(0.7), agent.Config.Temperature, 0.0001)
		assert.Equal(t, int32(4000), agent.Config.MaxTokens)

		// Validate trigger
		assert.Equal(t, trigger.Type("webhook"), config.Trigger.Type)
		require.NotNil(t, config.Trigger.Config)
		assert.Equal(t, "/test-webhook", config.Trigger.Config.URL)

		// Validate env
		assert.Equal(t, "1.0.0", config.Env["WORKFLOW_VERSION"])
		assert.Equal(t, "3", config.Env["MAX_RETRIES"])
	})

	t.Run("Should return error for invalid workflow configuration", func(t *testing.T) {
		cwd, dstPath := setupTest(t, "invalid_workflow.yaml")
		config, err := Load(cwd, dstPath)
		require.Error(t, err)
		require.Nil(t, config)
		assert.Contains(t, err.Error(), "condition or routes are required for decision task type")
	})
}

func Test_WorkflowConfigValidation(t *testing.T) {
	workflowID := "test-workflow"

	t.Run("Should validate valid workflow configuration", func(t *testing.T) {
		cwd, err := common.CWDFromPath("/test/path")
		require.NoError(t, err)
		config := &Config{
			ID: workflowID,
			Trigger: trigger.Config{
				Type: trigger.TriggerTypeWebhook,
				Config: &trigger.WebhookConfig{
					URL: "/test",
				},
			},
			cwd: cwd,
		}

		err = config.Validate()
		require.NoError(t, err)
	})

	t.Run("Should return error when CWD is missing", func(t *testing.T) {
		config := &Config{
			ID: "test-workflow",
		}

		err := config.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "current working directory is required for test-workflow")
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
			Env: common.EnvMap{
				"KEY1": "value1",
			},
		}

		otherConfig := &Config{
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
