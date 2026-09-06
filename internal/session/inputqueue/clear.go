package inputqueue

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/store"
)

func (s *Service) Clear(
	ctx context.Context,
	req store.SessionInputClearRequest,
) (store.SessionInputClearResult, error) {
	owner, ok := s.store.(store.SessionInputClearStore)
	if !ok {
		return store.SessionInputClearResult{}, errors.New("session: durable queue clear is unavailable")
	}
	req.Now = s.now()
	return owner.ClearSessionInputs(ctx, req)
}

func (s *Service) PendingClearTraces(ctx context.Context, sessionID string) ([]store.SessionInputClearTrace, error) {
	owner, ok := s.store.(store.SessionInputClearStore)
	if !ok {
		return nil, nil
	}
	return owner.ListPendingSessionInputClearTraces(ctx, sessionID)
}

func (s *Service) MarkClearTraceProjected(ctx context.Context, sessionID, entryID string) error {
	owner, ok := s.store.(store.SessionInputClearStore)
	if !ok {
		return errors.New("session: durable queue clear is unavailable")
	}
	return owner.MarkSessionInputClearTraceProjected(ctx, sessionID, entryID, s.now())
}
