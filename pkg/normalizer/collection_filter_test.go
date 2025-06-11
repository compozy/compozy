package normalizer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterEvaluator_IsTruthy(t *testing.T) {
	evaluator := NewFilterEvaluator()

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
			result := evaluator.IsTruthy(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFilterEvaluator_EvaluateFilter(t *testing.T) {
	evaluator := NewFilterEvaluator()

	t.Run("Should return true for empty filter", func(t *testing.T) {
		result, err := evaluator.EvaluateFilter("", map[string]any{})
		require.NoError(t, err)
		assert.True(t, result)
	})

	t.Run("Should evaluate simple template expressions", func(t *testing.T) {
		context := map[string]any{
			"item":  "test",
			"index": 5,
		}

		result, err := evaluator.EvaluateFilter(`{{ eq .item "test" }}`, context)
		require.NoError(t, err)
		assert.True(t, result)

		result, err = evaluator.EvaluateFilter(`{{ eq .item "other" }}`, context)
		require.NoError(t, err)
		assert.False(t, result)
	})

	t.Run("Should handle numeric comparisons", func(t *testing.T) {
		context := map[string]any{
			"index": 3,
		}

		result, err := evaluator.EvaluateFilter(`{{ lt .index 5 }}`, context)
		require.NoError(t, err)
		assert.True(t, result)

		result, err = evaluator.EvaluateFilter(`{{ gt .index 5 }}`, context)
		require.NoError(t, err)
		assert.False(t, result)
	})
}
