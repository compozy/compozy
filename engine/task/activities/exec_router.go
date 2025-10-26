package activities

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/task/tasks"
	"github.com/compozy/compozy/engine/task/tasks/shared"
	"github.com/compozy/compozy/engine/task/uc"
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
	tasksFactory       tasks.Factory
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
	tasksFactory tasks.Factory,
	templateEngine *tplengine.TemplateEngine,
	evaluator *task.CELEvaluator,
) (*ExecuteRouter, error) {
	if evaluator == nil {
		return nil, fmt.Errorf("condition evaluator is required")
	}
	return &ExecuteRouter{
		loadWorkflowUC:     uc.NewLoadWorkflow(workflows, workflowRepo),
		createStateUC:      uc.NewCreateState(taskRepo, configStore),
		tasksFactory:       tasksFactory,
		templateEngine:     templateEngine,
		conditionEvaluator: evaluator,
	}, nil
}

func (a *ExecuteRouter) Run(ctx context.Context, input *ExecuteRouterInput) (*task.MainTaskResponse, error) {
	if err := validateExecuteRouterInput(input); err != nil {
		return nil, err
	}
	workflowState, workflowConfig, err := a.loadWorkflowUC.Execute(ctx, &uc.LoadWorkflowInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
	})
	if err != nil {
		return nil, err
	}
	normalizedConfig, normContext, err := a.normalizeRouterConfig(ctx, workflowState, workflowConfig, input.TaskConfig)
	if err != nil {
		return nil, err
	}
	taskState, err := a.createStateUC.Execute(ctx, &uc.CreateStateInput{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     normalizedConfig,
	})
	if err != nil {
		return nil, err
	}
	evaluation := a.evaluateRouter(ctx, normalizedConfig, workflowState, workflowConfig, normContext, taskState)
	if evaluation.executionErr == nil {
		taskState.Output = evaluation.output
	}
	response, err := a.buildRouterResponse(ctx, normalizedConfig, workflowState, workflowConfig, taskState, evaluation)
	if err != nil {
		return nil, err
	}
	return response, evaluation.executionErr
}

// routerEvaluationResult captures the outcome of router evaluation.
type routerEvaluationResult struct {
	output       *core.Output
	route        *task.Config
	executionErr error
}

// buildRouterResponse renders the final task response using tasks handlers.
func (a *ExecuteRouter) buildRouterResponse(
	ctx context.Context,
	taskConfig *task.Config,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskState *task.State,
	evaluation *routerEvaluationResult,
) (*task.MainTaskResponse, error) {
	handler, err := a.tasksFactory.CreateResponseHandler(ctx, task.TaskTypeRouter)
	if err != nil {
		return nil, fmt.Errorf("failed to create router response handler: %w", err)
	}
	responseInput := buildRouterResponseInput(
		taskConfig,
		taskState,
		workflowConfig,
		workflowState,
		evaluation.executionErr,
		evaluation.route,
	)
	result, err := handler.HandleResponse(ctx, responseInput)
	if err != nil {
		return nil, fmt.Errorf("failed to handle router response: %w", err)
	}
	return NewResponseConverter().ConvertToMainTaskResponse(result), nil
}

// evaluateRouter determines the next route and collects evaluation metadata.
func (a *ExecuteRouter) evaluateRouter(
	ctx context.Context,
	taskConfig *task.Config,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	normCtx *shared.NormalizationContext,
	taskState *task.State,
) *routerEvaluationResult {
	expression, err := ensureRouterTask(taskConfig)
	if err != nil {
		return &routerEvaluationResult{executionErr: err}
	}
	if err := a.conditionEvaluator.ValidateExpression(expression); err != nil {
		return &routerEvaluationResult{executionErr: fmt.Errorf("invalid condition expression: %w", err)}
	}
	evalContext, err := a.buildEvaluationData(normCtx, workflowState, taskState)
	if err != nil {
		return &routerEvaluationResult{executionErr: err}
	}
	conditionResult, err := a.evaluateConditionResult(ctx, expression, evalContext)
	if err != nil {
		return &routerEvaluationResult{executionErr: err}
	}
	routeTask, routeErr := a.resolveRouterRoute(taskConfig, workflowConfig, conditionResult)
	if routeErr != nil {
		return &routerEvaluationResult{executionErr: routeErr}
	}
	return &routerEvaluationResult{
		output: buildRouterOutput(conditionResult, routeTask.ID),
		route:  routeTask,
	}
}

