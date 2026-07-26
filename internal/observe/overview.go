package observe

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
)

// OverviewStore is the aggregate persistence capability behind the home overview.
type OverviewStore interface {
	UpsertTokenUsageDaily(ctx context.Context, update store.TokenUsageDailyUpdate) error
	ListTokenUsageByDay(ctx context.Context, query store.OverviewDayQuery) ([]store.TokenUsageDay, error)
	ListTokenUsageByAgent(ctx context.Context, query store.OverviewDayQuery) ([]store.TokenUsageAgentTotal, error)
	SumTokenUsageCost(ctx context.Context, query store.OverviewDayQuery) ([]store.TokenUsageCostGroup, error)
	CountTaskRunOutcomesByDay(ctx context.Context, query store.OverviewSinceQuery) ([]store.TaskRunOutcomeDay, error)
	CountTasksClosedByDay(ctx context.Context, query store.OverviewSinceQuery) ([]store.TaskClosedDay, error)
	CountEventsByHourWeekday(
		ctx context.Context,
		query store.OverviewSinceQuery,
	) ([]store.EventHourWeekdayBucket, error)
	LatestEventSummaryAt(ctx context.Context, workspaceID string) (time.Time, error)
	CountNetworkMessagesSince(ctx context.Context, query store.OverviewSinceQuery) (int, error)
	CountHookDispatchesSince(ctx context.Context, query store.OverviewSinceQuery) (store.HookDispatchCounts, error)
	LongestUserSessionSince(ctx context.Context, query store.OverviewSinceQuery) (*store.LongestSessionSample, error)
}

var _ OverviewStore = (*globaldb.GlobalDB)(nil)

// QueryObserveOverview composes the home overview read model (global by default,
// optionally workspace-scoped).
func (o *Observer) QueryObserveOverview(ctx context.Context, query OverviewQuery) (OverviewView, error) {
	if ctx == nil {
		return OverviewView{}, errors.New("observe: overview context is required")
	}
	if err := query.Validate(); err != nil {
		return OverviewView{}, err
	}
	overviewStore, ok := o.registry.(OverviewStore)
	if !ok {
		return OverviewView{}, errors.New("observe: overview store is not configured")
	}

	now := o.now()
	parts, err := o.fetchOverviewParts(ctx, overviewStore, query, now)
	if err != nil {
		return OverviewView{}, err
	}
	outcomeDays := zeroFilledOutcomeDays(parts.rawOutcomes, now, overviewOutcomeWindowDays)

	return OverviewView{
		GeneratedAt: now.UTC(),
		Attention:   parts.attention,
		Today:       overviewToday(outcomeDays, parts.closedToday, store.LocalDay(now)),
		Outcomes:    overviewOutcomes(outcomeDays, overviewOutcomeWindowDays),
		Usage:       parts.usage,
		Pulse:       parts.pulse,
		Network:     OverviewNetwork{MessagesToday: parts.messagesToday},
		System: OverviewSystem{
			HookRunsToday:     parts.hooks.Runs,
			HookFailuresToday: parts.hooks.Failures,
			RetentionDays:     o.retention.RetentionDays,
		},
		Freshness: parts.freshness,
	}, nil
}

// overviewParts holds the independently-fetched pieces of one overview read.
type overviewParts struct {
	attention     OverviewAttention
	rawOutcomes   []store.TaskRunOutcomeDay
	closedToday   []store.TaskClosedDay
	usage         OverviewUsage
	pulse         OverviewPulse
	messagesToday int
	hooks         store.HookDispatchCounts
	freshness     TaskDashboardFreshness
}

