package signal

import (
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/task2/signal"
	"github.com/compozy/compozy/engine/workflow"
	task2helpers "github.com/compozy/compozy/test/integration/task2/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestSignalResponseHandler_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests")
	}

	t.Run("Should process signal task response with dispatch logging", func(t *testing.T) {
		t.Parallel()

		// Setup test infrastructure
		ts := task2helpers.NewTestSetup(t)

		// Create signal response handler
		handler := signal.NewResponseHandler(ts.TemplateEngine, ts.ContextBuilder, ts.BaseHandler)

		// Create workflow state
		workflowState, workflowExecID := ts.CreateWorkflowState(t, "test-workflow")

		// Create task state with signal dispatch info
		taskState := ts.CreateTaskState(t, &task2helpers.TaskStateConfig{
			WorkflowID:     "test-workflow",
			WorkflowExecID: workflowExecID,
			TaskID:         "test-signal-task",
			Status:         core.StatusPending,
			Output:         &core.Output{"signal": "user-action", "target": "wait-task-1"},
		})

		// Prepare input with signal configuration
		input := &shared.ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:      "test-signal-task",
					Type:    task.TaskTypeSignal,
					Outputs: &core.Input{"dispatched": "{{ .output.signal }}"},
				},
			},
			TaskState:      taskState,
			WorkflowConfig: &workflow.Config{ID: "test-workflow"},
			WorkflowState:  workflowState,
		}

		// Mock output transformation
		ts.OutputTransformer.On("TransformOutput", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(map[string]any{"dispatched": "user-action"}, nil)

		// Act - process the response
		result, err := handler.HandleResponse(ts.Context, input)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, core.StatusSuccess, result.State.Status)

		// Verify state was saved to database
		savedState := ts.GetSavedTaskState(t, taskState.TaskExecID)
		assert.Equal(t, core.StatusSuccess, savedState.Status)
		assert.NotNil(t, savedState.Output)

		ts.OutputTransformer.AssertExpectations(t)
	})

	t.Run("Should handle signal dispatch failure", func(t *testing.T) {
		t.Parallel()

		// Setup test infrastructure
		ts := task2helpers.NewTestSetup(t)

		// Create signal response handler
		handler := signal.NewResponseHandler(ts.TemplateEngine, ts.ContextBuilder, ts.BaseHandler)

		// Create workflow state
		workflowState, workflowExecID := ts.CreateWorkflowState(t, "test-workflow")

		// Create task state
		taskState := ts.CreateTaskState(t, &task2helpers.TaskStateConfig{
			WorkflowID:     "test-workflow",
			WorkflowExecID: workflowExecID,
			TaskID:         "test-failed-signal",
			Status:         core.StatusPending,
		})

		// Prepare input with dispatch error
		input := &shared.ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   "test-failed-signal",
					Type: task.TaskTypeSignal,
					OnError: &core.ErrorTransition{
						Next: func() *string { s := "error-handler"; return &s }(),
					},
				},
			},
			TaskState:      taskState,
			ExecutionError: assert.AnError, // Simulating dispatch failure
			WorkflowConfig: &workflow.Config{ID: "test-workflow"},
			WorkflowState:  workflowState,
		}

		// Act - process the response
		result, err := handler.HandleResponse(ts.Context, input)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, core.StatusFailed, result.State.Status)
		assert.NotNil(t, result.State.Error)

		// Verify state was saved to database
		savedState := ts.GetSavedTaskState(t, taskState.TaskExecID)
		assert.Equal(t, core.StatusFailed, savedState.Status)
		assert.NotNil(t, savedState.Error)
	})
}

func TestSignalResponseHandler_Type(t *testing.T) {
	t.Run("Should return signal task type", func(t *testing.T) {
		handler := signal.NewResponseHandler(nil, nil, nil)
		assert.Equal(t, task.TaskTypeSignal, handler.Type())
	})
}
