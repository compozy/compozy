package testutil

import (
	"context"

	"github.com/compozy/compozy/internal/acp"
	core "github.com/compozy/compozy/internal/api/core"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/observe"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

type StubObserver struct {
	QueryEventsFn      func(context.Context, store.EventSummaryQuery) ([]store.EventSummary, error)
	OnAgentEventFn     func(context.Context, string, acp.AgentEvent)
	QueryHookCatalogFn func(context.Context, hookspkg.CatalogFilter) ([]hookspkg.CatalogEntry, error)
	QueryHookRunsFn    func(context.Context, store.HookRunQuery) ([]hookspkg.HookRunRecord, error)
	QueryHookEventsFn  func(context.Context, hookspkg.EventFilter) ([]hookspkg.EventDescriptor, error)

	HealthFn             func(context.Context) (observe.Health, error)
	QueryTokenStatsFn    func(context.Context, store.TokenStatsQuery) ([]store.TokenStats, error)
	QueryTaskDashboardFn func(context.Context, observe.TaskDashboardQuery) (observe.TaskDashboardView, error)
	QueryTaskInboxFn     func(
		context.Context,
		observe.TaskInboxQuery,
		taskpkg.ActorIdentity,
	) (observe.TaskInboxView, error)
	QueryObserveOverviewFn func(context.Context, observe.OverviewQuery) (observe.OverviewView, error)
}

func (s StubObserver) QueryEvents(ctx context.Context, query store.EventSummaryQuery) ([]store.EventSummary, error) {
	if s.QueryEventsFn != nil {
		return s.QueryEventsFn(ctx, query)
	}
	return nil, nil
}

func (s StubObserver) OnAgentEvent(ctx context.Context, sessionID string, event any) {
	if s.OnAgentEventFn == nil {
		return
	}
	agentEvent, ok := event.(acp.AgentEvent)
	if !ok {
		return
	}
	s.OnAgentEventFn(ctx, sessionID, agentEvent)
}

func (s StubObserver) QueryTaskDashboard(
	ctx context.Context,
	query observe.TaskDashboardQuery,
) (observe.TaskDashboardView, error) {
	if s.QueryTaskDashboardFn != nil {
		return s.QueryTaskDashboardFn(ctx, query)
	}
	return observe.TaskDashboardView{}, nil
}

func (s StubObserver) QueryTaskInbox(
	ctx context.Context,
	query observe.TaskInboxQuery,
	actor taskpkg.ActorIdentity,
) (observe.TaskInboxView, error) {
	if s.QueryTaskInboxFn != nil {
		return s.QueryTaskInboxFn(ctx, query, actor)
	}
	return observe.TaskInboxView{}, nil
}

func (s StubObserver) QueryObserveOverview(
	ctx context.Context,
	query observe.OverviewQuery,
) (observe.OverviewView, error) {
	if s.QueryObserveOverviewFn != nil {
		return s.QueryObserveOverviewFn(ctx, query)
	}
	return observe.OverviewView{}, nil
}

func (s StubObserver) Health(ctx context.Context) (observe.Health, error) {
	if s.HealthFn != nil {
		return s.HealthFn(ctx)
	}
	return observe.Health{Status: "ok"}, nil
}

func (s StubObserver) QueryHookCatalog(
	ctx context.Context,
	filter hookspkg.CatalogFilter,
) ([]hookspkg.CatalogEntry, error) {
	if s.QueryHookCatalogFn != nil {
		return s.QueryHookCatalogFn(ctx, filter)
	}
	return nil, nil
}

func (s StubObserver) QueryHookRuns(ctx context.Context, query store.HookRunQuery) ([]hookspkg.HookRunRecord, error) {
	if s.QueryHookRunsFn != nil {
		return s.QueryHookRunsFn(ctx, query)
	}
	return nil, nil
}

func (s StubObserver) QueryHookEvents(
	ctx context.Context,
	filter hookspkg.EventFilter,
) ([]hookspkg.EventDescriptor, error) {
	if s.QueryHookEventsFn != nil {
		return s.QueryHookEventsFn(ctx, filter)
	}
	return nil, nil
}

func (s StubObserver) QueryTokenStats(
	ctx context.Context,
	query store.TokenStatsQuery,
) ([]store.TokenStats, error) {
	if s.QueryTokenStatsFn != nil {
		return s.QueryTokenStatsFn(ctx, query)
	}
	return nil, nil
}

var _ core.Observer = (*StubObserver)(nil)
