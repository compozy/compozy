package globaldb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

// UpsertTokenUsageDaily merges one turn of token usage into the daily rollup bucket.
func (g *ObserveRepo) UpsertTokenUsageDaily(ctx context.Context, update store.TokenUsageDailyUpdate) error {
	if err := g.checkReady(ctx, "upsert token usage daily"); err != nil {
		return err
	}
	if err := update.Validate(); err != nil {
		return err
	}
	if err := g.queries.UpsertTokenUsageDaily(ctx, g.tokenUsageDailyParams(update)); err != nil {
		return fmt.Errorf("store: upsert token usage daily for day %q: %w", update.Day, err)
	}
	return nil
}

func (g *ObserveRepo) tokenUsageDailyParams(update store.TokenUsageDailyUpdate) sqlcgen.UpsertTokenUsageDailyParams {
	if update.UpdatedAt.IsZero() {
		update.UpdatedAt = g.now()
	}
	if update.Turns <= 0 {
		update.Turns = 1
	}
	return sqlcgen.UpsertTokenUsageDailyParams{
		Day:          strings.TrimSpace(update.Day),
		WorkspaceID:  strings.TrimSpace(update.WorkspaceID),
		AgentName:    strings.TrimSpace(update.AgentName),
		InputTokens:  overviewTokenDelta(update.InputTokens),
		OutputTokens: overviewTokenDelta(update.OutputTokens),
		TotalTokens:  overviewTokenDelta(update.TotalTokens),
		TotalCost:    nullableObserveFloat64(update.CostAmount),
		CostCurrency: nullableObserveStringPointer(update.CostCurrency),
		CostStatus:   update.CostStatus,
		CostSource:   update.CostSource,
		TurnCount:    update.Turns,
		UpdatedAt:    store.FormatTimestamp(update.UpdatedAt),
	}
}

// ListTokenUsageByDay returns summed daily token usage inside the rollup window.
func (g *ObserveRepo) ListTokenUsageByDay(
	ctx context.Context,
	query store.OverviewDayQuery,
) ([]store.TokenUsageDay, error) {
	if err := g.checkReady(ctx, "list token usage by day"); err != nil {
		return nil, err
	}
	if err := query.Validate(); err != nil {
		return nil, err
	}

	rows, err := g.queries.ListTokenUsageDailyByDay(ctx, sqlcgen.ListTokenUsageDailyByDayParams{
		SinceDay:    strings.TrimSpace(query.SinceDay),
		WorkspaceID: strings.TrimSpace(query.WorkspaceID),
	})
	if err != nil {
		return nil, fmt.Errorf("store: query token usage by day: %w", err)
	}

	days := make([]store.TokenUsageDay, 0, len(rows))
	for _, row := range rows {
		days = append(days, store.TokenUsageDay{
			Day:          row.Day,
			InputTokens:  row.InputTokens,
			OutputTokens: row.OutputTokens,
			TotalTokens:  row.TotalTokens,
		})
	}
	return days, nil
}

// ListTokenUsageByAgent returns summed per-agent token usage inside the rollup window.
func (g *ObserveRepo) ListTokenUsageByAgent(
	ctx context.Context,
	query store.OverviewDayQuery,
) ([]store.TokenUsageAgentTotal, error) {
	if err := g.checkReady(ctx, "list token usage by agent"); err != nil {
		return nil, err
	}
	if err := query.Validate(); err != nil {
		return nil, err
	}

	rows, err := g.queries.ListTokenUsageDailyByAgent(ctx, sqlcgen.ListTokenUsageDailyByAgentParams{
		SinceDay:    strings.TrimSpace(query.SinceDay),
		WorkspaceID: strings.TrimSpace(query.WorkspaceID),
	})
	if err != nil {
		return nil, fmt.Errorf("store: query token usage by agent: %w", err)
	}

	totals := make([]store.TokenUsageAgentTotal, 0, len(rows))
	for _, row := range rows {
		totals = append(totals, store.TokenUsageAgentTotal{
			AgentName:   strings.TrimSpace(row.AgentName),
			TotalTokens: row.TotalTokens,
		})
	}
	return totals, nil
}

