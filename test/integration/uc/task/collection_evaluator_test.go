package task

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectionEvaluator_EvaluateItems(t *testing.T) {
	t.Parallel()

	evaluator := uc.NewCollectionEvaluator()
	ctx := context.Background()

	t.Run("should evaluate static array items", func(t *testing.T) {
		input := &uc.EvaluationInput{
			ItemsExpr: `["item1", "item2", "item3"]`,
			Context:   map[string]any{},
		}

		result, err := evaluator.EvaluateItems(ctx, input)
		require.NoError(t, err)
		assert.Equal(t, 3, result.TotalCount)
		assert.Equal(t, 3, result.FilteredCount)
		assert.Len(t, result.Items, 3)
		assert.Equal(t, "item1", result.Items[0])
		assert.Equal(t, "item2", result.Items[1])
		assert.Equal(t, "item3", result.Items[2])
	})

	t.Run("should evaluate dynamic expression items", func(t *testing.T) {
		context := map[string]any{
			"workflow": map[string]any{
				"input": map[string]any{
					"users": []map[string]any{
						{"id": "user1", "name": "Alice"},
						{"id": "user2", "name": "Bob"},
						{"id": "user3", "name": "Charlie"},
					},
				},
			},
		}

		input := &uc.EvaluationInput{
			ItemsExpr: "{{ .workflow.input.users }}",
			Context:   context,
		}

		result, err := evaluator.EvaluateItems(ctx, input)
		require.NoError(t, err)
		assert.Equal(t, 3, result.TotalCount)
		assert.Equal(t, 3, result.FilteredCount)
		assert.Len(t, result.Items, 3)

		// Verify item structure
		users := result.Items
		assert.Equal(t, "user1", users[0].(map[string]any)["id"])
		assert.Equal(t, "Alice", users[0].(map[string]any)["name"])
	})

	t.Run("should apply filter to items", func(t *testing.T) {
		context := map[string]any{
			"workflow": map[string]any{
				"input": map[string]any{
					"numbers": []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
				},
			},
		}

		input := &uc.EvaluationInput{
			ItemsExpr:  "{{ .workflow.input.numbers }}",
			FilterExpr: "{{ eq (mod .item 2) 0 }}", // Even numbers only
			Context:    context,
			ItemVar:    "item",
			IndexVar:   "index",
		}

		result, err := evaluator.EvaluateItems(ctx, input)
		require.NoError(t, err)
		assert.Equal(t, 10, result.TotalCount)
		assert.Equal(t, 5, result.FilteredCount) // 2, 4, 6, 8, 10
		assert.Len(t, result.Items, 5)

		// Verify filtered items are even numbers
		for _, item := range result.Items {
			num := item.(float64)
			assert.Equal(t, 0, int(num)%2, "Should be even number")
		}
	})

	t.Run("should apply complex filter with object properties", func(t *testing.T) {
		context := map[string]any{
			"workflow": map[string]any{
				"input": map[string]any{
					"users": []map[string]any{
						{"id": "user1", "name": "Alice", "active": true, "notified": false},
						{"id": "user2", "name": "Bob", "active": true, "notified": true},
						{"id": "user3", "name": "Charlie", "active": false, "notified": false},
						{"id": "user4", "name": "David", "active": true, "notified": false},
					},
				},
			},
		}

		input := &uc.EvaluationInput{
			ItemsExpr:  "{{ .workflow.input.users }}",
			FilterExpr: "{{ and .item.active (not .item.notified) }}", // Active and not notified
			Context:    context,
			ItemVar:    "item",
			IndexVar:   "index",
		}

		result, err := evaluator.EvaluateItems(ctx, input)
		require.NoError(t, err)
		assert.Equal(t, 4, result.TotalCount)
		assert.Equal(t, 2, result.FilteredCount) // Alice and David
		assert.Len(t, result.Items, 2)

		// Verify filtered users are active and not notified
		for _, item := range result.Items {
			user := item.(map[string]any)
			assert.True(t, user["active"].(bool), "User should be active")
			assert.False(t, user["notified"].(bool), "User should not be notified")
		}
	})

	t.Run("should use default variable names when not provided", func(t *testing.T) {
		context := map[string]any{
			"workflow": map[string]any{
				"input": map[string]any{
					"items": []string{"a", "b", "c"},
				},
			},
		}

		input := &uc.EvaluationInput{
			ItemsExpr:  "{{ .workflow.input.items }}",
			FilterExpr: "{{ ne .item \"b\" }}", // Exclude "b"
			Context:    context,
			// ItemVar and IndexVar not provided - should use defaults
		}

		result, err := evaluator.EvaluateItems(ctx, input)
		require.NoError(t, err)
		assert.Equal(t, 3, result.TotalCount)
		assert.Equal(t, 2, result.FilteredCount) // "a" and "c"
		assert.Len(t, result.Items, 2)
		assert.Equal(t, "a", result.Items[0])
		assert.Equal(t, "c", result.Items[1])
	})

	t.Run("should handle single item expression", func(t *testing.T) {
		input := &uc.EvaluationInput{
			ItemsExpr: "single-item",
			Context:   map[string]any{},
		}

		result, err := evaluator.EvaluateItems(ctx, input)
		require.NoError(t, err)
		assert.Equal(t, 1, result.TotalCount)
		assert.Equal(t, 1, result.FilteredCount)
		assert.Len(t, result.Items, 1)
		assert.Equal(t, "single-item", result.Items[0])
	})

	t.Run("should handle empty arrays from context", func(t *testing.T) {
		context := map[string]any{
			"empty_list": []any{},
		}

		input := &uc.EvaluationInput{
			ItemsExpr: "{{ .empty_list }}",
			Context:   context,
		}

		result, err := evaluator.EvaluateItems(ctx, input)
		require.NoError(t, err)
		assert.Equal(t, 0, result.TotalCount)
		assert.Equal(t, 0, result.FilteredCount)
		assert.Len(t, result.Items, 0)
	})

	t.Run("should handle static empty arrays", func(t *testing.T) {
		input := &uc.EvaluationInput{
			ItemsExpr: "[]",
			Context:   map[string]any{},
		}

		result, err := evaluator.EvaluateItems(ctx, input)
		require.NoError(t, err)
		assert.Equal(t, 0, result.TotalCount)
		assert.Equal(t, 0, result.FilteredCount)
		assert.Len(t, result.Items, 0)
	})

	t.Run("should return error for missing items expression", func(t *testing.T) {
		input := &uc.EvaluationInput{
			ItemsExpr: "",
			Context:   map[string]any{},
		}

		_, err := evaluator.EvaluateItems(ctx, input)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "items expression is required")
	})

	t.Run("should return error for invalid template syntax", func(t *testing.T) {
		input := &uc.EvaluationInput{
			ItemsExpr: "{{ .nonexistent.deeply.nested.field }}",
			Context:   map[string]any{},
		}

		_, err := evaluator.EvaluateItems(ctx, input)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to evaluate items")
	})

	t.Run("should return error for security risk in items expression", func(t *testing.T) {
		input := &uc.EvaluationInput{
			ItemsExpr: "{{ exec \"rm -rf /\" }}",
			Context:   map[string]any{},
		}

		_, err := evaluator.EvaluateItems(ctx, input)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "potential security risk detected")
	})

	t.Run("should return error for security risk in filter expression", func(t *testing.T) {
		input := &uc.EvaluationInput{
			ItemsExpr:  `["item1", "item2"]`,
			FilterExpr: "{{ system \"malicious command\" }}",
			Context:    map[string]any{},
		}

		_, err := evaluator.EvaluateItems(ctx, input)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "potential security risk detected")
	})

	t.Run("should return error for invalid variable names", func(t *testing.T) {
		input := &uc.EvaluationInput{
			ItemsExpr: `["item1", "item2"]`,
			Context:   map[string]any{},
			ItemVar:   "123invalid", // Invalid variable name
		}

		_, err := evaluator.EvaluateItems(ctx, input)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid item variable name")
	})

	t.Run("should return error for collection size exceeding limits", func(t *testing.T) {
		// Create a large array that exceeds the limit
		largeArray := make([]string, task.DefaultMaxCollectionItems+1)
		for i := range largeArray {
			largeArray[i] = "item"
		}

		context := map[string]any{
			"largeArray": largeArray,
		}

		input := &uc.EvaluationInput{
			ItemsExpr: "{{ .largeArray }}",
			Context:   context,
		}

		_, err := evaluator.EvaluateItems(ctx, input)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "collection size")
		assert.Contains(t, err.Error(), "exceeds maximum allowed")
	})
}

