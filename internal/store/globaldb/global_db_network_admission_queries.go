package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/agh/internal/store"
)

func findNetworkWakeSource(
	ctx context.Context,
	exec networkSQLExecutor,
	workspaceID string,
	ownerKey string,
	envelopeID string,
) (string, bool, error) {
	// dynamic-sql: the composite primary key is the permanent per-owner envelope dedup key.
	const query = `
SELECT wake_id
FROM network_wake_sources
WHERE workspace_id = ? AND owner_key = ? AND envelope_id = ?`
	var wakeID string
	if err := exec.QueryRowContext(ctx, query, workspaceID, ownerKey, envelopeID).Scan(&wakeID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("store: lookup network wake source: %w", err)
	}
	return wakeID, true, nil
}

func findOpenNetworkWake(
	ctx context.Context,
	exec networkSQLExecutor,
	workspaceID string,
	ownerKey string,
) (openNetworkWake, bool, error) {
	// dynamic-sql: accounting joins canonical task-run state solely for the claim-vs-coalesce fence.
	const query = `
SELECT w.wake_id, w.task_run_id, w.root_id, w.reserved_wall_ms, w.coalesce_until, tr.status
FROM network_live_wakes AS w
JOIN task_runs AS tr ON tr.id = w.task_run_id
WHERE w.workspace_id = ? AND w.owner_key = ? AND w.state = 'open'`
	var row openNetworkWake
	var reservedWallMS int64
	var coalesceUntilRaw string
	if err := exec.QueryRowContext(ctx, query, workspaceID, ownerKey).Scan(
		&row.reservation.WakeID,
		&row.reservation.TaskRunID,
		&row.rootID,
		&reservedWallMS,
		&coalesceUntilRaw,
		&row.runStatus,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return openNetworkWake{}, false, nil
		}
		return openNetworkWake{}, false, fmt.Errorf("store: lookup open network wake: %w", err)
	}
	coalesceUntil, err := store.ParseTimestamp(coalesceUntilRaw)
	if err != nil {
		return openNetworkWake{}, false, fmt.Errorf("store: parse network wake coalesce cutoff: %w", err)
	}
	row.reservation.OwnerKey = ownerKey
	row.reservation.WorkspaceID = workspaceID
	row.reservation.ReservedWallTime = (time.Duration(reservedWallMS) * time.Millisecond).String()
	row.reservation.CoalesceUntil = coalesceUntil
	return row, true, nil
}

func getNetworkWakeBudget(
	ctx context.Context,
	exec networkSQLExecutor,
	workspaceID string,
	ownerKey string,
) (networkWakeBudget, error) {
	// dynamic-sql: owner budgets are serialized by the enclosing immediate transaction.
	const query = `
SELECT wakes_used, wall_ms_used, input_tokens_used, output_tokens_used
FROM network_participation_budgets
WHERE workspace_id = ? AND owner_key = ?`
	var budget networkWakeBudget
	if err := exec.QueryRowContext(ctx, query, workspaceID, ownerKey).Scan(
		&budget.wakesUsed,
		&budget.wallMSUsed,
		&budget.inputTokensUsed,
		&budget.outputTokensUsed,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return networkWakeBudget{}, nil
		}
		return networkWakeBudget{}, fmt.Errorf("store: get network participation budget: %w", err)
	}
	return budget, nil
}

func insertNetworkWakeLedger(
	ctx context.Context,
	exec networkSQLExecutor,
	message store.NetworkConversationMessage,
	input store.NetworkWakeAdmissionInput,
	reservation store.WakeReservation,
	reservedWallMS int64,
	reservedInput int64,
	reservedOutput int64,
	reservedAt time.Time,
) error {
	// dynamic-sql: the ledger is accounting evidence, never claim authority.
	const query = `
INSERT INTO network_live_wakes (
  wake_id, task_run_id, owner_key, workspace_id, channel, root_id, depth,
  state, coalesce_until, reserved_wall_ms, reserved_at,
  input_tokens, output_tokens, usage_state
) VALUES (?, ?, ?, ?, ?, ?, ?, 'open', ?, ?, ?, ?, ?, '')`
	if _, err := exec.ExecContext(
		ctx,
		query,
		reservation.WakeID,
		reservation.TaskRunID,
		input.OwnerKey,
		message.WorkspaceID,
		message.Channel,
		input.RootID,
		input.Depth,
		store.FormatTimestamp(reservation.CoalesceUntil),
		reservedWallMS,
		store.FormatTimestamp(reservedAt),
		reservedInput,
		reservedOutput,
	); err != nil {
		return fmt.Errorf("store: insert network wake ledger: %w", err)
	}
	return nil
}

func insertNetworkWakeSource(
	ctx context.Context,
	exec networkSQLExecutor,
	workspaceID string,
	ownerKey string,
	envelopeID string,
	wakeID string,
) error {
	// dynamic-sql: source identity is immutable after admission or coalescing.
	const query = `
INSERT INTO network_wake_sources (workspace_id, owner_key, envelope_id, wake_id)
VALUES (?, ?, ?, ?)`
	if _, err := exec.ExecContext(ctx, query, workspaceID, ownerKey, envelopeID, wakeID); err != nil {
		return fmt.Errorf("store: insert network wake source: %w", err)
	}
	return nil
}

func reserveNetworkWakeBudget(
	ctx context.Context,
	exec networkSQLExecutor,
	workspaceID string,
	ownerKey string,
	wallMS int64,
	inputTokens int64,
	outputTokens int64,
	now time.Time,
) error {
	// dynamic-sql: reservation and ledger insert share the same immediate transaction.
	const query = `
INSERT INTO network_participation_budgets (
  workspace_id, owner_key, wakes_used, wall_ms_used, input_tokens_used, output_tokens_used,
  exhausted_reason, updated_at
) VALUES (?, ?, 1, ?, ?, ?, '', ?)
ON CONFLICT(workspace_id, owner_key) DO UPDATE SET
  wakes_used = wakes_used + 1,
  wall_ms_used = wall_ms_used + excluded.wall_ms_used,
  input_tokens_used = input_tokens_used + excluded.input_tokens_used,
  output_tokens_used = output_tokens_used + excluded.output_tokens_used,
  exhausted_reason = '',
  updated_at = excluded.updated_at`
	if _, err := exec.ExecContext(
		ctx,
		query,
		workspaceID,
		ownerKey,
		wallMS,
		inputTokens,
		outputTokens,
		store.FormatTimestamp(now),
	); err != nil {
		return fmt.Errorf("store: reserve network participation budget: %w", err)
	}
	return nil
}

func writeNetworkWakeBudgetExhaustion(
	ctx context.Context,
	exec networkSQLExecutor,
	workspaceID string,
	ownerKey string,
	budget networkWakeBudget,
	reason string,
	now time.Time,
) error {
	// dynamic-sql: exhausted state is visible even when no prior budget row exists.
	const query = `
INSERT INTO network_participation_budgets (
  workspace_id, owner_key, wakes_used, wall_ms_used, input_tokens_used, output_tokens_used,
  exhausted_reason, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(workspace_id, owner_key) DO UPDATE SET
  exhausted_reason = excluded.exhausted_reason,
  updated_at = excluded.updated_at`
	if _, err := exec.ExecContext(
		ctx,
		query,
		workspaceID,
		ownerKey,
		budget.wakesUsed,
		budget.wallMSUsed,
		budget.inputTokensUsed,
		budget.outputTokensUsed,
		reason,
		store.FormatTimestamp(now),
	); err != nil {
		return fmt.Errorf("store: record network participation exhaustion: %w", err)
	}
	return nil
}
