package state

import (
	"fmt"
	"time"

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
	ID        ID                     `json:"id"`
	Status    nats.EvStatusType      `json:"status"`
	Input     common.Input           `json:"input,omitempty"`
	Output    common.Output          `json:"output,omitempty"`
	Env       common.EnvMap          `json:"environment,omitempty"`
	Errors    []string               `json:"errors,omitempty"`
	Context   map[string]interface{} `json:"context,omitempty"`
	UpdatedAt time.Time              `json:"updated_at"`
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

// UpdateFromEvent updates the state based on an event
func (s *BaseState) UpdateFromEvent(event nats.Event) error {
	// Update the status if available
	status, err := nats.StatusFromEvent(event)
	if err == nil && status != "" {
		s.Status = status
	}

	// Update inputs or outputs based on event payload
	if result, err := nats.ResultFromEvent(event); err == nil && result != nil {
		// If there's output in the result, update the state's output
		if result.GetOutput() != nil {
			// Convert structpb.Struct to common.Output (map[string]interface{})
			outputMap := make(common.Output)
			for k, v := range result.GetOutput().AsMap() {
				outputMap[k] = v
			}

			// Merge with existing output
			for k, v := range outputMap {
				s.Output[k] = v
			}
		}

		// If there's an error in the result, store it in the state
		if result.GetError() != nil {
			if s.Errors == nil {
				s.Errors = make([]string, 0)
			}
			s.Errors = append(s.Errors, result.GetError().GetMessage())
		}
	}

	// Update the context if available in the payload
	if payload := event.GetPayload(); payload != nil {
		if ctx := payload.GetContext(); ctx != nil {
			// Convert context to map[string]interface{} and merge with existing context
			contextMap := ctx.AsMap()
			for k, v := range contextMap {
				s.Context[k] = v
			}
		}
	}

	// Update the last updated timestamp
	s.UpdatedAt = time.Now().UTC()

	return nil
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
