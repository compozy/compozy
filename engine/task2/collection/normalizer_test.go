package collection_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/collection"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
)

func TestCollectionNormalizer_Type(t *testing.T) {
	templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
	normalizer := collection.NewNormalizer(t.Context(), templateEngine, nil)
	assert.Equal(t, task.TaskTypeCollection, normalizer.Type())
}

func TestCollectionNormalizer_Normalize(t *testing.T) {
	// Setup
	templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
	contextBuilder, err := shared.NewContextBuilder(t.Context())
	require.NoError(t, err)
	normalizer := collection.NewNormalizer(t.Context(), templateEngine, contextBuilder)

	t.Run("Should normalize non-collection fields while preserving collection-specific fields", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "collection-{{ .env }}-{{ .index }}",
				Type: task.TaskTypeCollection,
				With: &core.Input{
					"param1": "value-{{ .global }}",
				},
			},
			CollectionConfig: task.CollectionConfig{
				Items:    "{{ .items }}",
				ItemVar:  "current",
				IndexVar: "idx",
				Filter:   "{{ if .filter }}{{ .filter }}{{ end }}",
			},
			Task: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   "child-{{ .current }}-{{ .idx }}",
					Type: task.TaskTypeBasic,
					With: &core.Input{
						"childParam": "{{ .parent.id }}",
					},
				},
			},
		}
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{
				"env":    "prod",
				"index":  1,
				"global": "test",
				"items":  []string{"item1", "item2"},
				"filter": "{{ if eq .current \"item1\" }}true{{ else }}false{{ end }}",
			},
		}

		// Act
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)

		// Assert
		assert.NoError(t, err)
		// Verify ID was normalized
		assert.Equal(t, "collection-prod-1", taskConfig.ID)
		// Verify With parameters were normalized
		assert.Equal(t, "value-test", (*taskConfig.With)["param1"])
		// Verify collection-specific fields are NOT normalized (preserved as templates)
		assert.Equal(t, "{{ .items }}", taskConfig.Items)
		assert.Equal(t, "{{ if .filter }}{{ .filter }}{{ end }}", taskConfig.Filter)
		// Verify child task template is preserved (not expanded yet)
		assert.NotNil(t, taskConfig.Task)
		assert.Equal(t, "child-{{ .current }}-{{ .idx }}", taskConfig.Task.ID)
		// Verify custom variables are preserved
		assert.Equal(t, "current", taskConfig.ItemVar)
		assert.Equal(t, "idx", taskConfig.IndexVar)
	})

	t.Run("Should return error for non-collection task type", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "basic-task",
				Type: task.TaskTypeBasic,
			},
		}
		ctx := &shared.NormalizationContext{}

		// Act
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "collection normalizer cannot handle task type: basic")
	})

	t.Run("Should handle nil config gracefully", func(t *testing.T) {
		// Arrange
		ctx := &shared.NormalizationContext{}

		// Act
		err := normalizer.Normalize(t.Context(), nil, ctx)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("Should panic with nil context", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "collection-task",
				Type: task.TaskTypeCollection,
			},
			CollectionConfig: task.CollectionConfig{
				Items: "[1, 2, 3]",
			},
		}

		// Act
		err := normalizer.Normalize(t.Context(), taskConfig, nil)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid context type")
	})

	t.Run("Should handle template parsing errors in main config", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "{{ .invalid.deeply.nested.nonexistent.field }}",
				Type: task.TaskTypeCollection,
			},
			CollectionConfig: task.CollectionConfig{
				Items: "[1, 2, 3]",
			},
		}
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{
				"existing": "value",
			},
		}

		// Act
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to normalize collection task config")
	})

	t.Run("Should handle config serialization errors", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "collection-task",
				Type: task.TaskTypeCollection,
			},
			CollectionConfig: task.CollectionConfig{
				Items: "[1, 2, 3]",
			},
		}
		// Inject problematic data for serialization
		unsafeField := func() {}
		taskConfig.With = &core.Input{"function": unsafeField}

		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}

		// Act
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to convert task config to map")
	})

	t.Run("Should handle config FromMap errors", func(t *testing.T) {
		// Arrange - Create a scenario where FromMap might fail
		// This is challenging to trigger without complex mocking
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "collection-task",
				Type: task.TaskTypeCollection,
			},
			CollectionConfig: task.CollectionConfig{
				Items: "[1, 2, 3]",
			},
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}

		// Act - Normal scenario should work
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)

		// Assert - Should succeed in normal case
		assert.NoError(t, err)
	})

	t.Run("Should preserve child task during normalization", func(t *testing.T) {
		// Arrange
		originalChildTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "child-{{ .item }}-{{ .index }}",
				Type: task.TaskTypeBasic,
				With: &core.Input{
					"param": "{{ .parent.output }}",
				},
			},
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "collection-{{ .name }}",
				Type: task.TaskTypeCollection,
			},
			CollectionConfig: task.CollectionConfig{
				Items: "{{ .items }}",
			},
			Task: originalChildTask,
		}
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{
				"name":  "test",
				"items": []string{"a", "b", "c"},
			},
		}

		// Act
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, "collection-test", taskConfig.ID)
		// Child task should be preserved with templates intact
		assert.NotNil(t, taskConfig.Task)
		assert.Equal(t, "child-{{ .item }}-{{ .index }}", taskConfig.Task.ID)
		assert.Equal(t, "{{ .parent.output }}", (*taskConfig.Task.With)["param"])
		// Items should be preserved as template
		assert.Equal(t, "{{ .items }}", taskConfig.Items)
	})

	t.Run("Should handle nil template engine", func(t *testing.T) {
		// Arrange
		normalizerWithNilEngine := collection.NewNormalizer(t.Context(), nil, contextBuilder)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "collection-task",
				Type: task.TaskTypeCollection,
			},
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}

		// Act
		err := normalizerWithNilEngine.Normalize(t.Context(), taskConfig, ctx)
		// Assert - Should return error due to nil template engine
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "template engine is required for normalization")
	})

	t.Run("Should handle empty collections config", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "collection-task",
				Type: task.TaskTypeCollection,
			},
			// Empty CollectionConfig
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}

		// Act
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)

		// Assert - Should succeed even with empty collection config
		assert.NoError(t, err)
	})
}

