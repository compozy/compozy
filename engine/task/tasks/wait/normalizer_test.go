package wait_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/tasks/shared"
	"github.com/compozy/compozy/engine/task/tasks/wait"
	"github.com/compozy/compozy/pkg/tplengine"
)

func TestWaitNormalizer_NewNormalizer(t *testing.T) {
	t.Run("Should create wait normalizer", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		contextBuilder, err := shared.NewContextBuilder(t.Context())
		require.NoError(t, err)

		// Act
		normalizer := wait.NewNormalizer(t.Context(), templateEngine, contextBuilder)

		// Assert
		assert.NotNil(t, normalizer)
	})

	t.Run("Should handle nil template engine", func(t *testing.T) {
		// Arrange
		contextBuilder, err := shared.NewContextBuilder(t.Context())
		require.NoError(t, err)

		// Act
		normalizer := wait.NewNormalizer(t.Context(), nil, contextBuilder)

		// Assert
		assert.NotNil(t, normalizer)
	})

	t.Run("Should handle nil context builder", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)

		// Act
		normalizer := wait.NewNormalizer(t.Context(), templateEngine, nil)

		// Assert
		assert.NotNil(t, normalizer)
	})

	t.Run("Should handle both nil parameters", func(t *testing.T) {
		// Act
		normalizer := wait.NewNormalizer(t.Context(), nil, nil)

		// Assert
		assert.NotNil(t, normalizer)
	})
}

func TestWaitNormalizer_NormalizeWithSignal(t *testing.T) {
	// Setup
	templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
	contextBuilder, err := shared.NewContextBuilder(t.Context())
	require.NoError(t, err)
	normalizer := wait.NewNormalizer(t.Context(), templateEngine, contextBuilder)

	t.Run("Should normalize wait task config with signal", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "wait-task-{{ .signal.type }}",
				Type: task.TaskTypeWait,
			},
			WaitTask: task.WaitTask{
				WaitFor: "{{ .signal.event_id }}",
			},
		}
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{
				"test_var": "test_value",
			},
		}
		signal := map[string]any{
			"type":     "timeout",
			"event_id": "signal-123",
		}

		// Act
		err := normalizer.NormalizeWithSignal(t.Context(), taskConfig, ctx, signal)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, "wait-task-timeout", taskConfig.ID)
		assert.Equal(t, "signal-123", taskConfig.WaitFor)
	})

	t.Run("Should handle nil signal", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "wait-task",
				Type: task.TaskTypeWait,
			},
			WaitTask: task.WaitTask{
				WaitFor: "test-signal",
			},
		}
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{
				"test_var": "test_value",
			},
		}

		// Act
		err := normalizer.NormalizeWithSignal(t.Context(), taskConfig, ctx, nil)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, "wait-task", taskConfig.ID)
		assert.Equal(t, "test-signal", taskConfig.WaitFor)
	})

	t.Run("Should handle nil config", func(t *testing.T) {
		// Arrange
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{},
		}
		signal := map[string]any{
			"type": "timeout",
		}

		// Act
		err := normalizer.NormalizeWithSignal(t.Context(), nil, ctx, signal)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("Should merge existing With values", func(t *testing.T) {
		// Arrange
		existingWith := core.Input{
			"existing_key": "existing_value",
			"shared_key":   "existing_shared",
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "wait-task",
				Type: task.TaskTypeWait,
			},
			WaitTask: task.WaitTask{
				WaitFor: "test-signal",
			},
		}
		taskConfig.With = &existingWith

		ctx := &shared.NormalizationContext{
			Variables: map[string]any{
				"test_var": "test_value",
			},
		}
		signal := map[string]any{
			"type": "timeout",
		}

		// Act
		err := normalizer.NormalizeWithSignal(t.Context(), taskConfig, ctx, signal)

		// Assert
		assert.NoError(t, err)
		require.NotNil(t, taskConfig.With)
		with := *taskConfig.With
		assert.Equal(t, "existing_value", with["existing_key"])
		assert.Equal(t, "existing_shared", with["shared_key"])
	})

	t.Run("Should handle complex signal structures", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "wait-task-{{ .signal.data.name }}",
				Type: task.TaskTypeWait,
			},
			WaitTask: task.WaitTask{
				WaitFor:   "{{ .signal.data.event_id }}",
				OnTimeout: "{{ .signal.data.timeout_action }}",
			},
		}
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{
				"test_var": "test_value",
			},
		}
		signal := map[string]any{
			"type": "complex",
			"data": map[string]any{
				"name":           "complex-wait",
				"event_id":       "signal-complex-123",
				"timeout_action": "abort",
			},
		}

		// Act
		err := normalizer.NormalizeWithSignal(t.Context(), taskConfig, ctx, signal)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, "wait-task-complex-wait", taskConfig.ID)
		assert.Equal(t, "signal-complex-123", taskConfig.WaitFor)
		assert.Equal(t, "abort", taskConfig.OnTimeout)
	})

	t.Run("Should return error for invalid signal structure", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "wait-task",
				Type: task.TaskTypeWait,
			},
		}
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{},
		}
		// Use a channel which cannot be converted to map
		signal := make(chan int)

		// Act
		err := normalizer.NormalizeWithSignal(t.Context(), taskConfig, ctx, signal)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to convert signal to map")
	})

	t.Run("Should return error for template processing errors", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "wait-task-{{ .signal.nonexistent }}",
				Type: task.TaskTypeWait,
			},
		}
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{},
		}
		signal := map[string]any{
			"type": "timeout",
		}

		// Act
		err := normalizer.NormalizeWithSignal(t.Context(), taskConfig, ctx, signal)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to normalize task config with signal context")
	})

	t.Run("Should handle non-map signal that converts to map", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "wait-task",
				Type: task.TaskTypeWait,
			},
			WaitTask: task.WaitTask{
				WaitFor: "test-signal",
			},
		}
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{},
		}
		// Use a struct that can be converted to map
		signal := struct {
			Type    string `json:"type"`
			EventID string `json:"event_id"`
		}{
			Type:    "struct-signal",
			EventID: "struct-event-123",
		}

		// Act
		err := normalizer.NormalizeWithSignal(t.Context(), taskConfig, ctx, signal)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, "wait-task", taskConfig.ID)
		assert.Equal(t, "test-signal", taskConfig.WaitFor)
	})
}

