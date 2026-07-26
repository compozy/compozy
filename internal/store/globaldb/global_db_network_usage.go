package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/store"
)

var _ store.NetworkUsageStore = (*NetworkRepo)(nil)

// GetNetworkUsage returns one bounded workspace-fenced page plus full-filter totals.
func (g *NetworkRepo) GetNetworkUsage(
	ctx context.Context,
	query store.NetworkUsageQuery,
) (store.NetworkUsageReport, error) {
	if err := g.checkReady(ctx, "get network usage"); err != nil {
		return store.NetworkUsageReport{}, err
	}
	normalized, cursor, err := store.NormalizeNetworkUsageQuery(query)
	if err != nil {
		return store.NetworkUsageReport{}, fmt.Errorf("store: validate network usage query: %w", err)
	}
	details, nextCursor, err := g.loadNetworkUsageDetails(ctx, normalized, cursor)
	if err != nil {
		return store.NetworkUsageReport{}, err
	}
	total, err := g.loadNetworkUsageSummary(ctx, normalized)
	if err != nil {
		return store.NetworkUsageReport{}, err
	}
	budget, err := g.loadNetworkUsageBudget(ctx, normalized)
	if err != nil {
		return store.NetworkUsageReport{}, err
	}
	return store.NetworkUsageReport{
		Details:    details,
		Total:      total,
		Budget:     budget,
		NextCursor: nextCursor,
	}, nil
}

func (g *NetworkRepo) loadNetworkUsageDetails(
	ctx context.Context,
	query store.NetworkUsageQuery,
	cursor *store.NetworkUsageCursorPosition,
) (details []store.NetworkWakeUsageDetail, nextCursor string, err error) {
	const selectUsage = `SELECT wake_id, task_run_id, owner_key, workspace_id, channel,
	root_id, depth, state, reserved_wall_ms, actual_wall_ms, reserved_at,
	settled_at, input_tokens, output_tokens, usage_state, reason
FROM network_live_wakes
WHERE workspace_id = ?
	AND (? = '' OR channel = ?)
	AND (? = '' OR owner_key = ?)
	AND (? = '' OR owner_key IN (?, ?) OR task_run_id = ?)
	AND (? = '' OR reserved_at < ? OR (reserved_at = ? AND wake_id < ?))
ORDER BY reserved_at DESC, wake_id DESC
LIMIT ?`
	ownerKey := networkUsageOwnerKey(query)
	cursorAt := ""
	cursorWakeID := ""
	if cursor != nil {
		cursorAt = store.FormatTimestamp(cursor.ReservedAt)
		cursorWakeID = cursor.WakeID
	}
	rows, err := g.db.QueryContext(
		ctx,
		selectUsage,
		query.WorkspaceID,
		query.Channel,
		query.Channel,
		ownerKey,
		ownerKey,
		query.RunID,
		"task_run:"+query.RunID,
		"loop_run:"+query.RunID,
		query.RunID,
		cursorAt,
		cursorAt,
		cursorAt,
		cursorWakeID,
		query.Limit+1,
	)
	if err != nil {
		return nil, "", fmt.Errorf("store: query network usage: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			closeErr = fmt.Errorf("store: close network usage rows: %w", closeErr)
			if err != nil {
				err = errors.Join(err, closeErr)
				return
			}
			err = closeErr
		}
	}()

	details = make([]store.NetworkWakeUsageDetail, 0, query.Limit)
	for rows.Next() {
		detail, scanErr := scanNetworkWakeUsage(rows)
		if scanErr != nil {
			return nil, "", scanErr
		}
		details = append(details, detail)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, "", fmt.Errorf("store: iterate network usage: %w", rowsErr)
	}
	if len(details) <= query.Limit {
		return details, "", nil
	}
	details = details[:query.Limit]
	last := details[len(details)-1]
	nextCursor, err = store.EncodeNetworkUsageCursor(query, store.NetworkUsageCursorPosition{
		ReservedAt: last.ReservedAt,
		WakeID:     last.WakeID,
	})
	if err != nil {
		return nil, "", fmt.Errorf("store: encode next network usage cursor: %w", err)
	}
	return details, nextCursor, nil
}

func (g *NetworkRepo) loadNetworkUsageSummary(
	ctx context.Context,
	query store.NetworkUsageQuery,
) (store.NetworkUsageSummary, error) {
	const selectSummary = `SELECT
	COUNT(*),
	COALESCE(SUM(CASE WHEN usage_state NOT IN (?, ?) THEN 1 ELSE 0 END), 0),
	COALESCE(SUM(CASE WHEN usage_state = ? THEN 1 ELSE 0 END), 0),
	COALESCE(SUM(CASE WHEN usage_state = ? THEN 1 ELSE 0 END), 0),
	COALESCE(SUM(COALESCE(actual_wall_ms, reserved_wall_ms)), 0),
	COALESCE(SUM(CASE WHEN usage_state = ? THEN 0 ELSE COALESCE(input_tokens, 0) END), 0),
	COALESCE(SUM(CASE WHEN usage_state = ? THEN 0 ELSE COALESCE(output_tokens, 0) END), 0)
FROM network_live_wakes
WHERE workspace_id = ?
	AND (? = '' OR channel = ?)
	AND (? = '' OR owner_key = ?)
	AND (? = '' OR owner_key IN (?, ?) OR task_run_id = ?)`
	ownerKey := networkUsageOwnerKey(query)
	var total store.NetworkUsageSummary
	var chargedWallMS int64
	err := g.db.QueryRowContext(
		ctx,
		selectSummary,
		store.NetworkWakeUsageActual,
		store.NetworkWakeUsageUnavailable,
		store.NetworkWakeUsageActual,
		store.NetworkWakeUsageUnavailable,
		store.NetworkWakeUsageUnavailable,
		store.NetworkWakeUsageUnavailable,
		query.WorkspaceID,
		query.Channel,
		query.Channel,
		ownerKey,
		ownerKey,
		query.RunID,
		"task_run:"+query.RunID,
		"loop_run:"+query.RunID,
		query.RunID,
	).Scan(
		&total.WakeCount,
		&total.ReservedWakeCount,
		&total.ActualWakeCount,
		&total.UnavailableWakeCount,
		&chargedWallMS,
		&total.InputTokens,
		&total.OutputTokens,
	)
	if err != nil {
		return store.NetworkUsageSummary{}, fmt.Errorf("store: query network usage summary: %w", err)
	}
	total.ChargedWallTime = time.Duration(chargedWallMS) * time.Millisecond
	return total, nil
}

