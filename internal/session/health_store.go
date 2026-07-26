package session

import (
	"context"
	"time"

	"github.com/compozy/agh/internal/heartbeat"
)

// HealthStore is the durable store used by metadata-only session health.
type HealthStore interface {
	UpsertSessionHealth(ctx context.Context, health heartbeat.SessionHealth) (heartbeat.SessionHealth, error)
	GetSessionHealth(ctx context.Context, sessionID string) (heartbeat.SessionHealth, error)
	ListSessionHealth(ctx context.Context, query heartbeat.SessionHealthListQuery) ([]heartbeat.SessionHealth, error)
	ListSessionHealthByIDs(ctx context.Context, sessionIDs []string) ([]heartbeat.SessionHealth, error)
	ListSessionHealthRecoveryInputs(ctx context.Context, limit int) ([]heartbeat.SessionHealth, error)
	MarkSessionHealthStale(ctx context.Context, cutoff time.Time, updatedAt time.Time) (int64, error)
}
