package executors

import (
	"fmt"

	"go.temporal.io/sdk/workflow"

	"github.com/compozy/compozy/engine/task"
	tkacts "github.com/compozy/compozy/engine/task/activities"
)

type TaskExecutor struct {
	*ContextBuilder
	// Simple task executors
	basicExecutor     *TaskBasicExecutor
	routerExecutor    *TaskRouterExecutor
	aggregateExecutor *TaskAggregateExecutor
	signalExecutor    *TaskSignalExecutor
	// Container task executors
	parallelExecutor   *ParallelTaskExecutor
	collectionExecutor *CollectionTaskExecutor
	compositeExecutor  *CompositeTaskExecutor
}

func NewTaskExecutor(contextBuilder *ContextBuilder) *TaskExecutor {
	e := &TaskExecutor{ContextBuilder: contextBuilder}
	e.basicExecutor = NewTaskBasicExecutor(contextBuilder)
	e.routerExecutor = NewTaskRouterExecutor(contextBuilder)
	e.aggregateExecutor = NewTaskAggregateExecutor(contextBuilder)
	e.signalExecutor = NewTaskSignalExecutor(contextBuilder)

	// Initialize container task executors
	containerHelpers := NewContainerHelpers(contextBuilder, e.HandleExecution)
	e.parallelExecutor = NewParallelTaskExecutor(containerHelpers)
	e.collectionExecutor = NewCollectionTaskExecutor(containerHelpers)
	e.compositeExecutor = NewCompositeTaskExecutor(containerHelpers)
	return e
}

func (e *TaskExecutor) ExecuteFirstTask() func(ctx workflow.Context) (task.Response, error) {
	return func(ctx workflow.Context) (task.Response, error) {
		ctx = e.BuildTaskContext(ctx, e.InitialTaskID)

		// Load task config via activity (deterministic)
		var taskConfig *task.Config
		actInput := &tkacts.LoadTaskConfigInput{
			WorkflowConfig: e.WorkflowConfig,
			TaskID:         e.InitialTaskID,
		}
		err := workflow.ExecuteActivity(ctx, tkacts.LoadTaskConfigLabel, actInput).Get(ctx, &taskConfig)
		if err != nil {
			return nil, err
		}
		return e.HandleExecution(ctx, taskConfig, 0)
	}
}

func (e *TaskExecutor) ExecuteTasks(response task.Response) func(ctx workflow.Context) (task.Response, error) {
	return func(ctx workflow.Context) (task.Response, error) {
		log := workflow.GetLogger(ctx)
		taskConfig := response.GetNextTask()
		taskID := taskConfig.ID
		ctx = e.BuildTaskContext(ctx, taskID)
		// Sleep if needed
		if err := e.sleepTask(ctx, taskConfig); err != nil {
			return nil, err
		}
		// Execute task
		taskResponse, err := e.HandleExecution(ctx, taskConfig, 0)
		if err != nil {
			return nil, err
		}
		// Dispatch next task if there is one
		if taskResponse.GetNextTask() == nil {
			log.Info("No more tasks to execute", "task_id", taskID)
			return nil, nil
		}
		// Ensure NextTask has a valid ID
		nextTaskID := taskResponse.GetNextTask().ID
		if nextTaskID == "" {
			log.Error("NextTask has empty ID", "current_task", taskID)
			return nil, fmt.Errorf("next task has empty ID for current task: %s", taskID)
		}
		return taskResponse, nil
	}
}

func (e *TaskExecutor) HandleExecution(
	ctx workflow.Context,
	taskConfig *task.Config,
	depth ...int,
) (task.Response, error) {
	log := workflow.GetLogger(ctx)
	taskID := taskConfig.ID
	taskType := taskConfig.Type
	currentDepth := 0
	if len(depth) > 0 {
		currentDepth = depth[0]
	}
	if currentDepth > 20 { // max_nesting_depth from config
		return nil, fmt.Errorf("maximum nesting depth exceeded: %d", currentDepth)
	}
	var response task.Response
	var err error

	switch taskType {
	case task.TaskTypeBasic:
		response, err = e.basicExecutor.Execute(ctx, taskConfig)
	case task.TaskTypeRouter:
		response, err = e.routerExecutor.Execute(ctx, taskConfig)
	case task.TaskTypeParallel:
		response, err = e.parallelExecutor.Execute(ctx, taskConfig, currentDepth)
	case task.TaskTypeCollection:
		response, err = e.collectionExecutor.Execute(ctx, taskConfig, currentDepth)
	case task.TaskTypeAggregate:
		response, err = e.aggregateExecutor.Execute(ctx, taskConfig)
	case task.TaskTypeComposite:
		response, err = e.compositeExecutor.Execute(ctx, taskConfig, currentDepth)
	case task.TaskTypeSignal:
		response, err = e.signalExecutor.Execute(ctx, taskConfig)
	default:
		return nil, fmt.Errorf("unsupported execution type: %s", taskType)
	}
	if err != nil {
		log.Error("Failed to execute task", "task_id", taskID, "depth", currentDepth, "error", err)
		return nil, err
	}
	log.Debug("Task executed successfully",
		"status", response.GetState().Status,
		"task_id", taskID,
		"depth", currentDepth,
	)
	return response, nil
}

func (e *TaskExecutor) sleepTask(ctx workflow.Context, taskConfig *task.Config) error {
	executor := NewTaskSleepExecutor(e.ContextBuilder, taskConfig)
	_, err := executor.Execute(ctx, taskConfig)
	if err != nil {
		return err
	}
	return nil
}
