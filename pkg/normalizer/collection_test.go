package normalizer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
)

func TestCollectionNormalizer_ExpandCollectionItems(t *testing.T) {
	ctx := context.Background()
	normalizer := NewCollectionNormalizer()

	t.Run("Should expand simple array template", func(t *testing.T) {
		config := &task.CollectionConfig{
			Items: `{{ .input.users }}`,
		}
		context := map[string]any{
			"input": map[string]any{
				"users": []any{"alice", "bob", "charlie"},
			},
		}

		result, err := normalizer.ExpandCollectionItems(ctx, config, context)

		require.NoError(t, err)
		assert.Equal(t, []any{"alice", "bob", "charlie"}, result)
	})

	t.Run("Should expand static array", func(t *testing.T) {
		config := &task.CollectionConfig{
			Items: `["item1", "item2", "item3"]`,
		}

		result, err := normalizer.ExpandCollectionItems(ctx, config, map[string]any{})

		require.NoError(t, err)
		assert.Equal(t, []any{"item1", "item2", "item3"}, result)
	})

	t.Run("Should convert map to key-value pairs", func(t *testing.T) {
		config := &task.CollectionConfig{
			Items: `{{ .input.config }}`,
		}
		context := map[string]any{
			"input": map[string]any{
				"config": map[string]any{
					"host": "localhost",
					"port": 8080,
				},
			},
		}

		result, err := normalizer.ExpandCollectionItems(ctx, config, context)

		require.NoError(t, err)
		require.Len(t, result, 2)

		// Create maps for comparison since order isn't guaranteed
		expectedKeys := map[string]any{
			"host": "localhost",
			"port": 8080,
		}

		resultKeys := make(map[string]any)
		for _, item := range result {
			itemMap, ok := item.(map[string]any)
			require.True(t, ok, "expected result item to be a map")
			require.Contains(t, itemMap, "key")
			require.Contains(t, itemMap, "value")
			resultKeys[itemMap["key"].(string)] = itemMap["value"]
		}

		assert.Equal(t, expectedKeys, resultKeys)
	})

	t.Run("Should handle single string item", func(t *testing.T) {
		config := &task.CollectionConfig{
			Items: `{{ .input.singleItem }}`,
		}
		context := map[string]any{
			"input": map[string]any{
				"singleItem": "solo",
			},
		}

		result, err := normalizer.ExpandCollectionItems(ctx, config, context)

		require.NoError(t, err)
		assert.Equal(t, []any{"solo"}, result)
	})

	t.Run("Should return error for empty items field", func(t *testing.T) {
		config := &task.CollectionConfig{
			Items: "",
		}

		_, err := normalizer.ExpandCollectionItems(ctx, config, map[string]any{})

		assert.Error(t, err)
	})
}

