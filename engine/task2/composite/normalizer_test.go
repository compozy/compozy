package composite_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/composite"
	"github.com/compozy/compozy/engine/task2/contracts"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/pkg/tplengine"
)

func TestCompositeNormalizer_NewNormalizer(t *testing.T) {
	t.Run("Should create composite normalizer", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		contextBuilder, err := shared.NewContextBuilder(t.Context())
		require.NoError(t, err)

		// Create a mock normalizer factory
		normalizerFactory := &mockNormalizerFactory{}

		// Act
		normalizer := composite.NewNormalizer(t.Context(), templateEngine, contextBuilder, normalizerFactory)

		// Assert
		assert.NotNil(t, normalizer)
	})

	t.Run("Should handle nil template engine", func(t *testing.T) {
		// Arrange
		contextBuilder, err := shared.NewContextBuilder(t.Context())
		require.NoError(t, err)

		// Create a mock normalizer factory
		normalizerFactory := &mockNormalizerFactory{}

		// Act
		normalizer := composite.NewNormalizer(t.Context(), nil, contextBuilder, normalizerFactory)

		// Assert
		assert.NotNil(t, normalizer)
	})

	t.Run("Should handle nil context builder", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)

		// Create a mock normalizer factory
		normalizerFactory := &mockNormalizerFactory{}

		// Act
		normalizer := composite.NewNormalizer(t.Context(), templateEngine, nil, normalizerFactory)

		// Assert
		assert.NotNil(t, normalizer)
	})

	t.Run("Should handle nil normalizer factory", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		contextBuilder, err := shared.NewContextBuilder(t.Context())
		require.NoError(t, err)

		// Act
		normalizer := composite.NewNormalizer(t.Context(), templateEngine, contextBuilder, nil)

		// Assert
		assert.NotNil(t, normalizer)
	})

	t.Run("Should handle all nil parameters", func(t *testing.T) {
		// Act
		normalizer := composite.NewNormalizer(t.Context(), nil, nil, nil)

		// Assert
		assert.NotNil(t, normalizer)
	})
}

// Mock normalizer factory for testing
type mockNormalizerFactory struct {
	mock.Mock
}

func (m *mockNormalizerFactory) CreateNormalizer(
	_ context.Context,
	taskType task.Type,
) (contracts.TaskNormalizer, error) {
	args := m.Called(taskType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(contracts.TaskNormalizer), args.Error(1)
}

// Mock task normalizer for testing
type mockTaskNormalizer struct {
	mock.Mock
}

func (m *mockTaskNormalizer) Type() task.Type {
	args := m.Called()
	return task.Type(args.String(0))
}

func (m *mockTaskNormalizer) Normalize(
	_ context.Context,
	config *task.Config,
	ctx contracts.NormalizationContext,
) error {
	args := m.Called(config, ctx)
	return args.Error(0)
}

func TestCompositeNormalizer_Type(t *testing.T) {
	t.Run("Should return correct task type", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		contextBuilder, err := shared.NewContextBuilder(t.Context())
		require.NoError(t, err)
		factory := &mockNormalizerFactory{}
		normalizer := composite.NewNormalizer(t.Context(), templateEngine, contextBuilder, factory)
		// Act
		taskType := normalizer.Type()
		// Assert
		assert.Equal(t, task.TaskTypeComposite, taskType)
	})
}

func TestCompositeNormalizer_Normalize_ErrorHandling(t *testing.T) {
	templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
	contextBuilder, err := shared.NewContextBuilder(t.Context())
	require.NoError(t, err)

	t.Run("Should handle nil config gracefully", func(t *testing.T) {
		// Arrange
		factory := &mockNormalizerFactory{}
		normalizer := composite.NewNormalizer(t.Context(), templateEngine, contextBuilder, factory)
		ctx := &shared.NormalizationContext{}
		// Act
		err := normalizer.Normalize(t.Context(), nil, ctx)
		// Assert
		assert.NoError(t, err)
	})

	t.Run("Should return error for wrong task type", func(t *testing.T) {
		// Arrange
		factory := &mockNormalizerFactory{}
		normalizer := composite.NewNormalizer(t.Context(), templateEngine, contextBuilder, factory)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
			},
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "composite normalizer cannot handle task type: basic")
	})

	t.Run("Should handle template parsing errors in main config", func(t *testing.T) {
		// Arrange
		factory := &mockNormalizerFactory{}
		normalizer := composite.NewNormalizer(t.Context(), templateEngine, contextBuilder, factory)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "{{ .invalid.deeply.nested.nonexistent.field }}",
				Type: task.TaskTypeComposite,
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
		assert.Contains(t, err.Error(), "failed to normalize composite task config")
	})

	t.Run("Should handle config serialization errors", func(t *testing.T) {
		// Arrange
		factory := &mockNormalizerFactory{}
		normalizer := composite.NewNormalizer(t.Context(), templateEngine, contextBuilder, factory)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeComposite,
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

	t.Run("Should handle sub-task normalization errors", func(t *testing.T) {
		// Arrange
		mockFactory := &mockNormalizerFactory{}
		mockSubNormalizer := &mockTaskNormalizer{}
		mockFactory.On("CreateNormalizer", task.TaskTypeBasic).Return(mockSubNormalizer, nil)
		mockSubNormalizer.On("Normalize", mock.Anything, mock.Anything).
			Return(errors.New("sub-task normalization failed"))

		normalizer := composite.NewNormalizer(t.Context(), templateEngine, contextBuilder, mockFactory)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "composite-task",
				Type: task.TaskTypeComposite,
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
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to normalize composite sub-tasks")
		mockFactory.AssertExpectations(t)
		mockSubNormalizer.AssertExpectations(t)
	})

	t.Run("Should handle normalizer factory errors", func(t *testing.T) {
		// Arrange
		mockFactory := &mockNormalizerFactory{}
		mockFactory.On("CreateNormalizer", task.TaskTypeBasic).Return(nil, errors.New("normalizer creation failed"))

		normalizer := composite.NewNormalizer(t.Context(), templateEngine, contextBuilder, mockFactory)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "composite-task",
				Type: task.TaskTypeComposite,
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
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create normalizer for task type basic")
		mockFactory.AssertExpectations(t)
	})

	t.Run("Should process composite task configuration successfully", func(t *testing.T) {
		// Arrange
		mockFactory := &mockNormalizerFactory{}
		mockSubNormalizer := &mockTaskNormalizer{}
		mockFactory.On("CreateNormalizer", task.TaskTypeBasic).Return(mockSubNormalizer, nil)
		mockSubNormalizer.On("Normalize", mock.Anything, mock.Anything).Return(nil)

		normalizer := composite.NewNormalizer(t.Context(), templateEngine, contextBuilder, mockFactory)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "{{ .name }}-composite",
				Type: task.TaskTypeComposite,
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
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{
				"name": "test",
			},
		}
		// Act
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)
		// Assert
		assert.NoError(t, err)
		assert.Equal(t, "test-composite", taskConfig.ID)
		mockFactory.AssertExpectations(t)
		mockSubNormalizer.AssertExpectations(t)
	})
}

