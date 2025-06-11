package normalizer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTypeConverter_ConvertToSlice(t *testing.T) {
	converter := NewTypeConverter()

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
			result := converter.ConvertToSlice(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}

	t.Run("Should convert map to key-value pairs", func(t *testing.T) {
		input := map[string]any{"x": 10, "y": 20}

		result := converter.ConvertToSlice(input)

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
