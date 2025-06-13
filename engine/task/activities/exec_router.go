package activities

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/engine/workflow"
)

const ExecuteRouterLabel = "ExecuteRouterTask"

type ExecuteRouterInput struct {
	WorkflowID     string       `json:"workflow_id"`
	WorkflowExecID core.ID      `json:"workflow_exec_id"`
	TaskConfig     *task.Config `json:"task_config"`
}

type ExecuteRouter struct {
	loadWorkflowUC *uc.LoadWorkflow
	createStateUC  *uc.CreateState
	taskResponder  *services.TaskResponder
}

// NewExecuteRouter creates a new ExecuteRouter activity
func NewExecuteRouter(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	configStore services.ConfigStore,
	cwd *core.PathCWD,
) *ExecuteRouter {
	configManager := services.NewConfigManager(configStore, cwd)
	return &ExecuteRouter{
		loadWorkflowUC: uc.NewLoadWorkflow(workflows, workflowRepo),
		createStateUC:  uc.NewCreateState(taskRepo, configManager),
		taskResponder:  services.NewTaskResponder(workflowRepo, taskRepo),
	}
}

func (a *ExecuteRouter) Run(ctx context.Context, input *ExecuteRouterInput) (*task.MainTaskResponse, error) {
	// Validate task type early
	taskConfig := input.TaskConfig
	if taskConfig == nil {
		return nil, fmt.Errorf("task config is required")
	}
	taskType := taskConfig.Type
	if taskType != task.TaskTypeRouter {
		return nil, fmt.Errorf("unsupported task type: %s", taskType)
	}
	// Load workflow state and config
	workflowState, workflowConfig, err := a.loadWorkflowUC.Execute(ctx, &uc.LoadWorkflowInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
	})
	if err != nil {
		return nil, err
	}
	// Normalize task config
	normalizer := uc.NewNormalizeConfig()
	normalizeInput := &uc.NormalizeConfigInput{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     input.TaskConfig,
	}
	err = normalizer.Execute(ctx, normalizeInput)
	if err != nil {
		return nil, err
	}
	// Create task state
	taskState, err := a.createStateUC.Execute(ctx, &uc.CreateStateInput{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     taskConfig,
	})
	if err != nil {
		return nil, err
	}
	output, routeResult, err := a.evaluateRouter(taskConfig, workflowConfig)
	if err != nil {
		return nil, err
	}
	taskState.Output = output
	// Use the NextTaskOverride to specify the router route directly
	var nextTaskOverride *task.Config
	if routeResult != nil {
		nextTaskOverride = routeResult
	}
	var executionError error // keep the variable for the responder signature
	return a.taskResponder.HandleMainTask(ctx, &services.MainTaskResponseInput{
		TaskState:        taskState,
		WorkflowConfig:   workflowConfig,
		TaskConfig:       taskConfig,
		ExecutionError:   executionError,
		NextTaskOverride: nextTaskOverride,
	})
}

func (a *ExecuteRouter) evaluateRouter(
	taskConfig *task.Config,
	workflowConfig *workflow.Config,
) (*core.Output, *task.Config, error) {
	if taskConfig.Type != task.TaskTypeRouter {
		return nil, nil, fmt.Errorf("task is not a router task")
	}

	condition := taskConfig.Condition
	routes := taskConfig.Routes
	if condition == "" {
		return nil, nil, fmt.Errorf("condition is required for router task")
	}
	if len(routes) == 0 {
		return nil, nil, fmt.Errorf("routes are required for router task")
	}
	// After normalization, the condition contains the evaluated result
	// Use it directly as a route key
	conditionResult := strings.TrimSpace(condition)
	if conditionResult == "" {
		return nil, nil, fmt.Errorf("condition evaluated to empty value")
	}
	// Look up the route using the condition result as the key
	routeValue, exists := routes[conditionResult]
	if !exists {
		return nil, nil, fmt.Errorf(
			"no route found for condition result '%s'. Available routes: %v",
			conditionResult,
			getRouteKeys(routes),
		)
	}
	nextTask, err := a.resolveRoute(routeValue, workflowConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve route '%s': %w", conditionResult, err)
	}
	output := &core.Output{
		"condition":   conditionResult,
		"route_taken": nextTask.ID,
		"router_type": "conditional",
	}
	return output, nextTask, nil
}

// getRouteKeys returns a slice of available route keys for error messages
func getRouteKeys(routes map[string]any) []string {
	keys := make([]string, 0, len(routes))
	for key := range routes {
		keys = append(keys, key)
	}
	return keys
}

func (a *ExecuteRouter) resolveRoute(routeValue any, workflowConfig *workflow.Config) (*task.Config, error) {
	switch route := routeValue.(type) {
	case string:
		return task.FindConfig(workflowConfig.Tasks, route)
	case map[string]any:
		taskConfig, err := core.FromMapDefault[task.Config](route)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal route: %w", err)
		}
		return &taskConfig, nil
	default:
		return nil, fmt.Errorf("invalid route type: %T", route)
	}
}
