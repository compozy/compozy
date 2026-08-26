package globaldb

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

// DeleteSession removes one durable session catalog row.
func (g *SessionRepo) DeleteSession(ctx context.Context, sessionID string) error {
	if err := g.checkReady(ctx, "delete session"); err != nil {
		return err
	}
	target := strings.TrimSpace(sessionID)
	if target == "" {
		return fmt.Errorf("store: session id is required")
	}
	return g.tasks.withTaskImmediateTransaction(ctx, "delete session", func(exec taskSQLExecutor) error {
		at := store.FormatTimestamp(g.now().UTC())
		if _, err := exec.ExecContext(ctx, `UPDATE calls SET state = 'canceled',
			failure_code = 'call_session_deleted', failure_detail = 'session deleted',
			settled_at = ?, updated_at = ?
			WHERE state IN ('queued','running') AND (parent_session_id = ? OR child_session_id = ?)`,
			at, at, target, target,
		); err != nil {
			return fmt.Errorf("store: settle calls for deleted session %q: %w", target, err)
		}
		result, err := exec.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, target)
		if err != nil {
			return fmt.Errorf("store: delete session %q: %w", target, err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("store: inspect deleted session %q: %w", target, err)
		}
		if affected == 0 {
			return fmt.Errorf("store: delete session %q: %w", target, store.ErrSessionNotFound)
		}
		return nil
	})
}