func TestCollectionNormalizer_ConvertToSlice(t *testing.T) {
	normalizer := NewCollectionNormalizer()

	tests := []struct {
		name     string
		input    any
		expected []any
	}{
		{"nil input", nil, []any{}},
		{"interface slice", []any{1, "two", 3.0}, []any{1, "two", 3.0}},
		{"string slice", []string{"a", "b", "c"}, []any{"a", "b", "c"}},
		{"int slice", []int{1, 2, 3}, []any{1, 2, 3}},
		{"float slice", []float64{1.1, 2.2, 3.3}, []any{1.1, 2.2, 3.3}},
		{"single string", "hello", []any{"hello"}},
		{"single int", 42, []any{42}},
		{"single bool", true, []any{true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizer.convertToSlice(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}

	t.Run("Should convert map to key-value pairs", func(t *testing.T) {
		input := map[string]any{"x": 10, "y": 20}

		result := normalizer.convertToSlice(input)

		require.Len(t, result, 2)

		expectedKeys := map[string]any{"x": 10, "y": 20}
		resultKeys := make(map[string]any)
		for _, item := range result {
			itemMap, ok := item.(map[string]any)
			require.True(t, ok)
			require.Contains(t, itemMap, "key")
			require.Contains(t, itemMap, "value")
			resultKeys[itemMap["key"].(string)] = itemMap["value"]
		}

		assert.Equal(t, expectedKeys, resultKeys)
	})
}

func TestCollectionNormalizer_FilterCollectionItems(t *testing.T) {
	ctx := context.Background()
	normalizer := NewCollectionNormalizer()

	t.Run("Should return all items when no filter", func(t *testing.T) {
		config := &task.CollectionConfig{Filter: ""}
		items := []any{"a", "b", "c"}

		result, err := normalizer.FilterCollectionItems(ctx, config, items, map[string]any{})

		require.NoError(t, err)
		assert.Equal(t, []any{"a", "b", "c"}, result)
	})

	t.Run("Should filter by item value", func(t *testing.T) {
		config := &task.CollectionConfig{
			Filter:  `{{ eq .item "b" }}`,
			ItemVar: "item",
		}
		items := []any{"a", "b", "c"}

		result, err := normalizer.FilterCollectionItems(ctx, config, items, map[string]any{})

		require.NoError(t, err)
		assert.Equal(t, []any{"b"}, result)
	})

	t.Run("Should filter by index", func(t *testing.T) {
		config := &task.CollectionConfig{
			Filter:   `{{ lt .index 2 }}`,
			IndexVar: "index",
		}
		items := []any{"a", "b", "c"}

		result, err := normalizer.FilterCollectionItems(ctx, config, items, map[string]any{})

		require.NoError(t, err)
		assert.Equal(t, []any{"a", "b"}, result)
	})

	t.Run("Should filter using custom variables", func(t *testing.T) {
		config := &task.CollectionConfig{
			Filter:   `{{ eq .val "keep" }}`,
			ItemVar:  "val",
			IndexVar: "idx",
		}
		items := []any{"skip", "keep", "skip", "keep"}

		result, err := normalizer.FilterCollectionItems(ctx, config, items, map[string]any{})

		require.NoError(t, err)
		assert.Equal(t, []any{"keep", "keep"}, result)
	})
}

func TestCollectionNormalizer_IsTruthy(t *testing.T) {
	normalizer := NewCollectionNormalizer()

	tests := []struct {
		name     string
		input    any
		expected bool
	}{
		{"nil", nil, false},
		{"true bool", true, true},
		{"false bool", false, false},
		{"true string", "true", true},
		{"false string", "false", false},
		{"empty string", "", false},
		{"non-empty string", "hello", true},
		{"zero int", 0, false},
		{"non-zero int", 42, true},
		{"zero float", 0.0, false},
		{"non-zero float", 3.14, true},
		{"empty slice", []any{}, false},
		{"non-empty slice", []any{1}, true},
		{"empty map", map[string]any{}, false},
		{"non-empty map", map[string]any{"key": "value"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizer.isTruthy(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCollectionNormalizer_CreateItemContext(t *testing.T) {
	normalizer := NewCollectionNormalizer()

	t.Run("Should create item context correctly", func(t *testing.T) {
		baseContext := map[string]any{
			"env":   map[string]string{"HOME": "/home/user"},
			"input": map[string]any{"data": "value"},
		}

		config := &task.CollectionConfig{
			ItemVar:  "item",
			IndexVar: "index",
		}

		item := "test-item"
		index := 42

		result := normalizer.CreateItemContext(baseContext, config, item, index)

		// Check that base context is preserved
		assert.Equal(t, baseContext["env"], result["env"])
		assert.Equal(t, baseContext["input"], result["input"])

		// Check that item variables are added
		assert.Equal(t, item, result[config.GetItemVar()])
		assert.Equal(t, index, result[config.GetIndexVar()])

		// Check that modifying the result doesn't affect the base context
		result["new_key"] = "new_value"
		assert.NotContains(t, baseContext, "new_key")
	})
}

func TestCollectionNormalizer_CreateProgressContext(t *testing.T) {
	normalizer := NewCollectionNormalizer()

	t.Run("Should create progress context correctly", func(t *testing.T) {
		baseContext := map[string]any{
			"input": map[string]any{"data": "value"},
			"env":   map[string]string{"HOME": "/home/user"},
		}

		progressInfo := &task.ProgressInfo{
			TotalChildren:  10,
			CompletedCount: 7,
			FailedCount:    2,
			RunningCount:   1,
			PendingCount:   0,
			CompletionRate: 0.7,
			FailureRate:    0.2,
			OverallStatus:  core.StatusRunning,
			StatusCounts:   map[core.StatusType]int{core.StatusSuccess: 7, core.StatusFailed: 2, core.StatusRunning: 1},
		}

		result := normalizer.CreateProgressContext(baseContext, progressInfo)

		// Check that base context is preserved
		assert.Equal(t, baseContext["input"], result["input"])
		assert.Equal(t, baseContext["env"], result["env"])

		// Check that progress info is added
		progress, ok := result["progress"].(map[string]any)
		require.True(t, ok, "progress should be a map")

		assert.Equal(t, 10, progress["total_children"])
		assert.Equal(t, 7, progress["completed_count"])
		assert.Equal(t, 2, progress["failed_count"])
		assert.Equal(t, 1, progress["running_count"])
		assert.Equal(t, 0, progress["pending_count"])
		assert.Equal(t, 0.7, progress["completion_rate"])
		assert.Equal(t, 0.2, progress["failure_rate"])
		assert.Equal(t, "RUNNING", progress["overall_status"])
		assert.Equal(t, true, progress["has_failures"])
		assert.Equal(t, false, progress["is_all_complete"])

		// Check that summary alias exists
		summary, ok := result["summary"].(map[string]any)
		require.True(t, ok, "summary should be a map")
		assert.Equal(t, progress, summary, "summary should be the same as progress")

		// Check that modifying the result doesn't affect the base context
		result["new_key"] = "new_value"
		assert.NotContains(t, baseContext, "new_key")
	})
}

func TestCollectionNormalizer_ApplyTemplateToConfig(t *testing.T) {
	normalizer := NewCollectionNormalizer()

	t.Run("Should apply template to action", func(t *testing.T) {
		config := &task.Config{
			BasicTask: task.BasicTask{
				Action: "process {{ .item }}",
			},
		}
		itemContext := map[string]any{
			"item":  "test-data",
			"index": 0,
		}

		err := normalizer.ApplyTemplateToConfig(config, itemContext)

		require.NoError(t, err)
		assert.Equal(t, "process test-data", config.Action)
	})

	t.Run("Should apply template to with parameters", func(t *testing.T) {
		config := &task.Config{
			BaseConfig: task.BaseConfig{
				With: &core.Input{
					"message": "Processing item {{ .item }} at index {{ .index }}",
					"value":   "{{ .item }}",
				},
			},
		}
		itemContext := map[string]any{
			"item":  "hello",
			"index": 5,
		}

		err := normalizer.ApplyTemplateToConfig(config, itemContext)

		require.NoError(t, err)
		require.NotNil(t, config.With)

		expected := map[string]any{
			"message": "Processing item hello at index 5",
			"value":   "hello",
		}

		for k, expectedV := range expected {
			actualV, exists := (*config.With)[k]
			assert.True(t, exists, "key %s should exist", k)
			assert.Equal(t, expectedV, actualV, "value for key %s", k)
		}
	})

	t.Run("Should handle no templates to apply", func(t *testing.T) {
		config := &task.Config{
			BasicTask: task.BasicTask{
				Action: "static-action",
			},
		}
		itemContext := map[string]any{
			"item": "test",
		}

		err := normalizer.ApplyTemplateToConfig(config, itemContext)

		require.NoError(t, err)
		assert.Equal(t, "static-action", config.Action)
	})
}
