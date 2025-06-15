package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/engine/workflow"
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
	taskResponder    *services.TaskResponder
	signalDispatcher services.SignalDispatcher
}

// NewExecuteSignal creates a new ExecuteSignal activity
func NewExecuteSignal(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	configStore services.ConfigStore,
	signalDispatcher services.SignalDispatcher,
	cwd *core.PathCWD,
) *ExecuteSignal {
	configManager := services.NewConfigManager(configStore, cwd)
	return &ExecuteSignal{
		loadWorkflowUC:   uc.NewLoadWorkflow(workflows, workflowRepo),
		createStateUC:    uc.NewCreateState(taskRepo, configManager),
		taskResponder:    services.NewTaskResponder(workflowRepo, taskRepo),
		signalDispatcher: signalDispatcher,
	}
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
	if taskType != task.TaskTypeSignal {
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
	// Dispatch the signal and capture the error
	executionError := a.dispatchSignal(ctx, taskConfig, input.WorkflowExecID.String(), input.ProjectName)
	// Set a simple success output if the signal was dispatched
	if executionError == nil {
		taskState.Output = &core.Output{
			"signal_dispatched": true,
			"signal_id":         taskConfig.Signal.ID,
		}
	}
	response, handleErr := a.taskResponder.HandleMainTask(ctx, &services.MainTaskResponseInput{
		WorkflowConfig: workflowConfig,
		TaskState:      taskState,
		TaskConfig:     taskConfig,
		ExecutionError: executionError, // Pass the error to the responder
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
