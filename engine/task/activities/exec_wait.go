package activities

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/engine/workflow"
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
	taskResponder  *services.TaskResponder
}

// NewExecuteWait creates a new ExecuteWait activity
func NewExecuteWait(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	configStore services.ConfigStore,
	cwd *core.PathCWD,
) (*ExecuteWait, error) {
	configManager, err := services.NewConfigManager(configStore, cwd)
	if err != nil {
		return nil, fmt.Errorf("failed to create config manager: %w", err)
	}
	// Pass dependencies to activity, use cases will be created in Run method
	return &ExecuteWait{
		loadWorkflowUC: uc.NewLoadWorkflow(workflows, workflowRepo),
		createStateUC:  uc.NewCreateState(taskRepo, configManager),
		taskResponder:  services.NewTaskResponder(workflowRepo, taskRepo),
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
	// Normalize task config
	normalizer, err := uc.NewNormalizeConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create normalizer: %w", err)
	}
	normalizeInput := &uc.NormalizeConfigInput{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     input.TaskConfig,
	}
	err = normalizer.Execute(ctx, normalizeInput)
	if err != nil {
		return nil, err
	}
	// Validate task type
	taskConfig := input.TaskConfig
	taskType := taskConfig.Type
	if taskType != task.TaskTypeWait {
		return nil, fmt.Errorf("unsupported task type: %s", taskType)
	}
	// Validate WaitFor signal name
	if strings.TrimSpace(taskConfig.WaitFor) == "" {
		return nil, fmt.Errorf("wait task must define a non-empty wait_for signal")
	}
	// Create task state
	taskState, err := a.createStateUC.Execute(ctx, &uc.CreateStateInput{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     taskConfig,
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
		"signal_name":   taskConfig.WaitFor,
		"has_processor": taskConfig.Processor != nil,
	}
	// Update task state to indicate we're waiting
	// This is different from other tasks that complete immediately
	response, handleErr := a.taskResponder.HandleMainTask(ctx, &services.MainTaskResponseInput{
		WorkflowConfig: workflowConfig,
		TaskState:      taskState,
		TaskConfig:     taskConfig,
		ExecutionError: nil, // No error, we're just waiting
	})
	if handleErr != nil {
		return nil, handleErr
	}
	return response, nil
}
