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

const ExecuteWaitLabel = "ExecuteWaitTask"

type ExecuteWaitInput struct {
	WorkflowID     string       `json:"workflow_id"`
	WorkflowExecID core.ID      `json:"workflow_exec_id"`
	TaskConfig     *task.Config `json:"task_config"`
}

type ExecuteWait struct {
	loadWorkflowUC *uc.LoadWorkflow
	createStateUC  *uc.CreateState
	task2Factory   task2.Factory
	templateEngine *tplengine.TemplateEngine
}

// NewExecuteWait creates a new ExecuteWait activity
func NewExecuteWait(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	configStore services.ConfigStore,
	_ *core.PathCWD,
	task2Factory task2.Factory,
	templateEngine *tplengine.TemplateEngine,
) (*ExecuteWait, error) {
	return &ExecuteWait{
		loadWorkflowUC: uc.NewLoadWorkflow(workflows, workflowRepo),
		createStateUC:  uc.NewCreateState(taskRepo, configStore),
		task2Factory:   task2Factory,
		templateEngine: templateEngine,
	}, nil
}

func (a *ExecuteWait) Run(ctx context.Context, input *ExecuteWaitInput) (*task.MainTaskResponse, error) {
	// Validate input
	if input.TaskConfig == nil {
		return nil, fmt.Errorf("task_config is required for wait task")
	}
	// Load workflow state and config
	workflowState, workflowConfig, err := a.loadWorkflowUC.Execute(ctx, &uc.LoadWorkflowInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
	})
	if err != nil {
		return nil, err
	}
	// Use task2 normalizer for wait tasks
	normalizer, err := a.task2Factory.CreateNormalizer(ctx, task.TaskTypeWait)
	if err != nil {
		return nil, fmt.Errorf("failed to create wait normalizer: %w", err)
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
		return nil, fmt.Errorf("failed to normalize wait task: %w", err)
	}
	// Validate task type
	if normalizedConfig.Type != task.TaskTypeWait {
		return nil, fmt.Errorf("unsupported task type: %s", normalizedConfig.Type)
	}
	// Validate WaitFor signal name
	if strings.TrimSpace(normalizedConfig.WaitFor) == "" {
		return nil, fmt.Errorf("wait task must define a non-empty wait_for signal")
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
	// Set status to WAITING for wait tasks
	taskState.Status = core.StatusWaiting
	// Set initial output with wait metadata
	// The actual waiting and signal processing happens in the workflow
	taskState.Output = &core.Output{
		"wait_status":   "waiting",
		"signal_name":   normalizedConfig.WaitFor,
		"has_processor": normalizedConfig.Processor != nil,
	}
	// Use task2 ResponseHandler for wait type
	handler, err := a.task2Factory.CreateResponseHandler(ctx, task.TaskTypeWait)
	if err != nil {
		return nil, fmt.Errorf("failed to create wait response handler: %w", err)
	}
	// Prepare input for response handler
	responseInput := &shared.ResponseInput{
		TaskConfig:     normalizedConfig,
		TaskState:      taskState,
		WorkflowConfig: workflowConfig,
		WorkflowState:  workflowState,
		ExecutionError: nil, // No error, we're just waiting
	}
	// Handle the response
	result, err := handler.HandleResponse(ctx, responseInput)
	if err != nil {
		return nil, fmt.Errorf("failed to handle wait response: %w", err)
	}
	// Convert shared.ResponseOutput to task.MainTaskResponse
	converter := NewResponseConverter()
	mainTaskResponse := converter.ConvertToMainTaskResponse(result)
	return mainTaskResponse, nil
}
