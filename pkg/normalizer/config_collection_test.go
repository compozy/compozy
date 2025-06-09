package normalizer

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
)

func TestCollectionConfigBuilder_CreateChildConfigs(t *testing.T) {
	builder := NewCollectionConfigBuilder()

	t.Run("Should create configs from task template", func(t *testing.T) {
		templateConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID: "template-task",
				With: &core.Input{
					"param": "{{ .item.value }}",
				},
			},
			BasicTask: task.BasicTask{
				Action: "{{ .item.action }}",
			},
		}

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID: "collection-task",
			},
			CollectionConfig: task.CollectionConfig{
				ItemVar:  "item",
				IndexVar: "index",
			},
			ParallelTask: task.ParallelTask{
				Task: templateConfig,
			},
		}

		filteredItems := []any{
			map[string]any{"action": "read", "value": "file1.txt"},
			map[string]any{"action": "write", "value": "file2.txt"},
		}

		templateContext := map[string]any{
			"baseParam": "baseValue",
		}

		result, err := builder.CreateChildConfigs(taskConfig, filteredItems, templateContext)

		require.NoError(t, err)
		require.Len(t, result, 2)

		// Check first child config
		assert.Equal(t, "collection-task_item_0", result[0].ID)
		assert.Equal(t, "read", result[0].Action)
		assert.Equal(t, "file1.txt", (*result[0].With)["param"])

		// Check second child config
		assert.Equal(t, "collection-task_item_1", result[1].ID)
		assert.Equal(t, "write", result[1].Action)
		assert.Equal(t, "file2.txt", (*result[1].With)["param"])
	})

	t.Run("Should create configs from tasks array", func(t *testing.T) {
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID: "collection-task",
			},
			CollectionConfig: task.CollectionConfig{
				ItemVar:  "item",
				IndexVar: "index",
			},
			ParallelTask: task.ParallelTask{
				Tasks: []task.Config{
					{
						BaseConfig: task.BaseConfig{
							ID: "task1",
						},
						BasicTask: task.BasicTask{
							Action: "step1-{{ .item }}",
						},
					},
					{
						BaseConfig: task.BaseConfig{
							ID: "task2",
						},
						BasicTask: task.BasicTask{
							Action: "step2-{{ .item }}",
						},
					},
				},
			},
		}

		filteredItems := []any{"itemA", "itemB"}
		templateContext := map[string]any{}

		result, err := builder.CreateChildConfigs(taskConfig, filteredItems, templateContext)

		require.NoError(t, err)
		require.Len(t, result, 4) // 2 items × 2 tasks = 4 configs

		// Check the naming pattern
		assert.Equal(t, "collection-task_item_0_task_0", result[0].ID)
		assert.Equal(t, "collection-task_item_0_task_1", result[1].ID)
		assert.Equal(t, "collection-task_item_1_task_0", result[2].ID)
		assert.Equal(t, "collection-task_item_1_task_1", result[3].ID)

		// Check actions are templated correctly
		assert.Equal(t, "step1-itemA", result[0].Action)
		assert.Equal(t, "step2-itemA", result[1].Action)
		assert.Equal(t, "step1-itemB", result[2].Action)
		assert.Equal(t, "step2-itemB", result[3].Action)
	})

	t.Run("Should return error for invalid task config", func(t *testing.T) {
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID: "invalid-task",
			},
			// No Task template and no Tasks array
		}

		filteredItems := []any{"item1"}
		templateContext := map[string]any{}

		_, err := builder.CreateChildConfigs(taskConfig, filteredItems, templateContext)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "collection task must have either a task template or tasks array")
	})
}

func TestCollectionConfigBuilder_DeepCopyConfig(t *testing.T) {
	t.Run("Should create independent copies", func(t *testing.T) {
		original := &task.Config{
			BaseConfig: task.BaseConfig{
				ID: "original",
				With: &core.Input{
					"param1": "value1",
					"param2": 42,
				},
			},
			BasicTask: task.BasicTask{
				Action: "original-action",
			},
		}

		copy1, err := deepCopyConfig(original)
		require.NoError(t, err)

		copy2, err := deepCopyConfig(original)
		require.NoError(t, err)

		// Modify the copies
		copy1.ID = "copy1"
		copy1.Action = "copy1-action"
		(*copy1.With)["param1"] = "modified1"

		copy2.ID = "copy2"
		copy2.Action = "copy2-action"
		(*copy2.With)["param2"] = 999

		// Original should be unchanged
		assert.Equal(t, "original", original.ID)
		assert.Equal(t, "original-action", original.Action)
		assert.Equal(t, "value1", (*original.With)["param1"])
		assert.Equal(t, 42, (*original.With)["param2"])

		// Copies should be independent
		assert.Equal(t, "copy1", copy1.ID)
		assert.Equal(t, "copy1-action", copy1.Action)
		assert.Equal(t, "modified1", (*copy1.With)["param1"])
		assert.Equal(t, 42, (*copy1.With)["param2"]) // Unchanged

		assert.Equal(t, "copy2", copy2.ID)
		assert.Equal(t, "copy2-action", copy2.Action)
		assert.Equal(t, "value1", (*copy2.With)["param1"]) // Unchanged
		assert.Equal(t, 999, (*copy2.With)["param2"])
	})

	t.Run("Should handle nil With field", func(t *testing.T) {
		original := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "original",
				With: nil,
			},
			BasicTask: task.BasicTask{
				Action: "original-action",
			},
		}

		copied, err := deepCopyConfig(original)
		require.NoError(t, err)

		assert.Equal(t, "original", copied.ID)
		assert.Equal(t, "original-action", copied.Action)
		assert.Nil(t, copied.With)

		// Modify copy should not affect original
		copied.ID = "modified"
		assert.Equal(t, "original", original.ID)
	})
}

func TestCollectionConfigBuilder_Integration(t *testing.T) {
	builder := NewCollectionConfigBuilder()

	t.Run("Should handle complex template scenarios", func(t *testing.T) {
		templateConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID: "template",
				With: &core.Input{
					"file":     "{{ .item.path }}",
					"mode":     "{{ .item.mode }}",
					"index":    "{{ .index }}",
					"total":    "{{ len .items }}",
					"previous": "{{ if gt .index 0 }}{{ index .items (sub .index 1) }}{{ end }}",
				},
			},
			BasicTask: task.BasicTask{
				Action: "process",
			},
		}

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID: "batch-processor",
			},
			CollectionConfig: task.CollectionConfig{
				ItemVar:  "item",
				IndexVar: "index",
			},
			ParallelTask: task.ParallelTask{
				Task: templateConfig,
			},
		}

		filteredItems := []any{
			map[string]any{"path": "/data/file1.txt", "mode": "read"},
			map[string]any{"path": "/data/file2.txt", "mode": "write"},
			map[string]any{"path": "/data/file3.txt", "mode": "append"},
		}

		templateContext := map[string]any{
			"items": filteredItems,
		}

		result, err := builder.CreateChildConfigs(taskConfig, filteredItems, templateContext)

		require.NoError(t, err)
		require.Len(t, result, 3)

		// Verify that each child config has the correct templated values
		for i, config := range result {
			assert.Equal(t, fmt.Sprintf("batch-processor_item_%d", i), config.ID)
			assert.Equal(t, "process", config.Action)

			expectedPath := filteredItems[i].(map[string]any)["path"]
			assert.Equal(t, expectedPath, (*config.With)["file"])

			expectedMode := filteredItems[i].(map[string]any)["mode"]
			assert.Equal(t, expectedMode, (*config.With)["mode"])
		}
	})
}
