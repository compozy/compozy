package orchestrator

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/domain/project"
	"github.com/compozy/compozy/engine/domain/workflow"
)

func (o *Orchestrator) TriggerWorkflow(
	_ context.Context,
	wfID common.CompID,
	pj *project.Config,
	tgInput *common.Input,
) (*workflow.TriggerWorkflowResponse, error) {
	ex, _, err := o.stManager.CreateWorkflowState(tgInput, pj)
	if err != nil {
		return nil, fmt.Errorf("failed to create workflow state: %w", err)
	}
	return workflow.TriggerWorkflow(wfID, o.natsClient, ex)
}
