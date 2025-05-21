package workflow

import (
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/state"
	"github.com/compozy/compozy/pkg/nats"
)

// -----------------------------------------------------------------------------
// Execution
// -----------------------------------------------------------------------------

type Execution struct {
	CorrID       common.CorrID
	ExecID       common.ExecID
	TriggerInput *common.Input
	ProjectEnv   common.EnvMap
	WorkflowEnv  common.EnvMap
}

func NewExecution(tgInput *common.Input, pjEnv common.EnvMap) *Execution {
	corrID := common.NewCorrID()
	execID := common.NewExecID()
	return &Execution{
		CorrID:       corrID,
		ExecID:       execID,
		TriggerInput: tgInput,
		ProjectEnv:   pjEnv,
		WorkflowEnv:  make(common.EnvMap),
	}
}

// -----------------------------------------------------------------------------
// Initializer
// -----------------------------------------------------------------------------

type StateInitializer struct {
	*state.CommonInitializer
	*Execution
}

func (wi *StateInitializer) Initialize() (*State, error) {
	env, err := wi.MergeEnv(wi.ProjectEnv, wi.WorkflowEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to merge env: %w", err)
	}
	bsState := &state.BaseState{
		StateID: state.NewID(nats.ComponentWorkflow, wi.CorrID, wi.ExecID),
		Status:  nats.StatusPending,
		Input:   &common.Input{},
		Output:  &common.Output{},
		Trigger: wi.TriggerInput,
		Env:     env,
	}
	st := &State{
		BaseState:      *bsState,
		WorkflowExecID: wi.ExecID,
	}
	if err := wi.Normalizer.ParseTemplates(st); err != nil {
		return nil, err
	}
	return st, nil
}

// -----------------------------------------------------------------------------
// State
// -----------------------------------------------------------------------------

type State struct {
	state.BaseState
	WorkflowExecID common.ExecID `json:"workflow_exec_id,omitempty"`
}

func NewState(exec *Execution) (*State, error) {
	initializer := &StateInitializer{
		CommonInitializer: state.NewCommonInitializer(),
		Execution:         exec,
	}
	st, err := initializer.Initialize()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize workflow state: %w", err)
	}
	return st, nil
}

// GetWorkflowExecID returns the workflow execution ID
func (s *State) GetWorkflowExecID() common.ExecID {
	return s.WorkflowExecID
}
