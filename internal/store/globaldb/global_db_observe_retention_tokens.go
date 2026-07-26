package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

func (g *ObserveRepo) SweepObservability(
	ctx context.Context,
	cutoff time.Time,
) (result store.ObservabilityRetentionSweepResult, err error) {
	if err := g.checkReady(ctx, "sweep observability"); err != nil {
		return store.ObservabilityRetentionSweepResult{}, err
	}
	if cutoff.IsZero() {
		return store.ObservabilityRetentionSweepResult{}, errors.New(
			"store: observability retention cutoff is required",
		)
	}

	result = store.ObservabilityRetentionSweepResult{CutoffAt: cutoff.UTC()}
	cutoffValue := store.FormatTimestamp(result.CutoffAt)

	tx, err := g.db.BeginTx(ctx, nil)
	if err != nil {
		return store.ObservabilityRetentionSweepResult{}, fmt.Errorf(
			"store: begin observability retention sweep: %w",
			err,
		)
	}
	defer func() {
		joinCleanupError(&err, rollbackTx(tx, "observability retention sweep"))
	}()

	queries := sqlcgen.New(tx)
	if result.DeletedEventSummaries, err = queries.DeleteEventSummariesBefore(ctx, cutoffValue); err != nil {
		err = fmt.Errorf("store: delete old event_summaries rows: %w", err)
		return store.ObservabilityRetentionSweepResult{}, err
	}
	if result.DeletedTokenStats, err = queries.DeleteTokenStatsBefore(ctx, cutoffValue); err != nil {
		err = fmt.Errorf("store: delete old token_stats rows: %w", err)
		return store.ObservabilityRetentionSweepResult{}, err
	}
	// token_usage_daily is keyed by daemon-local day buckets, so the cutoff is
	// compared in LocalDay form (not the RFC3339 timestamp the other tables use).
	if result.DeletedTokenUsageDaily, err = queries.DeleteTokenUsageDailyBefore(
		ctx,
		store.LocalDay(result.CutoffAt),
	); err != nil {
		err = fmt.Errorf("store: delete old token_usage_daily rows: %w", err)
		return store.ObservabilityRetentionSweepResult{}, err
	}
	if result.DeletedPermissionLogs, err = queries.DeletePermissionLogsBefore(ctx, cutoffValue); err != nil {
		err = fmt.Errorf("store: delete old permission_log rows: %w", err)
		return store.ObservabilityRetentionSweepResult{}, err
	}

	if err := tx.Commit(); err != nil {
		return store.ObservabilityRetentionSweepResult{}, fmt.Errorf(
			"store: commit observability retention sweep: %w",
			err,
		)
	}
	return result, nil
}

// UpdateTokenStats merges one or more turns of token usage into the session aggregate.
func (g *ObserveRepo) UpdateTokenStats(ctx context.Context, update store.TokenStatsUpdate) error {
	if err := g.checkReady(ctx, "update token stats"); err != nil {
		return err
	}
	if err := update.Validate(); err != nil {
		return err
	}
	if err := g.queries.UpsertTokenStats(ctx, g.tokenStatsParams(update)); err != nil {
		return fmt.Errorf("store: upsert token stats for session %q: %w", update.SessionID, err)
	}
	return nil
}

func (g *ObserveRepo) tokenStatsParams(update store.TokenStatsUpdate) sqlcgen.UpsertTokenStatsParams {
	if update.UpdatedAt.IsZero() {
		update.UpdatedAt = g.now()
	}
	if update.Turns <= 0 {
		update.Turns = 1
	}
	return sqlcgen.UpsertTokenStatsParams{
		ID: store.NewID("tok"), SessionID: update.SessionID, AgentName: update.AgentName,
		InputTokens: nullableObserveInt64(update.InputTokens), OutputTokens: nullableObserveInt64(update.OutputTokens),
		TotalTokens: nullableObserveInt64(update.TotalTokens), TotalCost: nullableObserveFloat64(update.CostAmount),
		CostCurrency: nullableObserveStringPointer(update.CostCurrency), CostStatus: update.CostStatus,
		CostSource: update.CostSource, TurnCount: update.Turns,
		UpdatedAt: store.FormatTimestamp(update.UpdatedAt),
	}
}

