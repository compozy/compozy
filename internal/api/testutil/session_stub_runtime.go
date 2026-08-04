package testutil

import (
	"context"

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
