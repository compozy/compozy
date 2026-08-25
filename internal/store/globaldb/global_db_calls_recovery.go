package globaldb

import (
	"context"
	"fmt"
	"strings"
	"time"

	callspkg "github.com/compozy/compozy/internal/calls"
	"github.com/compozy/compozy/internal/store"
)

func (g *CallRepo) ListDueCalls(
	ctx context.Context,
	now time.Time,
	limit int,
) ([]callspkg.CallRecord, error) {
	if err := g.checkReady(ctx, "list due calls"); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	rows, err := g.db.QueryContext(ctx, `SELECT `+callSelectColumnsSQL+` FROM calls
		WHERE deadline_at IS NOT NULL AND deadline_at <= ? AND state IN ('queued', 'running')
		ORDER BY deadline_at ASC, call_id ASC LIMIT ?`, store.FormatTimestamp(now), limit)
	if err != nil {
		return nil, fmt.Errorf("store: list due calls: %w", err)
	}
	defer func() { err = joinRowsCloseError(rows, err, "due calls") }()
	records := make([]callspkg.CallRecord, 0)
	for rows.Next() {
		record, scanErr := scanCallRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate due calls: %w", err)
	}
	return records, nil
}

func (g *CallRepo) FenceSessionDrain(ctx context.Context, rootSessionID string, at time.Time) error {
	if err := g.checkReady(ctx, "fence session drain"); err != nil {
		return err
	}
	result, err := g.db.ExecContext(ctx, `UPDATE sessions SET draining_at = COALESCE(draining_at, ?),
		updated_at = ? WHERE id = ?`, store.FormatTimestamp(at), store.FormatTimestamp(at),
		strings.TrimSpace(rootSessionID))
	if err != nil {
		return fmt.Errorf("store: fence session drain %q: %w", rootSessionID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: inspect session drain fence %q: %w", rootSessionID, err)
	}
	if affected != 1 {
		return fmt.Errorf("store: session %q was not found", rootSessionID)
	}
	return nil
}

func (g *CallRepo) ListOpenSubtreeCalls(
	ctx context.Context,
	rootSessionID string,
) (records []callspkg.CallRecord, err error) {
	if err := g.checkReady(ctx, "list open subtree calls"); err != nil {
		return nil, err
	}
	rows, err := g.db.QueryContext(ctx, `WITH RECURSIVE descendants(id) AS (
		SELECT id FROM sessions WHERE id = ?
		UNION
		SELECT child.id FROM sessions child JOIN descendants parent ON child.parent_session_id = parent.id
	) SELECT `+callSelectColumnsSQL+` FROM calls
	WHERE state IN ('queued', 'running') AND (
		parent_session_id IN (SELECT id FROM descendants) OR child_session_id IN (SELECT id FROM descendants)
	) ORDER BY depth DESC, created_at DESC, call_id DESC`, strings.TrimSpace(rootSessionID))
	if err != nil {
		return nil, fmt.Errorf("store: list open subtree calls: %w", err)
	}
	defer func() { err = joinRowsCloseError(rows, err, "open subtree calls") }()
	records = make([]callspkg.CallRecord, 0)
	for rows.Next() {
		record, scanErr := scanCallRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate open subtree calls: %w", err)
	}
	return records, nil
}

func (g *CallRepo) CountPreservedSubtreeResults(ctx context.Context, rootSessionID string) (int, error) {
	if err := g.checkReady(ctx, "count preserved subtree results"); err != nil {
		return 0, err
	}
	var count int
	err := g.db.QueryRowContext(ctx, `WITH RECURSIVE descendants(id) AS (
		SELECT id FROM sessions WHERE id = ?
		UNION
		SELECT child.id FROM sessions child JOIN descendants parent ON child.parent_session_id = parent.id
	) SELECT COUNT(*) FROM calls
	WHERE state = 'completed' AND result_ref IS NOT NULL AND (
		parent_session_id IN (SELECT id FROM descendants) OR child_session_id IN (SELECT id FROM descendants)
	)`, strings.TrimSpace(rootSessionID)).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("store: count preserved subtree results: %w", err)
	}
	return count, nil
}
