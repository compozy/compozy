package globaldb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
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
		ProfileID:    strings.TrimSpace(update.ProfileID),
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
	return g.queryTokenUsageDays(ctx, query)
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

	return g.queryTokenUsageAgents(ctx, query)
}

// ListTokenUsageByProfile returns owner-labeled usage buckets inside the rollup window.
func (g *ObserveRepo) ListTokenUsageByProfile(
	ctx context.Context,
	query store.OverviewDayQuery,
) ([]store.TokenUsageProfileTotal, error) {
	if err := g.checkReady(ctx, "list token usage by profile"); err != nil {
		return nil, err
	}
	if err := query.Validate(); err != nil {
		return nil, err
	}
	return g.queryTokenUsageProfiles(ctx, query)
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

	return g.queryTokenUsageCosts(ctx, query)
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

	return g.queryTaskRunOutcomeDays(ctx, query)
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

	return g.queryTaskClosedDays(ctx, query)
}

// CountEventsByHourWeekday returns hour-by-weekday session event counts inside the window.
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

	return g.queryEventHourWeekdays(ctx, query)
}

// LatestEventSummaryAt returns the newest event summary timestamp, zero when none exist.
func (g *ObserveRepo) LatestEventSummaryAt(
	ctx context.Context,
	query store.OverviewWorkspaceQuery,
) (time.Time, error) {
	if err := g.checkReady(ctx, "latest event summary timestamp"); err != nil {
		return time.Time{}, err
	}
	if err := query.Validate(); err != nil {
		return time.Time{}, err
	}
	return g.queryLatestEventSummaryAt(ctx, query)
}

// CountNetworkMessagesSince counts durable network audit envelopes inside the window.
func (g *ObserveRepo) CountNetworkMessagesSince(ctx context.Context, query store.OverviewSinceQuery) (int, error) {
	if err := g.checkReady(ctx, "count network messages"); err != nil {
		return 0, err
	}
	if err := query.Validate(); err != nil {
		return 0, err
	}

	return g.queryNetworkMessageCount(ctx, query)
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

	return g.queryHookDispatchCounts(ctx, query)
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

	return g.queryLongestUserSession(ctx, query)
}

// overviewTokenDelta treats a nil delta as a zero contribution; negative deltas
// are rejected upstream by TokenUsageDailyUpdate.Validate.
func overviewTokenDelta(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}
