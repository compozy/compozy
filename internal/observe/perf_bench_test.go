package observe

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
)

var (
	benchmarkObserveNow = time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	benchmarkEvent      = acp.AgentEvent{
		Type:       acp.EventTypePermission,
		Title:      "permission.request",
		Text:       "assistant requested filesystem access",
		Resource:   "/workspaces/acme/projects/demo/README.md",
		Decision:   "allow",
		StopReason: "completed",
		ToolCallID: "tool-call-42",
	}
	benchmarkSnapshot     = buildBenchmarkTaskSnapshot(1024)
	benchmarkMetricsQuery = TaskMetricsQuery{Since: benchmarkObserveNow.Add(-24 * time.Hour)}
	benchmarkObserver     = buildBenchmarkBridgeObserver(256, 4)
)

func BenchmarkSummarizeEventPermission(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		_ = summarizeEvent(benchmarkEvent)
	}
}

func BenchmarkTaskSummaryFromSnapshotLarge(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		_ = taskSummaryFromSnapshot(benchmarkSnapshot, func() time.Time { return benchmarkObserveNow })
	}
}

func BenchmarkTaskMetricsFromSnapshotLarge(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		_ = taskMetricsFromSnapshot(
			benchmarkSnapshot,
			benchmarkMetricsQuery,
			func() time.Time { return benchmarkObserveNow },
		)
	}
}

func BenchmarkCollectBridgeHealthLarge(b *testing.B) {
	ctx := context.Background()
	b.ReportAllocs()

	for b.Loop() {
		if _, _, err := benchmarkObserver.collectBridgeHealth(ctx); err != nil {
			b.Fatalf("collectBridgeHealth() error = %v", err)
		}
	}
}

