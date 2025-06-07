package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/task"
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
	loadWorkflowUC   *uc.LoadWorkflow
	createStateUC    *uc.CreateState
	executeUC        *uc.ExecuteTask
	handleResponseUC *uc.HandleResponse
}

// NewExecuteBasic creates a new ExecuteBasic activity
func NewExecuteBasic(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	runtime *runtime.Manager,
) *ExecuteBasic {
	return &ExecuteBasic{
		loadWorkflowUC:   uc.NewLoadWorkflow(workflows, workflowRepo),
		createStateUC:    uc.NewCreateState(taskRepo),
		executeUC:        uc.NewExecuteTask(runtime),
		handleResponseUC: uc.NewHandleResponse(workflowRepo, taskRepo),
	}
}

func (a *ExecuteBasic) Run(ctx context.Context, input *ExecuteBasicInput) (*task.Response, error) {
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
	output, err := a.executeUC.Execute(ctx, &uc.ExecuteTaskInput{
		TaskConfig: taskConfig,
	})
	handleError := HandleError(
		a.handleResponseUC,
		output,
		workflowConfig,
		taskConfig,
	)
	if err != nil {
		return handleError(ctx, taskState, err)
	}
	// Update state with result
	taskState.Output = output
	response, err := a.handleResponseUC.Execute(ctx, &uc.HandleResponseInput{
		TaskState:      taskState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     taskConfig,
		ExecutionError: nil,
	})
	if err != nil {
		return handleError(ctx, taskState, err)
	}
	return response, nil
}

func HandleError(
	handleResponseUC *uc.HandleResponse,
	output *core.Output,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
) func(ctx context.Context, taskState *task.State, err error) (*task.Response, error) {
	return func(ctx context.Context, taskState *task.State, err error) (*task.Response, error) {
		if output == nil {
			taskState.UpdateStatus(core.StatusFailed)
			taskState.Error = core.NewError(err, "execution_error", nil)
			return &task.Response{State: taskState}, nil
		}
		transitInput := &uc.HandleResponseInput{
			TaskState:      taskState,
			WorkflowConfig: workflowConfig,
			TaskConfig:     taskConfig,
			ExecutionError: err,
		}
		response, err := handleResponseUC.Execute(ctx, transitInput)
		if err != nil {
			return nil, err
		}
		return response, nil
	}
}
