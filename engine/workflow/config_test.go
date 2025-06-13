package workflow

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/compozy/compozy/engine/core"
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
		CWD, dstPath := setupTest(t, "basic_workflow.yaml")
		ev := ref.NewEvaluator(ref.WithGlobalScope(globalScope))
		config, err := LoadAndEval(CWD, dstPath, ev)
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
		CWD, dstPath := setupTest(t, "invalid_workflow.yaml")
		config, err := Load(CWD, dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		err = config.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "condition is required for router tasks")
	})
}

func Test_WorkflowConfigValidation(t *testing.T) {
	workflowID := "test-workflow"

	t.Run("Should validate valid workflow configuration", func(t *testing.T) {
		CWD, err := core.CWDFromPath("/test/path")
		require.NoError(t, err)
		config := &Config{
			ID:   workflowID,
			Opts: Opts{},
			CWD:  CWD,
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
