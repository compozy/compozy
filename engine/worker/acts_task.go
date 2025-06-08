package worker

import (
	"fmt"

	"go.temporal.io/sdk/workflow"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	tkacts "github.com/compozy/compozy/engine/task/activities"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/pkg/logger"
)

type TaskExecutor struct {
	*ContextBuilder
}

func NewTaskExecutor(contextBuilder *ContextBuilder) *TaskExecutor {
	return &TaskExecutor{ContextBuilder: contextBuilder}
}

func (e *TaskExecutor) ExecuteFirstTask() func(ctx workflow.Context) (*task.Response, error) {
	return func(ctx workflow.Context) (*task.Response, error) {
		ctx = e.BuildTaskContext(ctx, e.InitialTaskID)
		loadTaskUC := uc.NewLoadTaskConfig(e.Workflows)
		taskConfig, err := loadTaskUC.Execute(ctx, &uc.LoadTaskConfigInput{
			WorkflowConfig: e.WorkflowConfig,
			TaskID:         e.InitialTaskID,
		})
		if err != nil {
			return nil, err
		}
		return e.HandleExecution(ctx, taskConfig)
	}
}

func (e *TaskExecutor) ExecuteTasks(response *task.Response) func(ctx workflow.Context) (*task.Response, error) {
	return func(ctx workflow.Context) (*task.Response, error) {
		logger := workflow.GetLogger(ctx)

		// Check if response is nil
		if response == nil {
			logger.Error("ExecuteTasks received nil response")
			return nil, fmt.Errorf("received nil task response")
		}

		// Check if there's a next task to execute
		if response.NextTask == nil {
			logger.Info("No next task to execute - workflow should complete")
			return nil, nil
		}

		taskConfig := response.NextTask
		taskID := taskConfig.ID
		ctx = e.BuildTaskContext(ctx, taskID)
		// Sleep if needed
		if err := e.sleepTask(ctx, taskConfig); err != nil {
			return nil, err
		}
		// Execute task
		taskResponse, err := e.HandleExecution(ctx, taskConfig)
		if err != nil {
			return nil, err
		}
		// Check if there's a next task and validate it
		if taskResponse.NextTask == nil {
			logger.Info("No more tasks to execute", "task_id", taskID)
			return taskResponse, nil // Return the response with NextTask = nil
		}
		// Ensure NextTask has a valid ID
		nextTaskID := taskResponse.NextTask.ID
		if nextTaskID == "" {
			logger.Error("NextTask has empty ID", "current_task", taskID)
			return nil, fmt.Errorf("next task has empty ID for current task: %s", taskID)
		}
		return taskResponse, nil
	}
}

func (e *TaskExecutor) HandleExecution(ctx workflow.Context, taskConfig *task.Config) (*task.Response, error) {
	taskID := taskConfig.ID
	taskType := taskConfig.Type
	var response *task.Response
	var err error
	switch taskType {
	case task.TaskTypeBasic:
		executeFn := e.ExecuteBasicTask(taskConfig)
		response, err = executeFn(ctx)
	case task.TaskTypeRouter:
		executeFn := e.ExecuteRouterTask(taskConfig)
		response, err = executeFn(ctx)
	case task.TaskTypeParallel:
		executeFn := e.HandleParallelTask(taskConfig)
		response, err = executeFn(ctx)
	case task.TaskTypeCollection:
		executeFn := e.HandleCollectionTask(taskConfig)
		response, err = executeFn(ctx)
	default:
		return nil, fmt.Errorf("unsupported execution type: %s", taskType)
	}
	if err != nil {
		logger.Error("Failed to execute task", "task_id", taskID, "error", err)
		return nil, err
	}
	if response == nil {
		logger.Error("Task execution returned nil response", "task_id", taskID)
		return nil, fmt.Errorf("task execution returned nil response for task: %s", taskID)
	}
	if response.State == nil {
		logger.Error("Task execution returned response with nil state", "task_id", taskID)
		return nil, fmt.Errorf("task execution returned response with nil state for task: %s", taskID)
	}
	logger.Info("Task executed successfully",
		"status", response.State.Status,
		"task_id", taskID,
	)
	return response, nil
}

func (e *TaskExecutor) ExecuteBasicTask(taskConfig *task.Config) func(ctx workflow.Context) (*task.Response, error) {
	return func(ctx workflow.Context) (*task.Response, error) {
		var response *task.Response
		actLabel := tkacts.ExecuteBasicLabel
		actInput := tkacts.ExecuteBasicInput{
			WorkflowID:     e.WorkflowID,
			WorkflowExecID: e.WorkflowExecID,
			TaskConfig:     taskConfig,
		}
		err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, &response)
		if err != nil {
			return nil, err
		}
		return response, nil
	}
}

