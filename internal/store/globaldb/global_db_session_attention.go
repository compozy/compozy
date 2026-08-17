package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
)

// GetSessionAttention returns the catalog-owned attention fence for one session.
func (g *SessionRepo) GetSessionAttention(
	ctx context.Context,
	sessionID string,
) (store.SessionAttention, error) {
	if err := g.checkReady(ctx, "get session attention"); err != nil {
		return store.SessionAttention{}, err
	}
	target := strings.TrimSpace(sessionID)
	if target == "" {
		return store.SessionAttention{}, errors.New("store: session id is required")
	}
	return getSessionAttention(ctx, g.db, target)
}

// MarkSessionSettled advances the catalog-owned settled fence.
func (g *SessionRepo) MarkSessionSettled(
	ctx context.Context,
	sessionID string,
	seen bool,
	at time.Time,
) (commit store.SessionAttentionCommit, err error) {
	if err := g.checkReady(ctx, "mark session settled"); err != nil {
		return store.SessionAttentionCommit{}, err
	}
	target := strings.TrimSpace(sessionID)
	if target == "" {
		return store.SessionAttentionCommit{}, errors.New("store: session id is required")
	}
	if at.IsZero() {
		at = g.now()
	}
	at = at.UTC()
	err = g.withImmediateTransaction(ctx, "mark session settled", func(exec globalSQLExecutor) error {
		before, readErr := getSessionAttention(ctx, exec, target)
		if readErr != nil {
			return readErr
		}
		nextRevision := before.AttentionRevision + 1
		query := `UPDATE sessions
SET attention_revision = ?, last_settled_revision = ?, attention_changed_at = ?,
    last_seen_revision = CASE WHEN ? THEN ? ELSE last_seen_revision END,
    last_seen_at = CASE WHEN ? THEN ? ELSE last_seen_at END
WHERE id = ?`
		// dynamic-sql: the seen branch is one expression inside the same canonical transaction.
		result, updateErr := exec.ExecContext(
			ctx,
			query,
			nextRevision,
			nextRevision,
			store.FormatTimestamp(at),
			seen,
			nextRevision,
			seen,
			store.FormatTimestamp(at),
			target,
		)
		if updateErr != nil {
			return fmt.Errorf("store: update settled attention for %q: %w", target, updateErr)
		}
		if affectedErr := requireOneAffected(result, target); affectedErr != nil {
			return affectedErr
		}
		after, readErr := getSessionAttention(ctx, exec, target)
		if readErr != nil {
			return readErr
		}
		commit = store.SessionAttentionCommit{Before: before, After: after, Outcome: "settled"}
		return nil
	})
	return commit, err
}

// MarkSessionSeen advances the seen fence only when unseen settled work exists.
func (g *SessionRepo) MarkSessionSeen(
	ctx context.Context,
	sessionID string,
	at time.Time,
) (commit store.SessionAttentionCommit, err error) {
	if err := g.checkReady(ctx, "mark session seen"); err != nil {
		return store.SessionAttentionCommit{}, err
	}
	target := strings.TrimSpace(sessionID)
	if target == "" {
		return store.SessionAttentionCommit{}, errors.New("store: session id is required")
	}
	if at.IsZero() {
		at = g.now()
	}
	at = at.UTC()
	err = g.withImmediateTransaction(ctx, "mark session seen", func(exec globalSQLExecutor) error {
		before, readErr := getSessionAttention(ctx, exec, target)
		if readErr != nil {
			return readErr
		}
		if !before.Unseen() {
			commit = store.SessionAttentionCommit{Before: before, After: before, Outcome: "already-seen"}
			return nil
		}
		nextRevision := before.AttentionRevision + 1
		// dynamic-sql: attention fences are catalog-owned and updated atomically.
		result, updateErr := exec.ExecContext(ctx, `UPDATE sessions
SET attention_revision = ?, last_seen_revision = ?, last_seen_at = ?, attention_changed_at = ?
WHERE id = ?`, nextRevision, nextRevision, store.FormatTimestamp(at), store.FormatTimestamp(at), target)
		if updateErr != nil {
			return fmt.Errorf("store: mark session %q seen: %w", target, updateErr)
		}
		if affectedErr := requireOneAffected(result, target); affectedErr != nil {
			return affectedErr
		}
		after, readErr := getSessionAttention(ctx, exec, target)
		if readErr != nil {
			return readErr
		}
		commit = store.SessionAttentionCommit{Before: before, After: after, Outcome: "seen"}
		return nil
	})
	return commit, err
}

func getSessionAttention(
	ctx context.Context,
	exec globalSQLExecutor,
	sessionID string,
) (store.SessionAttention, error) {
	var row sessionAttentionRow
	err := exec.QueryRowContext(ctx, `SELECT pending_permission_count, pending_clarify_count,
attention_revision, last_settled_revision, last_seen_revision, last_seen_at, attention_changed_at
FROM sessions WHERE id = ?`, sessionID).Scan(
		&row.pendingPermissionCount,
		&row.pendingClarifyCount,
		&row.attentionRevision,
		&row.lastSettledRevision,
		&row.lastSeenRevision,
		&row.lastSeenAt,
		&row.attentionChangedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return store.SessionAttention{}, fmt.Errorf("%w: %s", store.ErrSessionNotFound, sessionID)
	}
	if err != nil {
		return store.SessionAttention{}, fmt.Errorf("store: read session attention %q: %w", sessionID, err)
	}
	return row.toStore()
}

type sessionAttentionRow struct {
	pendingPermissionCount int
	pendingClarifyCount    int
	attentionRevision      int64
	lastSettledRevision    int64
	lastSeenRevision       int64
	lastSeenAt             sql.NullString
	attentionChangedAt     sql.NullString
}

func (r sessionAttentionRow) toStore() (store.SessionAttention, error) {
	result := store.SessionAttention{
		PendingPermissionCount: r.pendingPermissionCount,
		PendingClarifyCount:    r.pendingClarifyCount,
		AttentionRevision:      r.attentionRevision,
		LastSettledRevision:    r.lastSettledRevision,
		LastSeenRevision:       r.lastSeenRevision,
	}
	var err error
	result.LastSeenAt, err = parseNullableAttentionTime(r.lastSeenAt, "last seen")
	if err != nil {
		return store.SessionAttention{}, err
	}
	result.AttentionChangedAt, err = parseNullableAttentionTime(r.attentionChangedAt, "attention changed")
	if err != nil {
		return store.SessionAttention{}, err
	}
	return result, nil
}

func parseNullableAttentionTime(value sql.NullString, label string) (*time.Time, error) {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return nil, nil
	}
	parsed, err := store.ParseTimestamp(value.String)
	if err != nil {
		return nil, fmt.Errorf("store: parse session %s at: %w", label, err)
	}
	return &parsed, nil
}

func requireOneAffected(result sql.Result, sessionID string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: read affected session rows: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("%w: %s", store.ErrSessionNotFound, sessionID)
	}
	return nil
}

var _ store.SessionAttentionStore = (*SessionRepo)(nil)
