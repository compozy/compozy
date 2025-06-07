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

type ExecuteBasicInput = DispatchOutput

type ExecuteBasic struct {
	loadDataUC       *uc.LoadExecData
	executeUC        *uc.ExecComponent
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
		loadDataUC:       uc.NewLoadExecData(workflows, workflowRepo),
		executeUC:        uc.NewExecComponent(runtime),
		handleResponseUC: uc.NewHandleResponse(workflowRepo, taskRepo),
	}
}

func (a *ExecuteBasic) Run(ctx context.Context, input *ExecuteBasicInput) (*task.Response, error) {
	state := input.TaskState
	taskType := input.TaskConfig.Type
	if taskType != task.TaskTypeBasic {
		return nil, fmt.Errorf("unsupported task type: %s", taskType)
	}
	if input.TaskConfig.Agent == nil && input.TaskConfig.Tool == nil {
		return nil, fmt.Errorf("unsupported component type: %s", state.Component)
	}
	// Load execution data with action ID from state
	execData, err := a.loadDataUC.Execute(ctx, &uc.LoadExecDataInput{
		WorkflowID:     state.WorkflowID,
		WorkflowExecID: state.WorkflowExecID,
		TaskID:         state.TaskID,
		ActionID:       state.ActionID,
	})
	if err != nil {
		return a.handleError(ctx, execData, state, err)
	}
	// Execute component
	output, err := a.executeUC.Execute(ctx, &uc.ExecComponentInput{
		TaskConfig: execData.TaskConfig,
	})
	if err != nil {
		return a.handleError(ctx, execData, state, err)
	}
	// Update state with result
	state.Output = output.Result
	transitOutput, err := a.handleResponseUC.Execute(ctx, &uc.HandleResponseInput{
		State:          state,
		WorkflowConfig: execData.WorkflowConfig,
		TaskConfig:     execData.TaskConfig,
		ExecutionError: nil,
	})
	if err != nil {
		return nil, err
	}
	return transitOutput.Response, nil
}

func (a *ExecuteBasic) handleError(
	ctx context.Context,
	loadOutput *uc.LoadExecDataOutput,
	state *task.State,
	executionErr error,
) (*task.Response, error) {
	if loadOutput == nil {
		state.UpdateStatus(core.StatusFailed)
		state.Error = core.NewError(executionErr, "execution_error", nil)
		return &task.Response{State: state}, nil
	}
	transitInput := &uc.HandleResponseInput{
		State:          state,
		WorkflowConfig: loadOutput.WorkflowConfig,
		TaskConfig:     loadOutput.TaskConfig,
		ExecutionError: executionErr,
	}
	transitOutput, err := a.handleResponseUC.Execute(ctx, transitInput)
	if err != nil {
		return nil, err
	}
	return transitOutput.Response, nil
}