func (e *TaskExecutor) ExecuteRouterTask(taskConfig *task.Config) func(ctx workflow.Context) (*task.Response, error) {
	return func(ctx workflow.Context) (*task.Response, error) {
		var response *task.Response
		actLabel := tkacts.ExecuteRouterLabel
		actInput := tkacts.ExecuteRouterInput{
			WorkflowID:     e.WorkflowID,
			WorkflowExecID: e.WorkflowExecID,
			TaskConfig:     taskConfig,
		}
		err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, &response)
		if err != nil {
			return nil, err
		}
		return response, nil
	}
}

func (e *TaskExecutor) HandleParallelTask(pConfig *task.Config) func(ctx workflow.Context) (*task.Response, error) {
	return func(ctx workflow.Context) (*task.Response, error) {
		logger := workflow.GetLogger(ctx)
		tasks := pConfig.Tasks
		numTasks := len(tasks)
		results := make([]*task.SubtaskResponse, numTasks)
		completed, failed := 0, 0
		pState, err := e.CreateParallelState(ctx, pConfig)
		if err != nil {
			return nil, err
		}
		// Execute subtasks in parallel
		for i := range tasks {
			taskConfig := tasks[i]
			workflow.Go(ctx, func(gCtx workflow.Context) {
				response, err := e.ExecuteParallelTask(gCtx, pState, &taskConfig)
				if err != nil {
					logger.Error("Failed to execute sub task",
						"parent_task_id", pConfig.ID,
						"sub_task_id", taskConfig.ID,
						"error", err)
					failed++
				} else {
					completed++
					logger.Info("Subtask completed successfully",
						"parent_task_id", pConfig.ID,
						"sub_task_id", taskConfig.ID)
				}
				results[i] = response
			})
		}

		// Wait for tasks to complete based on strategy
		err = workflow.Await(ctx, func() bool {
			strategy := pConfig.GetStrategy()
			switch strategy {
			case task.StrategyWaitAll:
				return (completed + failed) >= numTasks
			case task.StrategyFailFast:
				return failed > 0 || completed >= numTasks
			case task.StrategyBestEffort:
				return (completed + failed) >= numTasks
			case task.StrategyRace:
				return completed > 0 || failed >= numTasks
			default:
				return (completed + failed) >= numTasks
			}
		})
		if err != nil {
			return nil, fmt.Errorf("failed to await parallel task: %w", err)
		}
		// Process parallel response with proper transitions
		finalResponse, err := e.GetParallelResponse(ctx, pState, results, pConfig)
		if err != nil {
			return nil, err
		}
		logger.Info("Parallel task execution completed",
			"task_id", pConfig.ID,
			"completed", completed,
			"failed", failed,
			"total", numTasks,
			"final_status", finalResponse.State.Status)
		return finalResponse, nil
	}
}

func (e *TaskExecutor) CreateParallelState(
	ctx workflow.Context,
	pConfig *task.Config,
) (*task.State, error) {
	var state *task.State
	actLabel := tkacts.CreateParallelStateLabel
	actInput := tkacts.CreateParallelStateInput{
		WorkflowID:     e.WorkflowID,
		WorkflowExecID: e.WorkflowExecID,
		TaskConfig:     pConfig,
	}
	err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, &state)
	if err != nil {
		return nil, err
	}
	return state, nil
}

func (e *TaskExecutor) ExecuteParallelTask(
	ctx workflow.Context,
	pState *task.State,
	taskConfig *task.Config,
) (*task.SubtaskResponse, error) {
	actLabel := tkacts.ExecuteParallelTaskLabel
	actInput := tkacts.ExecuteParallelTaskInput{
		WorkflowID:     e.WorkflowID,
		WorkflowExecID: e.WorkflowExecID,
		ParentState:    pState,
		TaskConfig:     taskConfig,
	}
	future := workflow.ExecuteActivity(ctx, actLabel, actInput)
	var response *task.SubtaskResponse
	err := future.Get(ctx, &response)
	if err != nil && response == nil {
		response = &task.SubtaskResponse{
			TaskID: taskConfig.ID,
			Output: nil,
			Status: core.StatusFailed,
			Error:  core.NewError(err, "subtask_execution_failed", nil),
		}
	}
	return response, err
}