// ensureRouterTask validates core router configuration constraints.
func ensureRouterTask(taskConfig *task.Config) (string, error) {
	if taskConfig.Type != task.TaskTypeRouter {
		return "", fmt.Errorf("task is not a router task")
	}
	expression := strings.TrimSpace(taskConfig.Condition)
	if expression == "" {
		return "", fmt.Errorf("condition is required for router task")
	}
	if len(taskConfig.Routes) == 0 {
		return "", fmt.Errorf("routes are required for router task")
	}
	return expression, nil
}

// evaluateConditionResult executes the CEL expression and coerces it to string.
func (a *ExecuteRouter) evaluateConditionResult(
	ctx context.Context,
	expression string,
	evalContext map[string]any,
) (string, error) {
	value, err := a.conditionEvaluator.EvaluateValue(ctx, expression, evalContext)
	if err != nil {
		return "", fmt.Errorf("failed to evaluate condition: %w", err)
	}
	if value == nil {
		return "", fmt.Errorf("router condition evaluated to nil")
	}
	conditionResult := strings.TrimSpace(fmt.Sprint(value))
	if conditionResult == "" {
		return "", fmt.Errorf("condition evaluated to empty value")
	}
	return conditionResult, nil
}

// resolveRouterRoute retrieves the next task configuration for the given result.
func (a *ExecuteRouter) resolveRouterRoute(
	taskConfig *task.Config,
	workflowConfig *workflow.Config,
	conditionResult string,
) (*task.Config, error) {
	routeValue, exists := taskConfig.Routes[conditionResult]
	if !exists {
		return nil, fmt.Errorf(
			"no route found for condition result '%s'. Available routes: %v",
			conditionResult,
			getRouteKeys(taskConfig.Routes),
		)
	}
	nextTask, err := a.resolveRoute(routeValue, workflowConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve route '%s': %w", conditionResult, err)
	}
	return nextTask, nil
}

// buildRouterOutput assembles the telemetry payload for router evaluation results.
func buildRouterOutput(conditionResult, nextTaskID string) *core.Output {
	return &core.Output{
		"condition":   conditionResult,
		"route_taken": nextTaskID,
		"router_type": "conditional",
	}
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
	sort.Strings(keys)
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

// validateExecuteRouterInput ensures the router request contains valid parameters.
func validateExecuteRouterInput(input *ExecuteRouterInput) error {
	if input == nil {
		return fmt.Errorf("input is required")
	}
	if input.TaskConfig == nil {
		return fmt.Errorf("task config is required")
	}
	if input.TaskConfig.Type != task.TaskTypeRouter {
		return fmt.Errorf("unsupported task type: %s", input.TaskConfig.Type)
	}
	return nil
}

// normalizeRouterConfig renders router templates and returns the normalization context.
func (a *ExecuteRouter) normalizeRouterConfig(
	ctx context.Context,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
) (*task.Config, *shared.NormalizationContext, error) {
	normalizer, err := a.tasksFactory.CreateNormalizer(ctx, task.TaskTypeRouter)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create router normalizer: %w", err)
	}
	contextBuilder, err := shared.NewContextBuilderWithContext(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create context builder: %w", err)
	}
	normContext := contextBuilder.BuildContext(ctx, workflowState, workflowConfig, taskConfig)
	if err := normalizer.Normalize(ctx, taskConfig, normContext); err != nil {
		return nil, nil, fmt.Errorf("failed to normalize router task: %w", err)
	}
	return taskConfig, normContext, nil
}

// buildRouterResponseInput prepares the shared response input for router handlers.
func buildRouterResponseInput(
	taskConfig *task.Config,
	taskState *task.State,
	workflowConfig *workflow.Config,
	workflowState *workflow.State,
	executionError error,
	routeResult *task.Config,
) *shared.ResponseInput {
	return &shared.ResponseInput{
		TaskConfig:       taskConfig,
		TaskState:        taskState,
		WorkflowConfig:   workflowConfig,
		WorkflowState:    workflowState,
		ExecutionError:   executionError,
		NextTaskOverride: routeResult,
	}
}
