package deadentity

import (
	"context"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/store"
)

type entityOperation struct {
	revision         uint64
	entityGeneration uint64
	done             chan struct{}
}

type workspaceState struct {
	_ byte
}

type entityState struct {
	mu sync.Mutex

	workspaceState       *workspaceState
	entityGeneration     uint64
	revision             uint64
	operation            *entityOperation
	loaded               bool
	consecutivePermanent int
	dead                 bool
	entity               store.DeadEntity
	nextProbe            time.Time
	clearPending         bool
}

func (s *Service) beginOperation(
	ctx context.Context,
	key store.DeadEntityKey,
) (*entityState, *entityOperation, error) {
	for {
		if err := contextError(ctx); err != nil {
			return nil, nil, err
		}
		state := s.stateFor(key)
		state.mu.Lock()
		if !s.stateActiveLocked(key, state) {
			state.mu.Unlock()
			continue
		}
		if state.operation == nil {
			state.revision++
			operation := &entityOperation{
				revision:         state.revision,
				entityGeneration: state.entityGeneration,
				done:             make(chan struct{}),
			}
			state.operation = operation
			state.mu.Unlock()
			return state, operation, nil
		}
		done := state.operation.done
		state.mu.Unlock()
		select {
		case <-done:
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		}
	}
}

func (s *Service) completeOperation(state *entityState, operation *entityOperation) {
	state.mu.Lock()
	if state.operation != nil && state.operation == operation && state.operation.revision == operation.revision {
		state.operation = nil
		close(operation.done)
	}
	state.mu.Unlock()
}

func (s *Service) stateFor(key store.DeadEntityKey) *entityState {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.states[key]
	if state == nil {
		workspace := s.workspaceStates[key.WorkspaceID]
		if workspace == nil {
			workspace = &workspaceState{}
			s.workspaceStates[key.WorkspaceID] = workspace
		}
		state = &entityState{workspaceState: workspace}
		s.states[key] = state
	}
	return state
}

func (s *Service) operationOwnerLocked(
	key store.DeadEntityKey,
	state *entityState,
	operation *entityOperation,
) bool {
	return state.operation != nil && state.operation == operation &&
		state.operation.revision == operation.revision &&
		state.entityGeneration == operation.entityGeneration &&
		s.stateActiveLocked(key, state)
}

func (s *Service) stateActiveLocked(key store.DeadEntityKey, state *entityState) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.states[key] == state &&
		s.workspaceStates[key.WorkspaceID] == state.workspaceState
}
