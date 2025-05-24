package task

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTest(t *testing.T, taskFile string) (cwd *common.CWD, dstPath string) {
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
		schema := config.InputSchema.Schema
		assert.Equal(t, "object", schema.GetType())
		require.NotNil(t, schema.GetProperties())
		assert.Contains(t, schema.GetProperties(), "code")
		assert.Contains(t, schema.GetProperties(), "language")
		if required, ok := schema["required"].([]string); ok && len(required) > 0 {
			assert.Contains(t, required, "code")
		}

		// Validate output schema
		outSchema := config.OutputSchema.Schema
		assert.Equal(t, "object", outSchema.GetType())
		require.NotNil(t, outSchema.GetProperties())
		assert.Contains(t, outSchema.GetProperties(), "formatted_code")
		if required, ok := outSchema["required"].([]string); ok && len(required) > 0 {
			assert.Contains(t, required, "formatted_code")
		}

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
		schema := config.InputSchema.Schema
		assert.Equal(t, "object", schema.GetType())
		require.NotNil(t, schema.GetProperties())
		assert.Contains(t, schema.GetProperties(), "code")
		assert.Contains(t, schema.GetProperties(), "review_score")
		if required, ok := schema["required"].([]string); ok && len(required) > 0 {
			assert.Contains(t, required, "code")
			assert.Contains(t, required, "review_score")
		}

		// Validate output schema
		outSchema := config.OutputSchema.Schema
		assert.Equal(t, "object", outSchema.GetType())
		require.NotNil(t, outSchema.GetProperties())
		assert.Contains(t, outSchema.GetProperties(), "status")
		assert.Contains(t, outSchema.GetProperties(), "comments")
		if required, ok := outSchema["required"].([]string); ok && len(required) > 0 {
			assert.Contains(t, required, "status")
		}

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
	taskCWD, err := common.CWDFromPath("/test/path")
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

	t.Run("Should return error for invalid package reference", func(t *testing.T) {
		config := &Config{
			ID:  taskID,
			Use: common.NewPackageRefConfig("invalid"),
			cwd: taskCWD,
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid package reference")
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
		assert.Contains(t, err.Error(), "condition or routes are required for decision task type")
	})

	t.Run("Should return error for decision task missing routes", func(t *testing.T) {
		config := &Config{
			ID:   "test-task",
			Type: TaskTypeDecision,
			cwd:  taskCWD,
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "condition or routes are required for decision task type")
	})

	t.Run("Should return error when input schema is used with ID reference", func(t *testing.T) {
		config := &Config{
			ID:  taskID,
			Use: common.NewPackageRefConfig("task(id=test-task)"),
			InputSchema: &schema.InputSchema{
				Schema: schema.Schema{
					"type": "object",
				},
			},
			cwd: taskCWD,
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "input schema not allowed for reference type id")
	})

	t.Run("Should return error when output schema is used with file reference", func(t *testing.T) {
		config := &Config{
			ID:  taskID,
			Use: common.NewPackageRefConfig("task(file=basic_task.yaml)"),
			OutputSchema: &schema.OutputSchema{
				Schema: schema.Schema{
					"type": "object",
				},
			},
			cwd: taskCWD,
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "output schema not allowed for reference type file")
	})

	t.Run("Should return error when schemas are used with dep reference", func(t *testing.T) {
		config := &Config{
			ID:  taskID,
			Use: common.NewPackageRefConfig("task(dep=compozy/tasks:test-task)"),
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
			cwd: taskCWD,
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "input schema not allowed for reference type dep")
	})

	t.Run("Should return error for task with invalid parameters", func(t *testing.T) {
		config := &Config{
			ID:   "test-task",
			Type: TaskTypeBasic,
			cwd:  taskCWD,
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
			With: &common.Input{
				"age": 42,
			},
		}

		err := config.ValidateParams(config.With)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "with parameters invalid for test-task")
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
			Env: common.EnvMap{
				"KEY1": "value1",
			},
			With: &common.Input{
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
			Env: common.EnvMap{
				"KEY2": "value2",
			},
			With: &common.Input{
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