// SumTokenUsageCost returns provenance-grouped cost totals inside the rollup window.
func (g *ObserveRepo) SumTokenUsageCost(
	ctx context.Context,
	query store.OverviewDayQuery,
) ([]store.TokenUsageCostGroup, error) {
	if err := g.checkReady(ctx, "sum token usage cost"); err != nil {
		return nil, err
	}
	if err := query.Validate(); err != nil {
		return nil, err
	}

	rows, err := g.queries.SumTokenUsageDailyCost(ctx, sqlcgen.SumTokenUsageDailyCostParams{
		SinceDay:    strings.TrimSpace(query.SinceDay),
		WorkspaceID: strings.TrimSpace(query.WorkspaceID),
	})
	if err != nil {
		return nil, fmt.Errorf("store: query token usage cost: %w", err)
	}

	groups := make([]store.TokenUsageCostGroup, 0, len(rows))
	for _, row := range rows {
		groups = append(groups, store.TokenUsageCostGroup{
			CostStatus:      row.CostStatus,
			CostSource:      row.CostSource,
			CostCurrency:    row.CostCurrency,
			TotalCost:       row.TotalCost,
			RowsWithoutCost: row.RowsWithoutCost,
			RowsTotal:       row.RowsTotal,
		})
	}
	return groups, nil
}

// CountTaskRunOutcomesByDay returns terminal worker-run outcome counts per daemon-local day.
func (g *ObserveRepo) CountTaskRunOutcomesByDay(
	ctx context.Context,
	query store.OverviewSinceQuery,
) ([]store.TaskRunOutcomeDay, error) {
	if err := g.checkReady(ctx, "count task run outcomes by day"); err != nil {
		return nil, err
	}
	if err := query.Validate(); err != nil {
		return nil, err
	}

	rows, err := g.queries.CountTaskRunOutcomesByDay(ctx, sqlcgen.CountTaskRunOutcomesByDayParams{
		Since:       store.FormatTimestamp(query.Since),
		WorkspaceID: strings.TrimSpace(query.WorkspaceID),
	})
	if err != nil {
		return nil, fmt.Errorf("store: query task run outcomes by day: %w", err)
	}

	days := make([]store.TaskRunOutcomeDay, 0, len(rows))
	for _, row := range rows {
		days = append(days, store.TaskRunOutcomeDay{
			Day:       row.Day,
			Completed: int(row.Completed),
			Failed:    int(row.Failed),
			Canceled:  int(row.Canceled),
		})
	}
	return days, nil
}

// CountTasksClosedByDay returns completed-task closure counts per daemon-local day.
func (g *ObserveRepo) CountTasksClosedByDay(
	ctx context.Context,
	query store.OverviewSinceQuery,
) ([]store.TaskClosedDay, error) {
	if err := g.checkReady(ctx, "count tasks closed by day"); err != nil {
		return nil, err
	}
	if err := query.Validate(); err != nil {
		return nil, err
	}

	rows, err := g.queries.CountTasksClosedByDay(ctx, sqlcgen.CountTasksClosedByDayParams{
		Since:       store.FormatTimestamp(query.Since),
		WorkspaceID: strings.TrimSpace(query.WorkspaceID),
	})
	if err != nil {
		return nil, fmt.Errorf("store: query tasks closed by day: %w", err)
	}

	days := make([]store.TaskClosedDay, 0, len(rows))
	for _, row := range rows {
		days = append(days, store.TaskClosedDay{Day: row.Day, Closed: int(row.Closed)})
	}
	return days, nil
}

// CountEventsByHourWeekday returns hour-by-weekday event counts inside the window.
func (g *ObserveRepo) CountEventsByHourWeekday(
	ctx context.Context,
	query store.OverviewSinceQuery,
) ([]store.EventHourWeekdayBucket, error) {
	if err := g.checkReady(ctx, "count events by hour and weekday"); err != nil {
		return nil, err
	}
	if err := query.Validate(); err != nil {
		return nil, err
	}

	rows, err := g.queries.CountEventSummariesByHourWeekday(ctx, sqlcgen.CountEventSummariesByHourWeekdayParams{
		Since:       store.FormatTimestamp(query.Since),
		WorkspaceID: strings.TrimSpace(query.WorkspaceID),
	})
	if err != nil {
		return nil, fmt.Errorf("store: query events by hour and weekday: %w", err)
	}

	buckets := make([]store.EventHourWeekdayBucket, 0, len(rows))
	for _, row := range rows {
		buckets = append(buckets, store.EventHourWeekdayBucket{
			Weekday: int(row.Weekday),
			Hour:    int(row.Hour),
			Events:  int(row.Events),
		})
	}
	return buckets, nil
}