func (e *TaskExecutor) GetParallelResponse(
	ctx workflow.Context,
	pState *task.State,
	results []*task.SubtaskResponse,
	pConfig *task.Config,
) (*task.Response, error) {
	var response *task.Response
	actLabel := tkacts.GetParallelResponseLabel
	actInput := tkacts.GetParallelResponseInput{
		ParentState:    pState,
		Results:        results,
		WorkflowConfig: e.WorkflowConfig,
		TaskConfig:     pConfig,
	}
	err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, &response)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (e *TaskExecutor) sleepTask(ctx workflow.Context, taskConfig *task.Config) error {
	// Check if task has sleep configuration
	taskID := taskConfig.ID
	sleepDuration, err := taskConfig.GetSleepDuration()
	if err != nil {
		logger.Error("Invalid sleep duration format", "task_id", taskID, "sleep", taskConfig.Sleep, "error", err)
		return err
	}
	if sleepDuration != 0 {
		if err := SleepWithPause(ctx, sleepDuration); err != nil {
			if err == workflow.ErrCanceled {
				return nil
			}
			logger.Error("Error during task sleep", "task_id", taskID, "error", err)
			return err
		}
		logger.Info("Task sleep completed", "task_id", taskID)
	}
	return nil
}

func (e *TaskExecutor) HandleCollectionTask(tConfig *task.Config) func(ctx workflow.Context) (*task.Response, error) {
	return func(ctx workflow.Context) (*task.Response, error) {
		logger := workflow.GetLogger(ctx)

		// Step 1: Prepare collection (atomic)
		prepareResult, err := e.PrepareCollection(ctx, tConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to prepare collection: %w", err)
		}

		// Step 2: Process items based on mode
		var itemResults []tkacts.ExecuteCollectionItemResult
		mode := tConfig.GetMode()
		if mode == task.CollectionModeParallel {
			itemResults, err = e.processItemsParallel(ctx, prepareResult, tConfig)
		} else {
			itemResults, err = e.processItemsSequential(ctx, prepareResult, tConfig)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to process collection items: %w", err)
		}

		// Step 3: Aggregate results (atomic)
		finalResponse, err := e.AggregateCollection(ctx, prepareResult, itemResults, tConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to aggregate collection results: %w", err)
		}

		// Check response validity before logging
		if finalResponse == nil {
			logger.Error("AggregateCollection returned nil response")
			return nil, fmt.Errorf("aggregate collection returned nil response")
		}
		if finalResponse.State == nil {
			logger.Error("AggregateCollection returned response with nil state")
			return nil, fmt.Errorf("aggregate collection returned response with nil state")
		}

		logger.Info("Collection task execution completed",
			"task_id", tConfig.ID,
			"total_items", prepareResult.TotalCount,
			"filtered_items", prepareResult.FilteredCount,
			"final_status", finalResponse.State.Status)
		logger.Info("About to return collection response", "has_next_task", finalResponse.NextTask != nil)

		return finalResponse, nil
	}
}

// Collection task execution methods

