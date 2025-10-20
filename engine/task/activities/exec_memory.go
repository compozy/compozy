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
	if err := validateExecuteMemoryInput(input); err != nil {
		return nil, err
	}
	workflowState, workflowConfig, err := a.loadWorkflowUC.Execute(ctx, &uc.LoadWorkflowInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load workflow: %w", err)
	}
	normalizedConfig, err := a.normalizeMemoryConfig(ctx, workflowState, workflowConfig, input.TaskConfig)
	if err != nil {
		return nil, err
	}
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
	handler, err := a.task2Factory.CreateResponseHandler(ctx, task.TaskTypeMemory)
	if err != nil {
		return nil, fmt.Errorf("failed to create memory response handler: %w", err)
	}
	responseInput := buildMemoryResponseInput(normalizedConfig, state, workflowConfig, workflowState, executionError)
	result, err := handler.HandleResponse(ctx, responseInput)
	if err != nil {
		return nil, fmt.Errorf("failed to handle memory response: %w", err)
	}
	// Convert shared.ResponseOutput to task.MainTaskResponse
	converter := NewResponseConverter()
	mainTaskResponse := converter.ConvertToMainTaskResponse(result)
	return mainTaskResponse, executionError
}

// validateExecuteMemoryInput ensures required parameters are present.
func validateExecuteMemoryInput(input *ExecuteMemoryInput) error {
	if input == nil || input.TaskConfig == nil {
		return fmt.Errorf("task_config is required for memory task")
	}
	return nil
}

// normalizeMemoryConfig renders templates and validates the memory task configuration.
func (a *ExecuteMemory) normalizeMemoryConfig(
	ctx context.Context,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
) (*task.Config, error) {
	normalizer, err := a.task2Factory.CreateNormalizer(ctx, task.TaskTypeMemory)
	if err != nil {
		return nil, fmt.Errorf("failed to create memory normalizer: %w", err)
	}
	contextBuilder, err := shared.NewContextBuilderWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create context builder: %w", err)
	}
	normContext := contextBuilder.BuildContext(ctx, workflowState, workflowConfig, taskConfig)
	if a.projectConfig != nil {
		contextBuilder.VariableBuilder.AddProjectToVariables(normContext.Variables, a.projectConfig.Name)
	}
	if err := normalizer.Normalize(ctx, taskConfig, normContext); err != nil {
		return nil, fmt.Errorf("failed to normalize memory task: %w", err)
	}
	return taskConfig, nil
}

// buildMemoryResponseInput prepares the shared response input for memory handlers.
func buildMemoryResponseInput(
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
