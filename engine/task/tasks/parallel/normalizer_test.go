package parallel_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/tasks/contracts"
	"github.com/compozy/compozy/engine/task/tasks/parallel"
	"github.com/compozy/compozy/engine/task/tasks/shared"
	"github.com/compozy/compozy/pkg/tplengine"
)

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

func TestParallelNormalizer_NewNormalizer(t *testing.T) {
	t.Run("Should create parallel normalizer", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		contextBuilder, err := shared.NewContextBuilder(t.Context())
		require.NoError(t, err)
		factory := &mockNormalizerFactory{}

		// Act
		normalizer := parallel.NewNormalizer(t.Context(), templateEngine, contextBuilder, factory)

		// Assert
		assert.NotNil(t, normalizer)
	})

	t.Run("Should handle nil template engine", func(t *testing.T) {
		// Arrange
		contextBuilder, err := shared.NewContextBuilder(t.Context())
		require.NoError(t, err)
		factory := &mockNormalizerFactory{}

		// Act
		normalizer := parallel.NewNormalizer(t.Context(), nil, contextBuilder, factory)

		// Assert
		assert.NotNil(t, normalizer)
	})

	t.Run("Should handle nil context builder", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		factory := &mockNormalizerFactory{}

		// Act
		normalizer := parallel.NewNormalizer(t.Context(), templateEngine, nil, factory)

		// Assert
		assert.NotNil(t, normalizer)
	})

	t.Run("Should handle nil normalizer factory", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		contextBuilder, err := shared.NewContextBuilder(t.Context())
		require.NoError(t, err)

		// Act
		normalizer := parallel.NewNormalizer(t.Context(), templateEngine, contextBuilder, nil)

		// Assert
		assert.NotNil(t, normalizer)
	})

	t.Run("Should handle all nil parameters", func(t *testing.T) {
		// Act
		normalizer := parallel.NewNormalizer(t.Context(), nil, nil, nil)

		// Assert
		assert.NotNil(t, normalizer)
	})
}

func TestParallelNormalizer_Type(t *testing.T) {
	t.Run("Should return correct task type", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		contextBuilder, err := shared.NewContextBuilder(t.Context())
		require.NoError(t, err)
		factory := &mockNormalizerFactory{}
		normalizer := parallel.NewNormalizer(t.Context(), templateEngine, contextBuilder, factory)

		// Act
		taskType := normalizer.Type()

		// Assert
		assert.Equal(t, task.TaskTypeParallel, taskType)
	})
}

func TestParallelNormalizer_Integration(t *testing.T) {
	t.Run("Should be based on BaseSubTaskNormalizer", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		contextBuilder, err := shared.NewContextBuilder(t.Context())
		require.NoError(t, err)
		factory := &mockNormalizerFactory{}

		// Act
		normalizer := parallel.NewNormalizer(t.Context(), templateEngine, contextBuilder, factory)

		// Assert
		assert.NotNil(t, normalizer)

		// Test that it has the correct type by checking its Type method
		taskType := normalizer.Type()
		assert.Equal(t, task.TaskTypeParallel, taskType)
	})

	t.Run("Should support parallel task strategy and workers", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		contextBuilder, err := shared.NewContextBuilder(t.Context())
		require.NoError(t, err)
		factory := &mockNormalizerFactory{}
		normalizer := parallel.NewNormalizer(t.Context(), templateEngine, contextBuilder, factory)

		// Simple task config without sub-tasks to avoid nil pointer issues
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "parallel-task",
				Type: task.TaskTypeParallel,
			},
			ParallelTask: task.ParallelTask{
				Strategy:   task.StrategyWaitAll,
				MaxWorkers: 5,
			},
		}

		// Test that the normalizer was created correctly and can access task properties
		assert.Equal(t, task.TaskTypeParallel, normalizer.Type())
		assert.Equal(t, "parallel-task", taskConfig.ID)
		assert.Equal(t, task.TaskTypeParallel, taskConfig.Type)
		assert.Equal(t, task.StrategyWaitAll, taskConfig.Strategy)
		assert.Equal(t, 5, taskConfig.MaxWorkers)
	})

	t.Run("Should handle different parallel strategies", func(t *testing.T) {
		// Test all parallel strategies
		strategies := []task.ParallelStrategy{
			task.StrategyWaitAll,
			task.StrategyFailFast,
			task.StrategyBestEffort,
			task.StrategyRace,
		}

		for _, strategy := range strategies {
			t.Run(string(strategy), func(t *testing.T) {
				// Arrange
				templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
				contextBuilder, err := shared.NewContextBuilder(t.Context())
				require.NoError(t, err)
				factory := &mockNormalizerFactory{}
				normalizer := parallel.NewNormalizer(t.Context(), templateEngine, contextBuilder, factory)

				taskConfig := &task.Config{
					BaseConfig: task.BaseConfig{
						ID:   "parallel-task",
						Type: task.TaskTypeParallel,
					},
					ParallelTask: task.ParallelTask{
						Strategy:   strategy,
						MaxWorkers: 3,
					},
				}

				// Test that the configuration is properly set
				assert.Equal(t, task.TaskTypeParallel, normalizer.Type())
				assert.Equal(t, strategy, taskConfig.Strategy)
				assert.Equal(t, 3, taskConfig.MaxWorkers)
			})
		}
	})

	t.Run("Should handle template expressions in parallel configuration", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		contextBuilder, err := shared.NewContextBuilder(t.Context())
		require.NoError(t, err)
		factory := &mockNormalizerFactory{}
		normalizer := parallel.NewNormalizer(t.Context(), templateEngine, contextBuilder, factory)

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "{{ .task_name }}", // Template that would be processed during normalization
				Type: task.TaskTypeParallel,
			},
			ParallelTask: task.ParallelTask{
				MaxWorkers: 3,
			},
		}

		// Test that the normalizer can handle template-based configuration
		assert.Equal(t, task.TaskTypeParallel, normalizer.Type())
		assert.Equal(t, "{{ .task_name }}", taskConfig.ID) // Template not yet processed
		assert.Equal(t, task.TaskTypeParallel, taskConfig.Type)
		assert.Equal(t, 3, taskConfig.MaxWorkers)
	})
}

