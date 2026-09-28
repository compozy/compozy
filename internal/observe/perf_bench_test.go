package observe

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"

	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
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
	benchmarkMetricsQuery = TaskMetricsQuery{
		ReadScope: store.ReadScope{AllProfiles: true},
		Since:     benchmarkObserveNow.Add(-24 * time.Hour),
	}
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

func buildBenchmarkTaskSnapshot(count int) taskSnapshot {
	tasks := make([]taskpkg.Summary, 0, count)
	runs := make([]taskpkg.Run, 0, count)
	events := make([]taskpkg.Event, 0, count*2)
	tasksByID := make(map[string]taskpkg.Summary, count)

	owners := []taskpkg.OwnerKind{
		taskpkg.OwnerKindHuman,
		taskpkg.OwnerKindAutomation,
		taskpkg.OwnerKindPool,
		taskpkg.OwnerKindHuman,
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
		taskpkg.OriginKindDaemon,
		taskpkg.OriginKindDaemon,
	}

	for i := range count {
		taskID := fmt.Sprintf("task-%04d", i)
		runID := fmt.Sprintf("run-%04d", i)
		origin := taskpkg.Origin{Kind: origins[i%len(origins)], Ref: fmt.Sprintf("origin-%d", i)}
		workspaceID := fmt.Sprintf("ws-%02d", i%32)
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

		run := taskpkg.Run{
			ID:        runID,
			TaskID:    taskID,
			Status:    runStatuses[i%len(runStatuses)],
			Attempt:   int32((i % 3) + 1),
			SessionID: fmt.Sprintf("sess-%04d", i%128),
			Origin:    origin,
			QueuedAt:  benchmarkObserveNow.Add(-time.Duration(i+10) * time.Minute),
			ClaimedAt: benchmarkObserveNow.Add(-time.Duration(i+8) * time.Minute),
			StartedAt: benchmarkObserveNow.Add(-time.Duration(i+6) * time.Minute),
			EndedAt:   benchmarkObserveNow.Add(-time.Duration(i+3) * time.Minute),
		}
		runs = append(runs, run)

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
	}

	return taskSnapshot{
		tasks:     tasks,
		runs:      runs,
		events:    events,
		tasksByID: tasksByID,
	}
}

func benchmarkRecoveryPayload(i int) json.RawMessage {
	action := []taskpkg.RunBootRecoveryAction{
		taskpkg.RunBootRecoveryRequeue,
		taskpkg.RunBootRecoveryMarkRunning,
		taskpkg.RunBootRecoveryFail,
	}[i%3]
	payload, err := json.Marshal(taskRecoveryPayload{Action: action})
	if err != nil {
		panic(fmt.Errorf("marshal benchmark recovery payload: %w", err))
	}
	return payload
}