// fetchOverviewParts fans the independent store reads out over one errgroup so
// their round-trips overlap; each goroutine writes a distinct field.
func (o *Observer) fetchOverviewParts(
	ctx context.Context,
	overviewStore OverviewStore,
	query OverviewQuery,
	now time.Time,
) (overviewParts, error) {
	todayStart := store.LocalDayStart(now, 0)
	var parts overviewParts
	group, gctx := errgroup.WithContext(ctx)
	group.Go(func() (err error) {
		parts.attention, err = o.overviewAttention(gctx, query)
		return err
	})
	group.Go(func() (err error) {
		parts.rawOutcomes, err = overviewStore.CountTaskRunOutcomesByDay(gctx, store.OverviewSinceQuery{
			WorkspaceID: query.WorkspaceID,
			Since:       store.LocalDayStart(now, overviewOutcomeWindowDays-1),
		})
		if err != nil {
			return fmt.Errorf("observe: query run outcomes: %w", err)
		}
		return nil
	})
	group.Go(func() (err error) {
		parts.closedToday, err = overviewStore.CountTasksClosedByDay(gctx, store.OverviewSinceQuery{
			WorkspaceID: query.WorkspaceID,
			Since:       todayStart,
		})
		if err != nil {
			return fmt.Errorf("observe: query tasks closed today: %w", err)
		}
		return nil
	})
	group.Go(func() (err error) {
		parts.usage, err = o.overviewUsage(gctx, overviewStore, query, now)
		return err
	})
	group.Go(func() (err error) {
		parts.pulse, err = o.overviewPulse(gctx, overviewStore, query, now)
		return err
	})
	group.Go(func() (err error) {
		parts.messagesToday, err = overviewStore.CountNetworkMessagesSince(gctx, store.OverviewSinceQuery{
			WorkspaceID: query.WorkspaceID,
			Since:       todayStart,
		})
		if err != nil {
			return fmt.Errorf("observe: count network messages today: %w", err)
		}
		return nil
	})
	group.Go(func() (err error) {
		parts.hooks, err = overviewStore.CountHookDispatchesSince(gctx, store.OverviewSinceQuery{
			WorkspaceID: query.WorkspaceID,
			Since:       todayStart,
		})
		if err != nil {
			return fmt.Errorf("observe: count hook dispatches today: %w", err)
		}
		return nil
	})
	group.Go(func() (err error) {
		parts.freshness, err = o.overviewFreshness(gctx, overviewStore, query.WorkspaceID, now)
		return err
	})
	if err := group.Wait(); err != nil {
		return overviewParts{}, err
	}
	return parts, nil
}

func overviewToday(
	outcomeDays []store.TaskRunOutcomeDay,
	closedToday []store.TaskClosedDay,
	today string,
) OverviewToday {
	summary := OverviewToday{}
	for _, day := range outcomeDays {
		if day.Day != today {
			continue
		}
		summary.RunsCompleted = day.Completed
		summary.RunsFailed = day.Failed
	}
	for _, day := range closedToday {
		if day.Day != today {
			continue
		}
		summary.TasksClosed = day.Closed
	}
	return summary
}

func overviewOutcomes(days []store.TaskRunOutcomeDay, windowDays int) OverviewOutcomes {
	outcomes := OverviewOutcomes{WindowDays: windowDays, Days: days}
	for _, day := range days {
		outcomes.Completed += day.Completed
		outcomes.Failed += day.Failed
		outcomes.Canceled += day.Canceled
	}
	total := outcomes.Completed + outcomes.Failed + outcomes.Canceled
	if total > 0 {
		outcomes.SuccessPct = 100 * float64(outcomes.Completed) / float64(total)
	}
	return outcomes
}

// zeroFilledOutcomeDays densifies the sparse daily outcome rows into one row per
// daemon-local day across the fixed window so fixed-window chart and CLI consumers
// render the labeled window, not just the days that happened to have terminal runs.
func zeroFilledOutcomeDays(
	counted []store.TaskRunOutcomeDay,
	now time.Time,
	windowDays int,
) []store.TaskRunOutcomeDay {
	byDay := make(map[string]store.TaskRunOutcomeDay, len(counted))
	for _, day := range counted {
		byDay[day.Day] = day
	}
	days := make([]store.TaskRunOutcomeDay, 0, windowDays)
	for offset := windowDays - 1; offset >= 0; offset-- {
		date := store.LocalDay(store.LocalDayStart(now, offset))
		day := store.TaskRunOutcomeDay{Day: date}
		if existing, ok := byDay[date]; ok {
			day = existing
			day.Day = date
		}
		days = append(days, day)
	}
	return days
}
