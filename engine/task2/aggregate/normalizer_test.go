package aggregate_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/aggregate"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/pkg/tplengine"
)

func TestAggregateNormalizer_NewNormalizer(t *testing.T) {
	t.Run("Should create aggregate normalizer", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		contextBuilder, err := shared.NewContextBuilder()
		require.NoError(t, err)

		// Act
		normalizer := aggregate.NewNormalizer(templateEngine, contextBuilder)

		// Assert
		assert.NotNil(t, normalizer)
	})

	t.Run("Should handle nil template engine", func(t *testing.T) {
		// Arrange
		contextBuilder, err := shared.NewContextBuilder()
		require.NoError(t, err)

		// Act
		normalizer := aggregate.NewNormalizer(nil, contextBuilder)

		// Assert
		assert.NotNil(t, normalizer)
	})

	t.Run("Should handle nil context builder", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)

		// Act
		normalizer := aggregate.NewNormalizer(templateEngine, nil)

		// Assert
		assert.NotNil(t, normalizer)
	})

	t.Run("Should handle both nil parameters", func(t *testing.T) {
		// Act
		normalizer := aggregate.NewNormalizer(nil, nil)

		// Assert
		assert.NotNil(t, normalizer)
	})
}

func TestAggregateNormalizer_Type(t *testing.T) {
	t.Run("Should return correct task type", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		contextBuilder, err := shared.NewContextBuilder()
		require.NoError(t, err)
		normalizer := aggregate.NewNormalizer(templateEngine, contextBuilder)

		// Act
		taskType := normalizer.Type()

		// Assert
		assert.Equal(t, task.TaskTypeAggregate, taskType)
	})
}

func TestAggregateNormalizer_Integration(t *testing.T) {
	t.Run("Should be based on BaseNormalizer", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		contextBuilder, err := shared.NewContextBuilder()
		require.NoError(t, err)
		normalizer := aggregate.NewNormalizer(templateEngine, contextBuilder)

		// Assert
		require.NotNil(t, normalizer)

		// Aggregate normalizer should inherit all BaseNormalizer functionality
		// The BaseNormalizer methods will be tested separately in shared/base_normalizer_test.go
		assert.Equal(t, task.TaskTypeAggregate, normalizer.Type())
	})
}

func TestAggregateNormalizer_Normalize_ErrorHandling(t *testing.T) {
	templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
	contextBuilder, err := shared.NewContextBuilder()
	require.NoError(t, err)
	normalizer := aggregate.NewNormalizer(templateEngine, contextBuilder)

	t.Run("Should handle nil config gracefully", func(t *testing.T) {
		// Arrange
		ctx := &shared.NormalizationContext{}
		// Act - Aggregate normalizer handles nil config gracefully since it only uses BaseNormalizer
		err := normalizer.Normalize(nil, ctx)
		// Assert
		assert.NoError(t, err)
	})

	t.Run("Should return error for wrong task type", func(t *testing.T) {
		// Arrange
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
		assert.Contains(t, err.Error(), "aggregate normalizer cannot handle task type: basic")
	})

	t.Run("Should handle template parsing errors in main config", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "{{ .invalid.deeply.nested.nonexistent.field }}",
				Type: task.TaskTypeAggregate,
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
		assert.Contains(t, err.Error(), "failed to normalize aggregate task config")
	})

	t.Run("Should handle config serialization errors", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeAggregate,
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

	t.Run("Should process aggregate task configuration successfully", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "{{ .name }}-aggregate",
				Type: task.TaskTypeAggregate,
			},
		}
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{
				"name": "test",
			},
		}
		// Act
		err := normalizer.Normalize(taskConfig, ctx)
		// Assert
		assert.NoError(t, err)
		assert.Equal(t, "test-aggregate", taskConfig.ID)
		assert.Equal(t, task.TaskTypeAggregate, taskConfig.Type)
	})
}

func TestAggregateNormalizer_BoundaryConditions(t *testing.T) {
	templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
	contextBuilder, err := shared.NewContextBuilder()
	require.NoError(t, err)

	t.Run("Should handle nil template engine", func(t *testing.T) {
		// Arrange
		normalizer := aggregate.NewNormalizer(nil, contextBuilder)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeAggregate,
			},
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act & Assert
		assert.Panics(t, func() {
			normalizer.Normalize(taskConfig, ctx)
		})
	})

	t.Run("Should handle nil context gracefully", func(t *testing.T) {
		// Arrange
		normalizer := aggregate.NewNormalizer(templateEngine, contextBuilder)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeAggregate,
			},
		}
		// Act
		err = normalizer.Normalize(taskConfig, nil)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid context type")
	})

	t.Run("Should handle empty aggregate configuration", func(t *testing.T) {
		// Arrange
		normalizer := aggregate.NewNormalizer(templateEngine, contextBuilder)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "empty-aggregate",
				Type: task.TaskTypeAggregate,
			},
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act
		err := normalizer.Normalize(taskConfig, ctx)
		// Assert
		assert.NoError(t, err)
		assert.Equal(t, "empty-aggregate", taskConfig.ID)
		assert.Equal(t, task.TaskTypeAggregate, taskConfig.Type)
	})

	t.Run("Should handle template expressions in aggregate task ID", func(t *testing.T) {
		// Arrange
		normalizer := aggregate.NewNormalizer(templateEngine, contextBuilder)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "{{ .prefix }}-aggregate-{{ .suffix }}",
				Type: task.TaskTypeAggregate,
			},
		}
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{
				"prefix": "data",
				"suffix": "processor",
			},
		}
		// Act
		err := normalizer.Normalize(taskConfig, ctx)
		// Assert
		assert.NoError(t, err)
		assert.Equal(t, "data-aggregate-processor", taskConfig.ID)
		assert.Equal(t, task.TaskTypeAggregate, taskConfig.Type)
	})

	t.Run("Should preserve aggregate task configuration", func(t *testing.T) {
		// Arrange
		normalizer := aggregate.NewNormalizer(templateEngine, contextBuilder)
		originalID := "original-aggregate"

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   originalID,
				Type: task.TaskTypeAggregate,
			},
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act
		err := normalizer.Normalize(taskConfig, ctx)
		// Assert
		assert.NoError(t, err)
		assert.Equal(t, originalID, taskConfig.ID)
		assert.Equal(t, task.TaskTypeAggregate, taskConfig.Type)
	})
}