// ListTokenStats returns aggregated token usage rows.
func (g *ObserveRepo) ListTokenStats(
	ctx context.Context,
	query store.TokenStatsQuery,
) (stats []store.TokenStats, err error) {
	if err := g.checkReady(ctx, "list token stats"); err != nil {
		return nil, err
	}
	if err := query.Validate(); err != nil {
		return nil, err
	}

	// dynamic-sql: optional session/agent filters and the caller limit change the statement shape.
	sqlQuery := `SELECT id, session_id, agent_name, input_tokens, output_tokens, total_tokens, total_cost, cost_currency, cost_status, cost_source, turn_count, updated_at FROM token_stats`
	where, args := store.BuildClauses(
		store.StringClause("session_id", query.SessionID),
		store.StringClause("agent_name", query.AgentName),
	)
	sqlQuery = store.AppendWhere(sqlQuery, where)
	sqlQuery += " ORDER BY updated_at DESC, id DESC"
	sqlQuery, args = store.AppendLimit(sqlQuery, args, query.Limit)

	rows, err := g.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query token stats: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("store: close token stats rows: %w", closeErr))
		}
	}()

	stats = make([]store.TokenStats, 0)
	for rows.Next() {
		stat, scanErr := scanTokenStats(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		stats = append(stats, stat)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate token stats: %w", err)
	}

	return stats, nil
}

func scanEventSummary(scanner rowScanner) (store.EventSummary, error) {
	var (
		summary        store.EventSummary
		contentJSONRaw string
		summaryText    sql.NullString
		leaseUntilRaw  string
		timestampRaw   string
	)
	if err := scanner.Scan(
		&summary.Sequence,
		&summary.ID,
		&summary.SessionID,
		&summary.WorkspaceID,
		&summary.Type,
		&summary.AgentName,
		&summary.Provider,
		&summary.Outcome,
		&contentJSONRaw,
		&summary.TaskID,
		&summary.RunID,
		&summary.WorkflowID,
		&summary.ClaimTokenHash,
		&leaseUntilRaw,
		&summary.CoordinatorSessionID,
		&summary.SchedulerReason,
		&summary.HookEvent,
		&summary.HookName,
		&summary.ActorKind,
		&summary.ActorID,
		&summary.ReleaseReason,
		&summary.ParentSessionID,
		&summary.RootSessionID,
		&summary.SpawnDepth,
		&summaryText,
		&timestampRaw,
	); err != nil {
		return store.EventSummary{}, fmt.Errorf("store: scan event summary: %w", err)
	}

	if summaryText.Valid {
		summary.Summary = summaryText.String
	}
	if strings.TrimSpace(contentJSONRaw) != "" {
		summary.Content = append(json.RawMessage(nil), []byte(contentJSONRaw)...)
	}
	if parsedLeaseUntil, err := store.ParseNullableTimestamp(leaseUntilRaw); err != nil {
		return store.EventSummary{}, err
	} else if parsedLeaseUntil != nil {
		summary.LeaseUntil = parsedLeaseUntil
	}
	timestamp, err := store.ParseTimestamp(timestampRaw)
	if err != nil {
		return store.EventSummary{}, err
	}
	summary.Timestamp = timestamp
	return summary, nil
}

func formatEventSummaryLeaseUntil(value *time.Time) string {
	if value == nil {
		return ""
	}
	return store.FormatNullableTimestamp(*value)
}

func nullableObserveString(value string) sql.NullString {
	return store.SQLNullString(value)
}

func nullableObserveStringPointer(value *string) sql.NullString {
	return store.SQLNullStringPointer(value)
}

func nullableObserveInt64(value *int64) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *value, Valid: true}
}

func nullableObserveFloat64(value *float64) sql.NullFloat64 {
	if value == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *value, Valid: true}
}

func scanTokenStats(scanner rowScanner) (store.TokenStats, error) {
	var (
		stats        store.TokenStats
		inputTokens  sql.NullInt64
		outputTokens sql.NullInt64
		totalTokens  sql.NullInt64
		totalCost    sql.NullFloat64
		costCurrency sql.NullString
		updatedAtRaw string
	)
	if err := scanner.Scan(
		&stats.ID,
		&stats.SessionID,
		&stats.AgentName,
		&inputTokens,
		&outputTokens,
		&totalTokens,
		&totalCost,
		&costCurrency,
		&stats.CostStatus,
		&stats.CostSource,
		&stats.TurnCount,
		&updatedAtRaw,
	); err != nil {
		return store.TokenStats{}, fmt.Errorf("store: scan token stats: %w", err)
	}

	stats.InputTokens = store.NullInt64(inputTokens)
	stats.OutputTokens = store.NullInt64(outputTokens)
	stats.TotalTokens = store.NullInt64(totalTokens)
	stats.TotalCost = store.NullFloat64(totalCost)
	stats.CostCurrency = store.NullString(costCurrency)

	updatedAt, err := store.ParseTimestamp(updatedAtRaw)
	if err != nil {
		return store.TokenStats{}, err
	}
	stats.UpdatedAt = updatedAt

	return stats, nil
}
