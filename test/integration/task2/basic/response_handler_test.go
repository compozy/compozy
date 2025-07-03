package basic

import (
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/basic"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
	task2helpers "github.com/compozy/compozy/test/integration/task2/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestBasicResponseHandler_Integration(t *testing.T) {
	t.Run("Should process basic task response with real database", func(t *testing.T) {
		t.Parallel()

		// Setup test infrastructure
		ts := task2helpers.NewTestSetup(t)

		// Create basic response handler
		handler := basic.NewResponseHandler(ts.TemplateEngine, ts.ContextBuilder, ts.BaseHandler)

		// Create workflow state
		workflowState, workflowExecID := ts.CreateWorkflowState(t, "test-workflow")

		// Create task state
		taskState := ts.CreateTaskState(t, &task2helpers.TaskStateConfig{
			WorkflowID:     "test-workflow",
			WorkflowExecID: workflowExecID,
			TaskID:         "test-basic-task",
			Status:         core.StatusPending,
			Output:         &core.Output{"data": "test-output"},
		})

		// Prepare input
		input := &shared.ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:      "test-basic-task",
					Type:    task.TaskTypeBasic,
					Outputs: &core.Input{"result": "{{ .output.data }}"},
				},
			},
			TaskState: taskState,
			WorkflowConfig: &workflow.Config{
				ID: "test-workflow",
			},
			WorkflowState: workflowState,
		}

		// Mock output transformation
		ts.OutputTransformer.On("TransformOutput", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(map[string]any{"result": "test-output"}, nil)

		// Act - process the response
		result, err := handler.HandleResponse(ts.Context, input)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, core.StatusSuccess, result.State.Status)

		// Verify it's a MainTaskResponse
		mainTaskResp, ok := result.Response.(*task.MainTaskResponse)
		require.True(t, ok)
		assert.Equal(t, taskState, mainTaskResp.State)

		// Verify state was saved to database
		savedState := ts.GetSavedTaskState(t, taskState.TaskExecID)
		assert.Equal(t, core.StatusSuccess, savedState.Status)
		assert.NotNil(t, savedState.Output)

		ts.OutputTransformer.AssertExpectations(t)
	})

	t.Run("Should handle failed basic task", func(t *testing.T) {
		t.Parallel()

		// Setup test infrastructure
		ts := task2helpers.NewTestSetup(t)

		// Create basic response handler
		handler := basic.NewResponseHandler(ts.TemplateEngine, ts.ContextBuilder, ts.BaseHandler)

		// Create workflow state
		workflowState, workflowExecID := ts.CreateWorkflowState(t, "test-workflow")

		// Create task state
		taskState := ts.CreateTaskState(t, &task2helpers.TaskStateConfig{
			WorkflowID:     "test-workflow",
			WorkflowExecID: workflowExecID,
			TaskID:         "test-failed-task",
			Status:         core.StatusPending,
		})

		// Prepare input with execution error
		input := &shared.ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   "test-failed-task",
					Type: task.TaskTypeBasic,
					OnError: &core.ErrorTransition{
						Next: func() *string { s := "error-handler"; return &s }(),
					},
				},
			},
			TaskState:      taskState,
			ExecutionError: assert.AnError,
			WorkflowConfig: &workflow.Config{
				ID: "test-workflow",
			},
			WorkflowState: workflowState,
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

func TestBasicResponseHandler_Type(t *testing.T) {
	t.Run("Should return basic task type", func(t *testing.T) {
		handler := basic.NewResponseHandler(nil, nil, nil)
		assert.Equal(t, task.TaskTypeBasic, handler.Type())
	})
}
