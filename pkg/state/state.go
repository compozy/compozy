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
	GetTrigger() *common.Input
	GetInput() *common.Input
	GetOutput() *common.Output
	SetStatus(status nats.EvStatusType)
	UpdateStatus(event any) error
	FromParentState(parent State) error
	WithEnv(env common.EnvMap) error
	WithInput(input common.Input) error
}

// -----------------------------------------------------------------------------
// Base State
// -----------------------------------------------------------------------------

type BaseState struct {
	ID      ID                `json:"id"`
	Status  nats.EvStatusType `json:"status"`
	Trigger common.Input      `json:"trigger,omitempty"`
	Input   common.Input      `json:"input,omitempty"`
	Output  common.Output     `json:"output,omitempty"`
	Env     common.EnvMap     `json:"environment,omitempty"`
	Errors  []string          `json:"errors,omitempty"`
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

func (s *BaseState) GetTrigger() *common.Input {
	return &s.Trigger
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
	return DefFromParentState(s, parent)
}

func (s *BaseState) WithEnv(env common.EnvMap) error {
	newEnv, err := DefWithEnv(s, env)
	if err != nil {
		return err
	}
	s.Env = *newEnv
	return nil
}

func (s *BaseState) WithInput(input common.Input) error {
	newInput, err := DefWithInput(s, input)
	if err != nil {
		return err
	}
	s.Input = *newInput
	return nil
}

func (s *BaseState) UpdateStatus(_ any) error {
	return nil
}

// -----------------------------------------------------------------------------
// State Factory
// -----------------------------------------------------------------------------

type Option func(*BaseState)

func NewEmptyState(opts ...Option) State {
	state := &BaseState{
		ID:      ID{},
		Status:  nats.StatusPending,
		Trigger: common.Input{},
		Input:   common.Input{},
		Output:  common.Output{},
		Env:     common.EnvMap{},
		Errors:  []string{},
	}
	for _, opt := range opts {
		opt(state)
	}
	return state
}

func WithID(id ID) Option {
	return func(s *BaseState) {
		s.ID = id
	}
}

func WithTrigger(trigger common.Input) Option {
	return func(s *BaseState) {
		s.Trigger = trigger
	}
}

func WithStatus(status nats.EvStatusType) Option {
	return func(s *BaseState) {
		s.Status = status
	}
}

func WithInput(input common.Input) Option {
	return func(s *BaseState) {
		s.Input = input
	}
}

func WithOutput(output common.Output) Option {
	return func(s *BaseState) {
		s.Output = output
	}
}

func WithEnv(env common.EnvMap) Option {
	return func(s *BaseState) {
		s.Env = env
	}
}

// -----------------------------------------------------------------------------
// State Map
// -----------------------------------------------------------------------------

// Map is a map of ID to State for efficient state lookups
type Map map[ID]State

// Get retrieves a state by its ID
func (sm Map) Get(id ID) (State, bool) {
	state, exists := sm[id]
	return state, exists
}

// Add adds a state to the map
func (sm Map) Add(state State) {
	sm[state.GetID()] = state
}

// Remove removes a state from the map
func (sm Map) Remove(id ID) {
	delete(sm, id)
}