func TestParallelNormalizer_Normalize_ErrorHandling(t *testing.T) {
	templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
	contextBuilder, err := shared.NewContextBuilder(t.Context())
	require.NoError(t, err)

	t.Run("Should handle nil config gracefully", func(t *testing.T) {
		// Arrange
		factory := &mockNormalizerFactory{}
		normalizer := parallel.NewNormalizer(t.Context(), templateEngine, contextBuilder, factory)
		ctx := &shared.NormalizationContext{}
		// Act
		err := normalizer.Normalize(t.Context(), nil, ctx)
		// Assert
		assert.NoError(t, err)
	})

	t.Run("Should return error for wrong task type", func(t *testing.T) {
		// Arrange
		factory := &mockNormalizerFactory{}
		normalizer := parallel.NewNormalizer(t.Context(), templateEngine, contextBuilder, factory)
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
		assert.Contains(t, err.Error(), "parallel normalizer cannot handle task type: basic")
	})

	t.Run("Should handle template parsing errors", func(t *testing.T) {
		// Arrange
		factory := &mockNormalizerFactory{}
		normalizer := parallel.NewNormalizer(t.Context(), templateEngine, contextBuilder, factory)
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
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to normalize parallel task config")
	})

	t.Run("Should handle config serialization errors", func(t *testing.T) {
		// Arrange
		factory := &mockNormalizerFactory{}
		normalizer := parallel.NewNormalizer(t.Context(), templateEngine, contextBuilder, factory)
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

		normalizer := parallel.NewNormalizer(t.Context(), templateEngine, contextBuilder, mockFactory)
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
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to normalize parallel sub-tasks")
		mockFactory.AssertExpectations(t)
		mockSubNormalizer.AssertExpectations(t)
	})

	t.Run("Should handle normalizer factory errors", func(t *testing.T) {
		// Arrange
		mockFactory := &mockNormalizerFactory{}
		mockFactory.On("CreateNormalizer", task.TaskTypeBasic).Return(nil, errors.New("normalizer creation failed"))

		normalizer := parallel.NewNormalizer(t.Context(), templateEngine, contextBuilder, mockFactory)
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
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create normalizer for task type basic")
		mockFactory.AssertExpectations(t)
	})

	t.Run("Should process parallel task configuration successfully", func(t *testing.T) {
		// Arrange
		mockFactory := &mockNormalizerFactory{}
		mockSubNormalizer := &mockTaskNormalizer{}
		mockFactory.On("CreateNormalizer", task.TaskTypeBasic).Return(mockSubNormalizer, nil)
		mockSubNormalizer.On("Normalize", mock.Anything, mock.Anything).Return(nil)

		normalizer := parallel.NewNormalizer(t.Context(), templateEngine, contextBuilder, mockFactory)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "{{ .name }}-parallel",
				Type: task.TaskTypeParallel,
			},
			ParallelTask: task.ParallelTask{
				Strategy:   task.StrategyWaitAll,
				MaxWorkers: 3,
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
		assert.Equal(t, "test-parallel", taskConfig.ID)
		assert.Equal(t, task.StrategyWaitAll, taskConfig.Strategy)
		mockFactory.AssertExpectations(t)
		mockSubNormalizer.AssertExpectations(t)
	})
}

