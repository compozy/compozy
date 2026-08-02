package deadentity

import (
	"context"
	"time"

	"github.com/compozy/compozy/internal/store"
)

func (s *Service) ensureLoaded(
	ctx context.Context,
	key store.DeadEntityKey,
	state *entityState,
	operation *entityOperation,
) (loaded bool, retired bool, err error) {
	state.mu.Lock()
	if !s.operationOwnerLocked(key, state, operation) {
		state.mu.Unlock()
		return false, true, nil
	}
	if state.loaded {
		state.mu.Unlock()
		return true, false, nil
	}
	state.mu.Unlock()

	if s.store == nil {
		state.mu.Lock()
		if !s.operationOwnerLocked(key, state, operation) {
			state.mu.Unlock()
			return false, true, nil
		}
		state.loaded = true
		state.mu.Unlock()
		return true, false, nil
	}
	entity, found, err := s.store.FindDeadEntity(ctx, key.WorkspaceID, key.Kind, key.EntityID)
	if err != nil {
		if ctxErr := contextError(ctx); ctxErr != nil {
			return false, false, ctxErr
		}
		s.logStoreFailure("load", key, err)
		return false, false, nil
	}
	if ctxErr := contextError(ctx); ctxErr != nil {
		return false, false, ctxErr
	}

	state.mu.Lock()
	if !s.operationOwnerLocked(key, state, operation) {
		state.mu.Unlock()
		return false, true, nil
	}
	state.loaded = true
	state.dead = found
	state.clearPending = false
	if found {
		state.entity = entity
		state.nextProbe = entity.MarkedAt.UTC().Add(s.recoveryInterval)
	} else {
		state.entity = store.DeadEntity{}
		state.nextProbe = time.Time{}
	}
	state.mu.Unlock()
	return true, false, nil
}

func (s *Service) invalidateLoadedState(
	key store.DeadEntityKey,
	state *entityState,
	operation *entityOperation,
) {
	state.mu.Lock()
	if s.operationOwnerLocked(key, state, operation) {
		state.loaded = false
	}
	state.mu.Unlock()
}

func (s *Service) markClearPending(
	key store.DeadEntityKey,
	state *entityState,
	operation *entityOperation,
) {
	state.mu.Lock()
	if s.operationOwnerLocked(key, state, operation) {
		state.dead = false
		state.nextProbe = time.Time{}
		state.clearPending = true
	}
	state.mu.Unlock()
}

func (s *Service) setPermanentFailureRetry(
	key store.DeadEntityKey,
	state *entityState,
	operation *entityOperation,
) {
	state.mu.Lock()
	if s.operationOwnerLocked(key, state, operation) {
		state.consecutivePermanent = s.permanentFailureThreshold - 1
	}
	state.mu.Unlock()
}
