package activities

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
)

// Note: Full integration tests for ExecuteSignal.Run() would require
// complex mocking of dependencies. Instead, we focus on testing the core
// signal dispatch logic directly and leave full integration testing to
// higher-level test suites.

func TestExecuteSignal_dispatchSignal(t *testing.T) {
	t.Run("Should dispatch signal with payload", func(t *testing.T) {
		mockDispatcher := services.NewMockSignalDispatcher()
		activity := &ExecuteSignal{
			signalDispatcher: mockDispatcher,
		}
		taskConfig := &task.Config{
			SignalTask: task.SignalTask{
				Signal: &task.SignalConfig{
					ID: "test-signal",
					Payload: map[string]any{
						"key": "value",
					},
				},
			},
		}
		ctx := context.WithValue(t.Context(), core.ProjectNameKey{}, "test-project")
		err := activity.dispatchSignal(ctx, taskConfig, "correlation-123", "test-project")
		assert.NoError(t, err)
		assert.Len(t, mockDispatcher.Calls, 1)
		assert.Equal(t, "test-signal", mockDispatcher.Calls[0].SignalName)
		assert.Equal(t, "value", mockDispatcher.Calls[0].Payload["key"])
		assert.Equal(t, "correlation-123", mockDispatcher.Calls[0].CorrelationID)
	})

	t.Run("Should dispatch signal with empty payload", func(t *testing.T) {
		mockDispatcher := services.NewMockSignalDispatcher()
		activity := &ExecuteSignal{
			signalDispatcher: mockDispatcher,
		}
		taskConfig := &task.Config{
			SignalTask: task.SignalTask{
				Signal: &task.SignalConfig{
					ID: "test-signal",
				},
			},
		}
		ctx := context.WithValue(t.Context(), core.ProjectNameKey{}, "test-project")
		err := activity.dispatchSignal(ctx, taskConfig, "correlation-123", "test-project")
		assert.NoError(t, err)
		assert.Len(t, mockDispatcher.Calls, 1)
		assert.Equal(t, "test-signal", mockDispatcher.Calls[0].SignalName)
		assert.NotNil(t, mockDispatcher.Calls[0].Payload)
		assert.Equal(t, "correlation-123", mockDispatcher.Calls[0].CorrelationID)
	})

	t.Run("Should return error for empty signal id", func(t *testing.T) {
		mockDispatcher := services.NewMockSignalDispatcher()
		activity := &ExecuteSignal{
			signalDispatcher: mockDispatcher,
		}
		taskConfig := &task.Config{
			SignalTask: task.SignalTask{
				Signal: &task.SignalConfig{
					ID: "",
				},
			},
		}
		ctx := context.WithValue(t.Context(), core.ProjectNameKey{}, "test-project")
		err := activity.dispatchSignal(ctx, taskConfig, "correlation-123", "test-project")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "signal.id is required")
		assert.Len(t, mockDispatcher.Calls, 0)
	})
}
