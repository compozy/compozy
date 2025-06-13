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

const ExecuteBasicLabel = "ExecuteBasicTask"

type ExecuteBasicInput struct {
	WorkflowID     string       `json:"workflow_id"`
	WorkflowExecID core.ID      `json:"workflow_exec_id"`
	TaskConfig     *task.Config `json:"task_config"`
}

type ExecuteBasic struct {
	loadWorkflowUC *uc.LoadWorkflow
	createStateUC  *uc.CreateState
	executeUC      *uc.ExecuteTask
	taskResponder  *services.TaskResponder
}

// NewExecuteBasic creates a new ExecuteBasic activity
func NewExecuteBasic(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	runtime *runtime.Manager,
	configStore services.ConfigStore,
) *ExecuteBasic {
	configManager := services.NewConfigManager(configStore)
	return &ExecuteBasic{
		loadWorkflowUC: uc.NewLoadWorkflow(workflows, workflowRepo),
		createStateUC:  uc.NewCreateState(taskRepo, configManager),
		executeUC:      uc.NewExecuteTask(runtime),
		taskResponder:  services.NewTaskResponder(workflowRepo, taskRepo),
	}
}

func (a *ExecuteBasic) Run(ctx context.Context, input *ExecuteBasicInput) (*task.MainTaskResponse, error) {
	// Load workflow state and config
	workflowState, workflowConfig, err := a.loadWorkflowUC.Execute(ctx, &uc.LoadWorkflowInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
	})
	if err != nil {
		return nil, err
	}
	// Normalize task config
	normalizer := uc.NewNormalizeConfig()
	normalizeInput := &uc.NormalizeConfigInput{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     input.TaskConfig,
	}
	err = normalizer.Execute(ctx, normalizeInput)
	if err != nil {
		return nil, err
	}
	// Validate task
	taskConfig := input.TaskConfig
	taskType := taskConfig.Type
	if taskType != task.TaskTypeBasic {
		return nil, fmt.Errorf("unsupported task type: %s", taskType)
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
	// Execute component
	output, executionError := a.executeUC.Execute(ctx, &uc.ExecuteTaskInput{
		TaskConfig:   taskConfig,
		WorkflowMCPs: uc.ProjectMCPConfigs(workflowConfig.MCPs),
	})

	taskState.Output = output

	// Handle main task response
	response, handleErr := a.taskResponder.HandleMainTask(ctx, &services.MainTaskResponseInput{
		WorkflowConfig: workflowConfig,
		TaskState:      taskState,
		TaskConfig:     taskConfig,
		ExecutionError: executionError,
	})
	if handleErr != nil {
		return nil, handleErr
	}

	// If there was an execution error, the task should be considered failed
	if executionError != nil {
		return response, executionError
	}

	return response, nil
}
