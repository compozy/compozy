package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
)

// -----------------------------------------------------------------------------
// LoadExecData
// -----------------------------------------------------------------------------

type LoadExecDataInput struct {
	WorkflowID     string  `json:"workflow_id"`
	WorkflowExecID core.ID `json:"workflow_exec_id"`
	TaskID         string  `json:"task_id"`
	ActionID       *string `json:"action_id,omitempty"`
}

type LoadExecDataOutput struct {
	WorkflowState  *workflow.State
	WorkflowConfig *workflow.Config
	TaskConfig     *task.Config
	TaskID         string
}

type LoadExecData struct {
	workflows    []*workflow.Config
	workflowRepo workflow.Repository
}

func NewLoadExecData(workflows []*workflow.Config, workflowRepo workflow.Repository) *LoadExecData {
	return &LoadExecData{
		workflows:    workflows,
		workflowRepo: workflowRepo,
	}
}

func (uc *LoadExecData) Execute(
	ctx context.Context,
	input *LoadExecDataInput,
) (*LoadExecDataOutput, error) {
	workflowState, err := uc.workflowRepo.GetState(ctx, input.WorkflowExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow state: %w", err)
	}
	workflowConfig, err := workflow.FindConfig(uc.workflows, input.WorkflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to find workflow config: %w", err)
	}
	taskID := input.TaskID
	if taskID == "" {
		taskID = workflowConfig.Tasks[0].ID
	}
	taskConfig, err := task.FindConfig(workflowConfig.Tasks, taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to find task config: %w", err)
	}
	result := &LoadExecDataOutput{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     taskConfig,
		TaskID:         taskID,
	}
	return result, nil
}
