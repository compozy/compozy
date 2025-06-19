package executors

import (
	"fmt"
	"sort"

	"go.temporal.io/sdk/workflow"

	"github.com/compozy/compozy/engine/task"
	tkacts "github.com/compozy/compozy/engine/task/activities"
)

// CompositeTaskExecutor handles composite task execution
type CompositeTaskExecutor struct {
	*ContainerHelpers
}

// NewCompositeTaskExecutor creates a new composite task executor
func NewCompositeTaskExecutor(helpers *ContainerHelpers) *CompositeTaskExecutor {
	return &CompositeTaskExecutor{
		ContainerHelpers: helpers,
	}
}

// Execute implements the Executor interface for composite tasks
func (e *CompositeTaskExecutor) Execute(
	ctx workflow.Context,
	taskConfig *task.Config,
	depth int,
) (task.Response, error) {
	return e.HandleCompositeTask(taskConfig, depth)(ctx)
}

// HandleCompositeTask handles composite task execution with optional depth parameter
func (e *CompositeTaskExecutor) HandleCompositeTask(
	config *task.Config,
	depth ...int,
) func(ctx workflow.Context) (task.Response, error) {
	return func(ctx workflow.Context) (task.Response, error) {
		log := workflow.GetLogger(ctx)
		currentDepth := 0
		if len(depth) > 0 {
			currentDepth = depth[0]
		}
		// Create parent state for composite task
		compositeState, err := e.CreateCompositeState(ctx, config)
		if err != nil {
			return nil, err
		}
		// Get child states that were created by CreateCompositeState
		childStates, err := e.ListChildStates(ctx, compositeState.TaskExecID)
		if err != nil {
			return nil, fmt.Errorf("failed to list child states: %w", err)
		}
		// Load child configs from composite metadata
		childIDs := make([]string, len(childStates))
		for i, st := range childStates {
			childIDs[i] = st.TaskID
		}
		var childCfgs map[string]*task.Config
		err = workflow.ExecuteActivity(ctx, tkacts.LoadCompositeConfigsLabel, &tkacts.LoadCompositeConfigsInput{
			ParentTaskExecID: compositeState.TaskExecID,
			TaskIDs:          childIDs,
		}).Get(ctx, &childCfgs)
		if err != nil {
			return nil, fmt.Errorf("failed to load child configs: %w", err)
		}
		// Sort child states by task ID to ensure deterministic ordering
		// This matches the order defined in config.Tasks
		sort.Slice(childStates, func(i, j int) bool {
			// Find index of each task in the config
			iIdx := e.findTaskIndex(config.Tasks, childStates[i].TaskID)
			jIdx := e.findTaskIndex(config.Tasks, childStates[j].TaskID)
			return iIdx < jIdx
		})
		// Execute subtasks sequentially (composite tasks are always sequential)
		for i, childState := range childStates {
			childConfig := childCfgs[childState.TaskID]
			// Execute child task
			err := e.executeChild(ctx, compositeState.TaskExecID, childState, childConfig, currentDepth)
			if err != nil {
				log.Error("Child task failed",
					"composite_task", config.ID,
					"child_task", childState.TaskID,
					"index", i,
					"depth", currentDepth+1,
					"error", err)
				// Composite tasks always fail immediately on any child failure
				return nil, fmt.Errorf("child task %s failed: %w", childState.TaskID, err)
			}
			log.Debug("Child task completed",
				"composite_task", config.ID,
				"child_task", childState.TaskID,
				"index", i,
				"depth", currentDepth+1)
		}
		// Generate final response using standard parent task processing
		return e.GetCompositeResponse(ctx, compositeState)
	}
}

// CreateCompositeState creates a composite state via activity
func (e *CompositeTaskExecutor) CreateCompositeState(
	ctx workflow.Context,
	config *task.Config,
) (*task.State, error) {
	var state *task.State
	actLabel := tkacts.CreateCompositeStateLabel
	actInput := tkacts.CreateCompositeStateInput{
		WorkflowID:     e.WorkflowID,
		WorkflowExecID: e.WorkflowExecID,
		TaskConfig:     config,
	}
	err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, &state)
	if err != nil {
		return nil, err
	}
	return state, nil
}

// GetCompositeResponse gets the final composite response via activity
func (e *CompositeTaskExecutor) GetCompositeResponse(
	ctx workflow.Context,
	compositeState *task.State,
) (task.Response, error) {
	var response *task.MainTaskResponse
	actLabel := tkacts.GetCompositeResponseLabel
	actInput := tkacts.GetCompositeResponseInput{
		ParentState:    compositeState,
		WorkflowConfig: e.WorkflowConfig,
	}
	err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, &response)
	if err != nil {
		return nil, err
	}
	return response, nil
}