// LatestEventSummaryAt returns the newest event summary timestamp, zero when none exist.
func (g *ObserveRepo) LatestEventSummaryAt(ctx context.Context, workspaceID string) (time.Time, error) {
	if err := g.checkReady(ctx, "latest event summary timestamp"); err != nil {
		return time.Time{}, err
	}

	latest, err := g.queries.MaxEventSummaryTimestamp(ctx, strings.TrimSpace(workspaceID))
	if err != nil {
		return time.Time{}, fmt.Errorf("store: query latest event summary timestamp: %w", err)
	}
	if strings.TrimSpace(latest) == "" {
		return time.Time{}, nil
	}
	parsed, err := store.ParseTimestamp(latest)
	if err != nil {
		return time.Time{}, fmt.Errorf("store: parse latest event summary timestamp: %w", err)
	}
	return parsed, nil
}

// CountNetworkMessagesSince counts durable network audit envelopes inside the window.
func (g *ObserveRepo) CountNetworkMessagesSince(ctx context.Context, query store.OverviewSinceQuery) (int, error) {
	if err := g.checkReady(ctx, "count network messages"); err != nil {
		return 0, err
	}
	if err := query.Validate(); err != nil {
		return 0, err
	}

	messages, err := g.queries.CountNetworkAuditSince(ctx, sqlcgen.CountNetworkAuditSinceParams{
		Since:       store.FormatTimestamp(query.Since),
		WorkspaceID: strings.TrimSpace(query.WorkspaceID),
	})
	if err != nil {
		return 0, fmt.Errorf("store: count network audit messages: %w", err)
	}
	return int(messages), nil
}

// CountHookDispatchesSince counts hook dispatch completions and failures inside the window.
func (g *ObserveRepo) CountHookDispatchesSince(
	ctx context.Context,
	query store.OverviewSinceQuery,
) (store.HookDispatchCounts, error) {
	if err := g.checkReady(ctx, "count hook dispatches"); err != nil {
		return store.HookDispatchCounts{}, err
	}
	if err := query.Validate(); err != nil {
		return store.HookDispatchCounts{}, err
	}

	row, err := g.queries.CountHookDispatchSince(ctx, sqlcgen.CountHookDispatchSinceParams{
		Since:       store.FormatTimestamp(query.Since),
		WorkspaceID: strings.TrimSpace(query.WorkspaceID),
	})
	if err != nil {
		return store.HookDispatchCounts{}, fmt.Errorf("store: count hook dispatches: %w", err)
	}
	return store.HookDispatchCounts{
		Runs:     int(row.Runs),
		Failures: int(row.Failures),
	}, nil
}

// LongestUserSessionSince returns the longest user session started inside the window, nil when none.
func (g *ObserveRepo) LongestUserSessionSince(
	ctx context.Context,
	query store.OverviewSinceQuery,
) (*store.LongestSessionSample, error) {
	if err := g.checkReady(ctx, "longest user session"); err != nil {
		return nil, err
	}
	if err := query.Validate(); err != nil {
		return nil, err
	}

	rows, err := g.queries.LongestUserSessionSince(ctx, sqlcgen.LongestUserSessionSinceParams{
		Now:         store.FormatTimestamp(g.now()),
		Since:       store.FormatTimestamp(query.Since),
		WorkspaceID: strings.TrimSpace(query.WorkspaceID),
	})
	if err != nil {
		return nil, fmt.Errorf("store: query longest user session: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}

	row := rows[0]
	startedAt, err := store.ParseTimestamp(row.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("store: parse longest session start: %w", err)
	}
	return &store.LongestSessionSample{
		SessionID:      row.ID,
		AgentName:      strings.TrimSpace(row.AgentName),
		StartedAt:      startedAt,
		RuntimeSeconds: row.RuntimeSeconds,
	}, nil
}

// overviewTokenDelta treats a nil delta as a zero contribution; negative deltas
// are rejected upstream by TokenUsageDailyUpdate.Validate.
func overviewTokenDelta(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}
