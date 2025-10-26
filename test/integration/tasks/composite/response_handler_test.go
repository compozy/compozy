package composite

import (
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/tasks/composite"
	"github.com/compozy/compozy/engine/task/tasks/shared"
	"github.com/compozy/compozy/engine/workflow"
	tkhelpers "github.com/compozy/compozy/test/integration/tasks/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCompositeResponseHandler_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests")
	}

	t.Run("Should process composite task response", func(t *testing.T) {
		t.Parallel()

		// Setup test infrastructure using centralized helper
		ts := tkhelpers.NewTestSetup(t)

		// Create composite response handler
		handler := composite.NewResponseHandler(ts.TemplateEngine, ts.ContextBuilder, ts.BaseHandler)

		// Create workflow state
		workflowState, workflowExecID := ts.CreateWorkflowState(t, "test-workflow")

		// Create a composite task state
		taskState := ts.CreateTaskState(t, &tkhelpers.TaskStateConfig{
			WorkflowID:     "test-workflow",
			WorkflowExecID: workflowExecID,
			TaskID:         "test-composite-task",
			Status:         core.StatusPending,
			Output:         &core.Output{"step": "completed"},
		})

		// Prepare input with composite configuration
		input := &shared.ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:      "test-composite-task",
					Type:    task.TaskTypeComposite,
					Outputs: &core.Input{"result": "{{ .output.step }}"},
				},
			},
			TaskState:      taskState,
			WorkflowConfig: &workflow.Config{ID: "test-workflow"},
			WorkflowState:  workflowState,
		}

		// Mock output transformation
		ts.OutputTransformer.On("TransformOutput", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(map[string]any{"result": "completed"}, nil)

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

	t.Run("Should handle subtask response for sequential execution", func(t *testing.T) {
		// Arrange
		handler := composite.NewResponseHandler(nil, nil, nil)

		parentState := &task.State{
			TaskExecID: core.MustNewID(),
			TaskID:     "parent-composite",
		}

		childState := &task.State{
			TaskExecID: core.MustNewID(),
			TaskID:     "step-1",
			Status:     core.StatusSuccess,
			Output:     &core.Output{"stepResult": "step1-complete"},
		}

		childConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID: "step-1",
			},
		}

		// Act
		response, err := handler.HandleSubtaskResponse(t.Context(), parentState, childState, childConfig)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, "step-1", response.TaskID)
		assert.Equal(t, core.StatusSuccess, response.Status)
		assert.Equal(t, childState.Output, response.Output)
	})

	t.Run("Should handle failed composite task", func(t *testing.T) {
		t.Parallel()

		// Setup test infrastructure using centralized helper
		ts := tkhelpers.NewTestSetup(t)

		// Create composite response handler
		handler := composite.NewResponseHandler(ts.TemplateEngine, ts.ContextBuilder, ts.BaseHandler)

		// Create workflow state
		workflowState, workflowExecID := ts.CreateWorkflowState(t, "test-workflow")

		// Create a failed composite task state
		taskState := ts.CreateTaskState(t, &tkhelpers.TaskStateConfig{
			WorkflowID:     "test-workflow",
			WorkflowExecID: workflowExecID,
			TaskID:         "test-failed-composite",
			Status:         core.StatusPending,
		})

		// Prepare input with execution error
		input := &shared.ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   "test-failed-composite",
					Type: task.TaskTypeComposite,
					OnError: &core.ErrorTransition{
						Next: func() *string { s := "error-handler"; return &s }(),
					},
				},
			},
			TaskState:      taskState,
			ExecutionError: assert.AnError,
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
