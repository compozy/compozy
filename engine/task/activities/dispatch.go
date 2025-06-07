package activities

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/engine/workflow"
)

const DispatchLabel = "DispatchTask"

type DispatchInput struct {
	WorkflowID     string  `json:"workflow_id"`
	WorkflowExecID core.ID `json:"workflow_exec_id"`
	TaskID         string  `json:"task_id"`
}

type DispatchOutput struct {
	TaskState  *task.State
	TaskConfig *task.Config
}

type Dispatch struct {
	loadDataUC    *uc.LoadExecData
	createStateUC *uc.CreateState
}

// NewDispatch creates a new Dispatch activity
func NewDispatch(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
) *Dispatch {
	return &Dispatch{
		loadDataUC:    uc.NewLoadExecData(workflows, workflowRepo),
		createStateUC: uc.NewCreateState(taskRepo),
	}
}

// Run dispatches a task by loading data and creating state
func (a *Dispatch) Run(ctx context.Context, input *DispatchInput) (*DispatchOutput, error) {
	// Load execution data
	loadInput := &uc.LoadExecDataInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
		TaskID:         input.TaskID,
	}
	execData, err := a.loadDataUC.Execute(ctx, loadInput)
	if err != nil {
		return nil, err
	}
	// Create task state
	createInput := &uc.CreateStateInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
		TaskID:         execData.TaskID,
		WorkflowState:  execData.WorkflowState,
		WorkflowConfig: execData.WorkflowConfig,
		TaskConfig:     execData.TaskConfig,
	}
	createOutput, err := a.createStateUC.Execute(ctx, createInput)
	if err != nil {
		return nil, err
	}
	return &DispatchOutput{
		TaskState:  createOutput.State,
		TaskConfig: execData.TaskConfig,
	}, nil
}
