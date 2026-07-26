package observe

import (
	"context"
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/testutil"
)

type blockingObserveTransport struct {
	releaseCh  chan struct{}
	blockStart bool
}

type catalogBridgeSource struct {
	instances          []bridgepkg.BridgeInstance
	routeCounts        map[string]int
	metrics            map[string]bridgepkg.BridgeDeliveryMetrics
	listInstancesCalls int
	listRoutesCalls    int
	batchCalls         int
	batchIDs           []string
	metricsCalls       int
	metricsBatchCalls  int
	metricsBatchIDs    []string
}

func (s *catalogBridgeSource) ListInstances(context.Context) ([]bridgepkg.BridgeInstance, error) {
	s.listInstancesCalls++
	return append([]bridgepkg.BridgeInstance(nil), s.instances...), nil
}

func (s *catalogBridgeSource) ListRoutes(context.Context, string) ([]bridgepkg.BridgeRoute, error) {
	s.listRoutesCalls++
	return nil, nil
}

func (s *catalogBridgeSource) DeliveryMetrics() map[string]bridgepkg.BridgeDeliveryMetrics {
	s.metricsCalls++
	return s.metrics
}

func (s *catalogBridgeSource) DeliveryMetricsFor(
	bridgeInstanceIDs []string,
) (map[string]bridgepkg.BridgeDeliveryMetrics, error) {
	s.metricsBatchCalls++
	s.metricsBatchIDs = append([]string(nil), bridgeInstanceIDs...)
	result := make(map[string]bridgepkg.BridgeDeliveryMetrics, len(bridgeInstanceIDs))
	for _, id := range bridgeInstanceIDs {
		if metrics, ok := s.metrics[id]; ok {
			result[id] = metrics
		}
	}
	return result, nil
}

func (s *catalogBridgeSource) CountBridgeRoutes(
	_ context.Context,
	bridgeInstanceIDs []string,
) (map[string]int, error) {
	s.batchCalls++
	s.batchIDs = append([]string(nil), bridgeInstanceIDs...)
	result := make(map[string]int, len(bridgeInstanceIDs))
	for _, id := range bridgeInstanceIDs {
		result[id] = s.routeCounts[id]
	}
	return result, nil
}

func (t *blockingObserveTransport) DeliverBridge(
	ctx context.Context,
	_ string,
	req bridgepkg.DeliveryRequest,
) (bridgepkg.DeliveryAck, error) {
	if t != nil && t.blockStart && req.Event.EventType == bridgepkg.DeliveryEventTypeStart {
		select {
		case <-t.releaseCh:
		case <-ctx.Done():
			return bridgepkg.DeliveryAck{}, ctx.Err()
		}
	}
	return bridgepkg.DeliveryAck{
		DeliveryID: req.Event.DeliveryID,
		Seq:        req.Event.Seq,
	}, nil
}

func TestHealthIncludesBridgeStatusCountsAndRouteSummary(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	starting := createObserveBridgeInstance(t, h, "brg-starting", bridgepkg.BridgeStatusStarting)
	ready := createObserveBridgeInstance(t, h, "brg-ready", bridgepkg.BridgeStatusReady)
	degraded := createObserveBridgeInstance(t, h, "brg-degraded", bridgepkg.BridgeStatusDegraded)
	authRequired := createObserveBridgeInstance(t, h, "brg-auth", bridgepkg.BridgeStatusAuthRequired)
	h.observer.RecordBridgeAuthFailure(authRequired.ID)

	upsertObserveRoute(t, h, starting, "peer-starting", "sess-starting")
	upsertObserveRoute(t, h, ready, "peer-ready-a", "sess-ready-a")
	upsertObserveRoute(t, h, ready, "peer-ready-b", "sess-ready-b")
	upsertObserveRoute(t, h, degraded, "peer-degraded", "sess-degraded")

	health, err := h.observer.Health(testutil.Context(t))
	if err != nil {
		t.Fatalf("Health() error = %v", err)
	}

	if got, want := health.Bridges.TotalInstances, 4; got != want {
		t.Fatalf("Health().Bridges.TotalInstances = %d, want %d", got, want)
	}
	if got, want := health.Bridges.RouteCount, 4; got != want {
		t.Fatalf("Health().Bridges.RouteCount = %d, want %d", got, want)
	}
	if got, want := health.Bridges.StatusCounts.Starting, 1; got != want {
		t.Fatalf("Health().Bridges.StatusCounts.Starting = %d, want %d", got, want)
	}
	if got, want := health.Bridges.StatusCounts.Ready, 1; got != want {
		t.Fatalf("Health().Bridges.StatusCounts.Ready = %d, want %d", got, want)
	}
	if got, want := health.Bridges.StatusCounts.Degraded, 1; got != want {
		t.Fatalf("Health().Bridges.StatusCounts.Degraded = %d, want %d", got, want)
	}
	if got, want := health.Bridges.StatusCounts.AuthRequired, 1; got != want {
		t.Fatalf("Health().Bridges.StatusCounts.AuthRequired = %d, want %d", got, want)
	}
	if got, want := health.Bridges.AuthFailuresTotal, 1; got != want {
		t.Fatalf("Health().Bridges.AuthFailuresTotal = %d, want %d", got, want)
	}

	observed := observeBridgeHealthMap(t, h)
	if got, want := observed[ready.ID].RouteCount, 2; got != want {
		t.Fatalf("QueryBridgeHealth(%s).RouteCount = %d, want %d", ready.ID, got, want)
	}
	if got, want := observed[authRequired.ID].Status, bridgepkg.BridgeStatusAuthRequired; got != want {
		t.Fatalf("QueryBridgeHealth(%s).Status = %q, want %q", authRequired.ID, got, want)
	}
}

