package agent

import (
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/state"
	"github.com/compozy/compozy/pkg/nats"
	pb "github.com/compozy/compozy/pkg/pb/agent"
)

// -----------------------------------------------------------------------------
// Initializer
// -----------------------------------------------------------------------------

type StateInitializer struct {
	*state.CommonInitializer
	*Execution
}

func (ai *StateInitializer) Initialize() (*State, error) {
	env, err := ai.MergeEnv(ai.TaskEnv, ai.AgentEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to merge env: %w", err)
	}
	input, err := ai.AgentInput.Merge(*ai.TaskInput)
	if err != nil {
		return nil, fmt.Errorf("failed to merge input: %w", err)
	}
	bsState := &state.BaseState{
		StateID: state.NewID(nats.ComponentAgent, ai.CorrID, ai.AgentExecID),
		Status:  nats.StatusPending,
		Input:   &input,
		Output:  &common.Output{},
		Trigger: ai.TriggerInput,
		Env:     env,
	}
	st := &State{
		BaseState:      *bsState,
		WorkflowExecID: ai.WorkflowExecID,
		TaskExecID:     ai.TaskExecID,
		AgentExecID:    ai.AgentExecID,
	}
	if err := ai.Normalizer.ParseTemplates(st); err != nil {
		return nil, err
	}
	return st, nil
}

// -----------------------------------------------------------------------------
// State
// -----------------------------------------------------------------------------

type State struct {
	state.BaseState
	WorkflowExecID common.ID `json:"workflow_exec_id"`
	TaskExecID     common.ID `json:"task_exec_id"`
	AgentExecID    common.ID `json:"agent_exec_id"`
}

func NewAgentState(exec *Execution) (*State, error) {
	initializer := &StateInitializer{
		CommonInitializer: state.NewCommonInitializer(),
		Execution:         exec,
	}
	st, err := initializer.Initialize()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize agent state: %w", err)
	}
	return st, nil
}

// UpdateFromEvent updates the agent state based on the event type
func (s *State) UpdateFromEvent(event any) error {
	switch evt := event.(type) {
	case *pb.AgentExecutionStartedEvent:
		return s.handleStartedEvent(evt)
	case *pb.AgentExecutionSuccessEvent:
		return s.handleSuccessEvent(evt)
	case *pb.AgentExecutionFailedEvent:
		return s.handleFailedEvent(evt)
	default:
		return fmt.Errorf("unsupported event type for agent state update: %T", evt)
	}
}

func (s *State) handleStartedEvent(_ *pb.AgentExecutionStartedEvent) error {
	s.Status = nats.StatusRunning
	return nil
}

func (s *State) handleSuccessEvent(evt *pb.AgentExecutionSuccessEvent) error {
	s.Status = nats.StatusSuccess
	if evt.GetPayload() == nil || evt.GetPayload().GetResult() == nil {
		return nil
	}
	state.SetResultData(&s.BaseState, evt.GetPayload().GetResult())
	return nil
}

func (s *State) handleFailedEvent(evt *pb.AgentExecutionFailedEvent) error {
	s.Status = nats.StatusFailed
	if evt.GetPayload() == nil || evt.GetPayload().GetResult() == nil {
		return nil
	}
	state.SetResultData(&s.BaseState, evt.GetPayload().GetResult())
	return nil
}