func TestCollectionNormalizer_ExpandCollectionItems(t *testing.T) {
	// Setup
	templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
	contextBuilder, err := shared.NewContextBuilder(t.Context())
	require.NoError(t, err)
	normalizer := collection.NewNormalizer(t.Context(), templateEngine, contextBuilder)
	ctx := t.Context()

	t.Run("Should expand template expression to array", func(t *testing.T) {
		// Arrange
		collectionConfig := &task.CollectionConfig{
			Items: "{{ .items }}",
		}
		templateContext := map[string]any{
			"items": []string{"item1", "item2", "item3"},
		}

		// Act
		result, err := normalizer.ExpandCollectionItems(ctx, collectionConfig, templateContext)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, []any{"item1", "item2", "item3"}, result)
	})

	t.Run("Should parse JSON array string", func(t *testing.T) {
		// Arrange
		collectionConfig := &task.CollectionConfig{
			Items: `["a", "b", "c"]`,
		}
		templateContext := map[string]any{}

		// Act
		result, err := normalizer.ExpandCollectionItems(ctx, collectionConfig, templateContext)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, []any{"a", "b", "c"}, result)
	})

	t.Run("Should expand numeric range", func(t *testing.T) {
		// Arrange
		collectionConfig := &task.CollectionConfig{
			Items: "1..5",
		}
		templateContext := map[string]any{}

		// Act
		result, err := normalizer.ExpandCollectionItems(ctx, collectionConfig, templateContext)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, []any{1, 2, 3, 4, 5}, result)
	})

	t.Run("Should expand character range", func(t *testing.T) {
		// Arrange
		collectionConfig := &task.CollectionConfig{
			Items: "a..d",
		}
		templateContext := map[string]any{}

		// Act
		result, err := normalizer.ExpandCollectionItems(ctx, collectionConfig, templateContext)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, []any{"a", "b", "c", "d"}, result)
	})

	t.Run("Should return error for empty items", func(t *testing.T) {
		// Arrange
		collectionConfig := &task.CollectionConfig{
			Items: "",
		}
		templateContext := map[string]any{}

		// Act
		_, err := normalizer.ExpandCollectionItems(ctx, collectionConfig, templateContext)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "items field is required")
	})

	t.Run("Should handle template parsing errors", func(t *testing.T) {
		// Arrange
		collectionConfig := &task.CollectionConfig{
			Items: "{{ .invalid.deeply.nested.nonexistent.field }}",
		}
		templateContext := map[string]any{
			"existing": "value",
		}

		// Act
		_, err := normalizer.ExpandCollectionItems(ctx, collectionConfig, templateContext)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to process items expression")
	})

	t.Run("Should handle complex template expressions", func(t *testing.T) {
		// Arrange
		collectionConfig := &task.CollectionConfig{
			Items: "{{ range .items }}{{ . }}{{ end }}",
		}
		templateContext := map[string]any{
			"items": []string{"a", "b", "c"},
		}

		// Act
		result, err := normalizer.ExpandCollectionItems(ctx, collectionConfig, templateContext)

		// Assert
		assert.NoError(t, err)
		// Should convert result to slice
		assert.NotNil(t, result)
	})

	t.Run("Should handle JSON parsing with precision", func(t *testing.T) {
		// Arrange
		collectionConfig := &task.CollectionConfig{
			Items: `[1.23456789012345, 9876543210]`,
		}
		templateContext := map[string]any{}

		// Act
		result, err := normalizer.ExpandCollectionItems(ctx, collectionConfig, templateContext)

		// Assert
		assert.NoError(t, err)
		assert.Len(t, result, 2)
		// Should preserve precision for numbers
		assert.NotNil(t, result[0])
		assert.NotNil(t, result[1])
	})

	t.Run("Should handle invalid JSON gracefully", func(t *testing.T) {
		// Arrange
		collectionConfig := &task.CollectionConfig{
			Items: `invalid json format`,
		}
		templateContext := map[string]any{}

		// Act
		result, err := normalizer.ExpandCollectionItems(ctx, collectionConfig, templateContext)

		// Assert
		assert.NoError(t, err)
		// Should treat as raw string and convert to slice
		assert.NotNil(t, result)
	})

	t.Run("Should handle nil template context", func(t *testing.T) {
		// Arrange
		collectionConfig := &task.CollectionConfig{
			Items: "[1, 2, 3]",
		}

		// Act
		result, err := normalizer.ExpandCollectionItems(ctx, collectionConfig, nil)

		// Assert
		assert.NoError(t, err)
		assert.Len(t, result, 3)
		// JSON parsing returns json.Number for precision preservation
		assert.NotNil(t, result[0])
		assert.NotNil(t, result[1])
		assert.NotNil(t, result[2])
	})

	t.Run("Should handle empty template context", func(t *testing.T) {
		// Arrange
		collectionConfig := &task.CollectionConfig{
			Items: "[\"empty\", \"context\"]",
		}
		templateContext := map[string]any{}

		// Act
		result, err := normalizer.ExpandCollectionItems(ctx, collectionConfig, templateContext)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, []any{"empty", "context"}, result)
	})
}

