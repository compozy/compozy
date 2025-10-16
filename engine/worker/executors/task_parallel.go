package executors

import (
	"fmt"
	"math"
	"sync/atomic"

	"go.temporal.io/sdk/workflow"

	"github.com/compozy/compozy/engine/task"
	tkacts "github.com/compozy/compozy/engine/task/activities"
)

// ParallelTaskExecutor handles parallel task execution
type ParallelTaskExecutor struct {
	*ContainerHelpers
}

// NewParallelTaskExecutor creates a new parallel task executor
func NewParallelTaskExecutor(helpers *ContainerHelpers) *ParallelTaskExecutor {
	return &ParallelTaskExecutor{
		ContainerHelpers: helpers,
	}
}

// Execute implements the Executor interface for parallel tasks
func (e *ParallelTaskExecutor) Execute(
	ctx workflow.Context,
	taskConfig *task.Config,
	depth int,
) (task.Response, error) {
	return e.HandleParallelTask(taskConfig, depth)(ctx)
}

// HandleParallelTask handles parallel task execution with optional depth parameter
func (e *ParallelTaskExecutor) HandleParallelTask(
	pConfig *task.Config,
	depth ...int,
) func(ctx workflow.Context) (task.Response, error) {
	return func(ctx workflow.Context) (task.Response, error) {
		log := workflow.GetLogger(ctx)
		currentDepth := 0
		if len(depth) > 0 {
			currentDepth = depth[0]
		}
		// TODO: ContinueAsNew guard (history safety) - implement when needed
		// if historyLength > 25000 { return continueAsNew() }
		var completed, failed int32
		pState, childStates, childCfgs, numTasks, err := e.setupParallelExecution(ctx, pConfig)
		if err != nil {
			return nil, err
		}
		// Execute subtasks in parallel using executeChild helper
		maxConcurrency := pConfig.GetMaxWorkers()
		e.executeChildrenInParallel(ctx, pState, childStates, func(cs *task.State) *task.Config {
			return childCfgs[cs.TaskID]
		}, pConfig, currentDepth, &completed, &failed, maxConcurrency)
		// Wait for tasks to complete based on strategy
		err = e.awaitStrategyCompletion(ctx, pConfig.GetStrategy(), &completed, &failed, numTasks)
		if err != nil {
			return nil, fmt.Errorf("failed to await parallel task: %w", err)
		}
		// Process parallel response with proper transitions
		finalResponse, err := e.GetParallelResponse(ctx, pState)
		if err != nil {
			return nil, err
		}
		completedCount := atomic.LoadInt32(&completed)
		failedCount := atomic.LoadInt32(&failed)
		log.Debug("Parallel task execution completed",
			"task_id", pConfig.ID,
			"completed", completedCount,
			"failed", failedCount,
			"total", numTasks,
			"final_status", finalResponse.GetState().Status)
		return finalResponse, nil
	}
}

// CreateParallelState creates a parallel state via activity
func (e *ParallelTaskExecutor) CreateParallelState(
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

// GetParallelResponse gets the final parallel response via activity
func (e *ParallelTaskExecutor) GetParallelResponse(
	ctx workflow.Context,
	pState *task.State,
) (task.Response, error) {
	var response *task.MainTaskResponse
	actLabel := tkacts.GetParallelResponseLabel
	actInput := tkacts.GetParallelResponseInput{
		ParentState:    pState,
		WorkflowConfig: e.WorkflowConfig,
	}
	err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, &response)
	if err != nil {
		return nil, err
	}
	return response, nil
}

// setupParallelExecution sets up the parallel task execution
func (e *ParallelTaskExecutor) setupParallelExecution(
	ctx workflow.Context,
	pConfig *task.Config,
) (*task.State, []*task.State, map[string]*task.Config, int32, error) {
	pState, err := e.CreateParallelState(ctx, pConfig)
	if err != nil {
		return nil, nil, nil, 0, err
	}
	// Get child states that were created by CreateParallelState
	childStates, err := e.ListChildStates(ctx, pState.TaskExecID)
	if err != nil {
		return nil, nil, nil, 0, fmt.Errorf("failed to list child states: %w", err)
	}
	// For nested parallel tasks, use the task's own configs; for root tasks, load from workflow
	var childCfgs map[string]*task.Config
	if len(pConfig.Tasks) > 0 {
		// Nested parallel task - use configs from the task itself
		childCfgs = make(map[string]*task.Config)
		for i := range pConfig.Tasks {
			cfg := &pConfig.Tasks[i]
			childCfgs[cfg.ID] = cfg
		}
	} else {
		// Root parallel task - load configs from workflow
		childIDs := make([]string, len(childStates))
		for i, st := range childStates {
			childIDs[i] = st.TaskID
		}
		err = workflow.ExecuteActivity(ctx, tkacts.LoadBatchConfigsLabel, &tkacts.LoadBatchConfigsInput{
			WorkflowConfig: e.WorkflowConfig,
			TaskIDs:        childIDs,
		}).Get(ctx, &childCfgs)
		if err != nil {
			return nil, nil, nil, 0, fmt.Errorf("failed to load child configs: %w", err)
		}
	}
	tasksLen := len(childStates)
	if int64(tasksLen) > math.MaxInt32 {
		return nil, nil, nil, 0, fmt.Errorf("too many tasks: %d", tasksLen)
	}
	numTasks := int32(tasksLen) // #nosec G115 - overflow check above
	return pState, childStates, childCfgs, numTasks, nil
}