func buildBenchmarkTaskSnapshot(count int) taskSnapshot {
	tasks := make([]taskpkg.Summary, 0, count)
	runs := make([]taskpkg.Run, 0, count)
	events := make([]taskpkg.Event, 0, count*2)
	audits := make([]store.NetworkAuditEntry, 0, count*2)
	tasksByID := make(map[string]taskpkg.Summary, count)
	runsByID := make(map[string]taskpkg.Run, count)
	taskChannels := make(map[string]string, count)

	owners := []taskpkg.OwnerKind{
		taskpkg.OwnerKindHuman,
		taskpkg.OwnerKindAutomation,
		taskpkg.OwnerKindPool,
		taskpkg.OwnerKindNetworkPeer,
	}
	taskStatuses := []taskpkg.Status{
		taskpkg.TaskStatusReady,
		taskpkg.TaskStatusBlocked,
		taskpkg.TaskStatusInProgress,
		taskpkg.TaskStatusCompleted,
	}
	runStatuses := []taskpkg.RunStatus{
		taskpkg.TaskRunStatusQueued,
		taskpkg.TaskRunStatusClaimed,
		taskpkg.TaskRunStatusStarting,
		taskpkg.TaskRunStatusRunning,
		taskpkg.TaskRunStatusCompleted,
	}
	origins := []taskpkg.OriginKind{
		taskpkg.OriginKindCLI,
		taskpkg.OriginKindAutomation,
		taskpkg.OriginKindNetwork,
		taskpkg.OriginKindDaemon,
	}
	channels := []string{"", "ops", "eng", "audit"}

	for i := range count {
		taskID := fmt.Sprintf("task-%04d", i)
		runID := fmt.Sprintf("run-%04d", i)
		origin := taskpkg.Origin{Kind: origins[i%len(origins)], Ref: fmt.Sprintf("origin-%d", i)}
		channel := channels[i%len(channels)]
		workspaceID := fmt.Sprintf("ws-%02d", i%32)
		networkSpec := participation.LocalSpec()
		if channel != "" {
			networkSpec = participation.Spec{
				Version:         participation.SpecVersion,
				Mode:            participation.ModeLive,
				WorkspaceID:     workspaceID,
				ChannelStrategy: participation.StrategyNamed,
				ChannelID:       channel,
				Source:          participation.SourceExplicitRequest,
			}
		}
		task := taskpkg.Summary{
			ID:          taskID,
			Scope:       []taskpkg.Scope{taskpkg.ScopeGlobal, taskpkg.ScopeWorkspace}[i%2],
			WorkspaceID: workspaceID,
			Title:       fmt.Sprintf("Task %04d", i),
			Status:      taskStatuses[i%len(taskStatuses)],
			CreatedBy:   taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "user"},
			Origin:      origin,
			CreatedAt:   benchmarkObserveNow.Add(-time.Duration(i+1) * time.Minute),
			UpdatedAt:   benchmarkObserveNow.Add(-time.Duration(i) * time.Minute),
		}
		if kind := owners[i%len(owners)]; kind != taskpkg.OwnerKindPool {
			task.Owner = &taskpkg.Ownership{Kind: kind, Ref: fmt.Sprintf("owner-%d", i%64)}
		} else {
			task.Owner = &taskpkg.Ownership{Kind: taskpkg.OwnerKindPool, Ref: "backlog"}
		}
		tasks = append(tasks, task)
		tasksByID[taskID] = task
		taskChannels[taskID] = channel

		run := taskpkg.Run{
			ID:              runID,
			TaskID:          taskID,
			Status:          runStatuses[i%len(runStatuses)],
			Attempt:         int32((i % 3) + 1),
			SessionID:       fmt.Sprintf("sess-%04d", i%128),
			Origin:          origin,
			RunNetworkState: &taskpkg.RunNetworkState{NetworkSpec: networkSpec},
			QueuedAt:        benchmarkObserveNow.Add(-time.Duration(i+10) * time.Minute),
			ClaimedAt:       benchmarkObserveNow.Add(-time.Duration(i+8) * time.Minute),
			StartedAt:       benchmarkObserveNow.Add(-time.Duration(i+6) * time.Minute),
			EndedAt:         benchmarkObserveNow.Add(-time.Duration(i+3) * time.Minute),
		}
		runs = append(runs, run)
		runsByID[runID] = run

		events = append(events,
			taskpkg.Event{
				ID:        fmt.Sprintf("evt-enqueue-%04d", i),
				TaskID:    taskID,
				RunID:     runID,
				EventType: taskEventRunEnqueued,
				Actor:     taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "scheduler"},
				Origin:    origin,
				Timestamp: benchmarkObserveNow.Add(-time.Duration(i+5) * time.Minute),
			},
			taskpkg.Event{
				ID:        fmt.Sprintf("evt-recovery-%04d", i),
				TaskID:    taskID,
				RunID:     runID,
				EventType: []string{taskEventRunRecovered, taskEventCanceled, taskEventRunForceStopped}[i%3],
				Actor:     taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "scheduler"},
				Origin:    origin,
				Payload:   benchmarkRecoveryPayload(i),
				Timestamp: benchmarkObserveNow.Add(-time.Duration(i+2) * time.Minute),
			},
		)

		audits = append(audits,
			store.NetworkAuditEntry{
				ID:        fmt.Sprintf("audit-enqueue-%04d", i),
				SessionID: run.SessionID,
				Direction: "received",
				Kind:      taskIngressAuditEnqueueAction,
				Channel:   normalizedChannel(channel),
				PeerFrom:  fmt.Sprintf("peer-%02d", i%32),
				PeerTo:    "daemon",
				MessageID: fmt.Sprintf("msg-%04d", i),
				Size:      256 + i%16,
				Timestamp: benchmarkObserveNow.Add(-time.Duration(i+1) * time.Minute),
			},
			store.NetworkAuditEntry{
				ID:        fmt.Sprintf("audit-mismatch-%04d", i),
				SessionID: run.SessionID,
				Direction: "rejected",
				Kind:      taskIngressAuditEnqueueAction,
				Channel:   normalizedChannel(channel),
				PeerFrom:  fmt.Sprintf("peer-%02d", i%32),
				PeerTo:    "daemon",
				MessageID: fmt.Sprintf("reject-%04d", i),
				Reason:    taskIngressChannelMismatch,
				Size:      128 + i%16,
				Timestamp: benchmarkObserveNow.Add(-time.Duration(i) * time.Minute),
			},
		)
	}

	return taskSnapshot{
		tasks:        tasks,
		runs:         runs,
		events:       events,
		audits:       audits,
		tasksByID:    tasksByID,
		runsByID:     runsByID,
		taskChannels: taskChannels,
	}
}