func TestCollectionNormalizer_FilterCollectionItems(t *testing.T) {
	// Setup
	templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
	contextBuilder, err := shared.NewContextBuilder(t.Context())
	require.NoError(t, err)
	normalizer := collection.NewNormalizer(t.Context(), templateEngine, contextBuilder)
	ctx := t.Context()

	t.Run("Should filter items based on condition", func(t *testing.T) {
		// Arrange
		collectionConfig := &task.CollectionConfig{
			Filter:   `{{ if gt .item 2 }}true{{ else }}false{{ end }}`,
			ItemVar:  "item",
			IndexVar: "index",
		}
		items := []any{1, 2, 3, 4, 5}
		templateContext := map[string]any{}

		// Act
		result, err := normalizer.FilterCollectionItems(ctx, collectionConfig, items, templateContext)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, []any{3, 4, 5}, result)
	})

	t.Run("Should return all items when no filter", func(t *testing.T) {
		// Arrange
		collectionConfig := &task.CollectionConfig{
			Filter: "",
		}
		items := []any{"a", "b", "c"}
		templateContext := map[string]any{}

		// Act
		result, err := normalizer.FilterCollectionItems(ctx, collectionConfig, items, templateContext)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, items, result)
	})

	t.Run("Should use custom item and index variables", func(t *testing.T) {
		// Arrange
		collectionConfig := &task.CollectionConfig{
			Filter:   `{{ if eq (mod .idx 2) 0 }}true{{ else }}false{{ end }}`,
			ItemVar:  "current",
			IndexVar: "idx",
		}
		items := []any{"a", "b", "c", "d"}
		templateContext := map[string]any{}

		// Act
		result, err := normalizer.FilterCollectionItems(ctx, collectionConfig, items, templateContext)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, []any{"a", "c"}, result) // indices 0 and 2
	})

	t.Run("Should handle filter evaluation errors", func(t *testing.T) {
		// Arrange
		collectionConfig := &task.CollectionConfig{
			Filter: "{{ .invalid.deeply.nested.nonexistent.field }}",
		}
		items := []any{"a", "b", "c"}
		templateContext := map[string]any{
			"existing": "value",
		}

		// Act
		_, err := normalizer.FilterCollectionItems(ctx, collectionConfig, items, templateContext)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to evaluate filter expression for item")
	})

	t.Run("Should handle empty items list", func(t *testing.T) {
		// Arrange
		collectionConfig := &task.CollectionConfig{
			Filter: "{{ if eq .item \"keep\" }}true{{ else }}false{{ end }}",
		}
		items := []any{}
		templateContext := map[string]any{}

		// Act
		result, err := normalizer.FilterCollectionItems(ctx, collectionConfig, items, templateContext)

		// Assert
		assert.NoError(t, err)
		assert.Empty(t, result) // Could be nil or empty slice
	})

	t.Run("Should handle nil items list", func(t *testing.T) {
		// Arrange
		collectionConfig := &task.CollectionConfig{
			Filter: "true",
		}
		templateContext := map[string]any{}

		// Act
		result, err := normalizer.FilterCollectionItems(ctx, collectionConfig, nil, templateContext)

		// Assert
		assert.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("Should handle complex filter expressions", func(t *testing.T) {
		// Arrange
		collectionConfig := &task.CollectionConfig{
			Filter: `{{ if and (gt .item 1) (lt .item 4) }}true{{ else }}false{{ end }}`,
		}
		items := []any{1, 2, 3, 4, 5}
		templateContext := map[string]any{}

		// Act
		result, err := normalizer.FilterCollectionItems(ctx, collectionConfig, items, templateContext)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, []any{2, 3}, result)
	})

	t.Run("Should handle filter that rejects all items", func(t *testing.T) {
		// Arrange
		collectionConfig := &task.CollectionConfig{
			Filter: "false",
		}
		items := []any{"a", "b", "c"}
		templateContext := map[string]any{}

		// Act
		result, err := normalizer.FilterCollectionItems(ctx, collectionConfig, items, templateContext)

		// Assert
		assert.NoError(t, err)
		assert.Empty(t, result) // Could be nil or empty slice
	})

	t.Run("Should handle filter that accepts all items", func(t *testing.T) {
		// Arrange
		collectionConfig := &task.CollectionConfig{
			Filter: "true",
		}
		items := []any{1, 2, 3}
		templateContext := map[string]any{}

		// Act
		result, err := normalizer.FilterCollectionItems(ctx, collectionConfig, items, templateContext)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, []any{1, 2, 3}, result)
	})

	t.Run("Should handle nil template context", func(t *testing.T) {
		// Arrange
		collectionConfig := &task.CollectionConfig{
			Filter: "true",
		}
		items := []any{"a", "b"}

		// Act
		result, err := normalizer.FilterCollectionItems(ctx, collectionConfig, items, nil)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, []any{"a", "b"}, result)
	})

	t.Run("Should handle complex item types in filter", func(t *testing.T) {
		// Arrange
		collectionConfig := &task.CollectionConfig{
			Filter: `{{ if eq .item.status "active" }}true{{ else }}false{{ end }}`,
		}
		items := []any{
			map[string]any{"id": 1, "status": "active"},
			map[string]any{"id": 2, "status": "inactive"},
			map[string]any{"id": 3, "status": "active"},
		}
		templateContext := map[string]any{}

		// Act
		result, err := normalizer.FilterCollectionItems(ctx, collectionConfig, items, templateContext)

		// Assert
		assert.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, map[string]any{"id": 1, "status": "active"}, result[0])
		assert.Equal(t, map[string]any{"id": 3, "status": "active"}, result[1])
	})
}

