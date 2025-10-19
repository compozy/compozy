package collection

import (
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/collection"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
	task2helpers "github.com/compozy/compozy/test/integration/task2/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCollectionResponseHandler_Integration(t *testing.T) {
	t.Run("Should process collection task response with context variables", func(t *testing.T) {
		t.Parallel()

		// Setup test infrastructure
		ts := task2helpers.NewTestSetup(t)

		// Create collection response handler
		handler := collection.NewResponseHandler(ts.TemplateEngine, ts.ContextBuilder, ts.BaseHandler)

		// Create workflow state
		workflowState, workflowExecID := ts.CreateWorkflowState(t, "test-workflow")

		// Create task state
		taskState := ts.CreateTaskState(t, &task2helpers.TaskStateConfig{
			WorkflowID:     "test-workflow",
			WorkflowExecID: workflowExecID,
			TaskID:         "test-collection-task",
			Status:         core.StatusSuccess,
			Output:         &core.Output{"items": []string{"item1", "item2"}},
		})

		// Prepare input with collection context variables
		input := &shared.ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   "test-collection-task",
					Type: task.TaskTypeCollection,
					With: &core.Input{
						"_collection_item":      "current-item",
						"_collection_index":     2,
						"_collection_item_var":  "customItem",
						"_collection_index_var": "customIndex",
					},
				},
			},
			TaskState:      taskState,
			WorkflowConfig: &workflow.Config{ID: "test-workflow"},
			WorkflowState:  workflowState,
		}

		// Act - process the response
		result, err := handler.HandleResponse(ts.Context, input)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, core.StatusSuccess, result.State.Status)

		// Verify state was saved to database
		savedState := ts.GetSavedTaskState(t, taskState.TaskExecID)
		assert.Equal(t, core.StatusSuccess, savedState.Status)
	})

	t.Run("Should handle deferred output transformation", func(t *testing.T) {
		t.Parallel()

		// Setup test infrastructure
		ts := task2helpers.NewTestSetup(t)

		// Create collection response handler
		handler := collection.NewResponseHandler(ts.TemplateEngine, ts.ContextBuilder, ts.BaseHandler)

		// Create workflow state
		workflowState, workflowExecID := ts.CreateWorkflowState(t, "test-workflow")

		// Create task state
		taskState := ts.CreateTaskState(t, &task2helpers.TaskStateConfig{
			WorkflowID:     "test-workflow",
			WorkflowExecID: workflowExecID,
			TaskID:         "test-collection-task",
			Status:         core.StatusSuccess,
			Output:         &core.Output{"items": []string{"item1", "item2"}},
		})

		// Prepare input for deferred transformation
		input := &shared.ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:      "test-collection-task",
					Type:    task.TaskTypeCollection,
					Outputs: &core.Input{"aggregated": "{{ .output.items }}"},
				},
			},
			TaskState:      taskState,
			WorkflowConfig: &workflow.Config{ID: "test-workflow"},
			WorkflowState:  workflowState,
		}

		// Mock expectations for deferred transformation
		ts.OutputTransformer.On("TransformOutput", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(map[string]any{"aggregated": []string{"item1", "item2"}}, nil)

		// Act - apply deferred transformation
		err := handler.ApplyDeferredOutputTransformation(ts.Context, input)

		// Assert
		require.NoError(t, err)

		// Verify state was updated with transformed output
		savedState := ts.GetSavedTaskState(t, taskState.TaskExecID)
		assert.Equal(t, core.StatusSuccess, savedState.Status)
		assert.NotNil(t, savedState.Output)

		ts.OutputTransformer.AssertExpectations(t)
	})

	t.Run("Should handle subtask response", func(t *testing.T) {
		// Arrange
		handler := collection.NewResponseHandler(nil, nil, nil)

		parentState := &task.State{
			TaskExecID: core.MustNewID(),
			TaskID:     "parent-collection",
		}

		childState := &task.State{
			TaskExecID: core.MustNewID(),
			TaskID:     "child-task",
			Status:     core.StatusSuccess,
			Output:     &core.Output{"result": "child-output"},
		}

		childConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID: "child-task",
			},
		}

		// Act
		response, err := handler.HandleSubtaskResponse(t.Context(), parentState, childState, childConfig)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, "child-task", response.TaskID)
		assert.Equal(t, core.StatusSuccess, response.Status)
		assert.Equal(t, childState.Output, response.Output)
	})
}
