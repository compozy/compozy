package state

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/pkg/nats"
)

// -----------------------------------------------------------------------------
// Types
// -----------------------------------------------------------------------------

type ID struct {
	Component nats.ComponentType `json:"component"`
	CorrID    common.ID          `json:"correlation_id"`
	ExecID    common.ID          `json:"execution_id"`
}

func NewID(comp nats.ComponentType, corrID common.ID, execID common.ID) ID {
	return ID{
		Component: comp,
		CorrID:    corrID,
		ExecID:    execID,
	}
}

func (id ID) String() string {
	// Format is "{component}_{correlationID}{executionID}"
	return fmt.Sprintf("%s_%s%s", id.Component, id.CorrID, id.ExecID)
}

func IDFromString(s string) (ID, error) {
	parts := strings.Split(s, "_")
	if len(parts) != 2 {
		return ID{}, fmt.Errorf("invalid state ID: %s", s)
	}
	compID := nats.ComponentType(parts[0])
	idPart := parts[1]
	const ksuidLength = 27
	if len(idPart) < 2*ksuidLength {
		return ID{}, fmt.Errorf("invalid ID part in state ID (expected at least %d chars): %s", 2*ksuidLength, s)
	}
	corrID := common.ID(idPart[:ksuidLength])
	execID := common.ID(idPart[ksuidLength:])
	return ID{Component: compID, CorrID: corrID, ExecID: execID}, nil
}

// -----------------------------------------------------------------------------
// State Interface
// -----------------------------------------------------------------------------

type State interface {
	GetID() ID
	GetCorrelationID() common.ID
	GetExecID() common.ID
	GetStatus() nats.EvStatusType
	GetEnv() *common.EnvMap
	GetTrigger() *common.Input
	GetInput() *common.Input
	GetOutput() *common.Output
	GetError() *Error
	SetStatus(status nats.EvStatusType)
	SetError(err *Error)
	UpdateFromEvent(event any) error
	FromParentState(parent State) error
	WithEnv(env common.EnvMap) error
	WithInput(input common.Input) error
}

// -----------------------------------------------------------------------------
// Error Structure
// -----------------------------------------------------------------------------

type Error struct {
	Message string         `json:"message,omitempty"`
	Code    string         `json:"code,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

// -----------------------------------------------------------------------------
// Base State
// -----------------------------------------------------------------------------

type BaseState struct {
	StateID ID                `json:"id"`
	Status  nats.EvStatusType `json:"status"`
	Trigger *common.Input     `json:"trigger,omitempty"`
	Input   *common.Input     `json:"input,omitempty"`
	Output  *common.Output    `json:"output,omitempty"`
	Env     *common.EnvMap    `json:"environment,omitempty"`
	Error   *Error            `json:"error,omitempty"`
}

func (s *BaseState) GetID() ID {
	return s.StateID
}

func (s *BaseState) GetCorrelationID() common.ID {
	return s.StateID.CorrID
}

func (s *BaseState) GetExecID() common.ID {
	return s.StateID.ExecID
}

func (s *BaseState) GetStatus() nats.EvStatusType {
	return s.Status
}

func (s *BaseState) GetEnv() *common.EnvMap {
	return s.Env
}

func (s *BaseState) GetTrigger() *common.Input {
	return s.Trigger
}

func (s *BaseState) GetInput() *common.Input {
	return s.Input
}

func (s *BaseState) GetOutput() *common.Output {
	return s.Output
}

func (s *BaseState) GetError() *Error {
	return s.Error
}

func (s *BaseState) SetStatus(status nats.EvStatusType) {
	s.Status = status
}

func (s *BaseState) SetError(err *Error) {
	s.Error = err
}

func (s *BaseState) FromParentState(parent State) error {
	return DefFromParent(s, parent)
}

func (s *BaseState) WithEnv(env common.EnvMap) error {
	newEnv, err := DefWithEnv(s, env)
	if err != nil {
		return err
	}
	s.Env = newEnv
	return nil
}

func (s *BaseState) WithInput(input common.Input) error {
	newInput, err := DefWithInput(s, input)
	if err != nil {
		return err
	}
	s.Input = newInput
	return nil
}

func (s *BaseState) UpdateFromEvent(_ any) error {
	return nil
}

// -----------------------------------------------------------------------------
// State Factory
// -----------------------------------------------------------------------------

type Option func(*BaseState)

func NewEmptyState(opts ...Option) State {
	state := &BaseState{
		StateID: ID{},
		Status:  nats.StatusPending,
		Trigger: &common.Input{},
		Input:   &common.Input{},
		Output:  &common.Output{},
		Env:     &common.EnvMap{},
		Error:   nil,
	}
	for _, opt := range opts {
		opt(state)
	}
	return state
}

func OptsWithID(id ID) Option {
	return func(s *BaseState) {
		s.StateID = id
	}
}

func OptsWithTrigger(trigger *common.Input) Option {
	return func(s *BaseState) {
		s.Trigger = trigger
	}
}

func OptsWithStatus(status nats.EvStatusType) Option {
	return func(s *BaseState) {
		s.Status = status
	}
}

func OptsWithInput(input *common.Input) Option {
	return func(s *BaseState) {
		s.Input = input
	}
}

func OptsWithOutput(output *common.Output) Option {
	return func(s *BaseState) {
		s.Output = output
	}
}

func OptsWithEnv(env *common.EnvMap) Option {
	return func(s *BaseState) {
		s.Env = env
	}
}

// -----------------------------------------------------------------------------
// State Map
// -----------------------------------------------------------------------------

type Map map[ID]State

func (sm Map) Get(id ID) (State, bool) {
	state, exists := sm[id]
	return state, exists
}

func (sm Map) Add(state State) {
	sm[state.GetID()] = state
}

func (sm Map) Remove(id ID) {
	delete(sm, id)
}
