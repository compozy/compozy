package executors

import (
	"errors"
	"fmt"

	temporalLog "go.temporal.io/sdk/log"
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
	waitExecutor      *TaskWaitExecutor
	memoryExecutor    *TaskMemoryExecutor
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
	e.waitExecutor = NewTaskWaitExecutor(contextBuilder, e.HandleExecution)
	e.memoryExecutor = NewTaskMemoryExecutor(contextBuilder)
	containerHelpers := NewContainerHelpers(contextBuilder, e.HandleExecution)
	e.parallelExecutor = NewParallelTaskExecutor(containerHelpers)
	e.collectionExecutor = NewCollectionTaskExecutor(containerHelpers)
	e.compositeExecutor = NewCompositeTaskExecutor(containerHelpers)
	return e
}

func (e *TaskExecutor) ExecuteFirstTask() func(ctx workflow.Context) (task.Response, error) {
	return func(ctx workflow.Context) (task.Response, error) {
		ctx = e.BuildTaskContext(ctx, e.InitialTaskID)

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
		if response == nil {
			log.Error("Received nil response")
			return nil, fmt.Errorf("received nil response")
		}
		taskConfig := response.GetNextTask()
		if taskConfig == nil {
			log.Info("No next task to execute")
			return nil, nil
		}
		taskID := taskConfig.ID
		ctx = e.BuildTaskContext(ctx, taskID)
		if err := e.sleepTask(ctx, taskConfig); err != nil {
			return nil, err
		}
		taskResponse, err := e.HandleExecution(ctx, taskConfig, 0)
		if err != nil {
			return nil, err
		}
		if taskResponse.GetNextTask() == nil {
			log.Info("No more tasks to execute", "task_id", taskID)
			return nil, nil
		}
		nextTaskID := taskResponse.GetNextTask().ID
		if nextTaskID == "" {
			log.Error("NextTask has empty ID", "current_task", taskID)
			return nil, fmt.Errorf("next task has empty ID for current task: %s", taskID)
		}
		return taskResponse, nil
	}
}

var errUnsupportedTaskType = errors.New("unsupported execution type")

// HandleExecution dispatches task execution to the appropriate executor based on task type
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
	if taskType == "" {
		taskType = task.TaskTypeBasic
	}
	response, err := e.executeByType(ctx, taskConfig, taskType, currentDepth, log)
	if err != nil {
		if !errors.Is(err, errUnsupportedTaskType) {
			log.Error("Failed to execute task", "task_id", taskID, "depth", currentDepth, "error", err)
		}
		return nil, err
	}
	if response == nil {
		log.Error("Nil response returned from task execution", "task_id", taskID)
		return nil, fmt.Errorf("nil response returned for task %s", taskID)
	}
	state := response.GetState()
	if state == nil {
		log.Error("Nil state in task response", "task_id", taskID)
		return nil, fmt.Errorf("nil state returned for task %s", taskID)
	}
	log.Debug("Task executed successfully",
		"status", state.Status,
		"task_id", taskID,
		"depth", currentDepth,
	)
	return response, nil
}

// executeByType delegates execution to the appropriate executor based on task type.
func (e *TaskExecutor) executeByType(
	ctx workflow.Context,
	taskConfig *task.Config,
	taskType task.Type,
	depth int,
	log temporalLog.Logger,
) (task.Response, error) {
	switch taskType {
	case task.TaskTypeBasic:
		return e.basicExecutor.Execute(ctx, taskConfig)
	case task.TaskTypeRouter:
		return e.routerExecutor.Execute(ctx, taskConfig)
	case task.TaskTypeParallel:
		return e.parallelExecutor.Execute(ctx, taskConfig, depth)
	case task.TaskTypeCollection:
		return e.collectionExecutor.Execute(ctx, taskConfig, depth)
	case task.TaskTypeAggregate:
		return e.aggregateExecutor.Execute(ctx, taskConfig)
	case task.TaskTypeComposite:
		return e.compositeExecutor.Execute(ctx, taskConfig, depth)
	case task.TaskTypeSignal:
		return e.signalExecutor.Execute(ctx, taskConfig)
	case task.TaskTypeWait:
		return e.waitExecutor.Execute(ctx, taskConfig)
	case task.TaskTypeMemory:
		return e.memoryExecutor.Execute(ctx, taskConfig)
	default:
		log.Error(
			"Unsupported execution type encountered",
			"task_type", taskType,
			"task_id", taskConfig.ID,
			"type_length", len(string(taskType)),
		)
		return nil, fmt.Errorf("%w: %s", errUnsupportedTaskType, taskType)
	}
}

func (e *TaskExecutor) sleepTask(ctx workflow.Context, taskConfig *task.Config) error {
	executor := NewTaskSleepExecutor(e.ContextBuilder)
	_, err := executor.Execute(ctx, taskConfig)
	if err != nil {
		return err
	}
	return nil
}
