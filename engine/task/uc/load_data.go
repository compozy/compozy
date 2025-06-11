package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	wf "github.com/compozy/compozy/engine/workflow"
	"go.temporal.io/sdk/workflow"
)

// -----------------------------------------------------------------------------
// LoadWorkflow
// -----------------------------------------------------------------------------

type LoadWorkflowInput struct {
	WorkflowID     string  `json:"workflow_id"`
	WorkflowExecID core.ID `json:"workflow_exec_id"`
}

type LoadWorkflow struct {
	workflows    []*wf.Config
	workflowRepo wf.Repository
}

func NewLoadWorkflow(workflows []*wf.Config, workflowRepo wf.Repository) *LoadWorkflow {
	return &LoadWorkflow{workflows: workflows, workflowRepo: workflowRepo}
}

func (uc *LoadWorkflow) Execute(
	ctx context.Context,
	input *LoadWorkflowInput,
) (*wf.State, *wf.Config, error) {
	workflowState, err := uc.workflowRepo.GetState(ctx, input.WorkflowExecID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get workflow state: %w", err)
	}
	workflowConfig, err := wf.FindConfig(uc.workflows, input.WorkflowID)
	if err != nil {
		// Additional debugging: list available workflow IDs
		var availableIDs []string
		for _, wf := range uc.workflows {
			if wf != nil {
				availableIDs = append(availableIDs, wf.ID)
			}
		}
		return nil, nil, fmt.Errorf("failed to find workflow config for ID '%s': %w (available workflows: %v)",
			input.WorkflowID, err, availableIDs)
	}
	return workflowState, workflowConfig, nil
}

// -----------------------------------------------------------------------------
// LoadTaskConfig
// -----------------------------------------------------------------------------

type LoadTaskConfigInput struct {
	WorkflowConfig *wf.Config `json:"workflow_config"`
	TaskID         string     `json:"task_id"`
}

type LoadTaskConfig struct {
	workflows []*wf.Config
}

func NewLoadTaskConfig(workflows []*wf.Config) *LoadTaskConfig {
	return &LoadTaskConfig{workflows: workflows}
}

func (uc *LoadTaskConfig) Execute(_ workflow.Context, input *LoadTaskConfigInput) (*task.Config, error) {
	workflowConfig := input.WorkflowConfig
	taskID := input.TaskID
	if taskID == "" {
		taskID = workflowConfig.Tasks[0].ID
	}
	taskConfig, err := task.FindConfig(workflowConfig.Tasks, taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to find task config: %w", err)
	}
	return taskConfig, nil
}