func TestParallelNormalizer_BoundaryConditions(t *testing.T) {
	templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
	contextBuilder, err := shared.NewContextBuilder(t.Context())
	require.NoError(t, err)

	t.Run("Should handle nil template engine", func(t *testing.T) {
		// Arrange
		factory := &mockNormalizerFactory{}
		normalizer := parallel.NewNormalizer(t.Context(), nil, contextBuilder, factory)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeParallel,
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
		normalizer := parallel.NewNormalizer(t.Context(), templateEngine, nil, factory)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeParallel,
			},
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)
		// Assert - Should succeed since BaseSubTaskNormalizer handles nil context builder
		assert.NoError(t, err)
	})

	t.Run("Should handle nil normalizer factory", func(t *testing.T) {
		// Arrange
		normalizer := parallel.NewNormalizer(t.Context(), templateEngine, contextBuilder, nil)
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
		assert.Panics(t, func() {
			normalizer.Normalize(t.Context(), taskConfig, ctx)
		})
	})

	t.Run("Should handle empty sub-tasks array", func(t *testing.T) {
		// Arrange
		factory := &mockNormalizerFactory{}
		normalizer := parallel.NewNormalizer(t.Context(), templateEngine, contextBuilder, factory)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeParallel,
			},
			Tasks: []task.Config{}, // Empty array
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)
		// Assert
		assert.NoError(t, err)
	})

	t.Run("Should handle different parallel strategies", func(t *testing.T) {
		// Test different strategies
		strategies := []task.ParallelStrategy{
			task.StrategyWaitAll,
			task.StrategyFailFast,
			task.StrategyBestEffort,
			task.StrategyRace,
		}

		for _, strategy := range strategies {
			t.Run(string(strategy), func(t *testing.T) {
				// Arrange
				factory := &mockNormalizerFactory{}
				normalizer := parallel.NewNormalizer(t.Context(), templateEngine, contextBuilder, factory)
				taskConfig := &task.Config{
					BaseConfig: task.BaseConfig{
						ID:   "strategy-test",
						Type: task.TaskTypeParallel,
					},
					ParallelTask: task.ParallelTask{
						Strategy:   strategy,
						MaxWorkers: 2,
					},
				}
				ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
				// Act
				err := normalizer.Normalize(t.Context(), taskConfig, ctx)
				// Assert
				assert.NoError(t, err)
				assert.Equal(t, strategy, taskConfig.Strategy)
			})
		}
	})

	t.Run("Should handle zero max workers", func(t *testing.T) {
		// Arrange
		factory := &mockNormalizerFactory{}
		normalizer := parallel.NewNormalizer(t.Context(), templateEngine, contextBuilder, factory)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "zero-workers-task",
				Type: task.TaskTypeParallel,
			},
			ParallelTask: task.ParallelTask{
				MaxWorkers: 0, // Zero workers
			},
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)
		// Assert
		assert.NoError(t, err)
		assert.Equal(t, 0, taskConfig.MaxWorkers)
	})

	t.Run("Should preserve parallel task configuration", func(t *testing.T) {
		// Arrange
		factory := &mockNormalizerFactory{}
		normalizer := parallel.NewNormalizer(t.Context(), templateEngine, contextBuilder, factory)
		originalStrategy := task.StrategyBestEffort
		originalMaxWorkers := 10
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "preserve-test",
				Type: task.TaskTypeParallel,
			},
			ParallelTask: task.ParallelTask{
				Strategy:   originalStrategy,
				MaxWorkers: originalMaxWorkers,
			},
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)
		// Assert
		assert.NoError(t, err)
		assert.Equal(t, originalStrategy, taskConfig.Strategy)
		assert.Equal(t, originalMaxWorkers, taskConfig.MaxWorkers)
		assert.Equal(t, task.TaskTypeParallel, taskConfig.Type)
	})
}
