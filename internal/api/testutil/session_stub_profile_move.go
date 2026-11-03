package testutil

import (
	"context"

	"github.com/compozy/compozy/internal/session"
)

// MoveToProfile delegates to MoveToProfileFn or reports a missing session.
func (s *StubSessionManager) MoveToProfile(
	ctx context.Context,
	opts session.MoveToProfileOpts,
) (session.MoveToProfileResult, error) {
	if s.MoveToProfileFn != nil {
		return s.MoveToProfileFn(ctx, opts)
	}
	return session.MoveToProfileResult{}, session.ErrSessionNotFound
}
