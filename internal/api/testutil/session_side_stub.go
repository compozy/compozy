package testutil

import (
	"context"
	"github.com/compozy/compozy/internal/session"
)

func (s *StubSessionManager) OpenSide(ctx context.Context, opts session.OpenSideOpts) (session.DeriveResult, error) {
	if s.OpenSideFn != nil {
		return s.OpenSideFn(ctx, opts)
	}
	return session.DeriveResult{}, session.ErrSideUnsupported
}
func (s *StubSessionManager) ListSides(ctx context.Context, workspaceID, id string, archived bool) ([]*session.Info, error) {
	if s.ListSidesFn != nil {
		return s.ListSidesFn(ctx, workspaceID, id, archived)
	}
	return []*session.Info{}, nil
}
