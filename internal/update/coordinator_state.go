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
	mu         sync.Mutex
	store      *OperationStore
	operation  *Operation
	generation string
}

func (s *coordinatorState) snapshot() *Operation {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneOperation(s.operation)
}

func (s *coordinatorState) transition(ctx context.Context, transition Transition) (*Operation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	updated, err := s.store.Transition(
		ctx,
		s.operation.ID,
		s.generation,
		s.operation.Revision,
		transition,
	)
	if err != nil {
		return nil, err
	}
	s.operation = cloneOperation(updated)
	return cloneOperation(updated), nil
}

func (s *coordinatorState) fence(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.store.Fence(ctx, s.operation.ID, s.generation, s.operation.Revision)
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
