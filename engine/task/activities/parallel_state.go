package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/engine/workflow"
)

const CreateParallelStateLabel = "CreateParallelState"

type CreateParallelStateInput struct {
	WorkflowID     string       `json:"workflow_id"`
	WorkflowExecID core.ID      `json:"workflow_exec_id"`
	TaskConfig     *task.Config `json:"task_config"`
}

type CreateParallelState struct {
	loadWorkflowUC   *uc.LoadWorkflow
	createStateUC    *uc.CreateState
	handleResponseUC *uc.HandleResponse
}

// NewCreateParallelState creates a new CreateParallelState activity
func NewCreateParallelState(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
) *CreateParallelState {
	return &CreateParallelState{
		loadWorkflowUC:   uc.NewLoadWorkflow(workflows, workflowRepo),
		createStateUC:    uc.NewCreateState(taskRepo),
		handleResponseUC: uc.NewHandleResponse(workflowRepo, taskRepo),
	}
}

func (a *CreateParallelState) Run(ctx context.Context, input *CreateParallelStateInput) (*task.State, error) {
	// Load workflow state and config
	workflowState, workflowConfig, err := a.loadWorkflowUC.Execute(ctx, &uc.LoadWorkflowInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
	})
	if err != nil {
		return nil, err
	}
	// Validate task
	taskConfig := input.TaskConfig
	taskType := taskConfig.Type
	if taskType != task.TaskTypeParallel {
		return nil, fmt.Errorf("unsupported task type: %s", taskType)
	}
	// Create task state
	state, err := a.createStateUC.Execute(ctx, &uc.CreateStateInput{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     taskConfig,
	})
	if err != nil {
		return nil, err
	}
	return state, nil
}