func buildBenchmarkBridgeObserver(instances int, routesPerInstance int) *Observer {
	source := benchmarkBridgeSource{
		instances: make([]bridgepkg.BridgeInstance, 0, instances),
		routes:    make(map[string][]bridgepkg.BridgeRoute, instances),
		metrics:   make(map[string]bridgepkg.BridgeDeliveryMetrics, instances),
	}
	state := make(map[string]observedBridgeState, instances)

	for i := range instances {
		id := fmt.Sprintf("bridge-%03d", i)
		statuses := []bridgepkg.BridgeStatus{
			bridgepkg.BridgeStatusReady,
			bridgepkg.BridgeStatusStarting,
			bridgepkg.BridgeStatusDegraded,
			bridgepkg.BridgeStatusAuthRequired,
		}
		instance := bridgepkg.BridgeInstance{
			ID:        id,
			Enabled:   true,
			Status:    statuses[i%len(statuses)],
			CreatedAt: benchmarkObserveNow.Add(-time.Duration(i+10) * time.Minute),
			UpdatedAt: benchmarkObserveNow.Add(-time.Duration(i) * time.Minute),
		}
		source.instances = append(source.instances, instance)

		routes := make([]bridgepkg.BridgeRoute, 0, routesPerInstance)
		for j := range routesPerInstance {
			routes = append(routes, bridgepkg.BridgeRoute{
				BridgeInstanceID: id,
				SessionID:        fmt.Sprintf("sess-%03d-%02d", i, j),
				AgentName:        "coder",
				CreatedAt:        benchmarkObserveNow.Add(-time.Duration(j+1) * time.Minute),
				UpdatedAt:        benchmarkObserveNow.Add(-time.Duration(j) * time.Minute),
			})
		}
		source.routes[id] = routes
		source.metrics[id] = bridgepkg.BridgeDeliveryMetrics{
			BridgeInstanceID:        id,
			DeliveryBacklog:         i % 5,
			DeliveryDroppedTotal:    i % 7,
			DeliveryDroppedByReason: map[string]int{"timeout": i % 3},
			DeliveryFailuresTotal:   i % 4,
			LastError:               fmt.Sprintf("error-%d", i%5),
			LastErrorAt:             benchmarkObserveNow.Add(-time.Duration(i+2) * time.Minute),
			LastSuccessAt:           benchmarkObserveNow.Add(-time.Duration(i+1) * time.Minute),
		}
		state[id] = observedBridgeState{
			authFailuresTotal: i % 3,
			runtimeStatus:     []bridgepkg.BridgeStatus{"", bridgepkg.BridgeStatusDegraded, bridgepkg.BridgeStatusError}[i%3],
			runtimeMessage:    fmt.Sprintf("runtime-%d", i%5),
			runtimeUpdatedAt:  benchmarkObserveNow.Add(-time.Duration(i) * time.Minute),
		}
	}

	return &Observer{
		now:          func() time.Time { return benchmarkObserveNow },
		bridgeSource: source,
		bridgeState:  state,
	}
}

type benchmarkBridgeSource struct {
	instances []bridgepkg.BridgeInstance
	routes    map[string][]bridgepkg.BridgeRoute
	metrics   map[string]bridgepkg.BridgeDeliveryMetrics
}

func (s benchmarkBridgeSource) ListInstances(context.Context) ([]bridgepkg.BridgeInstance, error) {
	return s.instances, nil
}

func (s benchmarkBridgeSource) ListRoutes(_ context.Context, bridgeInstanceID string) ([]bridgepkg.BridgeRoute, error) {
	return s.routes[bridgeInstanceID], nil
}

func (s benchmarkBridgeSource) CountBridgeRoutes(
	_ context.Context,
	bridgeInstanceIDs []string,
) (map[string]int, error) {
	counts := make(map[string]int, len(bridgeInstanceIDs))
	for _, bridgeInstanceID := range bridgeInstanceIDs {
		counts[bridgeInstanceID] = len(s.routes[bridgeInstanceID])
	}
	return counts, nil
}

func (s benchmarkBridgeSource) DeliveryMetrics() map[string]bridgepkg.BridgeDeliveryMetrics {
	return s.metrics
}

func (s benchmarkBridgeSource) DeliveryMetricsFor(
	bridgeInstanceIDs []string,
) (map[string]bridgepkg.BridgeDeliveryMetrics, error) {
	metrics := make(map[string]bridgepkg.BridgeDeliveryMetrics, len(bridgeInstanceIDs))
	for _, id := range bridgeInstanceIDs {
		if item, ok := s.metrics[id]; ok {
			metrics[id] = item
		}
	}
	return metrics, nil
}

func benchmarkRecoveryPayload(i int) json.RawMessage {
	action := []taskpkg.RunBootRecoveryAction{
		taskpkg.RunBootRecoveryRequeue,
		taskpkg.RunBootRecoveryMarkRunning,
		taskpkg.RunBootRecoveryFail,
	}[i%3]
	payload, _ := json.Marshal(taskRecoveryPayload{Action: action})
	return payload
}

func normalizedChannel(channel string) string {
	if channel == "" {
		return "default"
	}
	return channel
}
