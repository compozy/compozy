package globaldb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
)

// RunRetentionPolicy describes the terminal-run retention rules used when
// selecting purge candidates.
type RunRetentionPolicy struct {
	KeepTerminalDays int
	KeepMax          int
	Now              time.Time
}

// CountWorkspaces returns the number of registered workspaces.
func (g *GlobalDB) CountWorkspaces(ctx context.Context) (int, error) {
	if err := g.requireContext(ctx, "count workspaces"); err != nil {
		return 0, err
	}

	var count int
	if err := g.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM workspaces`).Scan(&count); err != nil {
		return 0, fmt.Errorf("globaldb: count workspaces: %w", err)
	}
	return count, nil
}

// CountActiveRuns returns the number of non-terminal runs across all workspaces.
func (g *GlobalDB) CountActiveRuns(ctx context.Context) (int, error) {
	if err := g.requireContext(ctx, "count active runs"); err != nil {
		return 0, err
	}

	var count int
	if err := g.db.QueryRowContext(
		ctx,
		`SELECT COUNT(1)
		 FROM runs
		 WHERE status NOT IN ('completed', 'failed', 'canceled', 'crashed')`,
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("globaldb: count active runs: %w", err)
	}
	return count, nil
}

// ListInterruptedRuns returns runs left in non-terminal daemon-owned states
// that must be reconciled on startup.
func (g *GlobalDB) ListInterruptedRuns(ctx context.Context) ([]Run, error) {
	if err := g.requireContext(ctx, "list interrupted runs"); err != nil {
		return nil, err
	}

	rows, err := g.db.QueryContext(
		ctx,
		`SELECT run_id, workspace_id, workflow_id, mode, status, presentation_mode,
		        started_at, ended_at, error_text, request_id
		 FROM runs
		 WHERE status IN ('starting', 'running')
		 ORDER BY started_at ASC, run_id ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("globaldb: query interrupted runs: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	result := make([]Run, 0)
	for rows.Next() {
		run, scanErr := scanRun(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("globaldb: iterate interrupted runs: %w", err)
	}
	return result, nil
}

// MarkRunCrashed mirrors startup reconciliation into the durable run index.
func (g *GlobalDB) MarkRunCrashed(
	ctx context.Context,
	runID string,
	endedAt time.Time,
	errorText string,
) (Run, error) {
	if err := g.requireContext(ctx, "mark run crashed"); err != nil {
		return Run{}, err
	}

	run, err := g.GetRun(ctx, runID)
	if err != nil {
		return Run{}, err
	}

	run.Status = "crashed"
	if endedAt.IsZero() {
		endedAt = g.now()
	}
	endedAt = endedAt.UTC()
	run.EndedAt = &endedAt
	run.ErrorText = strings.TrimSpace(errorText)
	return g.UpdateRun(ctx, run)
}

// RunCrashUpdate captures one durable crash reconciliation update.
type RunCrashUpdate struct {
	RunID     string
	EndedAt   time.Time
	ErrorText string
}

// MarkRunsCrashed updates multiple runs inside one transaction.
func (g *GlobalDB) MarkRunsCrashed(ctx context.Context, updates []RunCrashUpdate) error {
	if err := g.requireContext(ctx, "mark runs crashed"); err != nil {
		return err
	}
	if len(updates) == 0 {
		return nil
	}

	tx, err := g.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("globaldb: begin mark runs crashed: %w", err)
	}

	committed := false
	defer func() {
		if !committed {
			rollbackPendingTx(tx)
		}
	}()

	stmt, err := tx.PrepareContext(
		ctx,
		`UPDATE runs
		 SET status = ?, ended_at = ?, error_text = ?
		 WHERE run_id = ?`,
	)
	if err != nil {
		return fmt.Errorf("globaldb: prepare mark runs crashed: %w", err)
	}
	defer func() {
		_ = stmt.Close()
	}()

	for _, update := range updates {
		runID := strings.TrimSpace(update.RunID)
		if runID == "" {
			return fmt.Errorf("globaldb: run id is required for crash update")
		}
		endedAt := update.EndedAt
		if endedAt.IsZero() {
			endedAt = g.now()
		}
		result, execErr := stmt.ExecContext(
			ctx,
			runStatusCrashed,
			store.FormatTimestamp(endedAt.UTC()),
			strings.TrimSpace(update.ErrorText),
			runID,
		)
		if execErr != nil {
			return fmt.Errorf("globaldb: update run %q crashed: %w", runID, execErr)
		}
		affected, rowsErr := result.RowsAffected()
		if rowsErr != nil {
			return fmt.Errorf("globaldb: rows affected for run %q: %w", runID, rowsErr)
		}
		if affected == 0 {
			return ErrRunNotFound
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("globaldb: commit mark runs crashed: %w", err)
	}
	committed = true
	return nil
}

// ListTerminalRunsForPurge selects terminal runs in oldest-first order while
// respecting the configured keep-count and keep-days bounds.
func (g *GlobalDB) ListTerminalRunsForPurge(
	ctx context.Context,
	policy RunRetentionPolicy,
) ([]Run, error) {
	if err := g.requireContext(ctx, "list terminal runs for purge"); err != nil {
		return nil, err
	}
	if err := validateRunRetentionPolicy(policy); err != nil {
		return nil, err
	}
	terminalRuns, err := g.listTerminalRuns(ctx)
	if err != nil {
		return nil, err
	}

	now := policy.Now
	if now.IsZero() {
		now = g.now()
	}
	cutoff := now.UTC().AddDate(0, 0, -policy.KeepTerminalDays)
	overflow := purgeOverflow(len(terminalRuns), policy.KeepMax)

	result := make([]Run, 0)
	seen := make(map[string]struct{}, len(terminalRuns))
	for idx := range terminalRuns {
		run := &terminalRuns[idx]
		if !shouldPurgeTerminalRun(run, cutoff, idx, overflow) {
			continue
		}
		if _, ok := seen[run.RunID]; ok {
			continue
		}
		seen[run.RunID] = struct{}{}
		result = append(result, *run)
	}
	return result, nil
}

func validateRunRetentionPolicy(policy RunRetentionPolicy) error {
	if policy.KeepTerminalDays < 0 {
		return fmt.Errorf(
			"globaldb: keep terminal days must be zero or greater (got %d)",
			policy.KeepTerminalDays,
		)
	}
	if policy.KeepMax < 0 {
		return fmt.Errorf("globaldb: keep max must be zero or greater (got %d)", policy.KeepMax)
	}
	return nil
}

func (g *GlobalDB) listTerminalRuns(ctx context.Context) ([]Run, error) {
	rows, err := g.db.QueryContext(
		ctx,
		`SELECT run_id, workspace_id, workflow_id, mode, status, presentation_mode,
		        started_at, ended_at, error_text, request_id
		 FROM runs
		 WHERE status IN ('completed', 'failed', 'canceled', 'crashed')
		 ORDER BY COALESCE(ended_at, started_at) ASC, run_id ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("globaldb: query purge candidates: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	terminalRuns := make([]Run, 0)
	for rows.Next() {
		run, scanErr := scanRun(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		terminalRuns = append(terminalRuns, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("globaldb: iterate purge candidates: %w", err)
	}
	return terminalRuns, nil
}

func purgeOverflow(total int, keepMax int) int {
	overflow := total - keepMax
	if overflow < 0 {
		return 0
	}
	return overflow
}

func shouldPurgeTerminalRun(run *Run, cutoff time.Time, index int, overflow int) bool {
	terminalAt := terminalRunAt(run)
	if terminalAt.Before(cutoff) || terminalAt.Equal(cutoff) {
		return true
	}
	return index < overflow
}

func terminalRunAt(run *Run) time.Time {
	if run == nil || run.EndedAt == nil {
		if run == nil {
			return time.Time{}
		}
		return run.StartedAt
	}
	return run.EndedAt.UTC()
}

// DeleteRun removes one durable run index row.
func (g *GlobalDB) DeleteRun(ctx context.Context, runID string) error {
	if err := g.requireContext(ctx, "delete run"); err != nil {
		return err
	}

	result, err := g.db.ExecContext(ctx, `DELETE FROM runs WHERE run_id = ?`, strings.TrimSpace(runID))
	if err != nil {
		return fmt.Errorf("globaldb: delete run %q: %w", strings.TrimSpace(runID), err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("globaldb: rows affected for run %q: %w", strings.TrimSpace(runID), err)
	}
	if affected == 0 {
		return ErrRunNotFound
	}
	return nil
}

// DeleteRuns removes multiple durable run index rows in one transaction.
func (g *GlobalDB) DeleteRuns(ctx context.Context, runIDs []string) error {
	if err := g.requireContext(ctx, "delete runs"); err != nil {
		return err
	}
	if len(runIDs) == 0 {
		return nil
	}

	tx, err := g.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("globaldb: begin delete runs: %w", err)
	}

	committed := false
	defer func() {
		if !committed {
			rollbackPendingTx(tx)
		}
	}()

	stmt, err := tx.PrepareContext(ctx, `DELETE FROM runs WHERE run_id = ?`)
	if err != nil {
		return fmt.Errorf("globaldb: prepare delete runs: %w", err)
	}
	defer func() {
		_ = stmt.Close()
	}()

	for _, runID := range runIDs {
		trimmed := strings.TrimSpace(runID)
		if trimmed == "" {
			continue
		}
		if _, err := stmt.ExecContext(ctx, trimmed); err != nil {
			return fmt.Errorf("globaldb: delete run %q: %w", trimmed, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("globaldb: commit delete runs: %w", err)
	}
	committed = true
	return nil
}

// WorkflowSlugsByIDs returns workflow slugs for the supplied workflow ids.
func (g *GlobalDB) WorkflowSlugsByIDs(ctx context.Context, ids []string) (map[string]string, error) {
	if err := g.requireContext(ctx, "list workflow slugs"); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return map[string]string{}, nil
	}

	seen := make(map[string]struct{}, len(ids))
	args := make([]any, 0, len(ids))
	placeholders := make([]string, 0, len(ids))
	for _, id := range ids {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		args = append(args, trimmed)
		placeholders = append(placeholders, "?")
	}
	if len(args) == 0 {
		return map[string]string{}, nil
	}

	var query strings.Builder
	query.Grow(len(`SELECT id, slug FROM workflows WHERE id IN ()`) + len(placeholders)*3)
	query.WriteString(`SELECT id, slug FROM workflows WHERE id IN (`)
	query.WriteString(strings.Join(placeholders, ", "))
	query.WriteByte(')')
	rows, err := g.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("globaldb: query workflow slugs: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	result := make(map[string]string, len(args))
	for rows.Next() {
		var (
			id   string
			slug string
		)
		if err := rows.Scan(&id, &slug); err != nil {
			return nil, fmt.Errorf("globaldb: scan workflow slug row: %w", err)
		}
		result[strings.TrimSpace(id)] = strings.TrimSpace(slug)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("globaldb: iterate workflow slugs: %w", err)
	}
	return result, nil
}

func rollbackPendingTx(tx interface{ Rollback() error }) {
	if tx == nil {
		return
	}
	if err := tx.Rollback(); err != nil {
		return
	}
}