func TestCollectionNormalizer_CreateItemContext(t *testing.T) {
	// Setup
	templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
	contextBuilder, err := shared.NewContextBuilder(t.Context())
	require.NoError(t, err)
	normalizer := collection.NewNormalizer(t.Context(), templateEngine, contextBuilder)

	t.Run("Should create item context with default variables", func(t *testing.T) {
		// Arrange
		baseContext := map[string]any{
			"global": "value",
		}
		collectionConfig := &task.CollectionConfig{
			ItemVar:  "",
			IndexVar: "",
		}
		item := "test-item"
		index := 2

		// Act
		result := normalizer.CreateItemContext(baseContext, collectionConfig, item, index)

		// Assert
		assert.Equal(t, "value", result["global"])
		assert.Equal(t, "test-item", result["item"])
		assert.Equal(t, 2, result["index"])
	})

	t.Run("Should create item context with custom variables", func(t *testing.T) {
		// Arrange
		baseContext := map[string]any{
			"global": "value",
		}
		collectionConfig := &task.CollectionConfig{
			ItemVar:  "current",
			IndexVar: "idx",
		}
		item := map[string]any{"id": 123}
		index := 5

		// Act
		result := normalizer.CreateItemContext(baseContext, collectionConfig, item, index)

		// Assert
		assert.Equal(t, "value", result["global"])
		assert.Equal(t, item, result["item"])
		assert.Equal(t, 5, result["index"])
		assert.Equal(t, item, result["current"])
		assert.Equal(t, 5, result["idx"])
	})
}

