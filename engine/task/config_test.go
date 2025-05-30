package task

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/ref"
	"github.com/compozy/compozy/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTest(t *testing.T, taskFile string) (cwd *core.CWD, projectRoot, dstPath string) {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	cwd, dstPath = utils.SetupTest(t, filename)
	projectRoot = cwd.PathStr()
	dstPath = filepath.Join(dstPath, taskFile)
	return
}

func Test_LoadTask(t *testing.T) {
	t.Run("Should load basic task configuration correctly", func(t *testing.T) {
		cwd, projectRoot, dstPath := setupTest(t, "basic_task.yaml")

		// Run the test
		ctx := context.Background()
		config, err := Load(ctx, cwd, projectRoot, dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Validate the config
		err = config.Validate()
		require.NoError(t, err)

		require.NotNil(t, config.ID)
		require.NotNil(t, config.Type)
		require.NotNil(t, config.Action)
		require.NotNil(t, config.Env)
		require.NotNil(t, config.With)
		require.NotNil(t, config.OnSuccess)
		require.NotNil(t, config.OnError)

		assert.Equal(t, "code-format", config.ID)
		assert.Equal(t, TaskTypeBasic, config.Type)

		// Validate env and with
		assert.Equal(t, "1.0.0", config.Env["FORMATTER_VERSION"])
		assert.Equal(t, 2, (*config.With)["indent_size"])
		assert.Equal(t, false, (*config.With)["use_tabs"])

		// Validate transitions
		assert.Equal(t, "next-task", *config.OnSuccess.Next)
		assert.Equal(t, "retry-task", *config.OnError.Next)
	})

	t.Run("Should load decision task configuration correctly", func(t *testing.T) {
		cwd, projectRoot, dstPath := setupTest(t, "decision_task.yaml")

		// Run the test
		ctx := context.Background()
		config, err := Load(ctx, cwd, projectRoot, dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Validate the config
		err = config.Validate()
		require.NoError(t, err)

		require.NotNil(t, config.ID)
		require.NotNil(t, config.Type)
		require.NotEmpty(t, config.Condition)
		require.NotNil(t, config.Routes)
		require.NotNil(t, config.Env)
		require.NotNil(t, config.With)
		require.NotNil(t, config.OnError)

		assert.Equal(t, "code-review", config.ID)
		assert.Equal(t, TaskTypeDecision, config.Type)
		assert.Equal(t, "review_score", config.Condition)
		assert.Equal(t, 3, len(config.Routes))

		// Validate routes
		assert.Equal(t, "deploy", config.Routes["approved"])
		assert.Equal(t, "update-code", config.Routes["needs_changes"])
		assert.Equal(t, "notify-team", config.Routes["rejected"])

		// Validate env and with
		assert.Equal(t, "0.8", config.Env["REVIEW_THRESHOLD"])
		assert.Equal(t, 0.7, (*config.With)["min_score"])
		assert.Equal(t, 10, (*config.With)["max_comments"])

		// Validate error transition
		assert.Equal(t, "retry-task", *config.OnError.Next)
	})

	t.Run("Should return error for invalid task configuration", func(t *testing.T) {
		cwd, projectRoot, dstPath := setupTest(t, "invalid_task.yaml")

		// Run the test
		ctx := context.Background()
		config, err := Load(ctx, cwd, projectRoot, dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Validate the config
		err = config.Validate()
		require.Error(t, err)
	})
}

func Test_TaskConfigValidation(t *testing.T) {
	taskID := "test-task"
	taskCWD, err := core.CWDFromPath("/test/path")
	require.NoError(t, err)

	// Create task metadata
	metadata := &core.ConfigMetadata{
		CWD:         taskCWD,
		FilePath:    "/test/path/task.yaml",
		ProjectRoot: "/test/path",
	}

	t.Run("Should validate valid basic task with agent executor", func(t *testing.T) {
		agentRef, err := ref.NewNodeFromString("agents.#(id==\"test-agent\")")
		require.NoError(t, err)

		config := &Config{
			ID:     taskID,
			Type:   TaskTypeBasic,
			Action: "test-action",
			Executor: Executor{
				Type: ExecutorAgent,
				Ref:  *agentRef,
			},
			metadata: metadata,
		}

		err = config.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should validate valid basic task with tool executor", func(t *testing.T) {
		toolRef, err := ref.NewNodeFromString("tools.#(id==\"test-tool\")")
		require.NoError(t, err)

		config := &Config{
			ID:   taskID,
			Type: TaskTypeBasic,
			Executor: Executor{
				Type: ExecutorTool,
				Ref:  *toolRef,
			},
			metadata: metadata,
		}

		err = config.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should validate valid decision task", func(t *testing.T) {
		agentRef, err := ref.NewNodeFromString("agents.#(id==\"decision-agent\")")
		require.NoError(t, err)

		config := &Config{
			ID:        taskID,
			Type:      TaskTypeDecision,
			Condition: "test-condition",
			Routes: map[string]string{
				"route1": "next1",
			},
			Executor: Executor{
				Type: ExecutorAgent,
				Ref:  *agentRef,
			},
			metadata: metadata,
		}

		err = config.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should return error when CWD is missing", func(t *testing.T) {
		agentRef, err := ref.NewNodeFromString("agents.#(id==\"test-agent\")")
		require.NoError(t, err)

		config := &Config{
			ID:   taskID,
			Type: TaskTypeBasic,
			Executor: Executor{
				Type: ExecutorAgent,
				Ref:  *agentRef,
			},
			metadata: &core.ConfigMetadata{},
		}

		err = config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "current working directory is required for test-task")
	})

	t.Run("Should return error for missing executor type", func(t *testing.T) {
		agentRef, err := ref.NewNodeFromString("agents.#(id==\"test-agent\")")
		require.NoError(t, err)

		config := &Config{
			ID:   taskID,
			Type: TaskTypeBasic,
			Executor: Executor{
				Ref: *agentRef,
			},
			metadata: metadata,
		}

		err = config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "executor type is required")
	})

	t.Run("Should return error for invalid executor type", func(t *testing.T) {
		agentRef, err := ref.NewNodeFromString("agents.#(id==\"test-agent\")")
		require.NoError(t, err)

		config := &Config{
			ID:   taskID,
			Type: TaskTypeBasic,
			Executor: Executor{
				Type: "invalid",
				Ref:  *agentRef,
			},
			metadata: metadata,
		}

		err = config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid executor type: invalid")
	})

	t.Run("Should return error for empty executor reference", func(t *testing.T) {
		config := &Config{
			ID:   taskID,
			Type: TaskTypeBasic,
			Executor: Executor{
				Type: ExecutorAgent,
			},
			metadata: metadata,
		}

		err = config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "executor reference is required")
	})

	t.Run("Should return error for invalid task type", func(t *testing.T) {
		agentRef, err := ref.NewNodeFromString("agents.#(id==\"test-agent\")")
		require.NoError(t, err)

		config := &Config{
			ID:   taskID,
			Type: "invalid",
			Executor: Executor{
				Type: ExecutorAgent,
				Ref:  *agentRef,
			},
			metadata: metadata,
		}

		err = config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid task type: invalid")
	})

	t.Run("Should return error for decision task missing condition", func(t *testing.T) {
		agentRef, err := ref.NewNodeFromString("agents.#(id==\"decision-agent\")")
		require.NoError(t, err)

		config := &Config{
			ID:   taskID,
			Type: TaskTypeDecision,
			Routes: map[string]string{
				"route1": "next1",
			},
			Executor: Executor{
				Type: ExecutorAgent,
				Ref:  *agentRef,
			},
			metadata: metadata,
		}

		err = config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "condition is required for decision tasks")
	})

	t.Run("Should return error for decision task missing routes", func(t *testing.T) {
		agentRef, err := ref.NewNodeFromString("agents.#(id==\"decision-agent\")")
		require.NoError(t, err)

		config := &Config{
			ID:        taskID,
			Type:      TaskTypeDecision,
			Condition: "test-condition",
			Executor: Executor{
				Type: ExecutorAgent,
				Ref:  *agentRef,
			},
			metadata: metadata,
		}

		err = config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "routes are required for decision tasks")
	})

	t.Run("Should return error for tool executor with action", func(t *testing.T) {
		toolRef, err := ref.NewNodeFromString("tools.#(id==\"test-tool\")")
		require.NoError(t, err)

		config := &Config{
			ID:     taskID,
			Type:   TaskTypeBasic,
			Action: "test-action",
			Executor: Executor{
				Type: ExecutorTool,
				Ref:  *toolRef,
			},
			metadata: metadata,
		}

		err = config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "action is not allowed when executor type is tool")
	})

	t.Run("Should return error for agent executor without action", func(t *testing.T) {
		agentRef, err := ref.NewNodeFromString("agents.#(id==\"test-agent\")")
		require.NoError(t, err)

		config := &Config{
			ID:   taskID,
			Type: TaskTypeBasic,
			Executor: Executor{
				Type: ExecutorAgent,
				Ref:  *agentRef,
			},
			metadata: metadata,
		}

		err = config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "action is required when executor type is agent")
	})

	t.Run("Should handle parameter validation gracefully", func(t *testing.T) {
		agentRef, err := ref.NewNodeFromString("agents.#(id==\"test-agent\")")
		require.NoError(t, err)

		config := &Config{
			ID:     taskID,
			Type:   TaskTypeBasic,
			Action: "test-action",
			Executor: Executor{
				Type: ExecutorAgent,
				Ref:  *agentRef,
			},
			With: &core.Input{
				"param": "value",
			},
			metadata: metadata,
		}

		// Parameter validation should now return nil since it's handled by the referenced agent/tool
		err = config.ValidateParams(config.With)
		assert.NoError(t, err)
	})
}