func TestBridgeCatalogHealthReadsOnlyTheRequestedPage(t *testing.T) {
	t.Parallel()

	t.Run("Should merge effective status without a registry read and batch page health", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 7, 10, 18, 0, 0, 0, time.UTC)
		instances := []bridgepkg.BridgeInstance{
			{ID: "brg-a", Enabled: true, Status: bridgepkg.BridgeStatusReady},
			{ID: "brg-b", Enabled: true, Status: bridgepkg.BridgeStatusReady},
			{ID: "brg-c", Enabled: false, Status: bridgepkg.BridgeStatusDisabled},
		}
		source := &catalogBridgeSource{
			instances:   instances,
			routeCounts: map[string]int{"brg-a": 9, "brg-b": 3, "brg-c": 7},
			metrics: map[string]bridgepkg.BridgeDeliveryMetrics{
				"brg-b": {DeliveryBacklog: 2},
				"brg-c": {DeliveryBacklog: 99, DeliveryFailuresTotal: 77},
			},
		}
		observer := &Observer{
			now:          func() time.Time { return now },
			bridgeSource: source,
			bridgeState:  make(map[string]observedBridgeState),
		}
		observer.RecordBridgeRuntimeIssue("brg-b", bridgepkg.BridgeStatusError, "adapter crashed")

		records := make([]bridgepkg.BridgeCatalogRecord, 0, len(instances))
		for _, instance := range instances {
			records = append(records, bridgepkg.BridgeCatalogRecordFromInstance(instance))
		}
		statuses, err := observer.QueryBridgeEffectiveStatuses(testutil.Context(t), records)
		if err != nil {
			t.Fatalf("QueryBridgeEffectiveStatuses() error = %v", err)
		}
		if got, want := statuses["brg-b"], bridgepkg.BridgeStatusError; got != want {
			t.Fatalf("effective brg-b status = %q, want %q", got, want)
		}
		if got, want := statuses["brg-c"], bridgepkg.BridgeStatusDisabled; got != want {
			t.Fatalf("effective brg-c status = %q, want %q", got, want)
		}

		health, err := observer.QueryBridgeHealthFor(testutil.Context(t), instances[1:2])
		if err != nil {
			t.Fatalf("QueryBridgeHealthFor() error = %v", err)
		}
		if got, want := len(health), 1; got != want {
			t.Fatalf("len(QueryBridgeHealthFor()) = %d, want %d", got, want)
		}
		if health[0].BridgeInstanceID != "brg-b" || health[0].Status != bridgepkg.BridgeStatusError ||
			health[0].RouteCount != 3 || health[0].DeliveryBacklog != 2 || health[0].LastError != "adapter crashed" {
			t.Fatalf("QueryBridgeHealthFor() = %#v, want page-scoped effective health", health)
		}
		if source.listInstancesCalls != 0 || source.listRoutesCalls != 0 || source.batchCalls != 1 ||
			source.metricsCalls != 0 || source.metricsBatchCalls != 1 {
			t.Fatalf(
				"source calls = listInstances:%d listRoutes:%d routes_batch:%d metrics:%d metrics_batch:%d, want 0/0/1/0/1",
				source.listInstancesCalls,
				source.listRoutesCalls,
				source.batchCalls,
				source.metricsCalls,
				source.metricsBatchCalls,
			)
		}
		if got, want := source.batchIDs, []string{"brg-b"}; len(got) != len(want) || got[0] != want[0] {
			t.Fatalf("batch ids = %#v, want %#v", got, want)
		}
		if got, want := source.metricsBatchIDs, []string{"brg-b"}; len(got) != len(want) || got[0] != want[0] {
			t.Fatalf("metrics batch ids = %#v, want %#v", got, want)
		}
	})

	t.Run("Should batch the complete health snapshot without per-instance route reads", func(t *testing.T) {
		t.Parallel()

		instances := []bridgepkg.BridgeInstance{
			{ID: "brg-a", Enabled: true, Status: bridgepkg.BridgeStatusReady},
			{ID: "brg-b", Enabled: true, Status: bridgepkg.BridgeStatusDegraded},
		}
		source := &catalogBridgeSource{
			instances:   instances,
			routeCounts: map[string]int{"brg-a": 4, "brg-b": 6},
		}
		observer := &Observer{
			now:          time.Now,
			bridgeSource: source,
			bridgeState:  make(map[string]observedBridgeState),
		}

		health, err := observer.QueryBridgeHealth(testutil.Context(t))
		if err != nil {
			t.Fatalf("QueryBridgeHealth() error = %v", err)
		}
		if len(health) != 2 || health[0].RouteCount+health[1].RouteCount != 10 {
			t.Fatalf("QueryBridgeHealth() = %#v, want two batched route counts", health)
		}
		if source.listInstancesCalls != 1 || source.batchCalls != 1 || source.listRoutesCalls != 0 {
			t.Fatalf(
				"source calls = listInstances:%d batch:%d listRoutes:%d, want 1/1/0",
				source.listInstancesCalls,
				source.batchCalls,
				source.listRoutesCalls,
			)
		}
	})

	t.Run("Should reject catalog reads when the bridge source is missing", func(t *testing.T) {
		t.Parallel()

		observer := &Observer{bridgeState: make(map[string]observedBridgeState)}
		if err := observer.CheckBridgeCatalogReady(); err == nil {
			t.Fatal("CheckBridgeCatalogReady() error = nil, want missing source error")
		}
		record := bridgepkg.BridgeCatalogRecord{
			ID:      "brg-missing-source",
			Enabled: true,
			Status:  bridgepkg.BridgeStatusReady,
		}
		if _, err := observer.QueryBridgeEffectiveStatuses(
			testutil.Context(t),
			[]bridgepkg.BridgeCatalogRecord{record},
		); err == nil {
			t.Fatal("QueryBridgeEffectiveStatuses() error = nil, want missing source error")
		}
		instance := bridgepkg.BridgeInstance{ID: record.ID, Enabled: true, Status: record.Status}
		if _, err := observer.QueryBridgeHealthFor(
			testutil.Context(t),
			[]bridgepkg.BridgeInstance{instance},
		); err == nil {
			t.Fatal("QueryBridgeHealthFor() error = nil, want missing source error")
		}
	})
}

