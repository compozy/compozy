package basic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
)

// MockTemplateEngine implements shared.TemplateEngine for testing
type MockTemplateEngine struct {
	mock.Mock
}

func (m *MockTemplateEngine) Process(template string, vars map[string]any) (string, error) {
	args := m.Called(template, vars)
	return args.String(0), args.Error(1)
}

func (m *MockTemplateEngine) ProcessMap(data map[string]any, vars map[string]any) (map[string]any, error) {
	args := m.Called(data, vars)
	return args.Get(0).(map[string]any), args.Error(1)
}

func (m *MockTemplateEngine) ProcessSlice(slice []any, vars map[string]any) ([]any, error) {
	args := m.Called(slice, vars)
	return args.Get(0).([]any), args.Error(1)
}

func (m *MockTemplateEngine) ProcessString(templateStr string, context map[string]any) (*shared.ProcessResult, error) {
	args := m.Called(templateStr, context)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*shared.ProcessResult), args.Error(1)
}

func (m *MockTemplateEngine) ParseMapWithFilter(
	data map[string]any,
	vars map[string]any,
	filter func(string) bool,
) (map[string]any, error) {
	args := m.Called(data, vars, filter)
	return args.Get(0).(map[string]any), args.Error(1)
}

func (m *MockTemplateEngine) ParseMap(data map[string]any, vars map[string]any) (map[string]any, error) {
	args := m.Called(data, vars)
	return args.Get(0).(map[string]any), args.Error(1)
}

func (m *MockTemplateEngine) ParseValue(value any, vars map[string]any) (any, error) {
	args := m.Called(value, vars)
	return args.Get(0), args.Error(1)
}

func TestNormalizer_Type(t *testing.T) {
	t.Run("Should return TaskTypeBasic", func(t *testing.T) {
		mockEngine := &MockTemplateEngine{}
		normalizer := NewNormalizer(mockEngine)

		assert.Equal(t, task.TaskTypeBasic, normalizer.Type())
	})
}

func TestNormalizer_Normalize(t *testing.T) {
	t.Run("Should normalize basic task config successfully", func(t *testing.T) {
		mockEngine := &MockTemplateEngine{}
		normalizer := NewNormalizer(mockEngine)

		// Setup test data
		originalWith := &core.Input{
			"key1": "value1",
		}
		config := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
				With: originalWith,
			},
			BasicTask: task.BasicTask{
				Action: "test-action",
			},
		}

		ctx := &shared.NormalizationContext{
			Variables: map[string]any{
				"workflow": map[string]any{"id": "test-workflow"},
				"task":     map[string]any{"id": "test-task"},
			},
		}

		// Setup mock expectations
		expectedParsedMap := map[string]any{
			"id":     "test-task",
			"type":   "basic",
			"action": "test-action-normalized",
			"with":   map[string]any{"key1": "value1"},
		}

		mockEngine.On("ParseMapWithFilter",
			mock.MatchedBy(func(data map[string]any) bool {
				return data["id"] == "test-task" && data["action"] == "test-action"
			}),
			ctx.Variables,
			mock.AnythingOfType("func(string) bool"),
		).Return(expectedParsedMap, nil)

		// Execute
		err := normalizer.Normalize(config, ctx)

		// Verify
		assert.NoError(t, err)
		assert.Equal(t, "test-action-normalized", config.Action)
		assert.Equal(t, originalWith, config.With) // With should be preserved
		mockEngine.AssertExpectations(t)
	})

	t.Run("Should handle nil config", func(t *testing.T) {
		mockEngine := &MockTemplateEngine{}
		normalizer := NewNormalizer(mockEngine)

		ctx := &shared.NormalizationContext{}

		err := normalizer.Normalize(nil, ctx)
		assert.NoError(t, err)
	})

	t.Run("Should reject wrong task type", func(t *testing.T) {
		mockEngine := &MockTemplateEngine{}
		normalizer := NewNormalizer(mockEngine)

		config := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeParallel, // Wrong type
			},
		}

		ctx := &shared.NormalizationContext{}

		err := normalizer.Normalize(config, ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "basic normalizer cannot handle task type")
	})

	t.Run("Should handle template processing error", func(t *testing.T) {
		mockEngine := &MockTemplateEngine{}
		normalizer := NewNormalizer(mockEngine)

		config := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
			},
		}

		ctx := &shared.NormalizationContext{
			Variables: map[string]any{},
		}

		// Mock template processing error
		mockEngine.On("ParseMapWithFilter",
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Return(map[string]any{}, assert.AnError)

		err := normalizer.Normalize(config, ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to normalize basic task config")
	})

	t.Run("Should preserve existing With values", func(t *testing.T) {
		mockEngine := &MockTemplateEngine{}
		normalizer := NewNormalizer(mockEngine)

		originalWith := &core.Input{
			"existing": "value",
			"key2":     "original",
		}

		config := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
				With: originalWith,
			},
		}

		ctx := &shared.NormalizationContext{
			Variables: map[string]any{},
		}

		// The ParseMapWithFilter should return a config map where templates are processed
		// but the "with" field should be excluded from template processing
		expectedParsedMap := map[string]any{
			"id":     "test-task",
			"type":   "basic",
			"action": "processed-action", // This would be template-processed
			// Note: "with" should be excluded from template processing by the filter
		}

		mockEngine.On("ParseMapWithFilter",
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Return(expectedParsedMap, nil)

		err := normalizer.Normalize(config, ctx)

		assert.NoError(t, err)
		// Since "with" is excluded from template processing, original values should remain
		assert.Equal(t, "value", (*config.With)["existing"])
		assert.Equal(t, "original", (*config.With)["key2"])
		assert.Equal(t, 2, len(*config.With)) // Only original fields should remain
	})
}

func TestNormalizer_FilterFunction(t *testing.T) {
	t.Run("Should exclude correct keys from template processing", func(t *testing.T) {
		mockEngine := &MockTemplateEngine{}
		normalizer := NewNormalizer(mockEngine)

		config := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
			},
		}

		ctx := &shared.NormalizationContext{
			Variables: map[string]any{},
		}

		var capturedFilter func(string) bool

		mockEngine.On("ParseMapWithFilter",
			mock.Anything,
			mock.Anything,
			mock.MatchedBy(func(filter func(string) bool) bool {
				capturedFilter = filter
				return true
			}),
		).Return(map[string]any{}, nil)

		_ = normalizer.Normalize(config, ctx)

		// Test the filter function
		assert.True(t, capturedFilter("agent"))
		assert.True(t, capturedFilter("tool"))
		assert.True(t, capturedFilter("outputs"))
		assert.True(t, capturedFilter("output"))
		assert.False(t, capturedFilter("id"))
		assert.False(t, capturedFilter("type"))
		assert.False(t, capturedFilter("action"))
		assert.False(t, capturedFilter("with"))
	})
}
