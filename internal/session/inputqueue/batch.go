package inputqueue

import (
	"context"
	"time"

	"github.com/compozy/compozy/internal/store"
)

type batchDispatchStore interface {
	ClaimSessionInputBatch(
		context.Context,
		string,
		int,
		func(*store.SessionInputQueueEntry) bool,
		time.Time,
	) ([]store.SessionInputQueueEntry, error)
	MarkSessionInputSentInTurn(context.Context, string, string, string, time.Time) error
	PromoteQueuedAgentSessionInputToSteer(
		context.Context,
		string,
		string,
		string,
		time.Time,
	) (store.SessionInputQueueEntry, bool, error)
	RevertPromotedAgentSessionInput(context.Context, string, string, time.Time) (bool, error)
}

// ClaimBatch leases up to limit consecutive entries behind an already claimed head while accept
// approves them. A store without batch support dispatches entries one at a time.
func (s *Service) ClaimBatch(
	ctx context.Context,
	sessionID string,
	limit int,
	accept func(*store.SessionInputQueueEntry) bool,
) ([]store.SessionInputQueueEntry, error) {
	owner, ok := s.store.(batchDispatchStore)
	if !ok {
		return nil, nil
	}
	return owner.ClaimSessionInputBatch(ctx, sessionID, limit, accept, s.now())
}

// MarkSentInTurn records successful dispatch together with the turn that consumed the entry.
func (s *Service) MarkSentInTurn(ctx context.Context, sessionID, entryID, turnID string) error {
	owner, ok := s.store.(batchDispatchStore)
	if !ok {
		return s.MarkSent(ctx, sessionID, entryID)
	}
	return owner.MarkSessionInputSentInTurn(ctx, sessionID, entryID, turnID, s.now())
}

// PromoteQueuedAgentToSteer stages one queued agent message as steering for targetTurnID while
// keeping its identity. It reports false when the entry is no longer eligible.
func (s *Service) PromoteQueuedAgentToSteer(
	ctx context.Context,
	sessionID, entryID, targetTurnID string,
) (store.SessionInputQueueEntry, bool, error) {
	owner, ok := s.store.(batchDispatchStore)
	if !ok {
		return store.SessionInputQueueEntry{}, false, nil
	}
	return owner.PromoteQueuedAgentSessionInputToSteer(ctx, sessionID, entryID, targetTurnID, s.now())
}

// RevertPromotedAgent returns a promoted agent message whose delivery never started to the queue.
func (s *Service) RevertPromotedAgent(ctx context.Context, sessionID, entryID string) (bool, error) {
	owner, ok := s.store.(batchDispatchStore)
	if !ok {
		return false, nil
	}
	return owner.RevertPromotedAgentSessionInput(ctx, sessionID, entryID, s.now())
}
