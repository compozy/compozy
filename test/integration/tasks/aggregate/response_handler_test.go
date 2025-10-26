package aggregate

import (
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/tasks/aggregate"
	"github.com/compozy/compozy/engine/task/tasks/shared"
	"github.com/compozy/compozy/engine/workflow"
	tkhelpers "github.com/compozy/compozy/test/integration/tasks/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestAggregateResponseHandler_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests")
	}

	t.Run("Should process aggregate task response with result validation", func(t *testing.T) {
		t.Parallel()

		// Setup test infrastructure
		ts := tkhelpers.NewTestSetup(t)

		// Create aggregate response handler
		handler := aggregate.NewResponseHandler(ts.TemplateEngine, ts.ContextBuilder, ts.BaseHandler)

		// Create workflow state
		workflowState, workflowExecID := ts.CreateWorkflowState(t, "test-workflow")

		// Create task state with aggregated results
		taskState := ts.CreateTaskState(t, &tkhelpers.TaskStateConfig{
			WorkflowID:     "test-workflow",
			WorkflowExecID: workflowExecID,
			TaskID:         "test-aggregate-task",
			Status:         core.StatusPending,
			Output: &core.Output{
				"aggregated": map[string]any{
					"total":   10,
					"average": 5.5,
					"results": []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
				},
			},
		})

		// Prepare input with aggregate configuration
		input := &shared.ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:      "test-aggregate-task",
					Type:    task.TaskTypeAggregate,
					Outputs: &core.Input{"summary": "{{ .output.aggregated }}"},
				},
			},
			TaskState:      taskState,
			WorkflowConfig: &workflow.Config{ID: "test-workflow"},
			WorkflowState:  workflowState,
		}

		// Mock output transformation
		ts.OutputTransformer.On("TransformOutput", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(map[string]any{
				"summary": map[string]any{
					"total":   10,
					"average": 5.5,
					"results": []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
				},
			}, nil)

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

	t.Run("Should handle empty aggregation results", func(t *testing.T) {
		t.Parallel()

		// Setup test infrastructure
		ts := tkhelpers.NewTestSetup(t)

		// Create aggregate response handler
		handler := aggregate.NewResponseHandler(ts.TemplateEngine, ts.ContextBuilder, ts.BaseHandler)

		// Create workflow state
		workflowState, workflowExecID := ts.CreateWorkflowState(t, "test-workflow")

		// Create task state with empty results
		taskState := ts.CreateTaskState(t, &tkhelpers.TaskStateConfig{
			WorkflowID:     "test-workflow",
			WorkflowExecID: workflowExecID,
			TaskID:         "test-empty-aggregate",
			Status:         core.StatusPending,
			Output:         &core.Output{"aggregated": []any{}}, // Empty results
		})

		// Prepare input
		input := &shared.ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:      "test-empty-aggregate",
					Type:    task.TaskTypeAggregate,
					Outputs: &core.Input{"results": "{{ .output.aggregated }}"},
				},
			},
			TaskState:      taskState,
			WorkflowConfig: &workflow.Config{ID: "test-workflow"},
			WorkflowState:  workflowState,
		}

		// Mock output transformation
		ts.OutputTransformer.On("TransformOutput", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(map[string]any{"results": []any{}}, nil)

		// Act - process the response
		result, err := handler.HandleResponse(ts.Context, input)

		// Assert - should handle empty results gracefully
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, core.StatusSuccess, result.State.Status)

		ts.OutputTransformer.AssertExpectations(t)
	})
}