func Test_TaskConfigMerge(t *testing.T) {
	t.Run("Should merge configurations correctly", func(t *testing.T) {
		next1 := "next1"
		next2 := "next2"
		base := &Config{
			Env: core.EnvMap{
				"KEY1": "value1",
			},
			With: &core.Input{
				"param1": "value1",
			},
			OnSuccess: &SuccessTransitionConfig{
				Next: &next1,
			},
			OnError: &ErrorTransitionConfig{
				Next: &next1,
			},
		}

		other := &Config{
			Env: core.EnvMap{
				"KEY2": "value2",
			},
			With: &core.Input{
				"param2": "value2",
			},
			OnSuccess: &SuccessTransitionConfig{
				Next: &next2,
			},
			OnError: &ErrorTransitionConfig{
				Next: &next2,
			},
		}

		err := base.Merge(other)
		require.NoError(t, err)

		// Check merged values
		assert.Equal(t, "value1", base.Env["KEY1"])
		assert.Equal(t, "value2", base.Env["KEY2"])
		assert.Equal(t, "value1", (*base.With)["param1"])
		assert.Equal(t, "value2", (*base.With)["param2"])
		assert.Equal(t, "next2", *base.OnSuccess.Next)
		assert.Equal(t, "next2", *base.OnError.Next)
	})
}