func TestTaskTemplateEvaluator_EvaluateTaskTemplate(t *testing.T) {
	t.Parallel()

	evaluator := uc.NewTaskTemplateEvaluator()

	t.Run("should evaluate task template with item variables", func(t *testing.T) {
		template := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "process_user_{{ .user_index }}",
				Type: task.TaskTypeBasic,
				With: &core.Input{
					"user_id": "{{ .user.id }}",
					"name":    "{{ .user.name }}",
					"index":   "{{ .user_index }}",
				},
			},
			BasicTask: task.BasicTask{
				Action: "process-user",
			},
		}

		item := map[string]any{
			"id":   "user123",
			"name": "Alice",
		}

		context := map[string]any{
			"workflow": map[string]any{
				"id": "test-workflow",
			},
		}

		result, err := evaluator.EvaluateTaskTemplate(
			template, item, 0, "user", "user_index", context,
		)
		require.NoError(t, err)

		assert.Equal(t, "process_user_0", result.ID)
		assert.Equal(t, task.TaskTypeBasic, result.Type)
		
		// Verify input values were templated correctly
		require.NotNil(t, result.With)
		assert.Equal(t, "user123", (*result.With)["user_id"])
		assert.Equal(t, "Alice", (*result.With)["name"])
		assert.Equal(t, "0", (*result.With)["index"]) // Template values are converted to strings
	})

	t.Run("should have access to full workflow context", func(t *testing.T) {
		template := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "task_{{ .item_index }}",
				Type: task.TaskTypeBasic,
				With: &core.Input{
					"item":        "{{ .item }}",
					"workflow_id": "{{ .workflow.id }}",
					"env_var":     "{{ .workflow.env.TEST_VAR }}",
				},
			},
			BasicTask: task.BasicTask{
				Action: "process-item",
			},
		}

		item := "test-item"
		context := map[string]any{
			"workflow": map[string]any{
				"id": "workflow-123",
				"env": map[string]any{
					"TEST_VAR": "test-value",
				},
			},
		}

		result, err := evaluator.EvaluateTaskTemplate(
			template, item, 1, "item", "item_index", context,
		)
		require.NoError(t, err)

		assert.Equal(t, "task_1", result.ID)
		
		// Verify context values were accessible
		require.NotNil(t, result.With)
		assert.Equal(t, "test-item", (*result.With)["item"])
		assert.Equal(t, "workflow-123", (*result.With)["workflow_id"])
		assert.Equal(t, "test-value", (*result.With)["env_var"])
	})

	t.Run("should handle complex nested templates", func(t *testing.T) {
		template := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "nested_task",
				Type: task.TaskTypeBasic,
				With: &core.Input{
					"config": map[string]any{
						"user_data": map[string]any{
							"id":     "{{ .user.id }}",
							"name":   "{{ .user.name }}",
							"status": "{{ if .user.active }}active{{ else }}inactive{{ end }}",
						},
						"metadata": map[string]any{
							"index":      "{{ .user_index }}",
							"processed":  false,
							"workflow":   "{{ .workflow.id }}",
						},
					},
				},
			},
			BasicTask: task.BasicTask{
				Action: "process-complex",
			},
		}

		item := map[string]any{
			"id":     "user456",
			"name":   "Bob",
			"active": true,
		}

		context := map[string]any{
			"workflow": map[string]any{
				"id": "complex-workflow",
			},
		}

		result, err := evaluator.EvaluateTaskTemplate(
			template, item, 2, "user", "user_index", context,
		)
		require.NoError(t, err)

		// Verify nested structure was templated correctly
		require.NotNil(t, result.With)
		config := (*result.With)["config"].(map[string]any)
		
		userData := config["user_data"].(map[string]any)
		assert.Equal(t, "user456", userData["id"])
		assert.Equal(t, "Bob", userData["name"])
		assert.Equal(t, "active", userData["status"])
		
		metadata := config["metadata"].(map[string]any)
		assert.Equal(t, "2", metadata["index"]) // Template values are converted to strings
		assert.Equal(t, "false", metadata["processed"]) // Template values are converted to strings
		assert.Equal(t, "complex-workflow", metadata["workflow"])
	})

	t.Run("should return error for invalid template syntax", func(t *testing.T) {
		template := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "{{ invalid template syntax",
				Type: task.TaskTypeBasic,
			},
		}

		item := "test-item"
		context := map[string]any{}

		_, err := evaluator.EvaluateTaskTemplate(
			template, item, 0, "item", "index", context,
		)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to process task template")
	})

	t.Run("should preserve non-templated fields", func(t *testing.T) {
		template := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:       "static_task",
				Type:     task.TaskTypeBasic,
				Strategy: task.StrategyWaitAll,
				Timeout:  "5m",
				With: &core.Input{
					"static_value":   "unchanged",
					"dynamic_value": "{{ .item }}",
				},
			},
			BasicTask: task.BasicTask{
				Action: "test-action",
			},
		}

		item := "dynamic-content"
		context := map[string]any{}

		result, err := evaluator.EvaluateTaskTemplate(
			template, item, 0, "item", "index", context,
		)
		require.NoError(t, err)

		assert.Equal(t, "static_task", result.ID)
		assert.Equal(t, task.TaskTypeBasic, result.Type)
		assert.Equal(t, task.StrategyWaitAll, result.Strategy)
		assert.Equal(t, "5m", result.Timeout)
		assert.Equal(t, "test-action", result.BasicTask.Action)
		
		// Verify both static and dynamic values
		require.NotNil(t, result.With)
		assert.Equal(t, "unchanged", (*result.With)["static_value"])
		assert.Equal(t, "dynamic-content", (*result.With)["dynamic_value"])
	})
}
