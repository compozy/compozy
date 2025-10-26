package parallel

import (
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/tasks/parallel"
	"github.com/compozy/compozy/engine/task/tasks/shared"
	"github.com/compozy/compozy/engine/workflow"
	tkhelpers "github.com/compozy/compozy/test/integration/tasks/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestParallelResponseHandler_Integration(t *testing.T) {
	t.Run("Should process parallel task response with strategy", func(t *testing.T) {
		// Setup test infrastructure
		ts := tkhelpers.NewTestSetup(t)

		// Create parallel response handler
		handler := parallel.NewResponseHandler(ts.TemplateEngine, ts.ContextBuilder, ts.BaseHandler)

		// Create workflow state
		workflowState, workflowExecID := ts.CreateWorkflowState(t, "test-workflow")

		// Create task state
		taskState := ts.CreateTaskState(t, &tkhelpers.TaskStateConfig{
			WorkflowID:     "test-workflow",
			WorkflowExecID: workflowExecID,
			TaskID:         "test-parallel-task",
			Status:         core.StatusSuccess,
			Output:         &core.Output{"results": []string{"r1", "r2"}},
		})

		// Prepare input with parallel strategy
		strategy := task.StrategyWaitAll
		input := &shared.ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   "test-parallel-task",
					Type: task.TaskTypeParallel,
					With: &core.Input{
						"strategy": string(strategy),
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
		// Setup test infrastructure
		ts := tkhelpers.NewTestSetup(t)

		// Create parallel response handler
		handler := parallel.NewResponseHandler(ts.TemplateEngine, ts.ContextBuilder, ts.BaseHandler)

		// Create workflow state
		_, workflowExecID := ts.CreateWorkflowState(t, "test-workflow")

		// Create task state
		taskState := ts.CreateTaskState(t, &tkhelpers.TaskStateConfig{
			WorkflowID:     "test-workflow",
			WorkflowExecID: workflowExecID,
			TaskID:         "test-parallel-task",
			Status:         core.StatusSuccess,
			Output:         &core.Output{"results": []string{"r1", "r2", "r3"}},
		})

		// Prepare input for deferred transformation
		input := &shared.ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:      "test-parallel-task",
					Type:    task.TaskTypeParallel,
					Outputs: &core.Input{"aggregated": "{{ .output.results }}"},
				},
			},
			TaskState:      taskState,
			WorkflowConfig: &workflow.Config{ID: "test-workflow"},
			WorkflowState:  &workflow.State{WorkflowID: "test-workflow", WorkflowExecID: workflowExecID},
		}

		// Mock expectations for deferred transformation
		ts.OutputTransformer.On("TransformOutput", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(map[string]any{"aggregated": []string{"r1", "r2", "r3"}}, nil)

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
		handler := parallel.NewResponseHandler(nil, nil, nil)

		parentState := &task.State{
			TaskExecID: core.MustNewID(),
			TaskID:     "parent-parallel",
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
		response, err := handler.HandleSubtaskResponse(
			t.Context(),
			parentState,
			childState,
			childConfig,
			task.StrategyWaitAll,
		)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, "child-task", response.TaskID)
		assert.Equal(t, core.StatusSuccess, response.Status)
		assert.Equal(t, childState.Output, response.Output)
	})

	t.Run("Should extract strategy from config", func(t *testing.T) {
		// Setup test infrastructure
		ts := tkhelpers.NewTestSetup(t)

		// Create parallel response handler
		handler := parallel.NewResponseHandler(ts.TemplateEngine, ts.ContextBuilder, ts.BaseHandler)

		// Create workflow state
		workflowState, workflowExecID := ts.CreateWorkflowState(t, "test-workflow")

		// Create task state
		taskState := ts.CreateTaskState(t, &tkhelpers.TaskStateConfig{
			WorkflowID:     "test-workflow",
			WorkflowExecID: workflowExecID,
			TaskID:         "test-strategy-task",
			Status:         core.StatusPending,
		})

		// Test different strategies
		testCases := []struct {
			name     string
			strategy string
			expected task.ParallelStrategy
		}{
			{"wait_all strategy", "wait_all", task.StrategyWaitAll},
			{"fail_fast strategy", "fail_fast", task.StrategyFailFast},
			{"best_effort strategy", "best_effort", task.StrategyBestEffort},
			{"race strategy", "race", task.StrategyRace},
			{"default strategy", "", task.StrategyWaitAll},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				inputData := &core.Input{}
				if tc.strategy != "" {
					(*inputData)["strategy"] = tc.strategy
				}

				input := &shared.ResponseInput{
					TaskConfig: &task.Config{
						BaseConfig: task.BaseConfig{
							ID:   "test-strategy-task",
							Type: task.TaskTypeParallel,
							With: inputData,
						},
					},
					TaskState:      taskState,
					WorkflowConfig: &workflow.Config{ID: "test-workflow"},
					WorkflowState:  workflowState,
				}

				// Process the response
				result, err := handler.HandleResponse(ts.Context, input)
				require.NoError(t, err)
				assert.NotNil(t, result)
			})
		}
	})
}
