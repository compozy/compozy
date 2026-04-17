package globaldb

import (
	"context"
	"fmt"
	"strings"
	"time"
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
		 WHERE LOWER(TRIM(status)) NOT IN ('completed', 'failed', 'cancelled', 'canceled', 'crashed')`,
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
		 WHERE LOWER(TRIM(status)) IN ('starting', 'running')
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
		 WHERE LOWER(TRIM(status)) IN ('completed', 'failed', 'cancelled', 'canceled', 'crashed')
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
