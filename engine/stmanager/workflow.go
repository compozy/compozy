package stmanager

import (
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/domain/project"
	"github.com/compozy/compozy/engine/domain/workflow"
	"github.com/compozy/compozy/pkg/nats"
)

func (stm *Manager) CreateWorkflowState(ti *common.Input, pj *project.Config) (*workflow.State, error) {
	ctx, err := workflow.NewContext(ti, pj.GetEnv())
	if err != nil {
		return nil, fmt.Errorf("failed to create execution: %w", err)
	}
	st, err := workflow.NewState(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create state: %w", err)
	}
	if err := stm.SaveState(st); err != nil {
		return nil, fmt.Errorf("failed to save state: %w", err)
	}
	return st, nil
}

func (stm *Manager) LoadWorkflowState(corrID common.ID, execID common.ID) (*workflow.State, error) {
	st, err := stm.GetWorkflowState(corrID, execID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow state: %w", err)
	}
	if st.GetStatus() != nats.StatusRunning {
		return nil, fmt.Errorf("workflow is not in running state")
	}
	wfState, ok := st.(*workflow.State)
	if !ok {
		return nil, fmt.Errorf("failed to cast workflow state: %w", err)
	}
	return wfState, nil
}
