package state

import (
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/pkg/nats"
)

// -----------------------------------------------------------------------------
// Types
// -----------------------------------------------------------------------------

type ID struct {
	Component     nats.ComponentType `json:"component"`
	ComponentID   string             `json:"component_id"`
	CorrelationID string             `json:"correlation_id"`
}

func NewID(component nats.ComponentType, componentID, correlationID string) ID {
	return ID{
		Component:     component,
		ComponentID:   componentID,
		CorrelationID: correlationID,
	}
}

func (id ID) String() string {
	return fmt.Sprintf("%s:%s:%s", id.Component, id.ComponentID, id.CorrelationID)
}

// -----------------------------------------------------------------------------
// State Interface
// -----------------------------------------------------------------------------

type State interface {
	GetID() ID
	GetStatus() nats.EvStatusType
	GetEnv() *common.EnvMap
	GetInput() *common.Input
	GetOutput() *common.Output
	SetStatus(status nats.EvStatusType)
	UpdateFromEvent(event nats.Event) error
	FromParentState(parent State) error
	WithEnv(env common.EnvMap) error
	WithInput(input common.Input) error
}

// -----------------------------------------------------------------------------
// Base State
// -----------------------------------------------------------------------------

type BaseState struct {
	ID     ID                `json:"id"`
	Status nats.EvStatusType `json:"status"`
	Input  common.Input      `json:"input,omitempty"`
	Output common.Output     `json:"output,omitempty"`
	Env    common.EnvMap     `json:"environment,omitempty"`
}

func (s *BaseState) GetID() ID {
	return s.ID
}

func (s *BaseState) GetStatus() nats.EvStatusType {
	return s.Status
}

func (s *BaseState) GetEnv() *common.EnvMap {
	return &s.Env
}

func (s *BaseState) GetInput() *common.Input {
	return &s.Input
}

func (s *BaseState) GetOutput() *common.Output {
	return &s.Output
}

func (s *BaseState) SetStatus(status nats.EvStatusType) {
	s.Status = status
}

func (s *BaseState) FromParentState(parent State) error {
	return FromParentState(s, parent)
}

func (s *BaseState) WithEnv(env common.EnvMap) error {
	newEnv, err := WithEnv(s, env)
	if err != nil {
		return err
	}
	s.Env = *newEnv
	return nil
}

func (s *BaseState) WithInput(input common.Input) error {
	newInput, err := WithInput(s, input)
	if err != nil {
		return err
	}
	s.Input = *newInput
	return nil
}

// We need this to avoid errors when implementing the State interface here
// but each component will implement this differently
func (s *BaseState) UpdateFromEvent(event nats.Event) error {
	return nil
}

// -----------------------------------------------------------------------------
// State Map
// -----------------------------------------------------------------------------

type StateMap map[ID]State

func (sm StateMap) Get(id ID) (State, bool) {
	state, exists := sm[id]
	return state, exists
}

func (sm StateMap) Add(state State) {
	sm[state.GetID()] = state
}

func (sm StateMap) Remove(id ID) {
	delete(sm, id)
}