func (g *NetworkRepo) loadNetworkUsageBudget(
	ctx context.Context,
	query store.NetworkUsageQuery,
) (*store.NetworkBudgetUsage, error) {
	ownerKey := networkUsageOwnerKey(query)
	if ownerKey == "" && query.RunID == "" {
		return nil, nil
	}
	ownerKeys := []string{ownerKey}
	if ownerKey == "" {
		ownerKeys = []string{"task_run:" + query.RunID, "loop_run:" + query.RunID}
	}
	const selectBudget = `SELECT owner_key, wakes_used, wall_ms_used, input_tokens_used,
	output_tokens_used, exhausted_reason, updated_at
FROM network_participation_budgets
WHERE workspace_id = ? AND owner_key IN (?, ?)
ORDER BY CASE WHEN owner_key = ? THEN 0 ELSE 1 END
LIMIT 1`
	primary := ownerKeys[0]
	secondary := primary
	if len(ownerKeys) > 1 {
		secondary = ownerKeys[1]
	}
	var budget store.NetworkBudgetUsage
	var wallMS int64
	var updatedAtRaw string
	err := g.db.QueryRowContext(
		ctx,
		selectBudget,
		query.WorkspaceID,
		primary,
		secondary,
		primary,
	).Scan(
		&budget.OwnerKey,
		&budget.WakesUsed,
		&wallMS,
		&budget.InputTokensUsed,
		&budget.OutputTokensUsed,
		&budget.ExhaustedReason,
		&updatedAtRaw,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: query network usage budget: %w", err)
	}
	updatedAt, err := store.ParseTimestamp(updatedAtRaw)
	if err != nil {
		return nil, fmt.Errorf("store: parse network usage budget updated_at: %w", err)
	}
	budget.WallTimeUsed = time.Duration(wallMS) * time.Millisecond
	budget.UpdatedAt = updatedAt
	return &budget, nil
}

func networkUsageOwnerKey(query store.NetworkUsageQuery) string {
	if query.Owner == nil {
		return ""
	}
	return participation.OwnerKey(*query.Owner)
}

func scanNetworkWakeUsage(rows *sql.Rows) (store.NetworkWakeUsageDetail, error) {
	var detail store.NetworkWakeUsageDetail
	var reservedWallMS int64
	var actualWallMS sql.NullInt64
	var reservedAtRaw string
	var settledAtRaw sql.NullString
	var inputTokens sql.NullInt64
	var outputTokens sql.NullInt64
	if err := rows.Scan(
		&detail.WakeID,
		&detail.TaskRunID,
		&detail.OwnerKey,
		&detail.WorkspaceID,
		&detail.Channel,
		&detail.RootID,
		&detail.Depth,
		&detail.State,
		&reservedWallMS,
		&actualWallMS,
		&reservedAtRaw,
		&settledAtRaw,
		&inputTokens,
		&outputTokens,
		&detail.UsageState,
		&detail.Reason,
	); err != nil {
		return store.NetworkWakeUsageDetail{}, fmt.Errorf("store: scan network usage: %w", err)
	}
	reservedAt, err := store.ParseTimestamp(reservedAtRaw)
	if err != nil {
		return store.NetworkWakeUsageDetail{}, fmt.Errorf("store: parse network usage reserved_at: %w", err)
	}
	detail.ReservedAt = reservedAt
	if settledAtRaw.Valid {
		settledAt, parseErr := store.ParseTimestamp(settledAtRaw.String)
		if parseErr != nil {
			return store.NetworkWakeUsageDetail{}, fmt.Errorf("store: parse network usage settled_at: %w", parseErr)
		}
		detail.SettledAt = &settledAt
	}
	chargedWallMS := reservedWallMS
	if actualWallMS.Valid {
		chargedWallMS = actualWallMS.Int64
	}
	detail.ChargedWallTime = time.Duration(chargedWallMS) * time.Millisecond
	detail.InputTokens = inputTokens.Int64
	detail.OutputTokens = outputTokens.Int64
	if strings.TrimSpace(detail.UsageState) == "" {
		detail.UsageState = store.NetworkWakeUsageReserved
	}
	if detail.UsageState == store.NetworkWakeUsageUnavailable {
		detail.InputTokens = 0
		detail.OutputTokens = 0
	}
	return detail, nil
}
