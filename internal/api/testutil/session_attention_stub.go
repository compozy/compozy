package testutil

import (
	"context"

	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
)

// StubSessionAttention configures the attention-specific behavior of StubSessionManager.
type StubSessionAttention struct {
	PresenceFn func(context.Context, string, string, bool) (string, error)
	PendingFn  func(context.Context, string, []string) ([]store.PendingInteraction, error)
	SummaryFn  func(context.Context) (store.SessionAttentionSummary, error)
	WaitFn     func(context.Context, session.WaitRequest) (session.WaitOutcome, error)
}

// SessionPresence acquires, renews, or releases a test presence lease.
func (s StubSessionManager) SessionPresence(
	ctx context.Context,
	sessionID string,
	leaseID string,
	visible bool,
) (string, error) {
	if s.Attention.PresenceFn != nil {
		return s.Attention.PresenceFn(ctx, sessionID, leaseID, visible)
	}
	return "", session.ErrSessionPresenceLeaseNotFound
}

// PendingInteractions lists a test session's durable interaction projection.
func (s StubSessionManager) PendingInteractions(
	ctx context.Context,
	sessionID string,
	statuses []string,
) ([]store.PendingInteraction, error) {
	if s.Attention.PendingFn != nil {
		return s.Attention.PendingFn(ctx, sessionID, statuses)
	}
	return []store.PendingInteraction{}, nil
}

// AttentionSummary returns the configured exact test aggregate.
func (s StubSessionManager) AttentionSummary(ctx context.Context) (store.SessionAttentionSummary, error) {
	if s.Attention.SummaryFn != nil {
		return s.Attention.SummaryFn(ctx)
	}
	return store.SessionAttentionSummary{}, nil
}

// WaitForBadge returns the configured bounded test wait outcome.
func (s StubSessionManager) WaitForBadge(
	ctx context.Context,
	request session.WaitRequest,
) (session.WaitOutcome, error) {
	if s.Attention.WaitFn != nil {
		return s.Attention.WaitFn(ctx, request)
	}
	return session.WaitOutcome{}, session.ErrWaitValidation
}

var _ core.SessionAttentionManager = (*StubSessionManager)(nil)
var _ core.SessionWaitManager = (*StubSessionManager)(nil)