func TestHealthTracksDeliveryBacklogWithoutChangingActiveSessions(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.source.sessions = []*session.Info{{
		ID:        "sess-live",
		AgentName: "coder",
		State:     session.StateActive,
	}}

	instance := createObserveBridgeInstance(t, h, "brg-backlog", bridgepkg.BridgeStatusReady)
	transport := &blockingObserveTransport{
		releaseCh:  make(chan struct{}),
		blockStart: true,
	}
	h.bridges.broker.SetTransport(transport)

	registration := registerObserveDelivery(t, h, instance, "sess-live", "turn-live", "peer-backlog")
	if err := h.bridges.broker.Deliver(
		testutil.Context(t),
		observeDeliveryEvent(registration, 1, bridgepkg.DeliveryEventTypeStart, "hello", false),
	); err != nil {
		t.Fatalf("Deliver(start) error = %v", err)
	}
	if err := h.bridges.broker.Deliver(
		testutil.Context(t),
		observeDeliveryEvent(registration, 2, bridgepkg.DeliveryEventTypeDelta, "hello again", false),
	); err != nil {
		t.Fatalf("Deliver(delta) error = %v", err)
	}

	waitForObserveCondition(t, func() bool {
		health, err := h.observer.Health(testutil.Context(t))
		return err == nil && health.Bridges.DeliveryBacklog == 1
	})

	health, err := h.observer.Health(testutil.Context(t))
	if err != nil {
		t.Fatalf("Health() error = %v", err)
	}
	if got, want := health.ActiveSessions, 1; got != want {
		t.Fatalf("Health().ActiveSessions = %d, want %d", got, want)
	}
	if got, want := health.Bridges.DeliveryBacklog, 1; got != want {
		t.Fatalf("Health().Bridges.DeliveryBacklog = %d, want %d", got, want)
	}

	close(transport.releaseCh)
	waitForObserveCondition(t, func() bool {
		health, err := h.observer.Health(testutil.Context(t))
		return err == nil && health.Bridges.DeliveryBacklog == 0
	})

	observed := observeBridgeHealthMap(t, h)
	if got := observed[instance.ID].LastSuccessAt; !got.Equal(h.now) {
		t.Fatalf("QueryBridgeHealth(%s).LastSuccessAt = %s, want %s", instance.ID, got, h.now)
	}
}

