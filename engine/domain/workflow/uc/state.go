package wfuc

import (
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/domain/project"
	"github.com/compozy/compozy/engine/domain/workflow"
	"github.com/compozy/compozy/engine/stmanager"
	"github.com/compozy/compozy/pkg/pb"
)

func CreateInitState(
	stManager *stmanager.Manager,
	pConfig *project.Config,
	workflows []*workflow.Config,
	cmd *pb.CmdWorkflowTrigger,
) error {
	workflowID := cmd.GetMetadata().WorkflowId
	triggerInputMap := cmd.GetDetails().GetTriggerInput().AsMap()
	if triggerInputMap == nil {
		return fmt.Errorf("trigger input is nil")
	}
	triggerInput := common.Input(triggerInputMap)
	metadata := cmd.GetMetadata()
	wConfig, err := workflow.FindConfig(workflows, workflowID)
	if err != nil {
		return err
	}

	// Create workflow state
	_, err = stManager.CreateWorkflowState(metadata, pConfig, wConfig, &triggerInput)
	if err != nil {
		return fmt.Errorf("failed to create workflow state: %w", err)
	}
	return nil
}

func FindStateAndConfig(
	stManager *stmanager.Manager,
	workflows []*workflow.Config,
	metadata *pb.WorkflowMetadata,
) (*workflow.State, *workflow.Config, error) {
	workflowID := metadata.GetWorkflowId()
	config, err := workflow.FindConfig(workflows, workflowID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find workflow config: %w", err)
	}
	stateID := workflow.GetWorkflowStateID(metadata)
	state, err := stManager.LoadWorkflowState(stateID, false)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load workflow state: %w", err)
	}
	return state, config, nil
}
