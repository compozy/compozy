package testutil

import (
	"context"

	"github.com/compozy/compozy/internal/session"
)

func (s *StubSessionManager) ContextEvents(ctx context.Context, id string) (session.ContextEvents, error) {
	if s.ContextEventsFn != nil {
		return s.ContextEventsFn(ctx, id)
	}
	var result session.ContextEvents
	var err error
	result.Usage, err = s.UsageEvents(ctx, id)
	if err != nil {
		return result, err
	}
	result.Deliveries, err = s.Deliveries(ctx, id)
	if err != nil {
		return result, err
	}
	result.Compactions, err = s.Compactions(ctx, id)
	if err != nil {
		return result, err
	}
	result.Settled, err = s.LatestSettledTurn(ctx, id)
	return result, err
}

func (s *StubSessionManager) WorkingDir(ctx context.Context, id string) (string, error) {
	if s.WorkingDirFn != nil {
		return s.WorkingDirFn(ctx, id)
	}
	return "", session.ErrSessionNotFound
}
