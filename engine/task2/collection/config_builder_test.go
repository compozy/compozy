package collection_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/collection"
	"github.com/compozy/compozy/pkg/tplengine"
)

func TestConfigBuilder_NewConfigBuilder(t *testing.T) {
	t.Run("Should create config builder with template engine", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)

		// Act
		builder := collection.NewConfigBuilder(templateEngine)

		// Assert
		assert.NotNil(t, builder)
		assert.Equal(t, templateEngine, builder.GetTemplateEngine())
	})

	t.Run("Should handle nil template engine", func(t *testing.T) {
		// Act
		builder := collection.NewConfigBuilder(nil)

		// Assert
		assert.NotNil(t, builder)
		assert.Nil(t, builder.GetTemplateEngine())
	})
}

func TestConfigBuilder_GetTemplateEngine(t *testing.T) {
	t.Run("Should return the template engine", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		builder := collection.NewConfigBuilder(templateEngine)

		// Act
		result := builder.GetTemplateEngine()

		// Assert
		assert.Equal(t, templateEngine, result)
	})
}

func TestConfigBuilder_BuildTaskConfig(t *testing.T) {
	// Setup
	templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
	builder := collection.NewConfigBuilder(templateEngine)

	t.Run("Should build task config for collection item", func(t *testing.T) {
		// Arrange
		collectionConfig := &task.CollectionConfig{
			Items:    "test_items",
			ItemVar:  "item",
			IndexVar: "index",
		}
		taskTemplate := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task-{{ .index }}",
				Type: task.TaskTypeBasic,
			},
			BasicTask: task.BasicTask{
				Action: "process-{{ .item }}",
			},
		}
		parentTaskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "parent-task",
				Type: task.TaskTypeCollection,
			},
			Task: taskTemplate,
		}
		item := "test-item"
		index := 1
		context := map[string]any{
			"test_var": "test_value",
		}

		// Act
		result, err := builder.BuildTaskConfig(collectionConfig, parentTaskConfig, item, index, context)

		// Assert
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "test-task-1", result.ID)
		assert.Equal(t, task.TaskTypeBasic, result.Type)
		require.NotNil(t, result.With)
		assert.Equal(t, item, (*result.With)["item"])
		assert.Equal(t, index, (*result.With)["index"])
	})

	t.Run("Should merge inputs from parent task, template task, and item context", func(t *testing.T) {
		// Arrange
		collectionConfig := &task.CollectionConfig{
			Items:    "test_items",
			ItemVar:  "myItem",
			IndexVar: "myIndex",
		}
		parentWith := core.Input{
			"parent_key": "parent_value",
			"shared_key": "parent_shared",
		}
		templateWith := core.Input{
			"template_key": "template_value",
			"shared_key":   "template_shared", // Should override parent
		}
		taskTemplate := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "task-{{ .index }}",
				Type: task.TaskTypeBasic,
			},
			BasicTask: task.BasicTask{
				Action: "test",
			},
		}
		taskTemplate.With = &templateWith
		parentTaskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "parent-task",
				Type: task.TaskTypeCollection,
			},
			Task: taskTemplate,
		}
		parentTaskConfig.With = &parentWith
		item := "test-item"
		index := 2
		context := map[string]any{}

		// Act
		result, err := builder.BuildTaskConfig(collectionConfig, parentTaskConfig, item, index, context)

		// Assert
		assert.NoError(t, err)
		require.NotNil(t, result)
		require.NotNil(t, result.With)
		with := *result.With
		assert.Equal(t, "parent_value", with["parent_key"])
		assert.Equal(t, "template_value", with["template_key"])
		assert.Equal(t, "template_shared", with["shared_key"]) // Template should override parent
		assert.Equal(t, item, with["item"])
		assert.Equal(t, index, with["index"])
		assert.Equal(t, item, with["myItem"])   // Custom item var
		assert.Equal(t, index, with["myIndex"]) // Custom index var
	})

	t.Run("Should handle nil parent with", func(t *testing.T) {
		// Arrange
		collectionConfig := &task.CollectionConfig{
			Items: "test_items",
		}
		taskTemplate := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "task-{{ .index }}",
				Type: task.TaskTypeBasic,
			},
		}
		parentTaskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "parent-task",
				Type: task.TaskTypeCollection,
			},
			Task: taskTemplate,
		}
		item := "test-item"
		index := 0
		context := map[string]any{}

		// Act
		result, err := builder.BuildTaskConfig(collectionConfig, parentTaskConfig, item, index, context)

		// Assert
		assert.NoError(t, err)
		require.NotNil(t, result)
		require.NotNil(t, result.With)
		with := *result.With
		assert.Equal(t, item, with["item"])
		assert.Equal(t, index, with["index"])
	})

	t.Run("Should handle nil template with", func(t *testing.T) {
		// Arrange
		collectionConfig := &task.CollectionConfig{
			Items: "test_items",
		}
		parentWith := core.Input{
			"parent_key": "parent_value",
		}
		taskTemplate := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "task-{{ .index }}",
				Type: task.TaskTypeBasic,
			},
		}
		parentTaskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "parent-task",
				Type: task.TaskTypeCollection,
			},
			Task: taskTemplate,
		}
		parentTaskConfig.With = &parentWith
		item := "test-item"
		index := 0
		context := map[string]any{}

		// Act
		result, err := builder.BuildTaskConfig(collectionConfig, parentTaskConfig, item, index, context)

		// Assert
		assert.NoError(t, err)
		require.NotNil(t, result)
		require.NotNil(t, result.With)
		with := *result.With
		assert.Equal(t, "parent_value", with["parent_key"])
		assert.Equal(t, item, with["item"])
		assert.Equal(t, index, with["index"])
	})

	t.Run("Should handle custom item and index variable names", func(t *testing.T) {
		// Arrange
		collectionConfig := &task.CollectionConfig{
			Items:    "test_items",
			ItemVar:  "currentItem",
			IndexVar: "currentIndex",
		}
		taskTemplate := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "task-{{ .currentIndex }}",
				Type: task.TaskTypeBasic,
			},
		}
		parentTaskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "parent-task",
				Type: task.TaskTypeCollection,
			},
			Task: taskTemplate,
		}
		item := "custom-item"
		index := 5
		context := map[string]any{}

		// Act
		result, err := builder.BuildTaskConfig(collectionConfig, parentTaskConfig, item, index, context)

		// Assert
		assert.NoError(t, err)
		require.NotNil(t, result)
		require.NotNil(t, result.With)
		with := *result.With
		assert.Equal(t, item, with["item"])          // Default item key
		assert.Equal(t, index, with["index"])        // Default index key
		assert.Equal(t, item, with["currentItem"])   // Custom item key
		assert.Equal(t, index, with["currentIndex"]) // Custom index key
	})

	t.Run("Should handle empty custom variable names", func(t *testing.T) {
		// Arrange
		collectionConfig := &task.CollectionConfig{
			Items:    "test_items",
			ItemVar:  "",
			IndexVar: "",
		}
		taskTemplate := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "task-{{ .index }}",
				Type: task.TaskTypeBasic,
			},
		}
		parentTaskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "parent-task",
				Type: task.TaskTypeCollection,
			},
			Task: taskTemplate,
		}
		item := "test-item"
		index := 0
		context := map[string]any{}

		// Act
		result, err := builder.BuildTaskConfig(collectionConfig, parentTaskConfig, item, index, context)

		// Assert
		assert.NoError(t, err)
		require.NotNil(t, result)
		require.NotNil(t, result.With)
		with := *result.With
		assert.Equal(t, item, with["item"])
		assert.Equal(t, index, with["index"])
		// Should not have empty key entries
		_, hasEmptyItemKey := with[""]
		assert.False(t, hasEmptyItemKey)
	})

	t.Run("Should preserve non-template task ID", func(t *testing.T) {
		// Arrange
		collectionConfig := &task.CollectionConfig{
			Items: "test_items",
		}
		taskTemplate := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "static-task-id",
				Type: task.TaskTypeBasic,
			},
		}
		parentTaskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "parent-task",
				Type: task.TaskTypeCollection,
			},
			Task: taskTemplate,
		}
		item := "test-item"
		index := 1
		context := map[string]any{}

		// Act
		result, err := builder.BuildTaskConfig(collectionConfig, parentTaskConfig, item, index, context)

		// Assert
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "static-task-id", result.ID)
	})

	t.Run("Should handle empty task ID", func(t *testing.T) {
		// Arrange
		collectionConfig := &task.CollectionConfig{
			Items: "test_items",
		}
		taskTemplate := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "",
				Type: task.TaskTypeBasic,
			},
		}
		parentTaskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "parent-task",
				Type: task.TaskTypeCollection,
			},
			Task: taskTemplate,
		}
		item := "test-item"
		index := 1
		context := map[string]any{}

		// Act
		result, err := builder.BuildTaskConfig(collectionConfig, parentTaskConfig, item, index, context)

		// Assert
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "", result.ID)
	})

	t.Run("Should return error when task template is nil", func(t *testing.T) {
		// Arrange
		collectionConfig := &task.CollectionConfig{
			Items: "test_items",
		}
		parentTaskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "parent-task",
				Type: task.TaskTypeCollection,
			},
			Task: nil, // No task template
		}
		item := "test-item"
		index := 1
		context := map[string]any{}

		// Act
		result, err := builder.BuildTaskConfig(collectionConfig, parentTaskConfig, item, index, context)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "collection task template is required")
	})

	t.Run("Should return error when task template processing fails", func(t *testing.T) {
		// Arrange
		collectionConfig := &task.CollectionConfig{
			Items: "test_items",
		}
		taskTemplate := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "task-{{ .nonexistent }}",
				Type: task.TaskTypeBasic,
			},
		}
		parentTaskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "parent-task",
				Type: task.TaskTypeCollection,
			},
			Task: taskTemplate,
		}
		item := "test-item"
		index := 1
		context := map[string]any{}

		// Act
		result, err := builder.BuildTaskConfig(collectionConfig, parentTaskConfig, item, index, context)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		// Can be either template processing error or type conversion error
		assert.True(t,
			strings.Contains(err.Error(), "failed to process task ID template") ||
				strings.Contains(err.Error(), "task ID is not a string"),
		)
	})
}
