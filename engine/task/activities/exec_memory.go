package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/task/uc"
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
	taskResponder         *services.TaskResponder
	execMemoryOperationUC *uc.ExecuteMemoryOperation
}

// NewExecuteMemory creates a new ExecuteMemory activity
func NewExecuteMemory(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	configStore services.ConfigStore,
	memoryManager memcore.ManagerInterface,
	cwd *core.PathCWD,
	templateEngine *tplengine.TemplateEngine,
) *ExecuteMemory {
	configManager := services.NewConfigManager(configStore, cwd)
	return &ExecuteMemory{
		loadWorkflowUC:        uc.NewLoadWorkflow(workflows, workflowRepo),
		createStateUC:         uc.NewCreateState(taskRepo, configManager),
		taskResponder:         services.NewTaskResponder(workflowRepo, taskRepo),
		execMemoryOperationUC: uc.NewExecuteMemoryOperation(memoryManager, templateEngine),
	}
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
	// Create state
	state, err := a.createStateUC.Execute(ctx, &uc.CreateStateInput{
		TaskConfig:     input.TaskConfig,
		WorkflowConfig: workflowConfig,
		WorkflowState:  workflowState,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create state: %w", err)
	}
	// Execute memory operation
	output, executionError := a.execMemoryOperationUC.Execute(ctx, &uc.ExecuteMemoryOperationInput{
		TaskConfig:    input.TaskConfig,
		MergedInput:   input.MergedInput,
		WorkflowState: workflowState,
	})
	if executionError == nil {
		state.Output = output
	}
	// Handle response using task responder
	response, handleErr := a.taskResponder.HandleMainTask(ctx, &services.MainTaskResponseInput{
		WorkflowConfig: workflowConfig,
		TaskState:      state,
		TaskConfig:     input.TaskConfig,
		ExecutionError: executionError,
	})
	if handleErr != nil {
		return nil, handleErr
	}
	return response, executionError
}
