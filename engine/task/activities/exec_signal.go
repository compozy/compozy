package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/engine/task2"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
)

const ExecuteSignalLabel = "ExecuteSignalTask"

type ExecuteSignalInput struct {
	WorkflowID     string       `json:"workflow_id"`
	WorkflowExecID core.ID      `json:"workflow_exec_id"`
	TaskConfig     *task.Config `json:"task_config"`
	ProjectName    string       `json:"project_name"`
}

type ExecuteSignal struct {
	loadWorkflowUC   *uc.LoadWorkflow
	createStateUC    *uc.CreateState
	task2Factory     task2.Factory
	templateEngine   *tplengine.TemplateEngine
	signalDispatcher services.SignalDispatcher
}

// NewExecuteSignal creates a new ExecuteSignal activity
func NewExecuteSignal(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	configStore services.ConfigStore,
	signalDispatcher services.SignalDispatcher,
	task2Factory task2.Factory,
	templateEngine *tplengine.TemplateEngine,
) (*ExecuteSignal, error) {
	return &ExecuteSignal{
		loadWorkflowUC:   uc.NewLoadWorkflow(workflows, workflowRepo),
		createStateUC:    uc.NewCreateState(taskRepo, configStore),
		task2Factory:     task2Factory,
		templateEngine:   templateEngine,
		signalDispatcher: signalDispatcher,
	}, nil
}

func (a *ExecuteSignal) Run(ctx context.Context, input *ExecuteSignalInput) (*task.MainTaskResponse, error) {
	if err := validateExecuteSignalInput(input); err != nil {
		return nil, err
	}
	workflowState, workflowConfig, err := a.loadWorkflowUC.Execute(ctx, &uc.LoadWorkflowInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
	})
	if err != nil {
		return nil, err
	}
	normalizedConfig, err := a.normalizeSignalConfig(ctx, workflowState, workflowConfig, input.TaskConfig)
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
	result, err := a.dispatchAndHandleSignal(
		ctx,
		normalizedConfig,
		workflowConfig,
		workflowState,
		taskState,
		input.ProjectName,
		input.WorkflowExecID.String(),
	)
	if err != nil {
		return nil, err
	}
	return result.response, result.executionErr
}

// signalExecutionResult captures the outcome of the signal dispatch flow.
type signalExecutionResult struct {
	response     *task.MainTaskResponse
	executionErr error
}

// dispatchAndHandleSignal sends the signal and composes the final response.
func (a *ExecuteSignal) dispatchAndHandleSignal(
	ctx context.Context,
	taskConfig *task.Config,
	workflowConfig *workflow.Config,
	workflowState *workflow.State,
	taskState *task.State,
	projectName string,
	workflowExecID string,
) (*signalExecutionResult, error) {
	executionError := a.dispatchSignal(ctx, taskConfig, workflowExecID, projectName)
	if executionError == nil {
		taskState.Output = &core.Output{
			"signal_dispatched": true,
			"signal_id":         taskConfig.Signal.ID,
		}
	}
	handler, err := a.task2Factory.CreateResponseHandler(ctx, task.TaskTypeSignal)
	if err != nil {
		return nil, fmt.Errorf("failed to create signal response handler: %w", err)
	}
	responseInput := buildSignalResponseInput(
		taskConfig,
		taskState,
		workflowConfig,
		workflowState,
		executionError,
	)
	result, err := handler.HandleResponse(ctx, responseInput)
	if err != nil {
		return nil, fmt.Errorf("failed to handle signal response: %w", err)
	}
	return &signalExecutionResult{
		response:     NewResponseConverter().ConvertToMainTaskResponse(result),
		executionErr: executionError,
	}, nil
}

func (a *ExecuteSignal) dispatchSignal(
	ctx context.Context,
	taskConfig *task.Config,
	correlationID string,
	projectName string,
) error {
	if taskConfig.Signal == nil || taskConfig.Signal.ID == "" {
		return fmt.Errorf("signal.id is required for signal task")
	}
	payload := taskConfig.Signal.Payload
	if payload == nil {
		payload = make(map[string]any)
	}
	ctx = core.WithProjectName(ctx, projectName)
	return a.signalDispatcher.DispatchSignal(ctx, taskConfig.Signal.ID, payload, correlationID)
}

// validateExecuteSignalInput ensures the signal request contains required data.
func validateExecuteSignalInput(input *ExecuteSignalInput) error {
	if input == nil || input.TaskConfig == nil {
		return fmt.Errorf("task_config is required for signal task")
	}
	return nil
}

// normalizeSignalConfig runs the signal normalizer and validates task type.
func (a *ExecuteSignal) normalizeSignalConfig(
	ctx context.Context,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
) (*task.Config, error) {
	normalizer, err := a.task2Factory.CreateNormalizer(ctx, task.TaskTypeSignal)
	if err != nil {
		return nil, fmt.Errorf("failed to create signal normalizer: %w", err)
	}
	contextBuilder, err := shared.NewContextBuilderWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create context builder: %w", err)
	}
	normContext := contextBuilder.BuildContext(ctx, workflowState, workflowConfig, taskConfig)
	if err := normalizer.Normalize(ctx, taskConfig, normContext); err != nil {
		return nil, fmt.Errorf("failed to normalize signal task: %w", err)
	}
	if taskConfig.Type != task.TaskTypeSignal {
		return nil, fmt.Errorf("unsupported task type: %s", taskConfig.Type)
	}
	return taskConfig, nil
}

// buildSignalResponseInput prepares the shared response input for signal handlers.
func buildSignalResponseInput(
	taskConfig *task.Config,
	taskState *task.State,
	workflowConfig *workflow.Config,
	workflowState *workflow.State,
	executionError error,
) *shared.ResponseInput {
	return &shared.ResponseInput{
		TaskConfig:     taskConfig,
		TaskState:      taskState,
		WorkflowConfig: workflowConfig,
		WorkflowState:  workflowState,
		ExecutionError: executionError,
	}
}
