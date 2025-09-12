package activities

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/engine/task2"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
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
	task2Factory   task2.Factory
	templateEngine *tplengine.TemplateEngine
}

// NewExecuteRouter creates a new ExecuteRouter activity
func NewExecuteRouter(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	configStore services.ConfigStore,
	_ *core.PathCWD,
	task2Factory task2.Factory,
	templateEngine *tplengine.TemplateEngine,
) (*ExecuteRouter, error) {
	return &ExecuteRouter{
		loadWorkflowUC: uc.NewLoadWorkflow(workflows, workflowRepo),
		createStateUC:  uc.NewCreateState(taskRepo, configStore),
		task2Factory:   task2Factory,
		templateEngine: templateEngine,
	}, nil
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
	// Use task2 normalizer for router tasks
	normalizer, err := a.task2Factory.CreateNormalizer(task.TaskTypeRouter)
	if err != nil {
		return nil, fmt.Errorf("failed to create router normalizer: %w", err)
	}
	// Create context builder to build proper normalization context
	contextBuilder, err := shared.NewContextBuilderWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create context builder: %w", err)
	}
	// Build proper normalization context with all template variables
	normContext := contextBuilder.BuildContext(workflowState, workflowConfig, input.TaskConfig)
	// Normalize the task configuration
	normalizedConfig := input.TaskConfig
	if err := normalizer.Normalize(normalizedConfig, normContext); err != nil {
		return nil, fmt.Errorf("failed to normalize router task: %w", err)
	}
	// Create task state
	taskState, err := a.createStateUC.Execute(ctx, &uc.CreateStateInput{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     normalizedConfig,
	})
	if err != nil {
		return nil, err
	}
	output, routeResult, executionError := a.evaluateRouter(normalizedConfig, workflowConfig)
	if executionError == nil {
		taskState.Output = output
	}
	// Use task2 ResponseHandler for router type
	handler, err := a.task2Factory.CreateResponseHandler(ctx, task.TaskTypeRouter)
	if err != nil {
		return nil, fmt.Errorf("failed to create router response handler: %w", err)
	}
	// Prepare input for response handler
	responseInput := &shared.ResponseInput{
		TaskConfig:       normalizedConfig,
		TaskState:        taskState,
		WorkflowConfig:   workflowConfig,
		WorkflowState:    workflowState,
		ExecutionError:   executionError,
		NextTaskOverride: routeResult,
	}
	// Handle the response
	result, err := handler.HandleResponse(ctx, responseInput)
	if err != nil {
		return nil, fmt.Errorf("failed to handle router response: %w", err)
	}
	// Convert shared.ResponseOutput to task.MainTaskResponse
	converter := NewResponseConverter()
	mainTaskResponse := converter.ConvertToMainTaskResponse(result)
	return mainTaskResponse, executionError
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
