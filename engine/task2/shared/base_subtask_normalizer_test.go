package shared_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/pkg/tplengine"
)

// Mock normalizer factory for testing
type mockNormalizerFactory struct {
	mock.Mock
}

func (m *mockNormalizerFactory) CreateNormalizer(taskType string) (shared.TaskNormalizer, error) {
	args := m.Called(taskType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(shared.TaskNormalizer), args.Error(1)
}

// Mock task normalizer for testing
type mockTaskNormalizer struct {
	mock.Mock
}

func (m *mockTaskNormalizer) Type() string {
	args := m.Called()
	return args.String(0)
}

func (m *mockTaskNormalizer) Normalize(config any, ctx *shared.NormalizationContext) error {
	args := m.Called(config, ctx)
	return args.Error(0)
}

func TestBaseSubTaskNormalizer_Type(t *testing.T) {
	t.Run("Should return correct task type", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		contextBuilder, err := shared.NewContextBuilder()
		require.NoError(t, err)
		factory := &mockNormalizerFactory{}
		normalizer := shared.NewBaseSubTaskNormalizer(
			templateEngine,
			contextBuilder,
			factory,
			task.TaskTypeParallel,
			"parallel",
		)
		// Act
		taskType := normalizer.Type()
		// Assert
		assert.Equal(t, task.TaskTypeParallel, taskType)
	})
}

func TestBaseSubTaskNormalizer_Normalize_ErrorHandling(t *testing.T) {
	templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
	contextBuilder, err := shared.NewContextBuilder()
	require.NoError(t, err)

	t.Run("Should handle nil config gracefully", func(t *testing.T) {
		// Arrange
		factory := &mockNormalizerFactory{}
		normalizer := shared.NewBaseSubTaskNormalizer(
			templateEngine,
			contextBuilder,
			factory,
			task.TaskTypeParallel,
			"parallel",
		)
		ctx := &shared.NormalizationContext{}
		// Act
		err := normalizer.Normalize(nil, ctx)
		// Assert
		assert.NoError(t, err)
	})

	t.Run("Should return error for wrong task type", func(t *testing.T) {
		// Arrange
		factory := &mockNormalizerFactory{}
		normalizer := shared.NewBaseSubTaskNormalizer(
			templateEngine,
			contextBuilder,
			factory,
			task.TaskTypeParallel,
			"parallel",
		)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
			},
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act
		err := normalizer.Normalize(taskConfig, ctx)
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "parallel normalizer cannot handle task type: basic")
	})

	t.Run("Should handle template parsing errors in main config", func(t *testing.T) {
		// Arrange
		factory := &mockNormalizerFactory{}
		normalizer := shared.NewBaseSubTaskNormalizer(
			templateEngine,
			contextBuilder,
			factory,
			task.TaskTypeParallel,
			"parallel",
		)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "{{ .invalid.deeply.nested.nonexistent.field }}",
				Type: task.TaskTypeParallel,
			},
		}
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{
				"existing": "value",
			},
		}
		// Act
		err := normalizer.Normalize(taskConfig, ctx)
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to normalize parallel task config")
	})

	t.Run("Should handle config serialization errors", func(t *testing.T) {
		// Arrange
		factory := &mockNormalizerFactory{}
		normalizer := shared.NewBaseSubTaskNormalizer(
			templateEngine,
			contextBuilder,
			factory,
			task.TaskTypeParallel,
			"parallel",
		)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeParallel,
			},
		}
		// Inject problematic data for serialization
		unsafeField := func() {}
		taskConfig.With = &core.Input{"function": unsafeField}

		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act
		err := normalizer.Normalize(taskConfig, ctx)
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to convert task config to map")
	})

	t.Run("Should handle sub-task normalization errors", func(t *testing.T) {
		// Arrange
		mockFactory := &mockNormalizerFactory{}
		mockSubNormalizer := &mockTaskNormalizer{}
		mockFactory.On("CreateNormalizer", "basic").Return(mockSubNormalizer, nil)
		mockSubNormalizer.On("Normalize", mock.Anything, mock.Anything).
			Return(errors.New("sub-task normalization failed"))

		normalizer := shared.NewBaseSubTaskNormalizer(
			templateEngine,
			contextBuilder,
			mockFactory,
			task.TaskTypeParallel,
			"parallel",
		)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "parallel-task",
				Type: task.TaskTypeParallel,
			},
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:   "sub-task-1",
						Type: task.TaskTypeBasic,
					},
				},
			},
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act
		err := normalizer.Normalize(taskConfig, ctx)
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to normalize parallel sub-tasks")
		assert.Contains(t, err.Error(), "failed to normalize sub-task sub-task-1")
		mockFactory.AssertExpectations(t)
		mockSubNormalizer.AssertExpectations(t)
	})

	t.Run("Should handle normalizer factory errors", func(t *testing.T) {
		// Arrange
		mockFactory := &mockNormalizerFactory{}
		mockFactory.On("CreateNormalizer", "basic").Return(nil, errors.New("normalizer creation failed"))

		normalizer := shared.NewBaseSubTaskNormalizer(
			templateEngine,
			contextBuilder,
			mockFactory,
			task.TaskTypeParallel,
			"parallel",
		)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "parallel-task",
				Type: task.TaskTypeParallel,
			},
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:   "sub-task-1",
						Type: task.TaskTypeBasic,
					},
				},
			},
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act
		err := normalizer.Normalize(taskConfig, ctx)
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create normalizer for task type basic")
		mockFactory.AssertExpectations(t)
	})

	t.Run("Should handle deeply nested context structures", func(t *testing.T) {
		// Arrange - Use a context builder with deeply nested data
		contextBuilder, _ := shared.NewContextBuilder()
		// We'll test with deeply nested but non-circular context data

		mockFactory := &mockNormalizerFactory{}
		mockSubNormalizer := &mockTaskNormalizer{}
		mockFactory.On("CreateNormalizer", "basic").Return(mockSubNormalizer, nil)
		mockSubNormalizer.On("Normalize", mock.Anything, mock.Anything).Return(nil)
		// The sub-normalizer should be called during normal operation

		normalizer := shared.NewBaseSubTaskNormalizer(
			templateEngine,
			contextBuilder,
			mockFactory,
			task.TaskTypeParallel,
			"parallel",
		)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "parallel-task",
				Type: task.TaskTypeParallel,
			},
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:   "sub-task-1",
						Type: task.TaskTypeBasic,
					},
				},
			},
		}
		// Create context with deeply nested but non-circular structure
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{
				"deeply": map[string]any{
					"nested": map[string]any{
						"structure": map[string]any{
							"value": "test",
						},
					},
				},
			},
		}

		// Act
		err := normalizer.Normalize(taskConfig, ctx)
		// Assert - Should succeed with deeply nested but non-circular data
		assert.NoError(t, err)
		mockFactory.AssertExpectations(t)
		mockSubNormalizer.AssertExpectations(t)
	})
}