func Test_LoadTaskWithNestedReferences(t *testing.T) {
	t.Run("Should resolve nested references when task executor references agent with action references", func(t *testing.T) {
		cwd, projectRoot, dstPath := setupTest(t, "basic_task.yaml")

		// Load the task
		ctx := context.Background()
		config, err := Load(ctx, cwd, projectRoot, dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Validate the task config
		err = config.Validate()
		require.NoError(t, err)

		// Verify basic task properties
		assert.Equal(t, "code-format", config.ID)
		assert.Equal(t, "agent", string(config.Executor.Type))
		assert.False(t, config.Executor.Ref.IsEmpty())

		// The executor should now contain the resolved agent configuration
		resolvedAgent, err := config.Executor.GetAgent()
		require.NoError(t, err)
		require.NotNil(t, resolvedAgent)

		// Verify the resolved agent has the expected properties
		assert.Equal(t, "code-formatter", resolvedAgent.ID)
		assert.Equal(t, "You are a code formatter. Format the provided code according to best practices.", resolvedAgent.Instructions)

		// The agent should have actions resolved from actions.yaml
		require.Len(t, resolvedAgent.Actions, 1)

		// The action should have actual data from actions.yaml, not just a $ref
		action := resolvedAgent.Actions[0]
		require.NotNil(t, action)
		assert.Equal(t, "format-code", action.ID)
		assert.Equal(t, "Format the provided code according to best practices and style guidelines", action.Prompt)

		// Verify the action has the input schema from actions.yaml
		require.NotNil(t, action.InputSchema)
		inputSchema := action.InputSchema.Schema
		assert.Equal(t, "object", inputSchema["type"])

		properties, ok := inputSchema["properties"].(map[string]any)
		require.True(t, ok)
		assert.Contains(t, properties, "code")
		assert.Contains(t, properties, "language")
		assert.Contains(t, properties, "style")
	})
}
