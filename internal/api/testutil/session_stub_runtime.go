package testutil

import (
	"context"

	"github.com/compozy/compozy/internal/acp"

	"github.com/compozy/compozy/internal/session"
)

func (s StubSessionManager) SetRuntimeSelection(
	ctx context.Context,
	id string,
	selection session.RuntimeSelection,
	expectedRevision int64,
) (*session.Info, error) {
	if s.SetRuntimeSelectionFn != nil {
		return s.SetRuntimeSelectionFn(ctx, id, selection, expectedRevision)
	}
	return nil, session.ErrSessionNotFound
}

func (s StubSessionManager) ClearRuntimeSelection(
	ctx context.Context,
	id string,
	expectedRevision int64,
) (*session.Info, error) {
	if s.ClearRuntimeSelectionFn != nil {
		return s.ClearRuntimeSelectionFn(ctx, id, expectedRevision)
	}
	return nil, session.ErrSessionNotFound
}

func (s StubSessionManager) RequestCompaction(
	ctx context.Context,
	id string,
) (session.CompactionRequestResult, <-chan acp.AgentEvent, error) {
	if s.RequestCompactionFn != nil {
		return s.RequestCompactionFn(ctx, id)
	}
	return session.CompactionRequestResult{}, nil, session.ErrCompactionUnsupported
}
func (s StubSessionManager) CompactionBoundary(ctx context.Context, id string) (*int64, error) {
	if s.CompactionBoundaryFn != nil {
		return s.CompactionBoundaryFn(ctx, id)
	}
	return nil, nil
}