func TestBaseSubTaskNormalizer_BoundaryConditions(t *testing.T) {
	templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
	contextBuilder, err := shared.NewContextBuilder()
	require.NoError(t, err)

	t.Run("Should handle nil template engine", func(t *testing.T) {
		// Arrange
		factory := &mockNormalizerFactory{}
		normalizer := shared.NewBaseSubTaskNormalizer(
			nil,
			contextBuilder,
			factory,
			task.TaskTypeParallel,
			"parallel",
		)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeParallel,
			},
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act & Assert
		// Should panic due to nil template engine
		assert.Panics(t, func() {
			normalizer.Normalize(taskConfig, ctx)
		})
	})

	t.Run("Should handle nil context builder gracefully", func(t *testing.T) {
		// Arrange
		factory := &mockNormalizerFactory{}
		normalizer := shared.NewBaseSubTaskNormalizer(
			templateEngine,
			nil,
			factory,
			task.TaskTypeParallel,
			"parallel",
		)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeParallel,
			},
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act
		err := normalizer.Normalize(taskConfig, ctx)
		// Assert - Should succeed since BaseSubTaskNormalizer handles nil gracefully
		assert.NoError(t, err)
	})

	t.Run("Should handle nil normalizer factory", func(t *testing.T) {
		// Arrange
		normalizer := shared.NewBaseSubTaskNormalizer(
			templateEngine,
			contextBuilder,
			nil,
			task.TaskTypeParallel,
			"parallel",
		)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeParallel,
			},
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:   "sub-task-1",
						Type: task.TaskTypeBasic,
					},
				},
			},
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act & Assert
		// Should panic due to nil factory when trying to normalize sub-tasks
		assert.Panics(t, func() {
			normalizer.Normalize(taskConfig, ctx)
		})
	})

	t.Run("Should handle empty sub-tasks array", func(t *testing.T) {
		// Arrange
		factory := &mockNormalizerFactory{}
		normalizer := shared.NewBaseSubTaskNormalizer(
			templateEngine,
			contextBuilder,
			factory,
			task.TaskTypeParallel,
			"parallel",
		)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeParallel,
			},
			Tasks: []task.Config{}, // Empty array
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act
		err := normalizer.Normalize(taskConfig, ctx)
		// Assert
		assert.NoError(t, err)
	})

	t.Run("Should handle task reference (Task field) normalization", func(t *testing.T) {
		// Arrange
		mockFactory := &mockNormalizerFactory{}
		mockSubNormalizer := &mockTaskNormalizer{}
		mockFactory.On("CreateNormalizer", "basic").Return(mockSubNormalizer, nil)
		mockSubNormalizer.On("Normalize", mock.Anything, mock.Anything).Return(nil)

		normalizer := shared.NewBaseSubTaskNormalizer(
			templateEngine,
			contextBuilder,
			mockFactory,
			task.TaskTypeParallel,
			"parallel",
		)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeParallel,
			},
			Task: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   "ref-task",
					Type: task.TaskTypeBasic,
				},
			},
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act
		err := normalizer.Normalize(taskConfig, ctx)
		// Assert
		assert.NoError(t, err)
		mockFactory.AssertExpectations(t)
		mockSubNormalizer.AssertExpectations(t)
	})

	t.Run("Should handle both Tasks array and Task reference", func(t *testing.T) {
		// Arrange
		mockFactory := &mockNormalizerFactory{}
		mockSubNormalizer := &mockTaskNormalizer{}
		mockFactory.On("CreateNormalizer", "basic").Return(mockSubNormalizer, nil)
		// Expect 3 calls: 2 for Tasks array + 1 for Task reference
		mockSubNormalizer.On("Normalize", mock.Anything, mock.Anything).Return(nil).Times(3)

		normalizer := shared.NewBaseSubTaskNormalizer(
			templateEngine,
			contextBuilder,
			mockFactory,
			task.TaskTypeParallel,
			"parallel",
		)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeParallel,
			},
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:   "sub-task-1",
						Type: task.TaskTypeBasic,
					},
				},
				{
					BaseConfig: task.BaseConfig{
						ID:   "sub-task-2",
						Type: task.TaskTypeBasic,
					},
				},
			},
			Task: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   "ref-task",
					Type: task.TaskTypeBasic,
				},
			},
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act
		err := normalizer.Normalize(taskConfig, ctx)
		// Assert
		assert.NoError(t, err)
		mockFactory.AssertExpectations(t)
		mockSubNormalizer.AssertExpectations(t)
	})

	t.Run("Should handle deeply nested sub-tasks", func(t *testing.T) {
		// Arrange
		mockFactory := &mockNormalizerFactory{}
		mockSubNormalizer := &mockTaskNormalizer{}
		// BaseSubTaskNormalizer only normalizes direct children, not nested grandchildren
		// So it will only call CreateNormalizer for the immediate parallel child
		mockFactory.On("CreateNormalizer", "parallel").Return(mockSubNormalizer, nil)
		mockSubNormalizer.On("Normalize", mock.Anything, mock.Anything).Return(nil)

		normalizer := shared.NewBaseSubTaskNormalizer(
			templateEngine,
			contextBuilder,
			mockFactory,
			task.TaskTypeParallel,
			"parallel",
		)

		// Create nested structure: parallel -> parallel -> basic
		// The root normalizer will only process the immediate parallel child
		// The nested basic task will be handled by the child parallel normalizer
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "root-parallel",
				Type: task.TaskTypeParallel,
			},
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:   "nested-parallel",
						Type: task.TaskTypeParallel,
					},
					Tasks: []task.Config{
						{
							BaseConfig: task.BaseConfig{
								ID:   "leaf-basic",
								Type: task.TaskTypeBasic,
							},
						},
					},
				},
			},
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act
		err := normalizer.Normalize(taskConfig, ctx)
		// Assert
		assert.NoError(t, err)
		mockFactory.AssertExpectations(t)
		mockSubNormalizer.AssertExpectations(t)
	})

	t.Run("Should preserve sub-task configuration after normalization", func(t *testing.T) {
		// Arrange
		mockFactory := &mockNormalizerFactory{}
		mockSubNormalizer := &mockTaskNormalizer{}
		mockFactory.On("CreateNormalizer", "basic").Return(mockSubNormalizer, nil)
		mockSubNormalizer.On("Normalize", mock.Anything, mock.Anything).Return(nil)

		normalizer := shared.NewBaseSubTaskNormalizer(
			templateEngine,
			contextBuilder,
			mockFactory,
			task.TaskTypeParallel,
			"parallel",
		)

		originalSubTaskID := "original-sub-task"
		originalSubTaskWith := &core.Input{"param": "value"}

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "parallel-task",
				Type: task.TaskTypeParallel,
			},
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:   originalSubTaskID,
						Type: task.TaskTypeBasic,
						With: originalSubTaskWith,
					},
				},
			},
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act
		err := normalizer.Normalize(taskConfig, ctx)
		// Assert
		assert.NoError(t, err)
		assert.Equal(t, originalSubTaskID, taskConfig.Tasks[0].ID)
		assert.Equal(t, originalSubTaskWith, taskConfig.Tasks[0].With)
		mockFactory.AssertExpectations(t)
		mockSubNormalizer.AssertExpectations(t)
	})
}
