package globaldb

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
)

// sessionDismissibleFailureCondition matches a stopped session whose failure still
// raises Needs you; canceled stops are not failures.
const sessionDismissibleFailureCondition = `state = 'stopped'
 AND trim(COALESCE(failure_kind, '')) NOT IN ('', '` + string(store.FailureCanceled) + `')
 AND failure_seen_at IS NULL`

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
		// dynamic-sql: one atomic update clears every dismissible source of the attention projection.
		result, updateErr := exec.ExecContext(ctx, `UPDATE sessions
SET ended_early_turn_id = '',
    later_returned_at = NULL,
    failure_seen_at = CASE WHEN `+sessionDismissibleFailureCondition+` THEN ? ELSE failure_seen_at END,
    last_seen_revision = CASE WHEN last_settled_revision > last_seen_revision
        THEN attention_revision + 1 ELSE last_seen_revision END,
    last_seen_at = CASE WHEN last_settled_revision > last_seen_revision THEN ? ELSE last_seen_at END,
    attention_revision = attention_revision + 1,
    attention_changed_at = ?
WHERE id = ? AND attention_revision = ? AND ended_early_turn_id = ?
  AND (ended_early_turn_id <> '' OR later_returned_at IS NOT NULL OR (`+
			sessionDismissibleFailureCondition+`))`,
			stamp, stamp, stamp, target, fence.AttentionRevision, strings.TrimSpace(fence.EndedEarlyTurnID))
		if updateErr != nil {
			return fmt.Errorf("store: dismiss session %q attention: %w", target, updateErr)
		}
		affected, affectedErr := result.RowsAffected()
		if affectedErr != nil {
			return fmt.Errorf("store: read dismissed session rows: %w", affectedErr)
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
