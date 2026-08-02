package deadentity

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/store"
)

// Service coordinates durable dead marks and opportunistic recovery attempts.
type Service struct {
	store  store.DeadEntityStore
	events store.EventSummaryStore
	logger *slog.Logger
	now    func() time.Time

	permanentFailureThreshold int
	recoveryInterval          time.Duration

	mu               sync.Mutex
	states           map[store.DeadEntityKey]*entityState
	workspaceStates  map[string]*workspaceState
	publicationTails map[store.DeadEntityKey]*transitionPublication
}

// New constructs a workspace-scoped dead-entity coordinator.
func New(deadStore store.DeadEntityStore, opts ...Option) *Service {
	service := &Service{
		store:                     deadStore,
		logger:                    slog.Default(),
		now:                       time.Now,
		permanentFailureThreshold: DefaultPermanentFailureThreshold,
		recoveryInterval:          DefaultRecoveryInterval,
		states:                    make(map[store.DeadEntityKey]*entityState),
		workspaceStates:           make(map[string]*workspaceState),
		publicationTails:          make(map[store.DeadEntityKey]*transitionPublication),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	if service.logger == nil {
		service.logger = slog.Default()
	}
	if service.now == nil {
		service.now = time.Now
	}
	if service.permanentFailureThreshold < 1 {
		service.permanentFailureThreshold = DefaultPermanentFailureThreshold
	}
	if service.recoveryInterval <= 0 {
		service.recoveryInterval = DefaultRecoveryInterval
	}
	return service
}

// BeforeProbe admits ordinary live attempts and one due recovery attempt per interval.
func (s *Service) BeforeProbe(ctx context.Context, key store.DeadEntityKey) (ProbeDecision, error) {
	normalized, err := validateRequest(ctx, key)
	if err != nil {
		return ProbeDecision{}, err
	}
	state, operation, err := s.beginOperation(ctx, normalized)
	if err != nil {
		return ProbeDecision{}, err
	}
	loaded, retired, err := s.ensureLoaded(ctx, normalized, state, operation)
	if err != nil {
		s.completeOperation(state, operation)
		return ProbeDecision{}, err
	}
	if retired {
		s.completeOperation(state, operation)
		return ProbeDecision{Allowed: true}, nil
	}
	if !loaded {
		s.completeOperation(state, operation)
		return ProbeDecision{Allowed: true}, nil
	}

	now := s.now().UTC()
	state.mu.Lock()
	if !s.operationOwnerLocked(normalized, state, operation) {
		state.mu.Unlock()
		s.completeOperation(state, operation)
		return ProbeDecision{Allowed: true}, nil
	}
	if !state.dead {
		state.mu.Unlock()
		s.completeOperation(state, operation)
		return ProbeDecision{Allowed: true}, nil
	}
	if state.nextProbe.IsZero() || !now.Before(state.nextProbe) {
		state.nextProbe = now.Add(s.recoveryInterval)
		decision := probeDecision(state, true, true)
		state.mu.Unlock()
		s.completeOperation(state, operation)
		return decision, nil
	}
	decision := probeDecision(state, false, false)
	state.mu.Unlock()
	s.completeOperation(state, operation)
	return decision, nil
}

// Status returns dead state without consuming a due recovery attempt.
func (s *Service) Status(ctx context.Context, key store.DeadEntityKey) (Status, error) {
	normalized, err := validateRequest(ctx, key)
	if err != nil {
		return Status{}, err
	}
	state, operation, err := s.beginOperation(ctx, normalized)
	if err != nil {
		return Status{}, err
	}
	loaded, retired, err := s.ensureLoaded(ctx, normalized, state, operation)
	if err != nil {
		s.completeOperation(state, operation)
		return Status{}, err
	}
	if retired || !loaded {
		s.completeOperation(state, operation)
		return Status{}, nil
	}

	state.mu.Lock()
	if !s.operationOwnerLocked(normalized, state, operation) || !state.dead {
		state.mu.Unlock()
		s.completeOperation(state, operation)
		return Status{}, nil
	}
	status := Status{
		Dead:     true,
		Reason:   state.entity.Reason,
		MarkedAt: state.entity.MarkedAt,
	}
	state.mu.Unlock()
	s.completeOperation(state, operation)
	return status, nil
}

// RecordSuccess clears a durable mark and restores ordinary admission.
func (s *Service) RecordSuccess(ctx context.Context, key store.DeadEntityKey) error {
	normalized, err := validateRequest(ctx, key)
	if err != nil {
		return err
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

	state.mu.Lock()
	if !s.operationOwnerLocked(normalized, state, operation) {
		state.mu.Unlock()
		s.completeOperation(state, operation)
		return nil
	}
	state.consecutivePermanent = 0
	if !state.dead && !state.clearPending {
		state.mu.Unlock()
		s.completeOperation(state, operation)
		return nil
	}
	cleared := state.entity
	state.mu.Unlock()

	if s.store == nil {
		s.markClearPending(normalized, state, operation)
		s.completeOperation(state, operation)
		return nil
	}
	if err := s.store.ClearDeadEntity(ctx, normalized.WorkspaceID, normalized.Kind, normalized.EntityID); err != nil {
		if ctxErr := contextError(ctx); ctxErr != nil {
			s.invalidateLoadedState(normalized, state, operation)
			s.completeOperation(state, operation)
			return ctxErr
		}
		s.markClearPending(normalized, state, operation)
		s.logStoreFailure("clear", normalized, err)
		s.completeOperation(state, operation)
		return nil
	}
	if ctxErr := contextError(ctx); ctxErr != nil {
		s.invalidateLoadedState(normalized, state, operation)
		s.completeOperation(state, operation)
		return ctxErr
	}

	state.mu.Lock()
	if !s.operationOwnerLocked(normalized, state, operation) {
		state.mu.Unlock()
		s.completeOperation(state, operation)
		return nil
	}
	state.dead = false
	state.nextProbe = time.Time{}
	state.clearPending = false
	state.entity = store.DeadEntity{}
	publication := s.reserveTransitionPublicationLocked(normalized, operation)
	state.mu.Unlock()
	s.completeOperation(state, operation)
	return s.publishTransition(ctx, publication, cleared, false)
}