func TestWaitNormalizer_Type(t *testing.T) {
	t.Run("Should return correct task type", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		contextBuilder, err := shared.NewContextBuilder(t.Context())
		require.NoError(t, err)
		normalizer := wait.NewNormalizer(t.Context(), templateEngine, contextBuilder)
		// Act
		taskType := normalizer.Type()
		// Assert
		assert.Equal(t, task.TaskTypeWait, taskType)
	})
}

func TestWaitNormalizer_Normalize_ErrorHandling(t *testing.T) {
	templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
	contextBuilder, err := shared.NewContextBuilder(t.Context())
	require.NoError(t, err)
	normalizer := wait.NewNormalizer(t.Context(), templateEngine, contextBuilder)

	t.Run("Should handle nil config gracefully", func(t *testing.T) {
		// Arrange
		ctx := &shared.NormalizationContext{}
		// Act
		err := normalizer.Normalize(t.Context(), nil, ctx)
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
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "wait normalizer cannot handle task type: basic")
	})

	t.Run("Should handle template parsing errors in main config", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "{{ .invalid.deeply.nested.nonexistent.field }}",
				Type: task.TaskTypeWait,
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
		assert.Contains(t, err.Error(), "failed to normalize wait task config")
	})

	t.Run("Should handle config serialization errors", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeWait,
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

	t.Run("Should process wait task configuration successfully", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "{{ .name }}-wait",
				Type: task.TaskTypeWait,
			},
			WaitTask: task.WaitTask{
				WaitFor:   "{{ .event_name }}",
				OnTimeout: "{{ .timeout_action }}",
			},
		}
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{
				"name":           "test",
				"event_name":     "user-action",
				"timeout_action": "retry",
			},
		}
		// Act
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)
		// Assert
		assert.NoError(t, err)
		assert.Equal(t, "test-wait", taskConfig.ID)
		assert.Equal(t, "user-action", taskConfig.WaitFor)
		assert.Equal(t, "retry", taskConfig.OnTimeout)
	})
}

