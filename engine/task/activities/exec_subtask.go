package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/engine/workflow"
)

const ExecuteSubtaskLabel = "ExecuteSubtask"

type ExecuteSubtaskInput struct {
	WorkflowID     string      `json:"workflow_id"`
	WorkflowExecID core.ID     `json:"workflow_exec_id"`
	ParentState    *task.State `json:"parent_state"`
	TaskExecID     string      `json:"task_exec_id"`
}

type ExecuteSubtask struct {
	loadWorkflowUC *uc.LoadWorkflow
	executeTaskUC  *uc.ExecuteTask
	taskResponder  *services.TaskResponder
	taskRepo       task.Repository
	configStore    services.ConfigStore
}

// NewExecuteSubtask creates a new ExecuteSubtask activity
func NewExecuteSubtask(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	runtime *runtime.Manager,
	configStore services.ConfigStore,
) *ExecuteSubtask {
	return &ExecuteSubtask{
		loadWorkflowUC: uc.NewLoadWorkflow(workflows, workflowRepo),
		executeTaskUC:  uc.NewExecuteTask(runtime),
		taskResponder:  services.NewTaskResponder(workflowRepo, taskRepo),
		taskRepo:       taskRepo,
		configStore:    configStore,
	}
}

func (a *ExecuteSubtask) Run(ctx context.Context, input *ExecuteSubtaskInput) (*task.SubtaskResponse, error) {
	// Load workflow state and config
	workflowState, workflowConfig, err := a.loadWorkflowUC.Execute(ctx, &uc.LoadWorkflowInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
	})
	if err != nil {
		return nil, err
	}

	// Load task config from store
	taskConfig, err := a.configStore.Get(ctx, input.TaskExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to load task config for taskExecID %s: %w", input.TaskExecID, err)
	}

	// Task config loaded from ConfigStore has item context but needs env normalization
	normalizer := uc.NewNormalizeConfig()
	normalizeInput := &uc.NormalizeConfigInput{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     taskConfig,
	}
	err = normalizer.Execute(ctx, normalizeInput)
	if err != nil {
		return nil, err
	}
	taskType := taskConfig.Type
	// TODO: we need to support parallel task execution here too
	if taskType != task.TaskTypeBasic {
		return nil, fmt.Errorf("unsupported task type: %s", taskType)
	}
	// Get the existing child state that was already created by CreateParallelState
	taskState, err := a.getChildState(ctx, input.ParentState.TaskExecID, taskConfig.ID)
	if err != nil {
		return nil, err
	}
	output, executionError := a.executeTaskUC.Execute(ctx, &uc.ExecuteTaskInput{
		TaskConfig:   taskConfig,
		WorkflowMCPs: uc.ProjectMCPConfigs(workflowConfig.MCPs),
	})
	taskState.Output = output
	if err := a.taskRepo.UpsertState(ctx, taskState); err != nil {
		return nil, fmt.Errorf("failed to persist task output: %w", err)
	}
	// Handle subtask response
	response, err := a.taskResponder.HandleSubtask(ctx, &services.SubtaskResponseInput{
		WorkflowConfig: workflowConfig,
		TaskState:      taskState,
		TaskConfig:     taskConfig,
		ExecutionError: executionError,
	})
	if err != nil {
		return nil, err
	}

	// Return the response with the execution status embedded.
	// We only return an error if there's an infrastructure issue that Temporal should retry.
	// Business logic failures are captured in the response status.
	return response, nil
}

// getChildState retrieves the existing child state for a specific task
func (a *ExecuteSubtask) getChildState(
	ctx context.Context,
	parentStateID core.ID,
	taskID string,
) (*task.State, error) {
	childStates, err := a.taskRepo.ListChildren(ctx, parentStateID)
	if err != nil {
		return nil, fmt.Errorf("failed to list child states: %w", err)
	}
	// Find the child state for this specific task
	for _, child := range childStates {
		if child.TaskID == taskID {
			return child, nil
		}
	}
	return nil, fmt.Errorf("child state not found for task ID: %s", taskID)
}