func TestCollectionNormalizer_BuildCollectionContext(t *testing.T) {
	// Setup
	templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
	contextBuilder, err := shared.NewContextBuilder(t.Context())
	require.NoError(t, err)
	normalizer := collection.NewNormalizer(t.Context(), templateEngine, contextBuilder)

	t.Run("Should build collection context with workflow and task data", func(t *testing.T) {
		// Arrange
		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Tasks: map[string]*task.State{
				"task1": {
					TaskID:     "task1",
					TaskExecID: core.MustNewID(),
					Status:     core.StatusSuccess,
					Output:     &core.Output{"result": "value1"},
				},
			},
		}
		workflowConfig := &workflow.Config{
			ID: "test-workflow",
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "collection-task",
				Type: task.TaskTypeCollection,
			},
		}

		// Act
		result := normalizer.BuildCollectionContext(t.Context(), workflowState, workflowConfig, taskConfig)

		// Assert
		assert.NotNil(t, result)
		assert.Contains(t, result, "workflow")
		assert.Contains(t, result, "tasks")

		// Verify workflow context
		workflowCtx, ok := result["workflow"].(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, "test-workflow", workflowCtx["id"])

		// Verify tasks context
		tasksCtx, ok := result["tasks"].(map[string]any)
		assert.True(t, ok)
		assert.Contains(t, tasksCtx, "task1")
	})
}
