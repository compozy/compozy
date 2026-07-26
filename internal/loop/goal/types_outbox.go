package goal

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/loop"
)

// SessionOutboxCause is the closed session-projection transition vocabulary.
type SessionOutboxCause string

const (
	SessionOutboxCauseStart   SessionOutboxCause = "start"
	SessionOutboxCauseReplace SessionOutboxCause = "replace"
	SessionOutboxCauseStatus  SessionOutboxCause = "status"
	SessionOutboxCauseClear   SessionOutboxCause = "clear"
	SessionOutboxCauseReseed  SessionOutboxCause = "reseed"
)

// SessionOutboxEvent is one restart-safe projection notification.
type SessionOutboxEvent struct {
	ID              int64
	EventID         string
	WorkspaceID     loop.WorkspaceID
	OriginSessionID string
	LoopRunID       loop.RunID
	BoundSessionID  *string
	Cause           SessionOutboxCause
	CreatedAt       time.Time
	DeliveredAt     *time.Time
}

// EnqueueSessionOutboxRequest inserts one deterministic idempotency identity.
type EnqueueSessionOutboxRequest struct {
	EventID         string
	WorkspaceID     loop.WorkspaceID
	OriginSessionID string
	LoopRunID       loop.RunID
	BoundSessionID  *string
	Cause           SessionOutboxCause
	CreatedAt       time.Time
}

// Validate rejects incomplete or open-ended session projection identities.
func (r EnqueueSessionOutboxRequest) Validate() error {
	fields := []struct {
		name  string
		value string
	}{
		{name: "event_id", value: r.EventID},
		{name: "workspace_id", value: string(r.WorkspaceID)},
		{name: "origin_session_id", value: r.OriginSessionID},
		{name: "loop_run_id", value: string(r.LoopRunID)},
	}
	for _, field := range fields {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("%w: goal outbox %s is required", loop.ErrValidation, field.name)
		}
	}
	if r.BoundSessionID != nil && strings.TrimSpace(*r.BoundSessionID) == "" {
		return fmt.Errorf("%w: goal outbox bound_session_id must be nonempty when present", loop.ErrValidation)
	}
	if !r.Cause.Valid() {
		return fmt.Errorf("%w: invalid goal outbox cause %q", loop.ErrValidation, r.Cause)
	}
	if r.Cause == SessionOutboxCauseClear && r.BoundSessionID != nil {
		return fmt.Errorf("%w: goal outbox clear must not retain bound_session_id", loop.ErrValidation)
	}
	if r.Cause != SessionOutboxCauseClear && r.BoundSessionID == nil {
		return fmt.Errorf("%w: goal outbox %s requires bound_session_id", loop.ErrValidation, r.Cause)
	}
	if r.CreatedAt.IsZero() {
		return fmt.Errorf("%w: goal outbox created_at is required", loop.ErrValidation)
	}
	return nil
}

// Valid reports whether the cause belongs to the closed projection vocabulary.
func (c SessionOutboxCause) Valid() bool {
	switch c {
	case SessionOutboxCauseStart,
		SessionOutboxCauseReplace,
		SessionOutboxCauseStatus,
		SessionOutboxCauseClear,
		SessionOutboxCauseReseed:
		return true
	default:
		return false
	}
}

// SessionOutboxStore bridges committed global state into the per-session event store.
type SessionOutboxStore interface {
	EnqueueGoalSessionOutbox(context.Context, EnqueueSessionOutboxRequest) (SessionOutboxEvent, error)
	ClaimGoalSessionOutbox(context.Context, int) ([]SessionOutboxEvent, error)
	AcknowledgeGoalSessionOutbox(context.Context, string, time.Time) error
}