func TestQueryBridgeHealthSurfacesAuthAndTerminalDeliveryFailuresPerInstance(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	authInstance := createObserveBridgeInstance(t, h, "brg-auth-failure", bridgepkg.BridgeStatusAuthRequired)
	deliveryInstance := createObserveBridgeInstance(t, h, "brg-delivery-failure", bridgepkg.BridgeStatusReady)
	otherInstance := createObserveBridgeInstance(t, h, "brg-other", bridgepkg.BridgeStatusReady)
	h.observer.RecordBridgeAuthFailure(authInstance.ID)

	registration := registerObserveDelivery(t, h, deliveryInstance, "sess-delivery", "turn-delivery", "peer-delivery")
	if err := h.bridges.broker.Deliver(
		testutil.Context(t),
		observeDeliveryEvent(registration, 1, bridgepkg.DeliveryEventTypeError, "boom", true),
	); err != nil {
		t.Fatalf("Deliver(error) error = %v", err)
	}

	observed := observeBridgeHealthMap(t, h)
	if got, want := observed[authInstance.ID].AuthFailuresTotal, 1; got != want {
		t.Fatalf("auth instance AuthFailuresTotal = %d, want %d", got, want)
	}
	if got, want := observed[deliveryInstance.ID].DeliveryFailuresTotal, 1; got != want {
		t.Fatalf("delivery instance DeliveryFailuresTotal = %d, want %d", got, want)
	}
	if got, want := observed[deliveryInstance.ID].LastError, "boom"; got != want {
		t.Fatalf("delivery instance LastError = %q, want %q", got, want)
	}
	if observed[otherInstance.ID].AuthFailuresTotal != 0 || observed[otherInstance.ID].DeliveryFailuresTotal != 0 {
		t.Fatalf("other instance health = %#v, want zero auth/delivery failures", observed[otherInstance.ID])
	}
}

func TestBridgeRuntimeOverridesAffectStatusCountsAndCanBeCleared(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	disabled := createObserveBridgeInstance(t, h, "brg-disabled", bridgepkg.BridgeStatusDisabled)
	ready := createObserveBridgeInstance(t, h, "brg-runtime", bridgepkg.BridgeStatusReady)

	h.observer.RecordBridgeRuntimeIssue(disabled.ID, bridgepkg.BridgeStatusError, "ignored for disabled")
	h.observer.RecordBridgeRuntimeIssue(ready.ID, bridgepkg.BridgeStatusError, "adapter crashed")

	health, err := h.observer.Health(testutil.Context(t))
	if err != nil {
		t.Fatalf("Health() error = %v", err)
	}
	if got, want := health.Bridges.StatusCounts.Disabled, 1; got != want {
		t.Fatalf("Health().Bridges.StatusCounts.Disabled = %d, want %d", got, want)
	}
	if got, want := health.Bridges.StatusCounts.Error, 1; got != want {
		t.Fatalf("Health().Bridges.StatusCounts.Error = %d, want %d", got, want)
	}

	observed := observeBridgeHealthMap(t, h)
	if got, want := observed[disabled.ID].Status, bridgepkg.BridgeStatusDisabled; got != want {
		t.Fatalf("disabled instance Status = %q, want %q", got, want)
	}
	if got, want := observed[ready.ID].Status, bridgepkg.BridgeStatusError; got != want {
		t.Fatalf("runtime instance Status = %q, want %q", got, want)
	}
	if got, want := observed[ready.ID].LastError, "adapter crashed"; got != want {
		t.Fatalf("runtime instance LastError = %q, want %q", got, want)
	}

	h.observer.ClearBridgeRuntimeIssue(ready.ID)

	health, err = h.observer.Health(testutil.Context(t))
	if err != nil {
		t.Fatalf("Health() after clear error = %v", err)
	}
	if got, want := health.Bridges.StatusCounts.Ready, 1; got != want {
		t.Fatalf("Health().Bridges.StatusCounts.Ready = %d, want %d", got, want)
	}
	if got := health.Bridges.StatusCounts.Error; got != 0 {
		t.Fatalf("Health().Bridges.StatusCounts.Error = %d, want 0", got)
	}

	observed = observeBridgeHealthMap(t, h)
	if got, want := observed[ready.ID].Status, bridgepkg.BridgeStatusReady; got != want {
		t.Fatalf("runtime instance Status after clear = %q, want %q", got, want)
	}
	if got := observed[ready.ID].LastError; got != "" {
		t.Fatalf("runtime instance LastError after clear = %q, want empty", got)
	}
}

