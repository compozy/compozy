package update

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	errCoordinatorDependencies = errors.New("update: coordinator store, manager, and runtime are required")
	errCoordinatorLeaseTiming  = errors.New("update: coordinator renew interval must be shorter than its lease")
)

type coordinatorState struct {
	mu           sync.RWMutex
	transitionMu sync.Mutex
	store        *OperationStore
	operation    *Operation
	generation   string
}

func (s *coordinatorState) snapshot() *Operation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneOperation(s.operation)
}

func (s *coordinatorState) transition(ctx context.Context, transition Transition) (*Operation, error) {
	s.transitionMu.Lock()
	defer s.transitionMu.Unlock()
	s.mu.RLock()
	operationID := s.operation.ID
	revision := s.operation.Revision
	s.mu.RUnlock()
	updated, err := s.store.Transition(
		ctx,
		operationID,
		s.generation,
		revision,
		transition,
	)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.operation = cloneOperation(updated)
	s.mu.Unlock()
	return cloneOperation(updated), nil
}

func (s *coordinatorState) fence(ctx context.Context) error {
	s.transitionMu.Lock()
	defer s.transitionMu.Unlock()
	s.mu.RLock()
	operationID := s.operation.ID
	revision := s.operation.Revision
	s.mu.RUnlock()
	return s.store.Fence(ctx, operationID, s.generation, revision)
}

func (c *Coordinator) renewLease(
	ctx context.Context,
	state *coordinatorState,
	done <-chan struct{},
	errorsCh chan<- error,
) {
	ticker := time.NewTicker(c.renewInterval)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			operation := state.snapshot()
			if operation == nil || operation.Holder == nil {
				continue
			}
			holder := cloneHolder(operation.Holder)
			holder.LeaseExpiresAt = c.now().Add(c.leaseDuration)
			_, err := state.transition(ctx, Transition{
				Kind: TransitionRenewLease, Actor: holder.Surface, Target: operation.ActiveTarget,
				Holder: holder, Percent: operation.Percent,
			})
			if err != nil {
				select {
				case errorsCh <- err:
				case <-done:
				case <-ctx.Done():
				}
				return
			}
		}
	}
}
