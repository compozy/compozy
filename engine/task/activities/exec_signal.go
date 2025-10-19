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
	// Load workflow state and config
	workflowState, workflowConfig, err := a.loadWorkflowUC.Execute(ctx, &uc.LoadWorkflowInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
	})
	if err != nil {
		return nil, err
	}
	// Use task2 normalizer for signal tasks
	normalizer, err := a.task2Factory.CreateNormalizer(ctx, task.TaskTypeSignal)
	if err != nil {
		return nil, fmt.Errorf("failed to create signal normalizer: %w", err)
	}
	// Create context builder to build proper normalization context
	contextBuilder, err := shared.NewContextBuilderWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create context builder: %w", err)
	}
	// Build proper normalization context with all template variables
	normContext := contextBuilder.BuildContext(ctx, workflowState, workflowConfig, input.TaskConfig)
	// Normalize the task configuration
	normalizedConfig := input.TaskConfig
	if err := normalizer.Normalize(ctx, normalizedConfig, normContext); err != nil {
		return nil, fmt.Errorf("failed to normalize signal task: %w", err)
	}
	// Validate task
	if normalizedConfig.Type != task.TaskTypeSignal {
		return nil, fmt.Errorf("unsupported task type: %s", normalizedConfig.Type)
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
	// Dispatch the signal and capture the error
	executionError := a.dispatchSignal(ctx, normalizedConfig, input.WorkflowExecID.String(), input.ProjectName)
	// Set a simple success output if the signal was dispatched
	if executionError == nil {
		taskState.Output = &core.Output{
			"signal_dispatched": true,
			"signal_id":         normalizedConfig.Signal.ID,
		}
	}
	// Use task2 ResponseHandler for signal type
	handler, err := a.task2Factory.CreateResponseHandler(ctx, task.TaskTypeSignal)
	if err != nil {
		return nil, fmt.Errorf("failed to create signal response handler: %w", err)
	}
	// Prepare input for response handler
	responseInput := &shared.ResponseInput{
		TaskConfig:     normalizedConfig,
		TaskState:      taskState,
		WorkflowConfig: workflowConfig,
		WorkflowState:  workflowState,
		ExecutionError: executionError,
	}
	// Handle the response
	result, err := handler.HandleResponse(ctx, responseInput)
	if err != nil {
		return nil, fmt.Errorf("failed to handle signal response: %w", err)
	}
	// Convert shared.ResponseOutput to task.MainTaskResponse
	converter := NewResponseConverter()
	mainTaskResponse := converter.ConvertToMainTaskResponse(result)
	return mainTaskResponse, executionError
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
	// Create the signal payload
	payload := taskConfig.Signal.Payload
	if payload == nil {
		payload = make(map[string]any)
	}
	// Add project name to context for signal dispatcher
	ctx = core.WithProjectName(ctx, projectName)
	// Use the signal dispatcher service
	return a.signalDispatcher.DispatchSignal(ctx, taskConfig.Signal.ID, payload, correlationID)
}
