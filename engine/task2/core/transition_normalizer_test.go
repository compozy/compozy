package core

import (
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock template engine for testing
type mockTemplateEngine struct {
	ParseMapFunc func(data map[string]any, context map[string]any) (map[string]any, error)
}

func (m *mockTemplateEngine) Process(template string, _ map[string]any) (string, error) {
	return template, nil
}

func (m *mockTemplateEngine) ProcessMap(data map[string]any, _ map[string]any) (map[string]any, error) {
	return data, nil
}

func (m *mockTemplateEngine) ProcessSlice(slice []any, _ map[string]any) ([]any, error) {
	return slice, nil
}

func (m *mockTemplateEngine) ProcessString(templateStr string, _ map[string]any) (*shared.ProcessResult, error) {
	return &shared.ProcessResult{Text: templateStr}, nil
}

func (m *mockTemplateEngine) ParseMapWithFilter(
	data map[string]any,
	vars map[string]any,
	_ func(string) bool,
) (map[string]any, error) {
	return m.ParseMap(data, vars)
}

func (m *mockTemplateEngine) ParseMap(data map[string]any, context map[string]any) (map[string]any, error) {
	if m.ParseMapFunc != nil {
		return m.ParseMapFunc(data, context)
	}
	return data, nil
}

func (m *mockTemplateEngine) ParseValue(value any, _ map[string]any) (any, error) {
	return value, nil
}

func TestTransitionNormalizer_Success(t *testing.T) {
	t.Run("Should normalize success transition", func(t *testing.T) {
		// Create mock template engine
		templateEngine := &mockTemplateEngine{
			ParseMapFunc: func(data map[string]any, _ map[string]any) (map[string]any, error) {
				// Simulate template processing
				if next, ok := data["next"].(string); ok && next == "task-{{.index}}" {
					data["next"] = "task-1"
				}
				return data, nil
			},
		}
		// Create normalizer
		normalizer := NewSuccessTransitionNormalizer(templateEngine)
		// Create transition
		next := "task-{{.index}}"
		inputData := core.Input{
			"key": "value",
		}
		transition := &core.SuccessTransition{
			Next: &next,
			With: &inputData,
		}
		// Create context
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{
				"index": 1,
			},
		}
		// Normalize
		err := normalizer.Normalize(transition, ctx)
		require.NoError(t, err)
		// Verify normalization
		assert.Equal(t, "task-1", *transition.Next)
		assert.Equal(t, transition.With, ctx.CurrentInput)
	})
}

func TestTransitionNormalizer_Error(t *testing.T) {
	t.Run("Should normalize error transition", func(t *testing.T) {
		// Create mock template engine
		templateEngine := &mockTemplateEngine{
			ParseMapFunc: func(data map[string]any, _ map[string]any) (map[string]any, error) {
				// Simulate template processing
				if next, ok := data["next"].(string); ok && next == "error-{{.index}}" {
					data["next"] = "error-2"
				}
				return data, nil
			},
		}
		// Create normalizer
		normalizer := NewErrorTransitionNormalizer(templateEngine)
		// Create transition
		next := "error-{{.index}}"
		inputData := core.Input{
			"error": "test",
		}
		transition := &core.ErrorTransition{
			Next: &next,
			With: &inputData,
		}
		// Create context
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{
				"index": 2,
			},
		}
		// Normalize
		err := normalizer.Normalize(transition, ctx)
		require.NoError(t, err)
		// Verify normalization
		assert.Equal(t, "error-2", *transition.Next)
		assert.Equal(t, transition.With, ctx.CurrentInput)
	})
}

func TestTransitionNormalizer_Nil(t *testing.T) {
	t.Run("Should handle nil success transition", func(t *testing.T) {
		templateEngine := &mockTemplateEngine{}
		normalizer := NewSuccessTransitionNormalizer(templateEngine)
		ctx := &shared.NormalizationContext{}
		err := normalizer.Normalize(nil, ctx)
		assert.NoError(t, err)
		assert.Nil(t, ctx.CurrentInput)
	})

	t.Run("Should handle nil error transition", func(t *testing.T) {
		templateEngine := &mockTemplateEngine{}
		normalizer := NewErrorTransitionNormalizer(templateEngine)
		ctx := &shared.NormalizationContext{}
		err := normalizer.Normalize(nil, ctx)
		assert.NoError(t, err)
		assert.Nil(t, ctx.CurrentInput)
	})
}
