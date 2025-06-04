package task

import (
	"context"
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

func setupTest(t *testing.T, taskFile string) (cwd *core.CWD, dstPath string) {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	cwd, dstPath = utils.SetupTest(t, filename)
	dstPath = filepath.Join(dstPath, taskFile)
	return
}

func Test_LoadTask(t *testing.T) {
	t.Run("Should load basic task configuration correctly", func(t *testing.T) {
		cwd, dstPath := setupTest(t, "basic_task.yaml")

		// Run the test
		config, err := Load(cwd, dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Validate the config
		err = config.Validate()
		require.NoError(t, err)

		require.NotNil(t, config.ID)
		require.NotNil(t, config.Type)
		require.NotNil(t, config.Action)
		require.NotNil(t, config.InputSchema)
		require.NotNil(t, config.OutputSchema)
		require.NotNil(t, config.Env)
		require.NotNil(t, config.With)
		require.NotNil(t, config.OnSuccess)
		require.NotNil(t, config.OnError)

		assert.Equal(t, "code-format", config.ID)
		assert.Equal(t, TaskTypeBasic, config.Type)

		// Validate input schema
		schema := config.InputSchema
		compiledSchema, err := schema.Compile()
		require.NoError(t, err)
		assert.Equal(t, []string{"object"}, []string(compiledSchema.Type))
		require.NotNil(t, compiledSchema.Properties)
		assert.Contains(t, (*compiledSchema.Properties), "code")
		assert.Contains(t, (*compiledSchema.Properties), "language")
		assert.Contains(t, compiledSchema.Required, "code")

		// Validate output schema
		outSchema := config.OutputSchema
		compiledOutSchema, err := outSchema.Compile()
		require.NoError(t, err)
		assert.Equal(t, []string{"object"}, []string(compiledOutSchema.Type))
		require.NotNil(t, compiledOutSchema.Properties)
		assert.Contains(t, (*compiledOutSchema.Properties), "formatted_code")
		assert.Contains(t, compiledOutSchema.Required, "formatted_code")

		// Validate env and with
		assert.Equal(t, "1.0.0", config.Env["FORMATTER_VERSION"])
		assert.Equal(t, 2, (*config.With)["indent_size"])
		assert.Equal(t, false, (*config.With)["use_tabs"])

		// Validate transitions
		assert.Equal(t, "next-task", *config.OnSuccess.Next)
		assert.Equal(t, "retry-task", *config.OnError.Next)
	})

	t.Run("Should load decision task configuration correctly", func(t *testing.T) {
		cwd, dstPath := setupTest(t, "decision_task.yaml")

		// Run the test
		config, err := Load(cwd, dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Validate the config
		err = config.Validate()
		require.NoError(t, err)

		require.NotNil(t, config.ID)
		require.NotNil(t, config.Type)
		require.NotEmpty(t, config.Condition)
		require.NotNil(t, config.Routes)
		require.NotNil(t, config.InputSchema)
		require.NotNil(t, config.OutputSchema)
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

		// Validate input schema
		schema := config.InputSchema
		compiledSchema2, err := schema.Compile()
		require.NoError(t, err)
		assert.Equal(t, []string{"object"}, []string(compiledSchema2.Type))
		require.NotNil(t, compiledSchema2.Properties)
		assert.Contains(t, (*compiledSchema2.Properties), "code")
		assert.Contains(t, (*compiledSchema2.Properties), "review_score")
		assert.Contains(t, compiledSchema2.Required, "code")
		assert.Contains(t, compiledSchema2.Required, "review_score")

		// Validate output schema
		outSchema := config.OutputSchema
		compiledOutSchema2, err := outSchema.Compile()
		require.NoError(t, err)
		assert.Equal(t, []string{"object"}, []string(compiledOutSchema2.Type))
		require.NotNil(t, compiledOutSchema2.Properties)
		assert.Contains(t, (*compiledOutSchema2.Properties), "status")
		assert.Contains(t, (*compiledOutSchema2.Properties), "comments")
		assert.Contains(t, compiledOutSchema2.Required, "status")

		// Validate env and with
		assert.Equal(t, "0.8", config.Env["REVIEW_THRESHOLD"])
		assert.Equal(t, 0.7, (*config.With)["min_score"])
		assert.Equal(t, 10, (*config.With)["max_comments"])

		// Validate error transition
		assert.Equal(t, "retry-task", *config.OnError.Next)
	})

	t.Run("Should return error for invalid task configuration", func(t *testing.T) {
		cwd, dstPath := setupTest(t, "invalid_task.yaml")

		// Run the test
		config, err := Load(cwd, dstPath)
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

	t.Run("Should validate valid basic task", func(t *testing.T) {
		config := &Config{
			ID:     taskID,
			Type:   TaskTypeBasic,
			Action: "test-action",
			cwd:    taskCWD,
		}

		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should validate valid decision task", func(t *testing.T) {
		config := &Config{
			ID:        taskID,
			Type:      TaskTypeDecision,
			Condition: "test-condition",
			Routes: map[string]string{
				"route1": "next1",
			},
			cwd: taskCWD,
		}

		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should return error when CWD is missing", func(t *testing.T) {
		config := &Config{
			ID:   "test-task",
			Type: TaskTypeBasic,
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "current working directory is required for test-task")
	})

	t.Run("Should return error for invalid task type", func(t *testing.T) {
		config := &Config{
			ID:   "test-task",
			Type: "invalid",
			cwd:  taskCWD,
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid task type: invalid")
	})

	t.Run("Should return error for decision task missing configuration", func(t *testing.T) {
		config := &Config{
			ID:   taskID,
			Type: TaskTypeDecision,
			cwd:  taskCWD,
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "condition is required for decision tasks")
	})

	t.Run("Should return error for decision task missing routes", func(t *testing.T) {
		config := &Config{
			ID:   "test-task",
			Type: TaskTypeDecision,
			cwd:  taskCWD,
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "condition is required for decision tasks")
	})

	t.Run("Should return error for task with invalid parameters", func(t *testing.T) {
		config := &Config{
			ID:   "test-task",
			Type: TaskTypeBasic,
			cwd:  taskCWD,
			InputSchema: &schema.Schema{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type": "string",
					},
				},
				"required": []string{"name"},
			},
			With: &core.Input{
				"age": 42,
			},
		}

		err := config.ValidateParams(context.Background(), config.With)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Required property 'name' is missing")
	})
}

func Test_TaskConfigCWD(t *testing.T) {
	t.Run("Should handle CWD operations correctly", func(t *testing.T) {
		config := &Config{}
		assert.Nil(t, config.GetCWD())

		config.SetCWD("/test/path")
		assert.Equal(t, "/test/path", config.GetCWD().PathStr())

		config.SetCWD("/new/path")
		assert.Equal(t, "/new/path", config.GetCWD().PathStr())
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
			OnSuccess: &SuccessTransition{
				Next: &next1,
			},
			OnError: &ErrorTransition{
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
			OnSuccess: &SuccessTransition{
				Next: &next2,
			},
			OnError: &ErrorTransition{
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

func Test_TaskReference(t *testing.T) {
	t.Run("Should load task reference correctly", func(t *testing.T) {
		cwd, dstPath := setupTest(t, "ref_task.yaml")
		ev := ref.NewEvaluator()

		// Run the test
		config, err := LoadAndEval(cwd, dstPath, ev)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Validate the config
		err = config.Validate()
		require.NoError(t, err)

		require.NotNil(t, config.ID)
		require.NotNil(t, config.Type)
		require.NotNil(t, config.Action)
		require.NotNil(t, config.With)

		assert.Equal(t, "code-format", config.ID)
		assert.Equal(t, TaskTypeBasic, config.Type)
		assert.Equal(t, "format", config.Action)

		// Validate the with parameters
		assert.Equal(t, "console.log('hello')", (*config.With)["code"])
		assert.Equal(t, "javascript", (*config.With)["language"])
		assert.Equal(t, 2, (*config.With)["indent_size"])
		assert.Equal(t, false, (*config.With)["use_tabs"])

		// Validate that the agent was properly evaluated and loaded
		agent := config.GetAgent()
		require.NotNil(t, agent, "Agent should be loaded from the $use directive")
		assert.Equal(t, "code-assistant", agent.ID)
		require.NotNil(t, agent.Config)
		assert.Equal(t, "anthropic", string(agent.Config.Provider))
		assert.Equal(t, "claude-3-opus-20240229", string(agent.Config.Model))
		assert.Equal(t, float32(0.7), agent.Config.Temperature)
		assert.Equal(t, int32(4000), agent.Config.MaxTokens)
		assert.Equal(t, "You are a powerful AI coding assistant.", agent.Instructions)
	})
}