func TestCompositeNormalizer_BoundaryConditions(t *testing.T) {
	templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
	contextBuilder, err := shared.NewContextBuilder(t.Context())
	require.NoError(t, err)

	t.Run("Should handle nil template engine", func(t *testing.T) {
		// Arrange
		factory := &mockNormalizerFactory{}
		normalizer := composite.NewNormalizer(t.Context(), nil, contextBuilder, factory)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeComposite,
			},
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)
		// Assert - Should return error due to nil template engine
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "template engine is required for normalization")
	})

	t.Run("Should handle nil context builder gracefully", func(t *testing.T) {
		// Arrange
		factory := &mockNormalizerFactory{}
		normalizer := composite.NewNormalizer(t.Context(), templateEngine, nil, factory)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeComposite,
			},
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)
		// Assert
		assert.NoError(t, err)
	})

	t.Run("Should handle nil normalizer factory", func(t *testing.T) {
		// Arrange
		normalizer := composite.NewNormalizer(t.Context(), templateEngine, contextBuilder, nil)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeComposite,
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
		assert.Panics(t, func() {
			normalizer.Normalize(t.Context(), taskConfig, ctx)
		})
	})

	t.Run("Should handle empty sub-tasks array", func(t *testing.T) {
		// Arrange
		factory := &mockNormalizerFactory{}
		normalizer := composite.NewNormalizer(t.Context(), templateEngine, contextBuilder, factory)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeComposite,
			},
			Tasks: []task.Config{}, // Empty array
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)
		// Assert
		assert.NoError(t, err)
	})

	t.Run("Should handle task reference (Task field) normalization", func(t *testing.T) {
		// Arrange
		mockFactory := &mockNormalizerFactory{}
		mockSubNormalizer := &mockTaskNormalizer{}
		mockFactory.On("CreateNormalizer", task.TaskTypeBasic).Return(mockSubNormalizer, nil)
		mockSubNormalizer.On("Normalize", mock.Anything, mock.Anything).Return(nil)

		normalizer := composite.NewNormalizer(t.Context(), templateEngine, contextBuilder, mockFactory)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeComposite,
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
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)
		// Assert
		assert.NoError(t, err)
		mockFactory.AssertExpectations(t)
		mockSubNormalizer.AssertExpectations(t)
	})

	t.Run("Should handle both Tasks array and Task reference", func(t *testing.T) {
		// Arrange
		mockFactory := &mockNormalizerFactory{}
		mockSubNormalizer := &mockTaskNormalizer{}
		mockFactory.On("CreateNormalizer", task.TaskTypeBasic).Return(mockSubNormalizer, nil)
		// Expect 3 calls: 2 for Tasks array + 1 for Task reference
		mockSubNormalizer.On("Normalize", mock.Anything, mock.Anything).Return(nil).Times(3)

		normalizer := composite.NewNormalizer(t.Context(), templateEngine, contextBuilder, mockFactory)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeComposite,
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
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)
		// Assert
		assert.NoError(t, err)
		mockFactory.AssertExpectations(t)
		mockSubNormalizer.AssertExpectations(t)
	})

	t.Run("Should preserve sub-task configuration after normalization", func(t *testing.T) {
		// Arrange
		mockFactory := &mockNormalizerFactory{}
		mockSubNormalizer := &mockTaskNormalizer{}
		mockFactory.On("CreateNormalizer", task.TaskTypeBasic).Return(mockSubNormalizer, nil)
		mockSubNormalizer.On("Normalize", mock.Anything, mock.Anything).Return(nil)

		normalizer := composite.NewNormalizer(t.Context(), templateEngine, contextBuilder, mockFactory)

		originalSubTaskID := "original-sub-task"
		originalSubTaskWith := &core.Input{"param": "value"}

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "composite-task",
				Type: task.TaskTypeComposite,
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
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)
		// Assert
		assert.NoError(t, err)
		assert.Equal(t, originalSubTaskID, taskConfig.Tasks[0].ID)
		assert.Equal(t, originalSubTaskWith, taskConfig.Tasks[0].With)
		mockFactory.AssertExpectations(t)
		mockSubNormalizer.AssertExpectations(t)
	})
}