func (e *TaskExecutor) PrepareCollection(
	ctx workflow.Context,
	tConfig *task.Config,
) (*tkacts.PrepareCollectionResult, error) {
	var result *tkacts.PrepareCollectionResult
	actLabel := tkacts.PrepareCollectionLabel
	actInput := tkacts.PrepareCollectionInput{
		WorkflowID:     e.WorkflowID,
		WorkflowExecID: e.WorkflowExecID,
		TaskConfig:     tConfig,
	}
	err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (e *TaskExecutor) processItemsParallel(
	ctx workflow.Context,
	prepareResult *tkacts.PrepareCollectionResult,
	tConfig *task.Config,
) ([]tkacts.ExecuteCollectionItemResult, error) {
	// Get collection state and evaluate items if needed
	collectionState := prepareResult.CollectionState
	if collectionState.CollectionState == nil {
		return nil, fmt.Errorf("collection state is nil")
	}

	// Evaluate dynamic items if needed
	items, err := e.evaluateCollectionItems(ctx, collectionState, tConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate collection items: %w", err)
	}

	itemCount := len(items)
	if itemCount == 0 {
		return []tkacts.ExecuteCollectionItemResult{}, nil
	}

	// Use futures for deterministic parallel execution
	futures := make([]workflow.Future, itemCount)
	for i, item := range items {
		itemIndex := i
		itemValue := item
		futures[i] = workflow.ExecuteActivity(
			ctx,
			tkacts.ExecuteCollectionItemLabel,
			&tkacts.ExecuteCollectionItemInput{
				ParentTaskExecID: prepareResult.TaskExecID,
				ItemIndex:        itemIndex,
				Item:             itemValue,
				TaskConfig:       tConfig.Template,
			},
		)
	}

	// Wait for all futures and collect results
	results := make([]tkacts.ExecuteCollectionItemResult, itemCount)
	for i, future := range futures {
		var result tkacts.ExecuteCollectionItemResult
		err := future.Get(ctx, &result)
		if err != nil {
			// Create a failed result for activity execution errors
			results[i] = tkacts.ExecuteCollectionItemResult{
				ItemIndex:  i,
				TaskExecID: core.MustNewID(),
				Status:     core.StatusFailed,
				Output:     nil,
				Error:      core.NewError(err, "collection_item_activity_failed", nil),
			}
		} else {
			results[i] = result
		}
	}

	return results, nil
}

func (e *TaskExecutor) processItemsSequential(
	ctx workflow.Context,
	prepareResult *tkacts.PrepareCollectionResult,
	tConfig *task.Config,
) ([]tkacts.ExecuteCollectionItemResult, error) {
	// Get collection state and evaluate items if needed
	collectionState := prepareResult.CollectionState
	if collectionState.CollectionState == nil {
		return nil, fmt.Errorf("collection state is nil")
	}
	// Evaluate dynamic items if needed
	items, err := e.evaluateCollectionItems(ctx, collectionState, tConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate collection items: %w", err)
	}
	if len(items) == 0 {
		return []tkacts.ExecuteCollectionItemResult{}, nil
	}
	batchSize := tConfig.GetBatch()
	var results []tkacts.ExecuteCollectionItemResult
	// Process items in batches
	for i := 0; i < len(items); i += batchSize {
		end := min(i+batchSize, len(items))
		// Process batch
		for j := i; j < end; j++ {
			result, err := e.ExecuteCollectionItem(
				ctx,
				prepareResult.TaskExecID,
				j,
				items[j],
				tConfig.Template,
			)
			if err != nil {
				if !tConfig.ContinueOnError {
					return nil, fmt.Errorf("collection item %d failed: %w", j, err)
				}
				// Create a failed result
				results = append(results, tkacts.ExecuteCollectionItemResult{
					ItemIndex:  j,
					TaskExecID: core.MustNewID(),
					Status:     core.StatusFailed,
					Output:     nil,
					Error:      core.NewError(err, "collection_item_execution_failed", nil),
				})
			} else {
				results = append(results, *result)
			}
		}
	}

	return results, nil
}

// evaluateCollectionItems handles dynamic item evaluation and filtering during execution
func (e *TaskExecutor) evaluateCollectionItems(
	ctx workflow.Context,
	collectionState *task.State,
	cConfig *task.Config,
) ([]any, error) {
	// Check if items are already evaluated and available
	if collectionState.CollectionState.Items != nil && len(collectionState.CollectionState.Items) > 0 {
		return collectionState.CollectionState.Items, nil
	}
	// If items are not evaluated and we have an expression, evaluate it now
	if collectionState.CollectionState.Items == nil && collectionState.CollectionState.ItemsExpression != "" {
		// Execute an activity to evaluate the dynamic items with current workflow state
		var evaluatedItems []any
		actInput := tkacts.EvaluateDynamicItemsInput{
			WorkflowID:       e.WorkflowID,
			WorkflowExecID:   e.WorkflowExecID,
			ItemsExpression:  collectionState.CollectionState.ItemsExpression,
			FilterExpression: collectionState.CollectionState.Filter,
			TaskConfig:       cConfig,
		}
		err := workflow.ExecuteActivity(ctx, tkacts.EvaluateDynamicItemsLabel, actInput).Get(ctx, &evaluatedItems)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate dynamic collection items: %w", err)
		}

		return evaluatedItems, nil
	}

	// Return existing items (static or already evaluated)
	return collectionState.CollectionState.Items, nil
}

func (e *TaskExecutor) ExecuteCollectionItem(
	ctx workflow.Context,
	parentTaskExecID core.ID,
	itemIndex int,
	item any,
	taskConfig *task.Config,
) (*tkacts.ExecuteCollectionItemResult, error) {
	var result *tkacts.ExecuteCollectionItemResult
	actLabel := tkacts.ExecuteCollectionItemLabel
	actInput := tkacts.ExecuteCollectionItemInput{
		ParentTaskExecID: parentTaskExecID,
		ItemIndex:        itemIndex,
		Item:             item,
		TaskConfig:       taskConfig,
	}
	err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (e *TaskExecutor) AggregateCollection(
	ctx workflow.Context,
	prepareResult *tkacts.PrepareCollectionResult,
	itemResults []tkacts.ExecuteCollectionItemResult,
	cConfig *task.Config,
) (*task.Response, error) {
	var response *task.Response
	actLabel := tkacts.AggregateCollectionLabel
	actInput := tkacts.AggregateCollectionInput{
		ParentTaskExecID: prepareResult.TaskExecID,
		ItemResults:      itemResults,
		TaskConfig:       cConfig,
	}
	err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, &response)
	if err != nil {
		return nil, err
	}
	return response, nil
}
