package testutil

import (
	"context"

	"github.com/compozy/agh/internal/acp"
	core "github.com/compozy/agh/internal/api/core"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/observe"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
)

type StubObserver struct {
	CheckBridgeCatalogReadyFn      func() error
	QueryEventsFn                  func(context.Context, store.EventSummaryQuery) ([]store.EventSummary, error)
	OnAgentEventFn                 func(context.Context, string, acp.AgentEvent)
	QueryHookCatalogFn             func(context.Context, hookspkg.CatalogFilter) ([]hookspkg.CatalogEntry, error)
	QueryHookRunsFn                func(context.Context, store.HookRunQuery) ([]hookspkg.HookRunRecord, error)
	QueryHookEventsFn              func(context.Context, hookspkg.EventFilter) ([]hookspkg.EventDescriptor, error)
	QueryBridgeHealthFn            func(context.Context) ([]observe.BridgeInstanceHealth, error)
	QueryBridgeEffectiveStatusesFn func(
		context.Context,
		[]bridgepkg.BridgeCatalogRecord,
	) (map[string]bridgepkg.BridgeStatus, error)
	QueryBridgeHealthForFn func(
		context.Context,
		[]bridgepkg.BridgeInstance,
	) ([]observe.BridgeInstanceHealth, error)
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

func (s StubObserver) CheckBridgeCatalogReady() error {
	if s.CheckBridgeCatalogReadyFn != nil {
		return s.CheckBridgeCatalogReadyFn()
	}
	return nil
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

func (s StubObserver) QueryBridgeHealth(ctx context.Context) ([]observe.BridgeInstanceHealth, error) {
	if s.QueryBridgeHealthFn != nil {
		return s.QueryBridgeHealthFn(ctx)
	}
	return nil, nil
}

func (s StubObserver) QueryBridgeEffectiveStatuses(
	ctx context.Context,
	records []bridgepkg.BridgeCatalogRecord,
) (map[string]bridgepkg.BridgeStatus, error) {
	if s.QueryBridgeEffectiveStatusesFn != nil {
		return s.QueryBridgeEffectiveStatusesFn(ctx, records)
	}
	health, err := s.QueryBridgeHealth(ctx)
	if err != nil {
		return nil, err
	}
	statuses := make(map[string]bridgepkg.BridgeStatus, len(records))
	for _, record := range records {
		statuses[record.ID] = record.Status
	}
	for _, item := range health {
		statuses[item.BridgeInstanceID] = item.Status
	}
	return statuses, nil
}

func (s StubObserver) QueryBridgeHealthFor(
	ctx context.Context,
	instances []bridgepkg.BridgeInstance,
) ([]observe.BridgeInstanceHealth, error) {
	if s.QueryBridgeHealthForFn != nil {
		return s.QueryBridgeHealthForFn(ctx, instances)
	}
	health, err := s.QueryBridgeHealth(ctx)
	if err != nil {
		return nil, err
	}
	requested := make(map[string]struct{}, len(instances))
	for _, instance := range instances {
		requested[instance.ID] = struct{}{}
	}
	filtered := make([]observe.BridgeInstanceHealth, 0, len(instances))
	for _, item := range health {
		if _, ok := requested[item.BridgeInstanceID]; ok {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
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
