package deadentity

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

type permanentFailureAction uint8

const (
	permanentFailureRetired permanentFailureAction = iota
	permanentFailureCounted
	permanentFailurePersist
)

// RecordFailure advances or resets the permanent-failure streak for one attempted probe.
func (s *Service) RecordFailure(
	ctx context.Context,
	key store.DeadEntityKey,
	class FailureClass,
	reason string,
) error {
	normalized, err := validateRequest(ctx, key)
	if err != nil {
		return err
	}
	if class != FailureTransient && class != FailurePermanent {
		return fmt.Errorf("%w: %d", ErrInvalidFailureClass, class)
	}
	state, operation, err := s.beginOperation(ctx, normalized)
	if err != nil {
		return err
	}
	loaded, retired, err := s.ensureLoaded(ctx, normalized, state, operation)
	if err != nil {
		s.completeOperation(state, operation)
		return err
	}
	if retired || !loaded {
		s.completeOperation(state, operation)
		return nil
	}
	if class == FailureTransient {
		s.recordTransientFailure(normalized, state, operation)
		s.completeOperation(state, operation)
		return nil
	}
	return s.recordPermanentFailure(ctx, normalized, state, operation, reason)
}

func (s *Service) recordTransientFailure(
	key store.DeadEntityKey,
	state *entityState,
	operation *entityOperation,
) {
	state.mu.Lock()
	if s.operationOwnerLocked(key, state, operation) && !state.dead {
		state.consecutivePermanent = 0
	}
	state.mu.Unlock()
}

func (s *Service) recordPermanentFailure(
	ctx context.Context,
	key store.DeadEntityKey,
	state *entityState,
	operation *entityOperation,
	reason string,
) error {
	entity, refreshing, action := s.preparePermanentFailure(key, state, operation, reason)
	if action != permanentFailurePersist {
		s.completeOperation(state, operation)
		return nil
	}
	if err := s.persistPermanentFailure(ctx, key, state, operation, entity, refreshing); err != nil {
		return err
	}
	return nil
}

func (s *Service) preparePermanentFailure(
	key store.DeadEntityKey,
	state *entityState,
	operation *entityOperation,
	reason string,
) (store.DeadEntity, bool, permanentFailureAction) {
	state.mu.Lock()
	defer state.mu.Unlock()
	if !s.operationOwnerLocked(key, state, operation) {
		return store.DeadEntity{}, false, permanentFailureRetired
	}
	if !state.dead && state.consecutivePermanent+1 < s.permanentFailureThreshold {
		state.consecutivePermanent++
		return store.DeadEntity{}, false, permanentFailureCounted
	}
	entity := store.DeadEntity{
		DeadEntityKey: key,
		Reason:        boundedReason(reason),
		MarkedAt:      s.now().UTC(),
	}
	return entity, state.dead, permanentFailurePersist
}

func (s *Service) persistPermanentFailure(
	ctx context.Context,
	key store.DeadEntityKey,
	state *entityState,
	operation *entityOperation,
	entity store.DeadEntity,
	refreshing bool,
) error {
	if s.store == nil {
		if !refreshing {
			s.setPermanentFailureRetry(key, state, operation)
		}
		s.completeOperation(state, operation)
		return nil
	}
	if err := s.store.MarkDeadEntity(ctx, entity); err != nil {
		return s.handlePermanentMarkFailure(ctx, key, state, operation, refreshing, err)
	}
	if ctxErr := contextError(ctx); ctxErr != nil {
		s.invalidateLoadedState(key, state, operation)
		s.completeOperation(state, operation)
		return ctxErr
	}
	publication, committed := s.commitPermanentMark(key, state, operation, entity, refreshing)
	if !committed {
		s.completeOperation(state, operation)
		return nil
	}
	s.completeOperation(state, operation)
	if refreshing {
		return nil
	}
	return s.publishTransition(ctx, publication, entity, true)
}

func (s *Service) handlePermanentMarkFailure(
	ctx context.Context,
	key store.DeadEntityKey,
	state *entityState,
	operation *entityOperation,
	refreshing bool,
	err error,
) error {
	if ctxErr := contextError(ctx); ctxErr != nil {
		s.invalidateLoadedState(key, state, operation)
		s.completeOperation(state, operation)
		return ctxErr
	}
	s.logStoreFailure(markOperationName(refreshing), key, err)
	if !refreshing {
		s.setPermanentFailureRetry(key, state, operation)
	}
	s.completeOperation(state, operation)
	return nil
}

func (s *Service) commitPermanentMark(
	key store.DeadEntityKey,
	state *entityState,
	operation *entityOperation,
	entity store.DeadEntity,
	refreshing bool,
) (*transitionPublication, bool) {
	state.mu.Lock()
	defer state.mu.Unlock()
	if !s.operationOwnerLocked(key, state, operation) {
		return nil, false
	}
	if refreshing {
		state.entity = entity
		return nil, true
	}
	state.dead = true
	state.entity = entity
	state.consecutivePermanent = 0
	state.nextProbe = entity.MarkedAt.Add(s.recoveryInterval)
	state.clearPending = false
	return s.reserveTransitionPublicationLocked(key, operation), true
}
