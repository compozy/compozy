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

// HandleExecution dispatches task execution to the appropriate executor based on task type
//
//nolint:gocyclo // This is a dispatcher function with necessary complexity
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
	maxDepth := 20 // default fallback
	if e.ProjectConfig != nil && e.ProjectConfig.Opts.MaxNestingDepth > 0 {
		maxDepth = e.ProjectConfig.Opts.MaxNestingDepth
	}
	if currentDepth >= maxDepth {
		return nil, fmt.Errorf("maximum nesting depth reached: %d (limit: %d)", currentDepth, maxDepth)
	}
	var response task.Response
	var err error

	// Handle empty task type by defaulting to basic
	if taskType == "" {
		taskType = task.TaskTypeBasic
	}

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
	case task.TaskTypeWait:
		response, err = e.waitExecutor.Execute(ctx, taskConfig)
	case task.TaskTypeMemory:
		response, err = e.memoryExecutor.Execute(ctx, taskConfig)
	default:
		log.Error(
			"Unsupported execution type encountered",
			"task_type",
			taskType,
			"task_id",
			taskID,
			"type_length",
			len(string(taskType)),
		)
		return nil, fmt.Errorf("unsupported execution type: %s", taskType)
	}
	if err != nil {
		log.Error("Failed to execute task", "task_id", taskID, "depth", currentDepth, "error", err)
		return nil, err
	}
	// Validate response and state before accessing
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

func (e *TaskExecutor) sleepTask(ctx workflow.Context, taskConfig *task.Config) error {
	executor := NewTaskSleepExecutor(e.ContextBuilder)
	_, err := executor.Execute(ctx, taskConfig)
	if err != nil {
		return err
	}
	return nil
}
