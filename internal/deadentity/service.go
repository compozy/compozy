package deadentity

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/compozy/agh/internal/redact"
	"github.com/compozy/agh/internal/store"
)

const maxPersistedReasonBytes = 512

type entityState struct {
	mu sync.Mutex

	loaded               bool
	consecutivePermanent int
	dead                 bool
	entity               store.DeadEntity
	nextProbe            time.Time
	clearPending         bool
}

// Service coordinates durable dead marks and opportunistic recovery attempts.
type Service struct {
	store  store.DeadEntityStore
	events store.EventSummaryStore
	logger *slog.Logger
	now    func() time.Time

	permanentFailureThreshold int
	recoveryInterval          time.Duration

	mu     sync.Mutex
	states map[store.DeadEntityKey]*entityState
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
	state := s.stateFor(normalized)
	state.mu.Lock()
	defer state.mu.Unlock()

	if !s.ensureLoaded(ctx, normalized, state) {
		return ProbeDecision{Allowed: true}, nil
	}
	if !state.dead {
		return ProbeDecision{Allowed: true}, nil
	}

	now := s.now().UTC()
	if state.nextProbe.IsZero() || !now.Before(state.nextProbe) {
		state.nextProbe = now.Add(s.recoveryInterval)
		return probeDecision(state, true, true), nil
	}
	return probeDecision(state, false, false), nil
}

// Status returns dead state without consuming a due recovery attempt.
func (s *Service) Status(ctx context.Context, key store.DeadEntityKey) (Status, error) {
	normalized, err := validateRequest(ctx, key)
	if err != nil {
		return Status{}, err
	}
	state := s.stateFor(normalized)
	state.mu.Lock()
	defer state.mu.Unlock()

	if !s.ensureLoaded(ctx, normalized, state) || !state.dead {
		return Status{}, nil
	}
	return Status{
		Dead:     true,
		Reason:   state.entity.Reason,
		MarkedAt: state.entity.MarkedAt,
	}, nil
}

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
	state := s.stateFor(normalized)
	state.mu.Lock()
	defer state.mu.Unlock()

	if !s.ensureLoaded(ctx, normalized, state) {
		return nil
	}
	if class == FailureTransient {
		if !state.dead {
			state.consecutivePermanent = 0
		}
		return nil
	}

	persistedReason := boundedReason(reason)
	if state.dead {
		s.refreshDeadMark(ctx, normalized, state, persistedReason)
		return nil
	}
	state.consecutivePermanent++
	if state.consecutivePermanent < s.permanentFailureThreshold {
		return nil
	}

	markedAt := s.now().UTC()
	entity := store.DeadEntity{
		DeadEntityKey: normalized,
		Reason:        persistedReason,
		MarkedAt:      markedAt,
	}
	if s.store == nil {
		state.consecutivePermanent = s.permanentFailureThreshold - 1
		return nil
	}
	if err := s.store.MarkDeadEntity(ctx, entity); err != nil {
		state.consecutivePermanent = s.permanentFailureThreshold - 1
		s.logStoreFailure("mark", normalized, err)
		return nil
	}
	state.dead = true
	state.entity = entity
	state.consecutivePermanent = 0
	state.nextProbe = markedAt.Add(s.recoveryInterval)
	s.emitTransition(ctx, entity, true)
	return nil
}

// RecordSuccess clears a durable mark and restores ordinary admission.
func (s *Service) RecordSuccess(ctx context.Context, key store.DeadEntityKey) error {
	normalized, err := validateRequest(ctx, key)
	if err != nil {
		return err
	}
	state := s.stateFor(normalized)
	state.mu.Lock()
	defer state.mu.Unlock()

	if !s.ensureLoaded(ctx, normalized, state) {
		return nil
	}
	state.consecutivePermanent = 0
	if !state.dead && !state.clearPending {
		return nil
	}

	cleared := state.entity
	state.dead = false
	state.nextProbe = time.Time{}
	if s.store == nil {
		state.clearPending = true
		return nil
	}
	if err := s.store.ClearDeadEntity(ctx, normalized.WorkspaceID, normalized.Kind, normalized.EntityID); err != nil {
		state.clearPending = true
		s.logStoreFailure("clear", normalized, err)
		return nil
	}
	state.clearPending = false
	state.entity = store.DeadEntity{}
	s.emitTransition(ctx, cleared, false)
	return nil
}

// List returns durable dead marks for one workspace for diagnostic projections.
func (s *Service) List(ctx context.Context, workspaceID string) ([]store.DeadEntity, error) {
	if ctx == nil {
		return nil, fmt.Errorf("deadentity: list context is required")
	}
	trimmed := strings.TrimSpace(workspaceID)
	if trimmed == "" {
		return nil, fmt.Errorf("%w: workspace_id is required", store.ErrInvalidDeadEntity)
	}
	if s == nil || s.store == nil {
		return []store.DeadEntity{}, nil
	}
	return s.store.ListDeadEntities(ctx, trimmed)
}

func (s *Service) stateFor(key store.DeadEntityKey) *entityState {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.states[key]
	if state == nil {
		state = &entityState{}
		s.states[key] = state
	}
	return state
}

func (s *Service) ensureLoaded(ctx context.Context, key store.DeadEntityKey, state *entityState) bool {
	if state.loaded {
		return true
	}
	if s.store == nil {
		state.loaded = true
		return true
	}
	entity, found, err := s.store.FindDeadEntity(ctx, key.WorkspaceID, key.Kind, key.EntityID)
	if err != nil {
		s.logStoreFailure("load", key, err)
		return false
	}
	state.loaded = true
	state.dead = found
	if found {
		state.entity = entity
		state.nextProbe = entity.MarkedAt.UTC().Add(s.recoveryInterval)
	}
	return true
}

func (s *Service) refreshDeadMark(
	ctx context.Context,
	key store.DeadEntityKey,
	state *entityState,
	reason string,
) {
	markedAt := s.now().UTC()
	entity := store.DeadEntity{DeadEntityKey: key, Reason: reason, MarkedAt: markedAt}
	if s.store == nil {
		return
	}
	if err := s.store.MarkDeadEntity(ctx, entity); err != nil {
		s.logStoreFailure("refresh", key, err)
		return
	}
	state.entity = entity
}

func validateRequest(ctx context.Context, key store.DeadEntityKey) (store.DeadEntityKey, error) {
	if ctx == nil {
		return store.DeadEntityKey{}, fmt.Errorf("deadentity: context is required")
	}
	normalized := key.Normalize()
	if err := normalized.Validate(); err != nil {
		return store.DeadEntityKey{}, err
	}
	return normalized, nil
}

func probeDecision(state *entityState, allowed bool, recovery bool) ProbeDecision {
	return ProbeDecision{
		Allowed:  allowed,
		Recovery: recovery,
		Dead:     state.dead,
		Reason:   state.entity.Reason,
		MarkedAt: state.entity.MarkedAt,
	}
}

func boundedReason(reason string) string {
	bounded := strings.TrimSpace(redact.String(reason))
	if bounded == "" {
		bounded = "permanent runtime failure"
	}
	for len(bounded) > maxPersistedReasonBytes {
		_, size := utf8.DecodeLastRuneInString(bounded)
		if size == 0 {
			break
		}
		bounded = bounded[:len(bounded)-size]
	}
	return bounded
}

func (s *Service) logStoreFailure(operation string, key store.DeadEntityKey, err error) {
	s.logger.Warn(
		"deadentity: durable transition failed open",
		"operation", operation,
		"workspace_id", key.WorkspaceID,
		"kind", key.Kind,
		"entity_id", key.EntityID,
		"error", err,
	)
}
