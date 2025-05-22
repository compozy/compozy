package orchestrator

import (
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/domain/project"
	"github.com/compozy/compozy/engine/domain/workflow"
	"github.com/compozy/compozy/engine/stmanager"
	pbwf "github.com/compozy/compozy/pkg/pb/workflow"
)

func createWorkflowState(
	stm stmanager.Manager,
	tgInput *common.Input,
	pj *project.Config,
) (*workflow.State, error) {
	exec, err := workflow.NewExecution(tgInput, pj.GetEnv())
	if err != nil {
		return nil, fmt.Errorf("failed to create execution: %w", err)
	}
	st, err := workflow.NewState(exec)
	if err != nil {
		return nil, fmt.Errorf("failed to create state: %w", err)
	}
	if err := stm.SaveState(st); err != nil {
		return nil, fmt.Errorf("failed to save state: %w", err)
	}
	return st, nil
}

func (o *Orchestrator) SendTriggerWorkflow(
	wfID common.ID,
	pj *project.Config,
	tgInput *common.Input,
) (*workflow.TriggerWorkflowResponse, error) {
	st, err := createWorkflowState(*o.stManager, tgInput, pj)
	if err != nil {
		return nil, fmt.Errorf("failed to create workflow state: %w", err)
	}
	return workflow.SendTrigger(wfID, o.natsClient, st)
}

func (o *Orchestrator) SendExecuteWorkflow(cmd *pbwf.WorkflowTriggerCommand) error {
	return workflow.SendExecute(o.natsClient, cmd)
}
