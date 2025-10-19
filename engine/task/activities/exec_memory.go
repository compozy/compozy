package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/engine/task2"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
)

const ExecuteMemoryLabel = "ExecuteMemoryTask"

type ExecuteMemoryInput struct {
	WorkflowID     string       `json:"workflow_id"`
	WorkflowExecID core.ID      `json:"workflow_exec_id"`
	TaskConfig     *task.Config `json:"task_config"`
	MergedInput    *core.Input  `json:"merged_input"`
}

type ExecuteMemory struct {
	loadWorkflowUC        *uc.LoadWorkflow
	createStateUC         *uc.CreateState
	task2Factory          task2.Factory
	execMemoryOperationUC *uc.ExecuteMemoryOperation
	templateEngine        *tplengine.TemplateEngine
	projectConfig         *project.Config
}

// NewExecuteMemory creates a new ExecuteMemory activity
func NewExecuteMemory(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	configStore services.ConfigStore,
	memoryManager memcore.ManagerInterface,
	_ *core.PathCWD,
	templateEngine *tplengine.TemplateEngine,
	projectConfig *project.Config,
	task2Factory task2.Factory,
) (*ExecuteMemory, error) {
	execMemoryOperationUC, err := uc.NewExecuteMemoryOperation(memoryManager, templateEngine)
	if err != nil {
		return nil, fmt.Errorf("failed to create memory operation use case: %w", err)
	}
	return &ExecuteMemory{
		loadWorkflowUC:        uc.NewLoadWorkflow(workflows, workflowRepo),
		createStateUC:         uc.NewCreateState(taskRepo, configStore),
		task2Factory:          task2Factory,
		execMemoryOperationUC: execMemoryOperationUC,
		templateEngine:        templateEngine,
		projectConfig:         projectConfig,
	}, nil
}

func (a *ExecuteMemory) Run(ctx context.Context, input *ExecuteMemoryInput) (*task.MainTaskResponse, error) {
	// Validate input
	if input.TaskConfig == nil {
		return nil, fmt.Errorf("task_config is required for memory task")
	}
	// Load workflow state and config
	workflowState, workflowConfig, err := a.loadWorkflowUC.Execute(ctx, &uc.LoadWorkflowInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load workflow: %w", err)
	}
	// Use task2 normalizer for memory tasks
	normalizer, err := a.task2Factory.CreateNormalizer(ctx, task.TaskTypeMemory)
	if err != nil {
		return nil, fmt.Errorf("failed to create memory normalizer: %w", err)
	}
	// Create context builder to build proper normalization context
	contextBuilder, err := shared.NewContextBuilderWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create context builder: %w", err)
	}
	// Build proper normalization context with all template variables
	normContext := contextBuilder.BuildContext(ctx, workflowState, workflowConfig, input.TaskConfig)
	// Add project information for memory operations
	if a.projectConfig != nil {
		contextBuilder.VariableBuilder.AddProjectToVariables(normContext.Variables, a.projectConfig.Name)
	}
	// Normalize the task configuration
	normalizedConfig := input.TaskConfig
	if err := normalizer.Normalize(ctx, normalizedConfig, normContext); err != nil {
		return nil, fmt.Errorf("failed to normalize memory task: %w", err)
	}
	// Create state
	state, err := a.createStateUC.Execute(ctx, &uc.CreateStateInput{
		TaskConfig:     normalizedConfig,
		WorkflowConfig: workflowConfig,
		WorkflowState:  workflowState,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create state: %w", err)
	}
	// Execute memory operation
	output, executionError := a.execMemoryOperationUC.Execute(ctx, &uc.ExecuteMemoryOperationInput{
		TaskConfig:    normalizedConfig,
		MergedInput:   input.MergedInput,
		WorkflowState: workflowState,
	})
	if executionError == nil {
		state.Output = output
	}
	// Use task2 ResponseHandler for memory type
	handler, err := a.task2Factory.CreateResponseHandler(ctx, task.TaskTypeMemory)
	if err != nil {
		return nil, fmt.Errorf("failed to create memory response handler: %w", err)
	}
	// Prepare input for response handler
	responseInput := &shared.ResponseInput{
		TaskConfig:     normalizedConfig,
		TaskState:      state,
		WorkflowConfig: workflowConfig,
		WorkflowState:  workflowState,
		ExecutionError: executionError,
	}
	// Handle the response
	result, err := handler.HandleResponse(ctx, responseInput)
	if err != nil {
		return nil, fmt.Errorf("failed to handle memory response: %w", err)
	}
	// Convert shared.ResponseOutput to task.MainTaskResponse
	converter := NewResponseConverter()
	mainTaskResponse := converter.ConvertToMainTaskResponse(result)
	return mainTaskResponse, executionError
}
