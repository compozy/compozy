package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

// dynamic-sql: receipt and queue CAS operations span generated query owners and must commit in one transaction.
const commitQueuedPromptAdmissionSQL = `
UPDATE session_prompt_admissions
SET state = ?, dispatch_committed_at = ?, completed_at = NULL, updated_at = ?
WHERE id = ? AND session_id = ? AND state = ?
`

// dynamic-sql: see commitQueuedPromptAdmissionSQL.
const completeQueuedPromptAdmissionSQL = `
UPDATE session_prompt_admissions
SET state = ?, completed_at = ?, indeterminate_reason = '', updated_at = ?
WHERE id = ? AND session_id = ? AND state = ?
`

// dynamic-sql: see commitQueuedPromptAdmissionSQL.
const indeterminateQueuedPromptAdmissionSQL = `
UPDATE session_prompt_admissions
SET state = ?, indeterminate_reason = ?, completed_at = NULL, updated_at = ?
WHERE id = ? AND session_id = ? AND state = ?
`

func commitQueuedPromptAdmissionDispatch(
	ctx context.Context,
	exec globalSQLExecutor,
	entry *store.SessionInputQueueEntry,
	nowRaw string,
) error {
	if entry.PromptAdmissionID == "" {
		return nil
	}
	affected, err := executePromptAdmissionTransition(
		ctx,
		exec,
		commitQueuedPromptAdmissionSQL,
		store.SessionPromptAdmissionDispatchCommitted,
		nowRaw,
		nowRaw,
		entry.PromptAdmissionID,
		entry.SessionID,
		store.SessionPromptAdmissionCompleted,
	)
	if err != nil {
		return fmt.Errorf("store: commit queued prompt admission dispatch: %w", err)
	}
	if affected != 1 {
		return queuedPromptAdmissionCASFailure(ctx, exec, entry)
	}
	return nil
}

func completeQueuedPromptAdmissionDispatch(
	ctx context.Context,
	exec globalSQLExecutor,
	entry *store.SessionInputQueueEntry,
	nowRaw string,
) error {
	if entry.PromptAdmissionID == "" {
		return nil
	}
	affected, err := executePromptAdmissionTransition(
		ctx,
		exec,
		completeQueuedPromptAdmissionSQL,
		store.SessionPromptAdmissionCompleted,
		nowRaw,
		nowRaw,
		entry.PromptAdmissionID,
		entry.SessionID,
		store.SessionPromptAdmissionDispatchCommitted,
	)
	if err != nil {
		return fmt.Errorf("store: complete queued prompt admission dispatch: %w", err)
	}
	if affected != 1 {
		return queuedPromptAdmissionCASFailure(ctx, exec, entry)
	}
	return nil
}

func markQueuedPromptAdmissionIndeterminate(
	ctx context.Context,
	exec globalSQLExecutor,
	entry *store.SessionInputQueueEntry,
	summary string,
	nowRaw string,
) error {
	if entry.PromptAdmissionID == "" {
		return nil
	}
	summary = strings.TrimSpace(summary)
	if summary == "" {
		summary = "dispatch outcome is unknown"
	}
	affected, err := executePromptAdmissionTransition(
		ctx,
		exec,
		indeterminateQueuedPromptAdmissionSQL,
		store.SessionPromptAdmissionIndeterminate,
		summary,
		nowRaw,
		entry.PromptAdmissionID,
		entry.SessionID,
		store.SessionPromptAdmissionDispatchCommitted,
	)
	if err != nil {
		return fmt.Errorf("store: mark queued prompt admission indeterminate: %w", err)
	}
	if affected != 1 {
		return queuedPromptAdmissionCASFailure(ctx, exec, entry)
	}
	return nil
}

func executePromptAdmissionTransition(
	ctx context.Context,
	exec globalSQLExecutor,
	query string,
	args ...any,
) (int64, error) {
	result, err := exec.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read prompt admission transition rows affected: %w", err)
	}
	return affected, nil
}

func queuedPromptAdmissionCASFailure(
	ctx context.Context,
	exec globalSQLExecutor,
	entry *store.SessionInputQueueEntry,
) error {
	var state string
	err := exec.QueryRowContext(
		ctx,
		`SELECT state FROM session_prompt_admissions WHERE id = ? AND session_id = ?`,
		entry.PromptAdmissionID,
		entry.SessionID,
	).Scan(&state)
	if errors.Is(err, sql.ErrNoRows) {
		return store.ErrSessionPromptAdmissionInProgress
	}
	if err != nil {
		return fmt.Errorf("store: read queued prompt admission after compare-and-swap failure: %w", err)
	}
	switch state {
	case store.SessionPromptAdmissionDispatchCommitted, store.SessionPromptAdmissionIndeterminate:
		return store.ErrSessionPromptDispatchIndeterminate
	default:
		return store.ErrSessionPromptAdmissionInProgress
	}
}
