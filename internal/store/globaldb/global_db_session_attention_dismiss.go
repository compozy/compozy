package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// DismissSessionAttention marks the operator-dismissible Needs you sources of one
// session as seen: a turn that ended early, a return from Later, and the failure of
// a stopped session. Marking them seen also observes any unseen settled turn.
// Pending interactions, unverified stops, and live supervision evidence stay with
// their owners. The next occurrence of any source raises it again, and a fence that
// no longer matches leaves a newer occurrence in place.
func (g *SessionRepo) DismissSessionAttention(
	ctx context.Context,
	sessionID string,
	fence store.SessionAttentionFence,
	at time.Time,
) (commit store.SessionAttentionCommit, err error) {
	if err := g.checkReady(ctx, "dismiss session attention"); err != nil {
		return store.SessionAttentionCommit{}, err
	}
	target := strings.TrimSpace(sessionID)
	if target == "" {
		return store.SessionAttentionCommit{}, errors.New("store: session id is required")
	}
	if at.IsZero() {
		at = g.now()
	}
	stamp := store.FormatTimestamp(at.UTC())
	err = g.withImmediateTransaction(ctx, "dismiss session attention", func(exec globalSQLExecutor) error {
		before, readErr := getSessionAttention(ctx, exec, target)
		if readErr != nil {
			return readErr
		}
		affected, updateErr := sqlcgen.New(exec).DismissSessionAttention(ctx, sqlcgen.DismissSessionAttentionParams{
			Stamp:           sql.NullString{String: stamp, Valid: true},
			ID:              target,
			Revision:        fence.AttentionRevision,
			TurnID:          strings.TrimSpace(fence.EndedEarlyTurnID),
			FailureIdentity: sql.NullString{String: fence.FailureIdentity, Valid: true},
		})
		if updateErr != nil {
			return fmt.Errorf("store: dismiss session %q attention: %w", target, updateErr)
		}
		if affected == 0 {
			if before.Fence() != fence {
				return fmt.Errorf("%w: %s", store.ErrSessionAttentionChanged, target)
			}
			commit = store.SessionAttentionCommit{Before: before, After: before, Outcome: "unchanged"}
			return nil
		}
		after, readErr := getSessionAttention(ctx, exec, target)
		if readErr != nil {
			return readErr
		}
		commit = store.SessionAttentionCommit{Before: before, After: after, Outcome: "dismissed"}
		return nil
	})
	return commit, err
}
