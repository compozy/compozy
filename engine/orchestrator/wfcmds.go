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
) error {
	exec := workflow.NewStateParams(tgInput, pj.GetEnv())

	// Save the state before triggering the workflow
	st, err := workflow.NewState(exec)
	if err != nil {
		return fmt.Errorf("failed to create state: %w", err)
	}
	if err := o.stManager.SaveState(st); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	// Trigger the workflow
	if err := workflow.TriggerWorkflow(wfID, o.natsClient, exec); err != nil {
		return fmt.Errorf("failed to trigger workflow: %w", err)
	}

	return nil
}
