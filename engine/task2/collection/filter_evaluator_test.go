package collection_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/compozy/compozy/engine/task2/collection"
	"github.com/compozy/compozy/pkg/tplengine"
)

func TestFilterEvaluator_NewFilterEvaluator(t *testing.T) {
	t.Run("Should create filter evaluator with template engine", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)

		// Act
		evaluator := collection.NewFilterEvaluator(templateEngine)

		// Assert
		assert.NotNil(t, evaluator)
	})

	t.Run("Should panic with nil template engine", func(t *testing.T) {
		// Act & Assert
		assert.Panics(t, func() {
			collection.NewFilterEvaluator(nil)
		}, "NewFilterEvaluator should panic when templateEngine is nil")
	})
}

func TestFilterEvaluator_EvaluateFilter(t *testing.T) {
	// Setup
	templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
	evaluator := collection.NewFilterEvaluator(templateEngine)

	t.Run("Should return true for empty filter expression", func(t *testing.T) {
		// Arrange
		filterExpr := ""
		context := map[string]any{
			"item": "test-item",
		}

		// Act
		result, err := evaluator.EvaluateFilter(filterExpr, context)

		// Assert
		assert.NoError(t, err)
		assert.True(t, result)
	})

	t.Run("Should evaluate boolean true expressions", func(t *testing.T) {
		testCases := []struct {
			name       string
			filterExpr string
			expected   bool
		}{
			{"literal true", "true", true},
			{"literal TRUE", "TRUE", true},
			{"literal yes", "yes", true},
			{"literal YES", "YES", true},
			{"literal 1", "1", true},
			{"literal on", "on", true},
			{"literal ON", "ON", true},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Arrange
				context := map[string]any{
					"item": "test-item",
				}

				// Act
				result, err := evaluator.EvaluateFilter(tc.filterExpr, context)

				// Assert
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			})
		}
	})

	t.Run("Should evaluate boolean false expressions", func(t *testing.T) {
		testCases := []struct {
			name       string
			filterExpr string
			expected   bool
		}{
			{"literal false", "false", false},
			{"literal FALSE", "FALSE", false},
			{"literal no", "no", false},
			{"literal NO", "NO", false},
			{"literal 0", "0", false},
			{"literal off", "off", false},
			{"literal OFF", "OFF", false},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Arrange
				context := map[string]any{
					"item": "test-item",
				}

				// Act
				result, err := evaluator.EvaluateFilter(tc.filterExpr, context)

				// Assert
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			})
		}
	})

	t.Run("Should evaluate template expressions with item context", func(t *testing.T) {
		// Arrange
		filterExpr := "{{ if eq .item \"active-item\" }}true{{ else }}false{{ end }}"
		context := map[string]any{
			"item":  "active-item",
			"index": 0,
		}

		// Act
		result, err := evaluator.EvaluateFilter(filterExpr, context)

		// Assert
		assert.NoError(t, err)
		assert.True(t, result)
	})

	t.Run("Should evaluate template expressions with false result", func(t *testing.T) {
		// Arrange
		filterExpr := "{{ if eq .item \"active-item\" }}true{{ else }}false{{ end }}"
		context := map[string]any{
			"item":  "inactive-item",
			"index": 1,
		}

		// Act
		result, err := evaluator.EvaluateFilter(filterExpr, context)

		// Assert
		assert.NoError(t, err)
		assert.False(t, result)
	})

	t.Run("Should treat non-empty non-boolean strings as true", func(t *testing.T) {
		// Arrange
		filterExpr := "{{ if .item.active }}active{{ end }}"
		context := map[string]any{
			"item": map[string]any{
				"active": true,
			},
		}

		// Act
		result, err := evaluator.EvaluateFilter(filterExpr, context)

		// Assert
		assert.NoError(t, err)
		assert.True(t, result) // "active" is non-empty, so it's true
	})

	t.Run("Should treat empty string result as false", func(t *testing.T) {
		// Arrange
		filterExpr := "{{ if .item.active }}active{{ end }}"
		context := map[string]any{
			"item": map[string]any{
				"active": false,
			},
		}

		// Act
		result, err := evaluator.EvaluateFilter(filterExpr, context)

		// Assert
		assert.NoError(t, err)
		assert.False(t, result) // Empty string result is false
	})

	t.Run("Should handle complex filter expressions", func(t *testing.T) {
		// Arrange
		filterExpr := "{{ and (gt .item.score 80) (eq .item.status \"active\") }}"
		context := map[string]any{
			"item": map[string]any{
				"score":  85,
				"status": "active",
			},
		}

		// Act
		result, err := evaluator.EvaluateFilter(filterExpr, context)

		// Assert
		assert.NoError(t, err)
		assert.True(t, result)
	})

	t.Run("Should handle nil context", func(t *testing.T) {
		// Arrange
		filterExpr := "true"

		// Act
		result, err := evaluator.EvaluateFilter(filterExpr, nil)

		// Assert
		assert.NoError(t, err)
		assert.True(t, result)
	})

	t.Run("Should handle whitespace around results", func(t *testing.T) {
		// Arrange
		filterExpr := "  true  "
		context := map[string]any{
			"item": "test-item",
		}

		// Act
		result, err := evaluator.EvaluateFilter(filterExpr, context)

		// Assert
		assert.NoError(t, err)
		assert.True(t, result)
	})

	t.Run("Should return error for template processing errors", func(t *testing.T) {
		// Arrange
		filterExpr := "{{ .nonexistent.field }}"
		context := map[string]any{
			"item": "test-item",
		}

		// Act
		result, err := evaluator.EvaluateFilter(filterExpr, context)

		// Assert
		assert.Error(t, err)
		assert.False(t, result)
		assert.Contains(t, err.Error(), "failed to process filter expression")
	})

	t.Run("Should handle numeric filter results", func(t *testing.T) {
		// Arrange
		filterExpr := "{{ .item.count }}"
		context := map[string]any{
			"item": map[string]any{
				"count": 5,
			},
		}

		// Act
		result, err := evaluator.EvaluateFilter(filterExpr, context)

		// Assert
		assert.NoError(t, err)
		assert.True(t, result) // "5" is non-empty, so it's true
	})

	t.Run("Should handle zero numeric filter results", func(t *testing.T) {
		// Arrange
		filterExpr := "{{ .item.count }}"
		context := map[string]any{
			"item": map[string]any{
				"count": 0,
			},
		}

		// Act
		result, err := evaluator.EvaluateFilter(filterExpr, context)

		// Assert
		assert.NoError(t, err)
		assert.False(t, result) // "0" evaluates to false
	})
}
