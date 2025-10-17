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
	loadWorkflowUC     *uc.LoadWorkflow
	createStateUC      *uc.CreateState
	task2Factory       task2.Factory
	templateEngine     *tplengine.TemplateEngine
	conditionEvaluator *task.CELEvaluator
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
	evaluator *task.CELEvaluator,
) (*ExecuteRouter, error) {
	if evaluator == nil {
		return nil, fmt.Errorf("condition evaluator is required")
	}
	return &ExecuteRouter{
		loadWorkflowUC:     uc.NewLoadWorkflow(workflows, workflowRepo),
		createStateUC:      uc.NewCreateState(taskRepo, configStore),
		task2Factory:       task2Factory,
		templateEngine:     templateEngine,
		conditionEvaluator: evaluator,
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
	output, routeResult, executionError := a.evaluateRouter(
		ctx,
		normalizedConfig,
		workflowState,
		workflowConfig,
		normContext,
		taskState,
	)
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
	ctx context.Context,
	taskConfig *task.Config,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	normCtx *shared.NormalizationContext,
	taskState *task.State,
) (*core.Output, *task.Config, error) {
	if taskConfig.Type != task.TaskTypeRouter {
		return nil, nil, fmt.Errorf("task is not a router task")
	}

	expression := strings.TrimSpace(taskConfig.Condition)
	if expression == "" {
		return nil, nil, fmt.Errorf("condition is required for router task")
	}
	if len(taskConfig.Routes) == 0 {
		return nil, nil, fmt.Errorf("routes are required for router task")
	}

	if err := a.conditionEvaluator.ValidateExpression(expression); err != nil {
		return nil, nil, fmt.Errorf("invalid condition expression: %w", err)
	}

	evalContext, err := a.buildEvaluationData(normCtx, workflowState, taskState)
	if err != nil {
		return nil, nil, err
	}
	value, err := a.conditionEvaluator.EvaluateValue(ctx, expression, evalContext)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to evaluate condition: %w", err)
	}
	if value == nil {
		return nil, nil, fmt.Errorf("router condition evaluated to nil")
	}
	conditionResult := strings.TrimSpace(fmt.Sprint(value))
	if conditionResult == "" {
		return nil, nil, fmt.Errorf("condition evaluated to empty value")
	}

	routeValue, exists := taskConfig.Routes[conditionResult]
	if !exists {
		return nil, nil, fmt.Errorf(
			"no route found for condition result '%s'. Available routes: %v",
			conditionResult,
			getRouteKeys(taskConfig.Routes),
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

func (a *ExecuteRouter) buildEvaluationData(
	normCtx *shared.NormalizationContext,
	workflowState *workflow.State,
	taskState *task.State,
) (map[string]any, error) {
	var base map[string]any
	if normCtx != nil {
		templateContext := normCtx.BuildTemplateContext()
		if templateContext != nil {
			copied, err := core.DeepCopy[map[string]any](templateContext)
			if err != nil {
				return nil, fmt.Errorf("failed to copy template context: %w", err)
			}
			base = copied
		}
	}
	if base == nil {
		base = make(map[string]any)
	}
	a.enrichWorkflowContext(base, workflowState)
	a.enrichTaskContext(base, taskState)
	return base, nil
}

func (a *ExecuteRouter) enrichWorkflowContext(base map[string]any, workflowState *workflow.State) {
	if workflowState == nil {
		return
	}
	wf, ok := base["workflow"].(map[string]any)
	if !ok || wf == nil {
		wf = make(map[string]any)
		base["workflow"] = wf
	}
	wf["id"] = workflowState.WorkflowID
	wf["status"] = workflowState.Status
	wf["exec_id"] = workflowState.WorkflowExecID.String()
	if workflowState.Input != nil {
		wf["input"] = *workflowState.Input
	}
	if workflowState.Output != nil {
		wf["output"] = *workflowState.Output
	}
	if workflowState.Error != nil {
		wf["error"] = workflowState.Error
	}
}

func (a *ExecuteRouter) enrichTaskContext(base map[string]any, taskState *task.State) {
	if taskState == nil {
		return
	}
	taskMap, ok := base["task"].(map[string]any)
	if !ok || taskMap == nil {
		taskMap = make(map[string]any)
		base["task"] = taskMap
	}
	taskMap["id"] = taskState.TaskID
	taskMap["status"] = taskState.Status
	taskMap["component"] = taskState.Component
	if taskState.Input != nil {
		taskMap["input"] = taskState.Input
	}
	if taskState.Output != nil {
		taskMap["output"] = taskState.Output
	}
	if taskState.Error != nil {
		taskMap["error"] = taskState.Error
	}
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