func TestWaitNormalizer_BoundaryConditions(t *testing.T) {
	templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
	contextBuilder, err := shared.NewContextBuilder(t.Context())
	require.NoError(t, err)

	t.Run("Should handle nil template engine", func(t *testing.T) {
		// Arrange
		normalizer := wait.NewNormalizer(t.Context(), nil, contextBuilder)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeWait,
			},
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)
		// Assert - Should return error due to nil template engine
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "template engine is required for normalization")
	})

	t.Run("Should handle nil context gracefully", func(t *testing.T) {
		// Arrange
		normalizer := wait.NewNormalizer(t.Context(), templateEngine, contextBuilder)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeWait,
			},
		}
		// Act
		err = normalizer.Normalize(t.Context(), taskConfig, nil)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid context type")
	})

	t.Run("Should handle empty wait configuration", func(t *testing.T) {
		// Arrange
		normalizer := wait.NewNormalizer(t.Context(), templateEngine, contextBuilder)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "empty-wait",
				Type: task.TaskTypeWait,
			},
			// Empty WaitTask
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)
		// Assert
		assert.NoError(t, err)
		assert.Equal(t, "", taskConfig.WaitFor)
		assert.Equal(t, "", taskConfig.OnTimeout)
	})

	t.Run("Should handle different wait event formats", func(t *testing.T) {
		// Arrange
		normalizer := wait.NewNormalizer(t.Context(), templateEngine, contextBuilder)
		testCases := []struct {
			name      string
			waitFor   string
			onTimeout string
		}{
			{"simple_event", "user-action", "retry"},
			{"namespaced_event", "api.response.success", "fallback"},
			{"pattern_event", "task-*.complete", "timeout"},
			{"uuid_event", "550e8400-e29b-41d4-a716-446655440000", "abort"},
			{"complex_pattern", "workflow.step[1-5].done", "escalate"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				taskConfig := &task.Config{
					BaseConfig: task.BaseConfig{
						ID:   "wait-test",
						Type: task.TaskTypeWait,
					},
					WaitTask: task.WaitTask{
						WaitFor:   tc.waitFor,
						OnTimeout: tc.onTimeout,
					},
				}
				ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
				// Act
				err := normalizer.Normalize(t.Context(), taskConfig, ctx)
				// Assert
				assert.NoError(t, err)
				assert.Equal(t, tc.waitFor, taskConfig.WaitFor)
				assert.Equal(t, tc.onTimeout, taskConfig.OnTimeout)
			})
		}
	})

	t.Run("Should preserve wait task configuration", func(t *testing.T) {
		// Arrange
		normalizer := wait.NewNormalizer(t.Context(), templateEngine, contextBuilder)
		originalWaitFor := "original-event"
		originalOnTimeout := "original-action"

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "preserve-wait",
				Type: task.TaskTypeWait,
			},
			WaitTask: task.WaitTask{
				WaitFor:   originalWaitFor,
				OnTimeout: originalOnTimeout,
			},
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)
		// Assert
		assert.NoError(t, err)
		assert.Equal(t, originalWaitFor, taskConfig.WaitFor)
		assert.Equal(t, originalOnTimeout, taskConfig.OnTimeout)
		assert.Equal(t, task.TaskTypeWait, taskConfig.Type)
	})

	t.Run("Should handle complex wait expressions", func(t *testing.T) {
		// Arrange
		normalizer := wait.NewNormalizer(t.Context(), templateEngine, contextBuilder)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "complex-wait",
				Type: task.TaskTypeWait,
			},
			WaitTask: task.WaitTask{
				WaitFor:   "{{ if .urgent }}urgent-event{{ else }}normal-event{{ end }}",
				OnTimeout: "{{ if .retry }}retry{{ else }}fail{{ end }}",
			},
		}
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{
				"urgent": true,
				"retry":  false,
			},
		}
		// Act
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)
		// Assert
		assert.NoError(t, err)
		assert.Equal(t, "urgent-event", taskConfig.WaitFor)
		assert.Equal(t, "fail", taskConfig.OnTimeout)
	})
}
