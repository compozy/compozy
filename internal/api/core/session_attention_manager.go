package core

import (
	"context"

	"github.com/compozy/compozy/internal/store"
)

// SessionAttentionManager owns operator presence and durable interaction discovery.
type SessionAttentionManager interface {
	SessionPresence(ctx context.Context, sessionID string, leaseID string, visible bool) (string, error)
	PendingInteractions(
		ctx context.Context,
		sessionID string,
		statuses []string,
	) ([]store.PendingInteraction, error)
	AttentionSummary(ctx context.Context) (store.SessionAttentionSummary, error)
}
