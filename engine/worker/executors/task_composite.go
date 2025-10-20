package executors

import (
	"fmt"
	"sort"

	temporalLog "go.temporal.io/sdk/log"
	"go.temporal.io/sdk/workflow"

	"github.com/compozy/compozy/engine/core"
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
		compositeState, err := e.CreateCompositeState(ctx, config)
		if err != nil {
			return nil, err
		}
		childStates, err := e.ListChildStates(ctx, compositeState.TaskExecID)
		if err != nil {
			return nil, fmt.Errorf("failed to list child states: %w", err)
		}
		childCfgs, err := e.loadCompositeChildConfigs(ctx, compositeState.TaskExecID, childStates)
		if err != nil {
			return nil, err
		}
		e.sortChildStates(childStates, config.Tasks)
		if err := e.executeCompositeChildren(
			ctx,
			log,
			compositeState,
			childStates,
			childCfgs,
			config,
			currentDepth,
		); err != nil {
			return nil, err
		}
		return e.GetCompositeResponse(ctx, compositeState)
	}
}

// loadCompositeChildConfigs fetches child task configs for a composite parent.
func (e *CompositeTaskExecutor) loadCompositeChildConfigs(
	ctx workflow.Context,
	parentTaskExecID core.ID,
	childStates []*task.State,
) (map[string]*task.Config, error) {
	childIDs := make([]string, len(childStates))
	for i, st := range childStates {
		childIDs[i] = st.TaskID
	}
	var childCfgs map[string]*task.Config
	err := workflow.ExecuteActivity(ctx, tkacts.LoadCompositeConfigsLabel, &tkacts.LoadCompositeConfigsInput{
		ParentTaskExecID: parentTaskExecID,
		TaskIDs:          childIDs,
	}).Get(ctx, &childCfgs)
	if err != nil {
		return nil, fmt.Errorf("failed to load child configs: %w", err)
	}
	return childCfgs, nil
}

// sortChildStates orders child states to match the composite configuration.
func (e *CompositeTaskExecutor) sortChildStates(childStates []*task.State, tasks []task.Config) {
	sort.Slice(childStates, func(i, j int) bool {
		iIdx := e.findTaskIndex(tasks, childStates[i].TaskID)
		jIdx := e.findTaskIndex(tasks, childStates[j].TaskID)
		return iIdx < jIdx
	})
}

// executeCompositeChildren runs composite subtasks sequentially, aborting on failure.
func (e *CompositeTaskExecutor) executeCompositeChildren(
	ctx workflow.Context,
	log temporalLog.Logger,
	compositeState *task.State,
	childStates []*task.State,
	childCfgs map[string]*task.Config,
	config *task.Config,
	currentDepth int,
) error {
	for index, childState := range childStates {
		childConfig := childCfgs[childState.TaskID]
		if err := e.executeChild(ctx, compositeState.TaskExecID, childState, childConfig, currentDepth); err != nil {
			log.Error("Child task failed",
				"composite_task", config.ID,
				"child_task", childState.TaskID,
				"index", index,
				"depth", currentDepth+1,
				"error", err)
			return fmt.Errorf("child task %s failed: %w", childState.TaskID, err)
		}
		log.Debug("Child task completed",
			"composite_task", config.ID,
			"child_task", childState.TaskID,
			"index", index,
			"depth", currentDepth+1)
	}
	return nil
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
