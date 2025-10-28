package activities

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/task/tasks"
	"github.com/compozy/compozy/engine/task/tasks/shared"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
)

const ExecuteWaitLabel = "ExecuteWaitTask"

type ExecuteWaitInput struct {
	WorkflowID     string       `json:"workflow_id"`
	WorkflowExecID core.ID      `json:"workflow_exec_id"`
	TaskConfig     *task.Config `json:"task_config"`
}

type ExecuteWait struct {
	loadWorkflowUC *uc.LoadWorkflow
	createStateUC  *uc.CreateState
	tasksFactory   tasks.Factory
	templateEngine *tplengine.TemplateEngine
}

// NewExecuteWait creates a new ExecuteWait activity
func NewExecuteWait(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	configStore services.ConfigStore,
	_ *core.PathCWD,
	tasksFactory tasks.Factory,
	templateEngine *tplengine.TemplateEngine,
) (*ExecuteWait, error) {
	return &ExecuteWait{
		loadWorkflowUC: uc.NewLoadWorkflow(workflows, workflowRepo),
		createStateUC:  uc.NewCreateState(taskRepo, configStore),
		tasksFactory:   tasksFactory,
		templateEngine: templateEngine,
	}, nil
}

func (a *ExecuteWait) Run(ctx context.Context, input *ExecuteWaitInput) (*task.MainTaskResponse, error) {
	if err := a.validateWaitInput(input); err != nil {
		return nil, err
	}
	workflowState, workflowConfig, err := a.loadWorkflowContext(ctx, input)
	if err != nil {
		return nil, err
	}
	normalizedConfig, err := a.normalizeWaitTask(ctx, workflowState, workflowConfig, input.TaskConfig)
	if err != nil {
		return nil, err
	}
	if err := validateWaitConfig(normalizedConfig); err != nil {
		return nil, err
	}
	timeout, err := parseTimeout(normalizedConfig)
	if err != nil {
		return nil, err
	}
	taskState, err := a.createWaitTaskState(ctx, workflowState, workflowConfig, normalizedConfig, timeout)
	if err != nil {
		return nil, err
	}
	responseInput := buildWaitResponseInput(normalizedConfig, taskState, workflowConfig, workflowState)
	return a.handleWaitResponse(ctx, responseInput)
}

// validateWaitInput checks that the activity input is properly configured.
func (a *ExecuteWait) validateWaitInput(input *ExecuteWaitInput) error {
	if input == nil {
		return fmt.Errorf("activity input is required")
	}
	if input.TaskConfig == nil {
		return fmt.Errorf("task_config is required for wait task")
	}
	return nil
}

// loadWorkflowContext loads the workflow state and config needed for processing.
func (a *ExecuteWait) loadWorkflowContext(
	ctx context.Context,
	input *ExecuteWaitInput,
) (*workflow.State, *workflow.Config, error) {
	workflowState, workflowConfig, err := a.loadWorkflowUC.Execute(ctx, &uc.LoadWorkflowInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
	})
	if err != nil {
		return nil, nil, err
	}
	return workflowState, workflowConfig, nil
}

// normalizeWaitTask prepares the wait configuration using the tasks normalizer.
func (a *ExecuteWait) normalizeWaitTask(
	ctx context.Context,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	cfg *task.Config,
) (*task.Config, error) {
	normalizer, err := a.tasksFactory.CreateNormalizer(ctx, task.TaskTypeWait)
	if err != nil {
		return nil, fmt.Errorf("failed to create wait normalizer: %w", err)
	}
	contextBuilder, err := shared.NewContextBuilderWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create context builder: %w", err)
	}
	normContext := contextBuilder.BuildContext(ctx, workflowState, workflowConfig, cfg)
	if err := normalizer.Normalize(ctx, cfg, normContext); err != nil {
		return nil, fmt.Errorf("failed to normalize wait task: %w", err)
	}
	return cfg, nil
}

// validateWaitConfig ensures the normalized configuration is a valid wait task.
func validateWaitConfig(cfg *task.Config) error {
	if cfg.Type != task.TaskTypeWait {
		return fmt.Errorf("unsupported task type: %s", cfg.Type)
	}
	if strings.TrimSpace(cfg.WaitFor) == "" {
		return fmt.Errorf("wait task must define a non-empty wait_for signal")
	}
	return nil
}

// parseTimeout parses and validates the timeout from the normalized configuration.
func parseTimeout(cfg *task.Config) (time.Duration, error) {
	if cfg.Timeout == "" {
		return 0, fmt.Errorf("wait task requires a timeout")
	}
	timeout, err := core.ParseHumanDuration(cfg.Timeout)
	if err != nil {
		return 0, fmt.Errorf("invalid timeout format: %w", err)
	}
	if timeout <= 0 {
		return 0, fmt.Errorf("wait task requires a positive timeout (got %s)", cfg.Timeout)
	}
	return timeout, nil
}

// createWaitTaskState creates and augments the state for the wait task.
func (a *ExecuteWait) createWaitTaskState(
	ctx context.Context,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	cfg *task.Config,
	timeout time.Duration,
) (*task.State, error) {
	taskState, err := a.createStateUC.Execute(ctx, &uc.CreateStateInput{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     cfg,
	})
	if err != nil {
		return nil, err
	}
	taskState.Status = core.StatusWaiting
	taskState.Output = &core.Output{
		"wait_status":     "waiting",
		"signal_name":     cfg.WaitFor,
		"has_processor":   cfg.Processor != nil,
		"timeout_seconds": int64(timeout.Seconds()),
	}
	return taskState, nil
}

// buildWaitResponseInput assembles the handler input for wait responses.
func buildWaitResponseInput(
	cfg *task.Config,
	state *task.State,
	workflowConfig *workflow.Config,
	workflowState *workflow.State,
) *shared.ResponseInput {
	return &shared.ResponseInput{
		TaskConfig:     cfg,
		TaskState:      state,
		WorkflowConfig: workflowConfig,
		WorkflowState:  workflowState,
		ExecutionError: nil,
	}
}

// handleWaitResponse delegates to the response handler and converts the output.
func (a *ExecuteWait) handleWaitResponse(
	ctx context.Context,
	responseInput *shared.ResponseInput,
) (*task.MainTaskResponse, error) {
	handler, err := a.tasksFactory.CreateResponseHandler(ctx, task.TaskTypeWait)
	if err != nil {
		return nil, fmt.Errorf("failed to create wait response handler: %w", err)
	}
	result, err := handler.HandleResponse(ctx, responseInput)
	if err != nil {
		return nil, fmt.Errorf("failed to handle wait response: %w", err)
	}
	converter := NewResponseConverter()
	return converter.ConvertToMainTaskResponse(result), nil
}