func createObserveBridgeInstance(
	t *testing.T,
	h *harness,
	id string,
	status bridgepkg.BridgeStatus,
) *bridgepkg.BridgeInstance {
	t.Helper()

	instance, err := h.bridges.CreateInstance(testutil.Context(t), bridgepkg.CreateInstanceRequest{
		ID:            id,
		Scope:         bridgepkg.ScopeWorkspace,
		WorkspaceID:   h.workspaceID,
		Platform:      "telegram",
		ExtensionName: "ext-telegram",
		DisplayName:   id,
		Enabled:       status != bridgepkg.BridgeStatusDisabled,
		Status:        status,
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	if err != nil {
		t.Fatalf("CreateInstance(%s) error = %v", id, err)
	}
	return instance
}

func upsertObserveRoute(t *testing.T, h *harness, instance *bridgepkg.BridgeInstance, peerID string, sessionID string) {
	t.Helper()

	if _, err := h.bridges.UpsertRoute(testutil.Context(t), bridgepkg.BridgeRoute{
		BridgeInstanceID: instance.ID,
		Scope:            instance.Scope,
		WorkspaceID:      instance.WorkspaceID,
		PeerID:           peerID,
		SessionID:        sessionID,
		AgentName:        "coder",
		LastActivityAt:   h.now,
	}); err != nil {
		t.Fatalf("UpsertRoute(%s) error = %v", peerID, err)
	}
}

func registerObserveDelivery(
	t *testing.T,
	h *harness,
	instance *bridgepkg.BridgeInstance,
	sessionID string,
	turnID string,
	peerID string,
) bridgepkg.DeliverySnapshot {
	t.Helper()

	snapshot, err := h.bridges.broker.RegisterPromptDelivery(testutil.Context(t), bridgepkg.PromptDeliveryRegistration{
		SessionID:     sessionID,
		TurnID:        turnID,
		ExtensionName: instance.ExtensionName,
		RoutingKey: bridgepkg.RoutingKey{
			Scope:            instance.Scope,
			WorkspaceID:      instance.WorkspaceID,
			BridgeInstanceID: instance.ID,
			PeerID:           peerID,
		},
		DeliveryTarget: bridgepkg.DeliveryTarget{
			BridgeInstanceID: instance.ID,
			PeerID:           peerID,
			Mode:             bridgepkg.DeliveryModeReply,
		},
	})
	if err != nil {
		t.Fatalf("RegisterPromptDelivery(%s) error = %v", instance.ID, err)
	}
	return *snapshot
}

func observeDeliveryEvent(
	snapshot bridgepkg.DeliverySnapshot,
	seq int64,
	eventType bridgepkg.DeliveryEventType,
	text string,
	final bool,
) bridgepkg.DeliveryEvent {
	var errorDetail *bridgepkg.DeliveryErrorDetail
	if eventType == bridgepkg.DeliveryEventTypeError {
		errorDetail = &bridgepkg.DeliveryErrorDetail{Message: text}
	}
	return bridgepkg.DeliveryEvent{
		DeliveryID:       snapshot.DeliveryID,
		BridgeInstanceID: snapshot.BridgeInstanceID,
		RoutingKey:       snapshot.RoutingKey,
		DeliveryTarget:   snapshot.DeliveryTarget,
		Seq:              seq,
		EventType:        eventType,
		Content:          bridgepkg.MessageContent{Text: text},
		Final:            final,
		Error:            errorDetail,
	}
}

func observeBridgeHealthMap(t *testing.T, h *harness) map[string]BridgeInstanceHealth {
	t.Helper()

	rows, err := h.observer.QueryBridgeHealth(testutil.Context(t))
	if err != nil {
		t.Fatalf("QueryBridgeHealth() error = %v", err)
	}

	health := make(map[string]BridgeInstanceHealth, len(rows))
	for _, row := range rows {
		health[row.BridgeInstanceID] = row
	}
	return health
}

func waitForObserveCondition(t *testing.T, fn func() bool) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition did not become true before timeout")
}
