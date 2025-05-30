package workflow

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTest(t *testing.T, workflowFile string) (cwd *core.CWD, projectRoot, dstPath string) {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	cwd, dstPath = utils.SetupTest(t, filename)
	projectRoot = cwd.PathStr()
	dstPath = filepath.Join(dstPath, workflowFile)
	return
}

func Test_LoadWorkflow(t *testing.T) {
	t.Run("Should load basic workflow configuration correctly", func(t *testing.T) {
		cwd, projectRoot, dstPath := setupTest(t, "basic_workflow.yaml")

		ctx := context.Background()
		config, err := Load(ctx, cwd, projectRoot, dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)
		require.NotNil(t, config.Opts)
		require.NotNil(t, config.ID)
		require.NotNil(t, config.Version)
		require.NotNil(t, config.Description)
		require.NotNil(t, config.Tasks)
		require.NotNil(t, config.Tools)
		require.NotNil(t, config.Agents)
		require.NotNil(t, config.Env)

		assert.Equal(t, "test-workflow", config.ID)
		assert.Equal(t, "1.0.0", config.Version)
		assert.Equal(t, "Test workflow for code formatting", config.Description)

		// Validate tasks
		require.Len(t, config.Tasks, 2)
		task := config.Tasks[0]
		assert.Equal(t, "format-code", task.ID)
		assert.Equal(t, "basic", string(task.Type))
		// Note: The new executor uses $ref instead of the old Use field
		assert.Equal(t, "agent", string(task.Executor.Type))
		assert.Equal(t, "format-code", task.Executor.Action)

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
		assert.Equal(t, agent.ProviderName("anthropic"), agentConfig.Config.Provider)
		assert.Equal(t, agent.ModelName("claude-3-opus"), agentConfig.Config.Model)
		assert.InDelta(t, float32(0.7), agentConfig.Config.Temperature, 0.0001)
		assert.Equal(t, int32(4000), agentConfig.Config.MaxTokens)

		// Validate env
		assert.Equal(t, "1.0.0", config.Env["WORKFLOW_VERSION"])
		assert.Equal(t, "3", config.Env["MAX_RETRIES"])
	})

	t.Run("Should return error for invalid workflow configuration", func(t *testing.T) {
		cwd, projectRoot, dstPath := setupTest(t, "invalid_workflow.yaml")

		ctx := context.Background()
		config, err := Load(ctx, cwd, projectRoot, dstPath)
		require.Error(t, err)
		require.Nil(t, config)
		// The error should come from task validation since the workflow loads but tasks are invalid
		assert.Contains(t, err.Error(), "failed to resolve executor")
	})
}

func Test_WorkflowConfigValidation(t *testing.T) {
	workflowID := "test-workflow"

	t.Run("Should validate valid workflow configuration", func(t *testing.T) {
		cwd, err := core.CWDFromPath("/test/path")
		require.NoError(t, err)

		metadata := &core.ConfigMetadata{
			CWD:         cwd,
			FilePath:    "/test/path/workflow.yaml",
			ProjectRoot: "/test/path",
		}

		config := &Config{
			ID:   workflowID,
			Opts: Opts{},
		}
		config.SetMetadata(metadata)

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

func Test_WorkflowConfigMetadata(t *testing.T) {
	t.Run("Should handle metadata operations correctly", func(t *testing.T) {
		cwd, err := core.CWDFromPath("/test/path")
		require.NoError(t, err)

		metadata := &core.ConfigMetadata{
			CWD:         cwd,
			FilePath:    "/test/path/workflow.yaml",
			ProjectRoot: "/test/path",
		}

		config := &Config{}
		config.SetMetadata(metadata)

		assert.Equal(t, "/test/path/workflow.yaml", config.GetMetadata().FilePath)
		assert.Equal(t, "/test/path", config.GetCWD().PathStr())

		// Test updating metadata
		newMetadata := &core.ConfigMetadata{
			CWD:         cwd,
			FilePath:    "/new/path/workflow.yaml",
			ProjectRoot: "/new/path",
		}
		config.SetMetadata(newMetadata)
		assert.Equal(t, "/new/path/workflow.yaml", config.GetMetadata().FilePath)
	})
}

func Test_WorkflowConfigMerge(t *testing.T) {
	t.Run("Should merge configurations correctly", func(t *testing.T) {
		baseConfig := &Config{
			Env: core.EnvMap{
				"KEY1": "value1",
			},
		}

		otherConfig := &Config{
			Env: core.EnvMap{
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
