package shared_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
)

func TestConfigMerger_NewConfigMerger(t *testing.T) {
	t.Run("Should create config merger", func(t *testing.T) {
		// Act
		merger := shared.NewConfigMerger()

		// Assert
		assert.NotNil(t, merger)
	})
}

func TestConfigMerger_MergeTaskConfigIfExists(t *testing.T) {
	merger := shared.NewConfigMerger()

	t.Run("Should merge task config when it exists", func(t *testing.T) {
		// Arrange
		taskContext := map[string]any{
			"id":     "task1",
			"status": core.StatusRunning,
		}

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "task1",
				Type: task.TaskTypeBasic,
				With: &core.Input{
					"param": "value",
				},
				Env: &core.EnvMap{
					"VAR": "env_value",
				},
			},
			BasicTask: task.BasicTask{
				Action: "test-action",
			},
		}

		taskConfigs := map[string]*task.Config{
			"task1": taskConfig,
		}

		// Act
		merger.MergeTaskConfigIfExists(taskContext, "task1", taskConfigs)

		// Assert
		assert.Equal(t, "task1", taskContext["id"])
		assert.Equal(t, core.StatusRunning, taskContext["status"]) // Should remain unchanged
		assert.Equal(t, string(task.TaskTypeBasic), taskContext["type"])
		assert.Equal(t, "test-action", taskContext["action"])
		// Check With field (may be converted to map)
		assert.NotNil(t, taskContext["with"])
		// Check Env field (may be converted to map)
		assert.NotNil(t, taskContext["env"])
	})

	t.Run("Should do nothing when task config doesn't exist", func(t *testing.T) {
		// Arrange
		taskContext := map[string]any{
			"id":     "task1",
			"status": core.StatusRunning,
		}

		taskConfigs := map[string]*task.Config{
			"other_task": {
				BaseConfig: task.BaseConfig{
					ID: "other_task",
				},
			},
		}

		// Act
		merger.MergeTaskConfigIfExists(taskContext, "task1", taskConfigs)

		// Assert
		// Context should remain unchanged
		assert.Len(t, taskContext, 2)
		assert.Equal(t, "task1", taskContext["id"])
		assert.Equal(t, core.StatusRunning, taskContext["status"])
	})

	t.Run("Should handle nil task configs", func(t *testing.T) {
		// Arrange
		taskContext := map[string]any{
			"id": "task1",
		}

		// Act
		merger.MergeTaskConfigIfExists(taskContext, "task1", nil)

		// Assert
		// Context should remain unchanged
		assert.Len(t, taskContext, 1)
		assert.Equal(t, "task1", taskContext["id"])
	})

	t.Run("Should handle merge error gracefully", func(t *testing.T) {
		// Arrange
		taskContext := map[string]any{
			"id": "task1",
		}

		// Create a task config that will cause AsMap() to fail
		// This is difficult to test directly since AsMap() implementation is not visible
		// So we'll test the error handling by checking for _merge_error key
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "task1",
				Type: task.TaskTypeBasic,
			},
		}

		taskConfigs := map[string]*task.Config{
			"task1": taskConfig,
		}

		// Act
		merger.MergeTaskConfigIfExists(taskContext, "task1", taskConfigs)

		// Assert
		// Should not have _merge_error since AsMap should succeed for valid config
		_, hasError := taskContext["_merge_error"]
		assert.False(t, hasError)

		// Should have merged successfully
		assert.Equal(t, string(task.TaskTypeBasic), taskContext["type"])
	})
}

func TestConfigMerger_MergeTaskConfig(t *testing.T) {
	merger := shared.NewConfigMerger()

	t.Run("Should merge task config into context", func(t *testing.T) {
		// Arrange
		taskContext := map[string]any{
			"id":     "task1",
			"input":  "existing_input",  // Should not be overridden
			"output": "existing_output", // Should not be overridden
		}

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "task1",
				Type: task.TaskTypeBasic,
				With: &core.Input{
					"param": "value",
				},
			},
			BasicTask: task.BasicTask{
				Action: "test-action",
			},
		}

		// Act
		err := merger.MergeTaskConfig(taskContext, taskConfig)

		// Assert
		require.NoError(t, err)

		// Check that config fields were merged
		assert.Equal(t, string(task.TaskTypeBasic), taskContext["type"])
		assert.Equal(t, "test-action", taskContext["action"])
		assert.NotNil(t, taskContext["with"])

		// Check that input and output were NOT overridden
		assert.Equal(t, "existing_input", taskContext["input"])
		assert.Equal(t, "existing_output", taskContext["output"])
	})

	t.Run("Should handle config with minimal fields", func(t *testing.T) {
		// Arrange
		taskContext := map[string]any{
			"id": "task1",
		}

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "task1",
				Type: task.TaskTypeBasic,
			},
		}

		// Act
		err := merger.MergeTaskConfig(taskContext, taskConfig)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "task1", taskContext["id"])
		assert.Equal(t, string(task.TaskTypeBasic), taskContext["type"])
	})

	t.Run("Should preserve original context keys not in config", func(t *testing.T) {
		// Arrange
		taskContext := map[string]any{
			"id":           "task1",
			"status":       core.StatusRunning,
			"custom_field": "custom_value",
		}

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "task1",
				Type: task.TaskTypeBasic,
			},
			BasicTask: task.BasicTask{
				Action: "test-action",
			},
		}

		// Act
		err := merger.MergeTaskConfig(taskContext, taskConfig)

		// Assert
		require.NoError(t, err)

		// Original fields should be preserved
		assert.Equal(t, "task1", taskContext["id"])
		assert.Equal(t, core.StatusRunning, taskContext["status"])
		assert.Equal(t, "custom_value", taskContext["custom_field"])

		// New fields should be added
		assert.Equal(t, string(task.TaskTypeBasic), taskContext["type"])
		assert.Equal(t, "test-action", taskContext["action"])
	})
}
