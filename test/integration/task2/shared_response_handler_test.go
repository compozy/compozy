package task2

import (
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/aggregate"
	"github.com/compozy/compozy/engine/task2/basic"
	"github.com/compozy/compozy/engine/task2/collection"
	"github.com/compozy/compozy/engine/task2/composite"
	"github.com/compozy/compozy/engine/task2/parallel"
	"github.com/compozy/compozy/engine/task2/router"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/task2/signal"
	"github.com/compozy/compozy/engine/task2/wait"
	"github.com/compozy/compozy/engine/workflow"
	task2helpers "github.com/compozy/compozy/test/integration/task2/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ResponseHandlerFactory creates response handlers for different task types
type ResponseHandlerFactory func(ts *task2helpers.TestSetup) shared.TaskResponseHandler

// TaskTestCase defines test scenarios for each task type
type TaskTestCase struct {
	TaskType    task.Type
	TaskID      string
	Output      *core.Output
	HandlerFunc ResponseHandlerFactory
}

// TestAllTaskTypesResponseHandlers runs unified tests across all task types
// This replaces 8 separate test files with identical patterns
func TestAllTaskTypesResponseHandlers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests")
	}

	// Define test cases for all task types
	testCases := []TaskTestCase{
		{
			TaskType: task.TaskTypeBasic,
			TaskID:   "test-basic-task",
			Output:   &core.Output{"data": "test-output"},
			HandlerFunc: func(ts *task2helpers.TestSetup) shared.TaskResponseHandler {
				handler, err := basic.NewResponseHandler(ts.TemplateEngine, ts.ContextBuilder, ts.BaseHandler)
				require.NoError(t, err)
				return handler
			},
		},
		{
			TaskType: task.TaskTypeAggregate,
			TaskID:   "test-aggregate-task",
			Output: &core.Output{
				"aggregated": map[string]any{
					"total":   10,
					"average": 5.5,
					"results": []int{1, 2, 3, 4, 5},
				},
			},
			HandlerFunc: func(ts *task2helpers.TestSetup) shared.TaskResponseHandler {
				return aggregate.NewResponseHandler(ts.TemplateEngine, ts.ContextBuilder, ts.BaseHandler)
			},
		},
		{
			TaskType: task.TaskTypeCollection,
			TaskID:   "test-collection-task",
			Output:   &core.Output{"items": []string{"item1", "item2", "item3"}},
			HandlerFunc: func(ts *task2helpers.TestSetup) shared.TaskResponseHandler {
				return collection.NewResponseHandler(ts.TemplateEngine, ts.ContextBuilder, ts.BaseHandler)
			},
		},
		{
			TaskType: task.TaskTypeComposite,
			TaskID:   "test-composite-task",
			Output:   &core.Output{"step": "completed"},
			HandlerFunc: func(ts *task2helpers.TestSetup) shared.TaskResponseHandler {
				return composite.NewResponseHandler(ts.TemplateEngine, ts.ContextBuilder, ts.BaseHandler)
			},
		},
		{
			TaskType: task.TaskTypeParallel,
			TaskID:   "test-parallel-task",
			Output:   &core.Output{"results": []string{"result1", "result2"}},
			HandlerFunc: func(ts *task2helpers.TestSetup) shared.TaskResponseHandler {
				return parallel.NewResponseHandler(ts.TemplateEngine, ts.ContextBuilder, ts.BaseHandler)
			},
		},
		{
			TaskType: task.TaskTypeRouter,
			TaskID:   "test-router-task",
			Output:   &core.Output{"route": "success"},
			HandlerFunc: func(ts *task2helpers.TestSetup) shared.TaskResponseHandler {
				return router.NewResponseHandler(ts.TemplateEngine, ts.ContextBuilder, ts.BaseHandler)
			},
		},
		{
			TaskType: task.TaskTypeSignal,
			TaskID:   "test-signal-task",
			Output:   &core.Output{"signal": "sent"},
			HandlerFunc: func(ts *task2helpers.TestSetup) shared.TaskResponseHandler {
				return signal.NewResponseHandler(ts.TemplateEngine, ts.ContextBuilder, ts.BaseHandler)
			},
		},
		{
			TaskType: task.TaskTypeWait,
			TaskID:   "test-wait-task",
			Output:   &core.Output{"waited": true},
			HandlerFunc: func(ts *task2helpers.TestSetup) shared.TaskResponseHandler {
				return wait.NewResponseHandler(ts.TemplateEngine, ts.ContextBuilder, ts.BaseHandler)
			},
		},
	}

	t.Run("Should process successful task responses for all task types", func(t *testing.T) {
		// Use single test setup for all task types to minimize database overhead
		ts := task2helpers.NewTestSetup(t)

		// Create workflow state once for all tasks - use unique workflow ID for this test group
		workflowState, workflowExecID := ts.CreateWorkflowState(t, "test-workflow-success")

		for _, tc := range testCases {
			tc := tc // capture loop variable
			t.Run(string(tc.TaskType), func(t *testing.T) {
				// Create handler for this task type
				handler := tc.HandlerFunc(ts)

				// Collection and parallel tasks require child tasks to complete before SUCCESS
				// For testing, we simulate already-completed tasks by setting initial status to SUCCESS
				initialStatus := core.StatusPending
				expectedStatus := core.StatusSuccess
				if tc.TaskType == task.TaskTypeCollection || tc.TaskType == task.TaskTypeParallel {
					initialStatus = core.StatusSuccess  // Simulate completed child tasks
					expectedStatus = core.StatusSuccess // Should remain SUCCESS
				}

				// Create task state
				taskState := ts.CreateTaskState(t, &task2helpers.TaskStateConfig{
					WorkflowID:     "test-workflow-success",
					WorkflowExecID: workflowExecID,
					TaskID:         tc.TaskID,
					Status:         initialStatus,
					Output:         tc.Output,
				})

				// Prepare input
				input := &shared.ResponseInput{
					TaskConfig: &task.Config{
						BaseConfig: task.BaseConfig{
							ID:      tc.TaskID,
							Type:    tc.TaskType,
							Outputs: &core.Input{"result": "{{ .output }}"},
						},
					},
					TaskState:      taskState,
					WorkflowConfig: &workflow.Config{ID: "test-workflow-success"},
					WorkflowState:  workflowState,
				}

				// Mock output transformation
				ts.OutputTransformer.On("TransformOutput", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(map[string]any{"result": tc.Output}, nil).Maybe()

				// Act - process the response
				result, err := handler.HandleResponse(ts.Context, input)

				// Assert
				require.NoError(t, err, "Handler should process %s task successfully", tc.TaskType)
				assert.NotNil(t, result, "Result should not be nil for %s task", tc.TaskType)
				assert.Equal(t, expectedStatus, result.State.Status, "Task %s should have expected status", tc.TaskType)

				// Verify state was saved to database
				savedState := ts.GetSavedTaskState(t, taskState.TaskExecID)
				assert.Equal(
					t,
					expectedStatus,
					savedState.Status,
					"Saved state should have expected status for %s",
					tc.TaskType,
				)
				assert.NotNil(t, savedState.Output, "Saved output should not be nil for %s", tc.TaskType)
			})
		}

		ts.OutputTransformer.AssertExpectations(t)
	})

	t.Run("Should handle failed tasks for all task types", func(t *testing.T) {
		// Use single test setup for all task types
		ts := task2helpers.NewTestSetup(t)

		// Create workflow state once for all tasks - use unique workflow ID for this test group
		workflowState, workflowExecID := ts.CreateWorkflowState(t, "test-workflow-failed")

		for _, tc := range testCases {
			tc := tc // capture loop variable
			t.Run(string(tc.TaskType), func(t *testing.T) {
				// Create handler for this task type
				handler := tc.HandlerFunc(ts)

				// Create task state
				taskState := ts.CreateTaskState(t, &task2helpers.TaskStateConfig{
					WorkflowID:     "test-workflow-failed",
					WorkflowExecID: workflowExecID,
					TaskID:         tc.TaskID + "-failed",
					Status:         core.StatusPending,
				})

				// Prepare input with execution error
				input := &shared.ResponseInput{
					TaskConfig: &task.Config{
						BaseConfig: task.BaseConfig{
							ID:   tc.TaskID + "-failed",
							Type: tc.TaskType,
							OnError: &core.ErrorTransition{
								Next: func() *string { s := "error-handler"; return &s }(),
							},
						},
					},
					TaskState:      taskState,
					ExecutionError: assert.AnError,
					WorkflowConfig: &workflow.Config{ID: "test-workflow-failed"},
					WorkflowState:  workflowState,
				}

				// Act - process the response
				result, err := handler.HandleResponse(ts.Context, input)

				// Assert
				require.NoError(t, err, "Handler should process failed %s task", tc.TaskType)
				assert.NotNil(t, result, "Result should not be nil for failed %s task", tc.TaskType)
				assert.Equal(
					t,
					core.StatusFailed,
					result.State.Status,
					"Failed %s task should have failed status",
					tc.TaskType,
				)
				assert.NotNil(t, result.State.Error, "Failed %s task should have error", tc.TaskType)

				// Verify state was saved to database
				savedState := ts.GetSavedTaskState(t, taskState.TaskExecID)
				assert.Equal(
					t,
					core.StatusFailed,
					savedState.Status,
					"Saved state should be failed for %s",
					tc.TaskType,
				)
				assert.NotNil(t, savedState.Error, "Saved error should not be nil for %s", tc.TaskType)
			})
		}
	})
}
