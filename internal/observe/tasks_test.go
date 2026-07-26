package observe

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
	aghworkspace "github.com/compozy/agh/internal/workspace"
)

func TestQueryTaskSummaryAggregatesByScopeOriginChannelAndOwner(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	createObserveTask(t, h, taskpkg.Task{
		ID:        "task-global-ready",
		Scope:     taskpkg.ScopeGlobal,
		Title:     "Global ready",
		Status:    taskpkg.TaskStatusReady,
		CreatedBy: taskActor(taskpkg.ActorKindHuman, "user-1"),
		Origin:    taskOrigin(taskpkg.OriginKindCLI, "agh task create"),
		CreatedAt: h.now.Add(time.Minute),
		UpdatedAt: h.now.Add(time.Minute),
	})
	createObserveTask(t, h, taskpkg.Task{
		ID:          "task-workspace-blocked",
		Scope:       taskpkg.ScopeWorkspace,
		WorkspaceID: h.workspaceID,
		Title:       "Workspace blocked",
		Status:      taskpkg.TaskStatusBlocked,
		Owner:       taskOwner(taskpkg.OwnerKindHuman, "alice"),
		CreatedBy:   taskActor(taskpkg.ActorKindNetworkPeer, "peer-ops"),
		Origin:      taskOrigin(taskpkg.OriginKindNetwork, "peer:peer-ops/channel:ops"),
		CreatedAt:   h.now.Add(2 * time.Minute),
		UpdatedAt:   h.now.Add(2 * time.Minute),
	})
	createObserveTask(t, h, taskpkg.Task{
		ID:          "task-workspace-completed",
		Scope:       taskpkg.ScopeWorkspace,
		WorkspaceID: h.workspaceID,
		Title:       "Workspace completed",
		Status:      taskpkg.TaskStatusCompleted,
		Owner:       taskOwner(taskpkg.OwnerKindPool, "backlog"),
		CreatedBy:   taskActor(taskpkg.ActorKindAutomation, "rule-1"),
		Origin:      taskOrigin(taskpkg.OriginKindAutomation, "run:rule-1"),
		CreatedAt:   h.now.Add(3 * time.Minute),
		UpdatedAt:   h.now.Add(3 * time.Minute),
		ClosedAt:    h.now.Add(4 * time.Minute),
	})

	createObserveRun(t, h, taskpkg.Run{
		ID:       "run-global-queued",
		TaskID:   "task-global-ready",
		Status:   taskpkg.TaskRunStatusQueued,
		Attempt:  1,
		Origin:   taskOrigin(taskpkg.OriginKindCLI, "agh task run"),
		QueuedAt: h.now.Add(10 * time.Minute),
	})
	createObserveRun(t, h, taskpkg.Run{
		ID:              "run-workspace-running",
		TaskID:          "task-workspace-blocked",
		Status:          taskpkg.TaskRunStatusRunning,
		Attempt:         1,
		ClaimedBy:       taskActorPtr(taskpkg.ActorKindDaemon, "scheduler"),
		SessionID:       "sess-ops-live",
		Origin:          taskOrigin(taskpkg.OriginKindNetwork, "peer:peer-ops/channel:ops"),
		RunNetworkState: observeRunNetworkState(h.workspaceID, "ops"),
		QueuedAt:        h.now.Add(11 * time.Minute),
		ClaimedAt:       h.now.Add(12 * time.Minute),
		StartedAt:       h.now.Add(13 * time.Minute),
	})
	createObserveRun(t, h, taskpkg.Run{
		ID:              "run-workspace-completed",
		TaskID:          "task-workspace-completed",
		Status:          taskpkg.TaskRunStatusCompleted,
		Attempt:         1,
		ClaimedBy:       taskActorPtr(taskpkg.ActorKindDaemon, "scheduler"),
		SessionID:       "sess-eng-done",
		Origin:          taskOrigin(taskpkg.OriginKindAutomation, "run:rule-1"),
		RunNetworkState: observeRunNetworkState(h.workspaceID, "eng"),
		QueuedAt:        h.now.Add(14 * time.Minute),
		ClaimedAt:       h.now.Add(15 * time.Minute),
		StartedAt:       h.now.Add(16 * time.Minute),
		EndedAt:         h.now.Add(18 * time.Minute),
	})

	summary, err := h.observer.QueryTaskSummary(testutil.Context(t), TaskSummaryQuery{})
	if err != nil {
		t.Fatalf("QueryTaskSummary() error = %v", err)
	}

	if got, want := summary.TotalTasks, 3; got != want {
		t.Fatalf("summary.TotalTasks = %d, want %d", got, want)
	}
	if got, want := summary.TotalRuns, 3; got != want {
		t.Fatalf("summary.TotalRuns = %d, want %d", got, want)
	}
	if !containsTaskTotal(summary.TaskTotals, taskpkg.ScopeGlobal, taskpkg.TaskStatusReady, "", 1) {
		t.Fatalf("summary.TaskTotals = %#v, want global/ready/unbound count 1", summary.TaskTotals)
	}
	if !containsTaskTotal(summary.TaskTotals, taskpkg.ScopeWorkspace, taskpkg.TaskStatusBlocked, "ops", 1) {
		t.Fatalf("summary.TaskTotals = %#v, want workspace/blocked/ops count 1", summary.TaskTotals)
	}
	if !containsTaskOriginTotal(summary.TaskOrigins, taskpkg.OriginKindNetwork, "ops", 1) {
		t.Fatalf("summary.TaskOrigins = %#v, want network/ops count 1", summary.TaskOrigins)
	}
	if !containsRunTotal(summary.RunTotals, taskpkg.TaskRunStatusCompleted, taskpkg.OriginKindAutomation, "eng", 1) {
		t.Fatalf("summary.RunTotals = %#v, want completed/automation/eng count 1", summary.RunTotals)
	}
	if !containsOwnerTotal(summary.OwnerTotals, taskpkg.OwnerKindHuman, "alice", 1) {
		t.Fatalf("summary.OwnerTotals = %#v, want human/alice count 1", summary.OwnerTotals)
	}
	if !containsQueueDepth(summary.QueueDepth, "", 1) {
		t.Fatalf("summary.QueueDepth = %#v, want unbound queue depth 1", summary.QueueDepth)
	}

	filtered, err := h.observer.QueryTaskSummary(testutil.Context(t), TaskSummaryQuery{
		OwnerKind:            taskpkg.OwnerKindHuman,
		OwnerRef:             "alice",
		ParticipationChannel: "ops",
	})
	if err != nil {
		t.Fatalf("QueryTaskSummary(filtered) error = %v", err)
	}
	if got, want := filtered.TotalTasks, 1; got != want {
		t.Fatalf("filtered.TotalTasks = %d, want %d", got, want)
	}
	if got, want := filtered.TotalRuns, 1; got != want {
		t.Fatalf("filtered.TotalRuns = %d, want %d", got, want)
	}
	if !containsRunTotal(filtered.RunTotals, taskpkg.TaskRunStatusRunning, taskpkg.OriginKindNetwork, "ops", 1) {
		t.Fatalf("filtered.RunTotals = %#v, want running/network/ops count 1", filtered.RunTotals)
	}
}

func TestTaskHealthFlagsStuckRunsByConfiguredThresholds(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.observer.taskHealthConfig = TaskHealthConfig{
		ClaimedStuckAfter:  5 * time.Minute,
		StartingStuckAfter: 10 * time.Minute,
		RunningStuckAfter:  15 * time.Minute,
	}

	liveStartedAt := h.observer.now().Add(-5 * time.Minute)
	h.source.sessions = []*session.Info{
		{
			ID:           "sess-live-running",
			Name:         "LIVE",
			AgentName:    "coder",
			WorkspaceID:  h.workspaceID,
			Workspace:    h.workspace,
			State:        session.StateActive,
			ACPSessionID: "acp-live-running",
			CreatedAt:    liveStartedAt,
			UpdatedAt:    liveStartedAt,
		},
		{
			ID:           "sess-live-starting",
			Name:         "LIVE2",
			AgentName:    "coder",
			WorkspaceID:  h.workspaceID,
			Workspace:    h.workspace,
			State:        session.StateActive,
			ACPSessionID: "acp-live-starting",
			CreatedAt:    liveStartedAt,
			UpdatedAt:    liveStartedAt,
		},
	}

	taskIDs := []string{"task-claimed", "task-starting-recent", "task-starting-stale", "task-running-stale"}
	for _, id := range taskIDs {
		createObserveTask(t, h, taskpkg.Task{
			ID:          id,
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: h.workspaceID,
			Title:       id,
			Status:      taskpkg.TaskStatusInProgress,
			CreatedBy:   taskActor(taskpkg.ActorKindHuman, "user"),
			Origin:      taskOrigin(taskpkg.OriginKindCLI, "agh task"),
			CreatedAt:   h.now,
			UpdatedAt:   h.now,
		})
	}

	now := h.observer.now()
	createObserveRun(t, h, taskpkg.Run{
		ID:        "run-claimed-stale",
		TaskID:    "task-claimed",
		Status:    taskpkg.TaskRunStatusClaimed,
		Attempt:   1,
		Origin:    taskOrigin(taskpkg.OriginKindCLI, "agh task"),
		QueuedAt:  now.Add(-40 * time.Minute),
		ClaimedAt: now.Add(-20 * time.Minute),
	})
	createObserveRun(t, h, taskpkg.Run{
		ID:              "run-starting-fresh",
		TaskID:          "task-starting-recent",
		Status:          taskpkg.TaskRunStatusStarting,
		Attempt:         1,
		SessionID:       "sess-live-starting",
		Origin:          taskOrigin(taskpkg.OriginKindCLI, "agh task"),
		QueuedAt:        now.Add(-15 * time.Minute),
		ClaimedAt:       now.Add(-4 * time.Minute),
		RunNetworkState: observeRunNetworkState(h.workspaceID, "ops"),
	})
	createObserveRun(t, h, taskpkg.Run{
		ID:              "run-starting-stale",
		TaskID:          "task-starting-stale",
		Status:          taskpkg.TaskRunStatusStarting,
		Attempt:         1,
		SessionID:       "sess-live-starting",
		Origin:          taskOrigin(taskpkg.OriginKindNetwork, "peer:peer-1/channel:ops"),
		QueuedAt:        now.Add(-25 * time.Minute),
		ClaimedAt:       now.Add(-12 * time.Minute),
		RunNetworkState: observeRunNetworkState(h.workspaceID, "ops"),
	})
	createObserveRun(t, h, taskpkg.Run{
		ID:              "run-running-stale",
		TaskID:          "task-running-stale",
		Status:          taskpkg.TaskRunStatusRunning,
		Attempt:         1,
		SessionID:       "sess-live-running",
		Origin:          taskOrigin(taskpkg.OriginKindAutomation, "run:auto-1"),
		QueuedAt:        now.Add(-30 * time.Minute),
		ClaimedAt:       now.Add(-28 * time.Minute),
		StartedAt:       now.Add(-20 * time.Minute),
		RunNetworkState: observeRunNetworkState(h.workspaceID, "eng"),
	})

	health, err := h.observer.Health(testutil.Context(t))
	if err != nil {
		t.Fatalf("Health() error = %v", err)
	}

	if got, want := len(health.Tasks.StuckRuns), 3; got != want {
		t.Fatalf("len(health.Tasks.StuckRuns) = %d, want %d", got, want)
	}
	if !containsStuckRun(health.Tasks.StuckRuns, "run-claimed-stale", taskpkg.TaskRunStatusClaimed) {
		t.Fatalf("health.Tasks.StuckRuns = %#v, want claimed stale run", health.Tasks.StuckRuns)
	}
	if !containsStuckRun(health.Tasks.StuckRuns, "run-starting-stale", taskpkg.TaskRunStatusStarting) {
		t.Fatalf("health.Tasks.StuckRuns = %#v, want starting stale run", health.Tasks.StuckRuns)
	}
	if !containsStuckRun(health.Tasks.StuckRuns, "run-running-stale", taskpkg.TaskRunStatusRunning) {
		t.Fatalf("health.Tasks.StuckRuns = %#v, want running stale run", health.Tasks.StuckRuns)
	}
	if containsStuckRun(health.Tasks.StuckRuns, "run-starting-fresh", taskpkg.TaskRunStatusStarting) {
		t.Fatalf("health.Tasks.StuckRuns = %#v, fresh starting run should not be flagged", health.Tasks.StuckRuns)
	}
	if got, want := health.Tasks.ActiveOrphanRuns, 0; got != want {
		t.Fatalf("health.Tasks.ActiveOrphanRuns = %d, want %d", got, want)
	}
	if got, want := health.Tasks.Status, "warn"; got != want {
		t.Fatalf("health.Tasks.Status = %q, want %q", got, want)
	}
}

func TestQueryTaskMetricsCountsDuplicateIngressAndChannelMismatch(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	createObserveTask(t, h, taskpkg.Task{
		ID:          "task-net",
		Scope:       taskpkg.ScopeWorkspace,
		WorkspaceID: h.workspaceID,
		Title:       "Network task",
		Status:      taskpkg.TaskStatusReady,
		CreatedBy:   taskActor(taskpkg.ActorKindNetworkPeer, "peer-ops"),
		Origin:      taskOrigin(taskpkg.OriginKindNetwork, "peer:peer-ops/channel:ops"),
		CreatedAt:   h.now,
		UpdatedAt:   h.now,
	})
	createObserveRun(t, h, taskpkg.Run{
		ID:              "run-net",
		TaskID:          "task-net",
		Status:          taskpkg.TaskRunStatusQueued,
		Attempt:         1,
		Origin:          taskOrigin(taskpkg.OriginKindNetwork, "peer:peer-ops/channel:ops"),
		RunNetworkState: observeRunNetworkState(h.workspaceID, "ops"),
		IdempotencyKey:  "idem-1",
		QueuedAt:        h.now.Add(time.Minute),
	})
	createObserveRun(t, h, taskpkg.Run{
		ID:              "run-net-current-eng",
		TaskID:          "task-net",
		Status:          taskpkg.TaskRunStatusQueued,
		Attempt:         2,
		Origin:          taskOrigin(taskpkg.OriginKindCLI, "agh task run"),
		RunNetworkState: observeRunNetworkState(h.workspaceID, "eng"),
		IdempotencyKey:  "idem-2",
		QueuedAt:        h.now.Add(6 * time.Minute),
	})
	createObserveEvent(t, h, taskpkg.Event{
		ID:        "evt-run-enqueued",
		TaskID:    "task-net",
		RunID:     "run-net",
		EventType: taskEventRunEnqueued,
		Actor:     taskActor(taskpkg.ActorKindNetworkPeer, "peer-ops"),
		Origin:    taskOrigin(taskpkg.OriginKindNetwork, "peer:peer-ops/channel:ops"),
		Timestamp: h.now.Add(2 * time.Minute),
		Payload:   mustJSON(t, map[string]any{"idempotency_key": "idem-1"}),
	})
	createObserveEvent(t, h, taskpkg.Event{
		ID:        "evt-run-force-stopped",
		TaskID:    "task-net",
		RunID:     "run-net",
		EventType: taskEventRunForceStopped,
		Actor:     taskActor(taskpkg.ActorKindDaemon, "scheduler"),
		Origin:    taskOrigin(taskpkg.OriginKindNetwork, "peer:peer-ops/channel:ops"),
		Timestamp: h.now.Add(2 * time.Minute),
		Payload:   mustJSON(t, map[string]any{"reason": "test"}),
	})
	createObserveNetworkChannel(t, h, "ops")
	createObserveAudit(t, h, store.NetworkAuditEntry{
		ID:          "naud-accepted-1",
		SessionID:   "netpeer:peer-ops",
		Direction:   "received",
		Kind:        taskIngressAuditEnqueueAction,
		WorkspaceID: h.workspaceID,
		Channel:     "ops",
		PeerFrom:    "peer-ops",
		MessageID:   "req-1",
		Size:        32,
		Timestamp:   h.now.Add(2 * time.Minute),
	})
	createObserveAudit(t, h, store.NetworkAuditEntry{
		ID:          "naud-accepted-2",
		SessionID:   "netpeer:peer-ops",
		Direction:   "received",
		Kind:        taskIngressAuditEnqueueAction,
		WorkspaceID: h.workspaceID,
		Channel:     "ops",
		PeerFrom:    "peer-ops",
		MessageID:   "req-2",
		Size:        32,
		Timestamp:   h.now.Add(3 * time.Minute),
	})
	createObserveAudit(t, h, store.NetworkAuditEntry{
		ID:          "naud-rejected-mismatch",
		SessionID:   "netpeer:peer-ops",
		Direction:   "rejected",
		Kind:        taskIngressAuditEnqueueAction,
		WorkspaceID: h.workspaceID,
		Channel:     "ops",
		PeerFrom:    "peer-ops",
		MessageID:   "req-3",
		Reason:      taskIngressChannelMismatch,
		Size:        32,
		Timestamp:   h.now.Add(4 * time.Minute),
	})
	createObserveAudit(t, h, store.NetworkAuditEntry{
		ID:          "naud-rejected-stale",
		SessionID:   "netpeer:peer-ops",
		Direction:   "rejected",
		Kind:        taskIngressAuditEnqueueAction,
		WorkspaceID: h.workspaceID,
		Channel:     "ops",
		PeerFrom:    "peer-ops",
		MessageID:   "req-4",
		Reason:      "stale_channel",
		Size:        32,
		Timestamp:   h.now.Add(5 * time.Minute),
	})

	metrics, err := h.observer.QueryTaskMetrics(testutil.Context(t), TaskMetricsQuery{
		Since:                h.now,
		ParticipationChannel: "ops",
	})
	if err != nil {
		t.Fatalf("QueryTaskMetrics() error = %v", err)
	}

	if got, want := metrics.DuplicateIngressTotal, 1; got != want {
		t.Fatalf("metrics.DuplicateIngressTotal = %d, want %d", got, want)
	}
	if got, want := metrics.ChannelMismatchTotal, 1; got != want {
		t.Fatalf("metrics.ChannelMismatchTotal = %d, want %d", got, want)
	}
	if !containsRunTotal(metrics.TaskRunsTotal, taskpkg.TaskRunStatusQueued, taskpkg.OriginKindNetwork, "ops", 1) {
		t.Fatalf("metrics.TaskRunsTotal = %#v, want queued/network/ops count 1", metrics.TaskRunsTotal)
	}
	if !containsQueueDepth(metrics.TaskQueueDepth, "ops", 1) {
		t.Fatalf("metrics.TaskQueueDepth = %#v, want ops queue depth 1", metrics.TaskQueueDepth)
	}

	engMetrics, err := h.observer.QueryTaskMetrics(testutil.Context(t), TaskMetricsQuery{
		Since:                h.now,
		ParticipationChannel: "eng",
	})
	if err != nil {
		t.Fatalf("QueryTaskMetrics(eng filter) error = %v", err)
	}
	if got := engMetrics.TaskForcedStopsTotal; got != 0 {
		t.Fatalf("engMetrics.TaskForcedStopsTotal = %d, want 0 from the ops run snapshot", got)
	}

	cliMetrics, err := h.observer.QueryTaskMetrics(testutil.Context(t), TaskMetricsQuery{
		Since:                h.now,
		ParticipationChannel: "ops",
		OriginKind:           taskpkg.OriginKindCLI,
	})
	if err != nil {
		t.Fatalf("QueryTaskMetrics(cli filter) error = %v", err)
	}
	if got := cliMetrics.DuplicateIngressTotal; got != 0 {
		t.Fatalf("cliMetrics.DuplicateIngressTotal = %d, want 0", got)
	}
	if got := cliMetrics.ChannelMismatchTotal; got != 0 {
		t.Fatalf("cliMetrics.ChannelMismatchTotal = %d, want 0", got)
	}
}

func TestQueryTaskDashboardAggregatesCardsAndBreakdown(t *testing.T) {
	t.Parallel()

	t.Run("Should aggregate dashboard cards queue health and breakdown from persisted task state", func(t *testing.T) {
		h := newHarness(t)
		h.observer.taskDashboardConfig.backlogWarnAfter = 30 * time.Minute
		h.observer.taskHealthConfig = TaskHealthConfig{
			ClaimedStuckAfter:  20 * time.Minute,
			StartingStuckAfter: 20 * time.Minute,
			RunningStuckAfter:  20 * time.Minute,
		}
		now := h.observer.now()
		h.source.sessions = []*session.Info{
			{
				ID:           "sess-live-running",
				Name:         "LIVE",
				AgentName:    "coder",
				WorkspaceID:  h.workspaceID,
				Workspace:    h.workspace,
				State:        session.StateActive,
				ACPSessionID: "acp-live-running",
				CreatedAt:    now.Add(-10 * time.Minute),
				UpdatedAt:    now.Add(-time.Minute),
			},
		}
		createObserveTask(t, h, taskpkg.Task{
			ID:        "task-ready",
			Scope:     taskpkg.ScopeGlobal,
			Title:     "Queued review",
			Priority:  taskpkg.PriorityUrgent,
			Status:    taskpkg.TaskStatusReady,
			CreatedBy: taskActor(taskpkg.ActorKindHuman, "user-1"),
			Origin:    taskOrigin(taskpkg.OriginKindCLI, "agh task"),
			CreatedAt: now.Add(-15 * time.Minute),
			UpdatedAt: now.Add(-10 * time.Minute),
		})
		createObserveTask(t, h, taskpkg.Task{
			ID:             "task-blocked-approval",
			Scope:          taskpkg.ScopeWorkspace,
			WorkspaceID:    h.workspaceID,
			Title:          "Approval gate",
			Status:         taskpkg.TaskStatusBlocked,
			ApprovalPolicy: taskpkg.ApprovalPolicyManual,
			ApprovalState:  taskpkg.ApprovalStatePending,
			CreatedBy:      taskActor(taskpkg.ActorKindHuman, "user-1"),
			Origin:         taskOrigin(taskpkg.OriginKindCLI, "agh task"),
			CreatedAt:      now.Add(-14 * time.Minute),
			UpdatedAt:      now.Add(-9 * time.Minute),
		})
		createObserveTask(t, h, taskpkg.Task{
			ID:             "task-blocked-deps",
			Scope:          taskpkg.ScopeWorkspace,
			WorkspaceID:    h.workspaceID,
			Title:          "Dependency gate",
			Status:         taskpkg.TaskStatusBlocked,
			ApprovalPolicy: taskpkg.ApprovalPolicyNone,
			ApprovalState:  taskpkg.ApprovalStateNotRequired,
			CreatedBy:      taskActor(taskpkg.ActorKindHuman, "user-1"),
			Origin:         taskOrigin(taskpkg.OriginKindCLI, "agh task"),
			CreatedAt:      now.Add(-13 * time.Minute),
			UpdatedAt:      now.Add(-8 * time.Minute),
		})
		createObserveDependency(t, h, taskpkg.Dependency{
			TaskID:          "task-blocked-approval",
			DependsOnTaskID: "task-ready",
			Kind:            taskpkg.DependencyKindBlocks,
			CreatedAt:       now.Add(-8 * time.Minute),
		})
		createObserveDependency(t, h, taskpkg.Dependency{
			TaskID:          "task-blocked-deps",
			DependsOnTaskID: "task-ready",
			Kind:            taskpkg.DependencyKindBlocks,
			CreatedAt:       now.Add(-7 * time.Minute),
		})
		createObserveTask(t, h, taskpkg.Task{
			ID:          "task-running",
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: h.workspaceID,
			Title:       "Live execution",
			Priority:    taskpkg.PriorityHigh,
			Status:      taskpkg.TaskStatusInProgress,
			Owner:       taskOwner(taskpkg.OwnerKindHuman, "alice"),
			CreatedBy:   taskActor(taskpkg.ActorKindHuman, "user-1"),
			Origin:      taskOrigin(taskpkg.OriginKindCLI, "agh task"),
			CreatedAt:   now.Add(-12 * time.Minute),
			UpdatedAt:   now.Add(-7 * time.Minute),
		})
		createObserveTask(t, h, taskpkg.Task{
			ID:          "task-failed",
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: h.workspaceID,
			Title:       "Failed job",
			Status:      taskpkg.TaskStatusFailed,
			CreatedBy:   taskActor(taskpkg.ActorKindAutomation, "rule-1"),
			Origin:      taskOrigin(taskpkg.OriginKindAutomation, "run:rule-1"),
			CreatedAt:   now.Add(-11 * time.Minute),
			UpdatedAt:   now.Add(-6 * time.Minute),
			ClosedAt:    now.Add(-2 * time.Minute),
		})
		createObserveTask(t, h, taskpkg.Task{
			ID:          "task-completed",
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: h.workspaceID,
			Title:       "Completed job",
			Status:      taskpkg.TaskStatusCompleted,
			CreatedBy:   taskActor(taskpkg.ActorKindAutomation, "rule-2"),
			Origin:      taskOrigin(taskpkg.OriginKindAutomation, "run:rule-2"),
			CreatedAt:   now.Add(-10 * time.Minute),
			UpdatedAt:   now.Add(-5 * time.Minute),
			ClosedAt:    now.Add(-time.Minute),
		})

		createObserveRun(t, h, taskpkg.Run{
			ID:       "run-queued",
			TaskID:   "task-ready",
			Status:   taskpkg.TaskRunStatusQueued,
			Attempt:  1,
			Origin:   taskOrigin(taskpkg.OriginKindCLI, "agh task"),
			QueuedAt: now.Add(-3 * time.Minute),
		})
		createObserveRun(t, h, taskpkg.Run{
			ID:              "run-running",
			TaskID:          "task-running",
			Status:          taskpkg.TaskRunStatusRunning,
			Attempt:         2,
			SessionID:       "sess-live-running",
			Origin:          taskOrigin(taskpkg.OriginKindCLI, "agh task"),
			RunNetworkState: observeRunNetworkState(h.workspaceID, "ops"),
			QueuedAt:        now.Add(-8 * time.Minute),
			ClaimedAt:       now.Add(-7 * time.Minute),
			StartedAt:       now.Add(-5 * time.Minute),
		})
		createObserveRun(t, h, taskpkg.Run{
			ID:              "run-failed",
			TaskID:          "task-failed",
			Status:          taskpkg.TaskRunStatusFailed,
			Attempt:         1,
			Origin:          taskOrigin(taskpkg.OriginKindAutomation, "run:rule-1"),
			RunNetworkState: observeRunNetworkState(h.workspaceID, "ops"),
			QueuedAt:        now.Add(-9 * time.Minute),
			ClaimedAt:       now.Add(-8 * time.Minute),
			StartedAt:       now.Add(-6 * time.Minute),
			EndedAt:         now.Add(-2 * time.Minute),
			Error:           "rate limit",
		})
		createObserveRun(t, h, taskpkg.Run{
			ID:              "run-completed",
			TaskID:          "task-completed",
			Status:          taskpkg.TaskRunStatusCompleted,
			Attempt:         1,
			Origin:          taskOrigin(taskpkg.OriginKindAutomation, "run:rule-2"),
			RunNetworkState: observeRunNetworkState(h.workspaceID, "eng"),
			QueuedAt:        now.Add(-8 * time.Minute),
			ClaimedAt:       now.Add(-6 * time.Minute),
			StartedAt:       now.Add(-4 * time.Minute),
			EndedAt:         now.Add(-time.Minute),
		})

		dashboard, err := h.observer.QueryTaskDashboard(testutil.Context(t), TaskDashboardQuery{})
		if err != nil {
			t.Fatalf("QueryTaskDashboard() error = %v", err)
		}

		if got, want := dashboard.Totals.TasksTotal, 6; got != want {
			t.Fatalf("dashboard.Totals.TasksTotal = %d, want %d", got, want)
		}
		if got, want := dashboard.Totals.RunsTotal, 4; got != want {
			t.Fatalf("dashboard.Totals.RunsTotal = %d, want %d", got, want)
		}
		if got, want := dashboard.Totals.BlockedTasks, 2; got != want {
			t.Fatalf("dashboard.Totals.BlockedTasks = %d, want %d", got, want)
		}
		if got, want := dashboard.Totals.AwaitingApprovalTasks, 1; got != want {
			t.Fatalf("dashboard.Totals.AwaitingApprovalTasks = %d, want %d", got, want)
		}
		if got, want := dashboard.Totals.DependencyBlockedTasks, 2; got != want {
			t.Fatalf("dashboard.Totals.DependencyBlockedTasks = %d, want %d", got, want)
		}
		if got, want := dashboard.Totals.ActiveRuns, 2; got != want {
			t.Fatalf("dashboard.Totals.ActiveRuns = %d, want %d", got, want)
		}
		if got, want := dashboard.Cards.InProgress.Tasks, 1; got != want {
			t.Fatalf("dashboard.Cards.InProgress.Tasks = %d, want %d", got, want)
		}
		if got, want := dashboard.Cards.Blocked.AwaitingDependencies, 2; got != want {
			t.Fatalf("dashboard.Cards.Blocked.AwaitingDependencies = %d, want %d", got, want)
		}
		if got, want := dashboard.Cards.Failed.FailedRuns, 1; got != want {
			t.Fatalf("dashboard.Cards.Failed.FailedRuns = %d, want %d", got, want)
		}
		if got, want := dashboard.Cards.Latency.ClaimLatencyMillis.Samples, 3; got != want {
			t.Fatalf("dashboard.Cards.Latency.ClaimLatencyMillis.Samples = %d, want %d", got, want)
		}
		if got, want := dashboard.Cards.Latency.ClaimLatencyMillis.AverageMillis,
			(time.Minute+2*time.Minute+time.Minute).Milliseconds()/3; got != want {
			t.Fatalf("dashboard.Cards.Latency.ClaimLatencyMillis.AverageMillis = %d, want %d", got, want)
		}
		if got, want := dashboard.Queue.Total, 1; got != want {
			t.Fatalf("dashboard.Queue.Total = %d, want %d", got, want)
		}
		if dashboard.Queue.BacklogWarning {
			t.Fatalf("dashboard.Queue.BacklogWarning = true, want false")
		}
		if got, want := dashboard.Health.Status, "ok"; got != want {
			t.Fatalf("dashboard.Health.Status = %q, want %q", got, want)
		}
		if got := activeRunIDs(
			dashboard.ActiveRuns.Items,
		); len(got) != 2 || got[0] != "run-running" ||
			got[1] != "run-queued" {
			t.Fatalf("dashboard.ActiveRuns.Items ids = %#v, want [run-running run-queued]", got)
		}
		if got, want := dashboard.ActiveRuns.Items[0].TaskOwner.Ref, "alice"; got != want {
			t.Fatalf("dashboard.ActiveRuns.Items[0].TaskOwner.Ref = %q, want %q", got, want)
		}
		if !containsDashboardBreakdown(dashboard.StatusBreakdown, taskpkg.TaskStatusCompleted, 1, 17) {
			t.Fatalf("dashboard.StatusBreakdown = %#v, want completed breakdown", dashboard.StatusBreakdown)
		}
		if got, want := dashboard.Freshness.Status, "current"; got != want {
			t.Fatalf("dashboard.Freshness.Status = %q, want %q", got, want)
		}
	})
}

func TestQueryTaskDashboardFlagsBacklogAndStaleSnapshots(t *testing.T) {
	t.Parallel()

	t.Run("Should flag stale live backlog snapshots", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		h.observer.taskDashboardConfig.backlogWarnAfter = 5 * time.Minute
		h.observer.taskDashboardConfig.staleAfter = 10 * time.Minute
		h.observer.taskHealthConfig = TaskHealthConfig{
			ClaimedStuckAfter:  30 * time.Minute,
			StartingStuckAfter: 30 * time.Minute,
			RunningStuckAfter:  30 * time.Minute,
		}

		now := h.observer.now()
		createObserveTask(t, h, taskpkg.Task{
			ID:          "task-backlog",
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: h.workspaceID,
			Title:       "Backlogged work",
			Status:      taskpkg.TaskStatusReady,
			CreatedBy:   taskActor(taskpkg.ActorKindHuman, "user"),
			Origin:      taskOrigin(taskpkg.OriginKindCLI, "agh task"),
			CreatedAt:   now.Add(-25 * time.Minute),
			UpdatedAt:   now.Add(-25 * time.Minute),
		})
		createObserveRun(t, h, taskpkg.Run{
			ID:       "run-backlog",
			TaskID:   "task-backlog",
			Status:   taskpkg.TaskRunStatusQueued,
			Attempt:  1,
			Origin:   taskOrigin(taskpkg.OriginKindCLI, "agh task"),
			QueuedAt: now.Add(-20 * time.Minute),
		})

		dashboard, err := h.observer.QueryTaskDashboard(testutil.Context(t), TaskDashboardQuery{})
		if err != nil {
			t.Fatalf("QueryTaskDashboard() error = %v", err)
		}

		if !dashboard.Queue.BacklogWarning {
			t.Fatal("dashboard.Queue.BacklogWarning = false, want true")
		}
		if got, want := dashboard.Queue.BacklogStatus, "warn"; got != want {
			t.Fatalf("dashboard.Queue.BacklogStatus = %q, want %q", got, want)
		}
		if got, want := dashboard.Freshness.Status, "stale"; got != want {
			t.Fatalf("dashboard.Freshness.Status = %q, want %q", got, want)
		}
		if !dashboard.Freshness.Stale {
			t.Fatal("dashboard.Freshness.Stale = false, want true")
		}
		if got, want := dashboard.Health.Status, "warn"; got != want {
			t.Fatalf("dashboard.Health.Status = %q, want %q", got, want)
		}
	})

	t.Run("Should return empty freshness for an empty snapshot", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)

		dashboard, err := h.observer.QueryTaskDashboard(testutil.Context(t), TaskDashboardQuery{})
		if err != nil {
			t.Fatalf("QueryTaskDashboard() error = %v", err)
		}

		if got, want := dashboard.Totals.TasksTotal, 0; got != want {
			t.Fatalf("dashboard.Totals.TasksTotal = %d, want %d", got, want)
		}
		if got, want := dashboard.Queue.Total, 0; got != want {
			t.Fatalf("dashboard.Queue.Total = %d, want %d", got, want)
		}
		if got, want := dashboard.Freshness.Status, "empty"; got != want {
			t.Fatalf("dashboard.Freshness.Status = %q, want %q", got, want)
		}
		if dashboard.Freshness.Stale {
			t.Fatal("dashboard.Freshness.Stale = true, want false")
		}
		if len(dashboard.ActiveRuns.Items) != 0 {
			t.Fatalf("len(dashboard.ActiveRuns.Items) = %d, want 0", len(dashboard.ActiveRuns.Items))
		}
	})
}

func TestQueryTaskDashboardSelectsRecentActiveRunsAndFiltersWorkspaces(t *testing.T) {
	t.Parallel()

	t.Run("Should select recent active runs and filter workspace scoped snapshots", func(t *testing.T) {
		h := newHarness(t)
		h.observer.taskDashboardConfig.activeRunLimit = 4
		h.observer.taskHealthConfig = TaskHealthConfig{
			ClaimedStuckAfter:  5 * time.Minute,
			StartingStuckAfter: 5 * time.Minute,
			RunningStuckAfter:  5 * time.Minute,
		}

		otherWorkspaceID := "ws-observe-other"
		otherWorkspaceRoot := filepath.Join(t.TempDir(), "other-workspace")
		if err := os.MkdirAll(otherWorkspaceRoot, 0o755); err != nil {
			t.Fatalf("MkdirAll(other workspace) error = %v", err)
		}
		if err := h.registry.InsertWorkspace(testutil.Context(t), aghworkspace.Workspace{
			ID:        otherWorkspaceID,
			RootDir:   otherWorkspaceRoot,
			Name:      "observe-other",
			CreatedAt: h.now,
			UpdatedAt: h.now,
		}); err != nil {
			t.Fatalf("InsertWorkspace(other) error = %v", err)
		}

		now := h.observer.now()
		makeTask := func(id string, workspaceID string, title string) {
			createObserveTask(t, h, taskpkg.Task{
				ID:          id,
				Scope:       taskpkg.ScopeWorkspace,
				WorkspaceID: workspaceID,
				Title:       title,
				Status:      taskpkg.TaskStatusInProgress,
				Owner:       taskOwner(taskpkg.OwnerKindHuman, "alice"),
				CreatedBy:   taskActor(taskpkg.ActorKindHuman, "user"),
				Origin:      taskOrigin(taskpkg.OriginKindCLI, "agh task"),
				CreatedAt:   now.Add(-30 * time.Minute),
				UpdatedAt:   now.Add(-30 * time.Minute),
			})
		}

		makeTask("task-running-recent", h.workspaceID, "Running recent")
		makeTask("task-running-stale", h.workspaceID, "Running stale")
		makeTask("task-starting", h.workspaceID, "Starting")
		makeTask("task-claimed", h.workspaceID, "Claimed")
		makeTask("task-queued", h.workspaceID, "Queued")
		makeTask("task-other-workspace", otherWorkspaceID, "Other workspace")

		createObserveRun(t, h, taskpkg.Run{
			ID:        "run-running-recent",
			TaskID:    "task-running-recent",
			Status:    taskpkg.TaskRunStatusRunning,
			Attempt:   1,
			Origin:    taskOrigin(taskpkg.OriginKindCLI, "agh task"),
			QueuedAt:  now.Add(-6 * time.Minute),
			ClaimedAt: now.Add(-5 * time.Minute),
			StartedAt: now.Add(-time.Minute),
		})
		createObserveRun(t, h, taskpkg.Run{
			ID:        "run-running-stale",
			TaskID:    "task-running-stale",
			Status:    taskpkg.TaskRunStatusRunning,
			Attempt:   1,
			Origin:    taskOrigin(taskpkg.OriginKindCLI, "agh task"),
			QueuedAt:  now.Add(-25 * time.Minute),
			ClaimedAt: now.Add(-22 * time.Minute),
			StartedAt: now.Add(-20 * time.Minute),
		})
		createObserveRun(t, h, taskpkg.Run{
			ID:        "run-starting",
			TaskID:    "task-starting",
			Status:    taskpkg.TaskRunStatusStarting,
			Attempt:   1,
			Origin:    taskOrigin(taskpkg.OriginKindCLI, "agh task"),
			QueuedAt:  now.Add(-8 * time.Minute),
			ClaimedAt: now.Add(-2 * time.Minute),
		})
		createObserveRun(t, h, taskpkg.Run{
			ID:        "run-claimed",
			TaskID:    "task-claimed",
			Status:    taskpkg.TaskRunStatusClaimed,
			Attempt:   1,
			Origin:    taskOrigin(taskpkg.OriginKindCLI, "agh task"),
			QueuedAt:  now.Add(-9 * time.Minute),
			ClaimedAt: now.Add(-3 * time.Minute),
		})
		createObserveRun(t, h, taskpkg.Run{
			ID:       "run-queued",
			TaskID:   "task-queued",
			Status:   taskpkg.TaskRunStatusQueued,
			Attempt:  1,
			Origin:   taskOrigin(taskpkg.OriginKindCLI, "agh task"),
			QueuedAt: now.Add(-4 * time.Minute),
		})
		createObserveRun(t, h, taskpkg.Run{
			ID:       "run-other-workspace",
			TaskID:   "task-other-workspace",
			Status:   taskpkg.TaskRunStatusQueued,
			Attempt:  1,
			Origin:   taskOrigin(taskpkg.OriginKindCLI, "agh task"),
			QueuedAt: now.Add(-2 * time.Minute),
		})

		dashboard, err := h.observer.QueryTaskDashboard(testutil.Context(t), TaskDashboardQuery{
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: h.workspaceID,
		})
		if err != nil {
			t.Fatalf("QueryTaskDashboard(filtered) error = %v", err)
		}

		if got, want := dashboard.Totals.TasksTotal, 5; got != want {
			t.Fatalf("dashboard.Totals.TasksTotal = %d, want %d", got, want)
		}
		if got, want := dashboard.ActiveRuns.Total, 5; got != want {
			t.Fatalf("dashboard.ActiveRuns.Total = %d, want %d", got, want)
		}
		if got := activeRunIDs(dashboard.ActiveRuns.Items); len(got) != 4 ||
			got[0] != "run-running-recent" ||
			got[1] != "run-running-stale" ||
			got[2] != "run-starting" ||
			got[3] != "run-claimed" {
			t.Fatalf("dashboard.ActiveRuns.Items ids = %#v, want running-recent/running-stale/starting/claimed", got)
		}
		if !dashboard.ActiveRuns.Items[1].Stuck || dashboard.ActiveRuns.Items[1].HealthStatus != "warn" {
			t.Fatalf("dashboard.ActiveRuns.Items[1] = %#v, want stuck warn run", dashboard.ActiveRuns.Items[1])
		}
	})
}

func TestTaskObserveQueryValidationAndConfigOption(t *testing.T) {
	t.Parallel()

	cfg := TaskHealthConfig{
		ClaimedStuckAfter:  time.Minute,
		StartingStuckAfter: 2 * time.Minute,
		RunningStuckAfter:  3 * time.Minute,
	}

	observer := &Observer{}
	WithTaskHealthConfig(cfg)(observer)

	if observer.taskHealthConfig != cfg {
		t.Fatalf("observer.taskHealthConfig = %#v, want %#v", observer.taskHealthConfig, cfg)
	}

	if err := (TaskSummaryQuery{Scope: taskpkg.Scope("bogus")}).Validate(); !errors.Is(err, taskpkg.ErrValidation) ||
		!strings.Contains(err.Error(), "scope") {
		t.Fatalf("TaskSummaryQuery.Validate(invalid scope) error = %v, want ErrValidation mentioning scope", err)
	}
	if err := (TaskSummaryQuery{OwnerKind: taskpkg.OwnerKind("bogus")}).Validate(); !errors.Is(
		err,
		taskpkg.ErrValidation,
	) ||
		!strings.Contains(err.Error(), "owner_kind") {
		t.Fatalf(
			"TaskSummaryQuery.Validate(invalid owner kind) error = %v, want ErrValidation mentioning owner_kind",
			err,
		)
	}
	if err := (TaskMetricsQuery{OriginKind: taskpkg.OriginKind("bogus")}).Validate(); !errors.Is(
		err,
		taskpkg.ErrValidation,
	) ||
		!strings.Contains(err.Error(), "origin_kind") {
		t.Fatalf(
			"TaskMetricsQuery.Validate(invalid origin kind) error = %v, want ErrValidation mentioning origin_kind",
			err,
		)
	}
	if err := (TaskDashboardQuery{OwnerKind: taskpkg.OwnerKind("bogus")}).Validate(); !errors.Is(
		err,
		taskpkg.ErrValidation,
	) ||
		!strings.Contains(err.Error(), "owner_kind") {
		t.Fatalf(
			"TaskDashboardQuery.Validate(invalid owner kind) error = %v, want ErrValidation mentioning owner_kind",
			err,
		)
	}
	if err := (TaskInboxQuery{Lane: TaskInboxLane("bogus")}).Validate(); !errors.Is(err, taskpkg.ErrValidation) ||
		!strings.Contains(err.Error(), "lane") {
		t.Fatalf("TaskInboxQuery.Validate(invalid lane) error = %v, want ErrValidation mentioning lane", err)
	}
}

func TestQueryTaskInboxAssignsLanesAndSupportsFilters(t *testing.T) {
	t.Parallel()

	t.Run("Should assign inbox lanes and support unread lane and search filters", func(t *testing.T) {
		h := newHarness(t)
		now := h.observer.now()
		alice := taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "alice"}

		createObserveTask(t, h, taskpkg.Task{
			ID:          "task-my-work-read",
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: h.workspaceID,
			Title:       "My work read",
			Priority:    taskpkg.PriorityHigh,
			Status:      taskpkg.TaskStatusReady,
			Owner:       taskOwner(taskpkg.OwnerKindHuman, "alice"),
			CreatedBy:   taskActor(taskpkg.ActorKindHuman, "alice"),
			Origin:      taskOrigin(taskpkg.OriginKindCLI, "agh task"),
			CreatedAt:   now.Add(-25 * time.Minute),
			UpdatedAt:   now.Add(-20 * time.Minute),
		})
		createObserveTriage(t, h, taskpkg.TriageState{
			TaskID:             "task-my-work-read",
			Actor:              alice,
			Read:               true,
			LastSeenActivityAt: now.Add(-20 * time.Minute),
			UpdatedAt:          now.Add(-19 * time.Minute),
		})

		createObserveTask(t, h, taskpkg.Task{
			ID:          "task-my-work-resurfaced",
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: h.workspaceID,
			Title:       "Resurfaced task",
			Priority:    taskpkg.PriorityUrgent,
			Status:      taskpkg.TaskStatusReady,
			Owner:       taskOwner(taskpkg.OwnerKindHuman, "alice"),
			CreatedBy:   taskActor(taskpkg.ActorKindHuman, "alice"),
			Origin:      taskOrigin(taskpkg.OriginKindCLI, "agh task"),
			CreatedAt:   now.Add(-35 * time.Minute),
			UpdatedAt:   now.Add(-5 * time.Minute),
		})
		createObserveTriage(t, h, taskpkg.TriageState{
			TaskID:             "task-my-work-resurfaced",
			Actor:              alice,
			Read:               true,
			Dismissed:          true,
			LastSeenActivityAt: now.Add(-30 * time.Minute),
			UpdatedAt:          now.Add(-29 * time.Minute),
		})

		createObserveTask(t, h, taskpkg.Task{
			ID:             "task-approval",
			Scope:          taskpkg.ScopeWorkspace,
			WorkspaceID:    h.workspaceID,
			Title:          "Needs approval",
			Status:         taskpkg.TaskStatusBlocked,
			ApprovalPolicy: taskpkg.ApprovalPolicyManual,
			ApprovalState:  taskpkg.ApprovalStatePending,
			CreatedBy:      taskActor(taskpkg.ActorKindHuman, "alice"),
			Origin:         taskOrigin(taskpkg.OriginKindCLI, "agh task"),
			CreatedAt:      now.Add(-18 * time.Minute),
			UpdatedAt:      now.Add(-4 * time.Minute),
		})

		createObserveTask(t, h, taskpkg.Task{
			ID:          "task-failed",
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: h.workspaceID,
			Title:       "Failed deploy",
			Status:      taskpkg.TaskStatusFailed,
			CreatedBy:   taskActor(taskpkg.ActorKindAutomation, "rule-1"),
			Origin:      taskOrigin(taskpkg.OriginKindAutomation, "run:rule-1"),
			CreatedAt:   now.Add(-22 * time.Minute),
			UpdatedAt:   now.Add(-10 * time.Minute),
			ClosedAt:    now.Add(-3 * time.Minute),
		})
		createObserveRun(t, h, taskpkg.Run{
			ID:              "run-failed-latest",
			TaskID:          "task-failed",
			Status:          taskpkg.TaskRunStatusFailed,
			Attempt:         2,
			Origin:          taskOrigin(taskpkg.OriginKindAutomation, "run:rule-1"),
			RunNetworkState: observeRunNetworkState(h.workspaceID, "ops"),
			QueuedAt:        now.Add(-9 * time.Minute),
			ClaimedAt:       now.Add(-8 * time.Minute),
			StartedAt:       now.Add(-7 * time.Minute),
			EndedAt:         now.Add(-3 * time.Minute),
			Error:           "boom",
		})

		createObserveTask(t, h, taskpkg.Task{
			ID:           "task-blocked",
			Scope:        taskpkg.ScopeWorkspace,
			WorkspaceID:  h.workspaceID,
			Title:        "Dependency blocked",
			Status:       taskpkg.TaskStatusBlocked,
			Paused:       true,
			PausedBy:     "alice",
			PausedAt:     now.Add(-8 * time.Minute),
			PausedReason: "waiting on a dependency decision",
			CreatedBy:    taskActor(taskpkg.ActorKindHuman, "alice"),
			Origin:       taskOrigin(taskpkg.OriginKindCLI, "agh task"),
			CreatedAt:    now.Add(-17 * time.Minute),
			UpdatedAt:    now.Add(-7 * time.Minute),
		})

		createObserveTask(t, h, taskpkg.Task{
			ID:          "task-archived",
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: h.workspaceID,
			Title:       "Archived review",
			Status:      taskpkg.TaskStatusReady,
			Owner:       taskOwner(taskpkg.OwnerKindHuman, "alice"),
			CreatedBy:   taskActor(taskpkg.ActorKindHuman, "alice"),
			Origin:      taskOrigin(taskpkg.OriginKindCLI, "agh task"),
			CreatedAt:   now.Add(-16 * time.Minute),
			UpdatedAt:   now.Add(-8 * time.Minute),
		})
		createObserveTriage(t, h, taskpkg.TriageState{
			TaskID:             "task-archived",
			Actor:              alice,
			Read:               true,
			Archived:           true,
			LastSeenActivityAt: now.Add(-8 * time.Minute),
			UpdatedAt:          now.Add(-7 * time.Minute),
		})

		createObserveTask(t, h, taskpkg.Task{
			ID:          "task-dismissed-hidden",
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: h.workspaceID,
			Title:       "Dismissed hidden",
			Status:      taskpkg.TaskStatusReady,
			Owner:       taskOwner(taskpkg.OwnerKindHuman, "alice"),
			CreatedBy:   taskActor(taskpkg.ActorKindHuman, "alice"),
			Origin:      taskOrigin(taskpkg.OriginKindCLI, "agh task"),
			CreatedAt:   now.Add(-30 * time.Minute),
			UpdatedAt:   now.Add(-25 * time.Minute),
		})
		createObserveTriage(t, h, taskpkg.TriageState{
			TaskID:             "task-dismissed-hidden",
			Actor:              alice,
			Read:               true,
			Dismissed:          true,
			LastSeenActivityAt: now.Add(-5 * time.Minute),
			UpdatedAt:          now.Add(-5 * time.Minute),
		})

		inbox, err := h.observer.QueryTaskInbox(testutil.Context(t), TaskInboxQuery{}, alice)
		if err != nil {
			t.Fatalf("QueryTaskInbox() error = %v", err)
		}

		if got, want := inbox.Total, 6; got != want {
			t.Fatalf("inbox.Total = %d, want %d", got, want)
		}
		if got, want := inbox.UnreadTotal, 4; got != want {
			t.Fatalf("inbox.UnreadTotal = %d, want %d", got, want)
		}
		if got, want := inbox.ArchivedTotal, 1; got != want {
			t.Fatalf("inbox.ArchivedTotal = %d, want %d", got, want)
		}

		myWork := requireInboxGroup(t, inbox.Groups, TaskInboxLaneMyWork)
		if got, want := myWork.Count, 2; got != want {
			t.Fatalf("myWork.Count = %d, want %d", got, want)
		}
		if got, want := myWork.UnreadCount, 1; got != want {
			t.Fatalf("myWork.UnreadCount = %d, want %d", got, want)
		}
		if got, want := inboxItemTaskIDs(
			myWork.Items,
		), []string{
			"task-my-work-resurfaced",
			"task-my-work-read",
		}; !equalStringSlices(
			got,
			want,
		) {
			t.Fatalf("myWork item ids = %#v, want %#v", got, want)
		}
		if myWork.Items[0].Triage.Dismissed {
			t.Fatalf("myWork.Items[0].Triage.Dismissed = true, want false after newer activity")
		}

		approvals := requireInboxGroup(t, inbox.Groups, TaskInboxLaneApprovals)
		if got, want := approvals.Count, 1; got != want {
			t.Fatalf("approvals.Count = %d, want %d", got, want)
		}
		if got, want := approvals.Items[0].BlockingReason, taskInboxBlockingReasonAwaitingApproval; got != want {
			t.Fatalf("approvals.Items[0].BlockingReason = %q, want %q", got, want)
		}

		failed := requireInboxGroup(t, inbox.Groups, TaskInboxLaneFailedRuns)
		if got, want := failed.Count, 1; got != want {
			t.Fatalf("failed.Count = %d, want %d", got, want)
		}
		if failed.Items[0].Run == nil || failed.Items[0].Run.ID != "run-failed-latest" {
			t.Fatalf("failed.Items[0].Run = %#v, want run-failed-latest", failed.Items[0].Run)
		}
		if got, want := failed.Items[0].BlockingReason, taskInboxBlockingReasonLatestRunFailed; got != want {
			t.Fatalf("failed.Items[0].BlockingReason = %q, want %q", got, want)
		}

		blocked := requireInboxGroup(t, inbox.Groups, TaskInboxLaneBlocked)
		if got, want := blocked.Count, 1; got != want {
			t.Fatalf("blocked.Count = %d, want %d", got, want)
		}
		if got, want := blocked.Items[0].BlockingReason, taskInboxBlockingReasonAwaitingDeps; got != want {
			t.Fatalf("blocked.Items[0].BlockingReason = %q, want %q", got, want)
		}

		archived := requireInboxGroup(t, inbox.Groups, TaskInboxLaneArchived)
		if got, want := archived.Count, 1; got != want {
			t.Fatalf("archived.Count = %d, want %d", got, want)
		}
		if archived.UnreadCount != 0 {
			t.Fatalf("archived.UnreadCount = %d, want 0", archived.UnreadCount)
		}

		approvalsOnly, err := h.observer.QueryTaskInbox(
			testutil.Context(t),
			TaskInboxQuery{Lane: TaskInboxLaneApprovals},
			alice,
		)
		if err != nil {
			t.Fatalf("QueryTaskInbox(approvals) error = %v", err)
		}
		if got, want := len(approvalsOnly.Groups), 1; got != want {
			t.Fatalf("len(approvalsOnly.Groups) = %d, want %d", got, want)
		}
		if approvalsOnly.Groups[0].Lane != TaskInboxLaneApprovals || approvalsOnly.Total != 1 {
			t.Fatalf("approvalsOnly = %#v, want approvals-only lane with total 1", approvalsOnly)
		}

		unread := true
		unreadOnly, err := h.observer.QueryTaskInbox(testutil.Context(t), TaskInboxQuery{Unread: &unread}, alice)
		if err != nil {
			t.Fatalf("QueryTaskInbox(unread) error = %v", err)
		}
		if got, want := unreadOnly.Total, 4; got != want {
			t.Fatalf("unreadOnly.Total = %d, want %d", got, want)
		}
		if requireInboxGroup(t, unreadOnly.Groups, TaskInboxLaneArchived).Count != 0 {
			t.Fatalf(
				"unread archived count = %d, want 0",
				requireInboxGroup(t, unreadOnly.Groups, TaskInboxLaneArchived).Count,
			)
		}

		searchOnly, err := h.observer.QueryTaskInbox(testutil.Context(t), TaskInboxQuery{Search: "resurfaced"}, alice)
		if err != nil {
			t.Fatalf("QueryTaskInbox(search) error = %v", err)
		}
		if got, want := searchOnly.Total, 1; got != want {
			t.Fatalf("searchOnly.Total = %d, want %d", got, want)
		}
		if got, want := requireInboxGroup(t, searchOnly.Groups, TaskInboxLaneMyWork).Items[0].Task.ID, "task-my-work-resurfaced"; got != want {
			t.Fatalf("searchOnly item id = %q, want %q", got, want)
		}
	})
}

func TestObserverHealthWrapsTaskHealthErrors(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.observer.bridgeSource = nil
	if err := h.registry.Close(testutil.Context(t)); err != nil {
		t.Fatalf("registry.Close() error = %v", err)
	}

	_, err := h.observer.Health(testutil.Context(t))
	if err == nil {
		t.Fatal("Health() error = nil, want wrapped task health failure")
	}
	if !strings.Contains(err.Error(), "observe: collect task health") {
		t.Fatalf("Health() error = %v, want collect task health context", err)
	}
}

func createObserveTask(t *testing.T, h *harness, record taskpkg.Task) {
	t.Helper()
	if err := h.registry.CreateTask(testutil.Context(t), record); err != nil {
		t.Fatalf("CreateTask(%q) error = %v", record.ID, err)
	}
}

func createObserveRun(t *testing.T, h *harness, run taskpkg.Run) {
	t.Helper()
	if err := h.registry.CreateTaskRun(testutil.Context(t), run); err != nil {
		t.Fatalf("CreateTaskRun(%q) error = %v", run.ID, err)
	}
}

func observeRunNetworkState(workspaceID, channel string) *taskpkg.RunNetworkState {
	return &taskpkg.RunNetworkState{NetworkSpec: participation.Spec{
		Version:         participation.SpecVersion,
		Mode:            participation.ModeLive,
		WorkspaceID:     workspaceID,
		ChannelStrategy: participation.StrategyNamed,
		ChannelID:       channel,
		Source:          participation.SourceExplicitRequest,
		Bounds: participation.Bounds{
			MaxWakes:         8,
			MaxWakeWallTime:  "5m",
			MaxTotalWallTime: "30m",
			MaxInputTokens:   200_000,
			MaxOutputTokens:  50_000,
			MaxWakeDepth:     3,
			CoalesceWindow:   "500ms",
		},
	}}
}

func createObserveDependency(t *testing.T, h *harness, dependency taskpkg.Dependency) {
	t.Helper()
	if err := h.registry.CreateDependency(testutil.Context(t), dependency); err != nil {
		t.Fatalf("CreateDependency(%q -> %q) error = %v", dependency.TaskID, dependency.DependsOnTaskID, err)
	}
}

func createObserveEvent(t *testing.T, h *harness, event taskpkg.Event) {
	t.Helper()
	if err := h.registry.CreateTaskEvent(testutil.Context(t), event); err != nil {
		t.Fatalf("CreateTaskEvent(%q) error = %v", event.ID, err)
	}
}

func createObserveNetworkChannel(t *testing.T, h *harness, channel string) {
	t.Helper()
	if err := h.registry.WriteNetworkChannel(testutil.Context(t), store.NetworkChannelEntry{
		WorkspaceID: h.workspaceID,
		Channel:     channel,
		Purpose:     "observe task test",
		CreatedBy:   "test",
		CreatedAt:   h.now,
		UpdatedAt:   h.now,
	}); err != nil {
		t.Fatalf("WriteNetworkChannel(%q) error = %v", channel, err)
	}
}

func createObserveTriage(t *testing.T, h *harness, state taskpkg.TriageState) {
	t.Helper()
	if err := h.registry.UpsertTaskTriageState(testutil.Context(t), state); err != nil {
		t.Fatalf("UpsertTaskTriageState(%q/%q) error = %v", state.Actor.Kind, state.Actor.Ref, err)
	}
}

func createObserveAudit(t *testing.T, h *harness, entry store.NetworkAuditEntry) {
	t.Helper()
	if err := h.registry.WriteNetworkAudit(testutil.Context(t), entry); err != nil {
		t.Fatalf("WriteNetworkAudit(%q) error = %v", entry.ID, err)
	}
}

func taskActor(kind taskpkg.ActorKind, ref string) taskpkg.ActorIdentity {
	return taskpkg.ActorIdentity{Kind: kind, Ref: ref}
}

func taskActorPtr(kind taskpkg.ActorKind, ref string) *taskpkg.ActorIdentity {
	actor := taskActor(kind, ref)
	return &actor
}

func taskOrigin(kind taskpkg.OriginKind, ref string) taskpkg.Origin {
	return taskpkg.Origin{Kind: kind, Ref: ref}
}

func taskOwner(kind taskpkg.OwnerKind, ref string) *taskpkg.Ownership {
	return &taskpkg.Ownership{Kind: kind, Ref: ref}
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return raw
}

func containsTaskTotal(
	rows []TaskStatusTotal,
	scope taskpkg.Scope,
	status taskpkg.Status,
	channel string,
	count int,
) bool {
	for _, item := range rows {
		if item.Scope == scope && item.Status == status && item.ChannelID == channel && item.Count == count {
			return true
		}
	}
	return false
}

func containsTaskOriginTotal(rows []TaskOriginTotal, origin taskpkg.OriginKind, channel string, count int) bool {
	for _, item := range rows {
		if item.OriginKind == origin && item.ChannelID == channel && item.Count == count {
			return true
		}
	}
	return false
}

func containsRunTotal(
	rows []TaskRunTotal,
	status taskpkg.RunStatus,
	origin taskpkg.OriginKind,
	channel string,
	count int,
) bool {
	for _, item := range rows {
		if item.Status == status && item.OriginKind == origin && item.ChannelID == channel && item.Count == count {
			return true
		}
	}
	return false
}

func containsOwnerTotal(rows []TaskOwnerTotal, ownerKind taskpkg.OwnerKind, ownerRef string, count int) bool {
	for _, item := range rows {
		if item.OwnerKind == ownerKind && item.OwnerRef == ownerRef && item.Count == count {
			return true
		}
	}
	return false
}

func containsQueueDepth(rows []TaskQueueDepth, channel string, count int) bool {
	for _, item := range rows {
		if item.ChannelID == channel && item.Count == count {
			return true
		}
	}
	return false
}

func containsStuckRun(rows []StuckTaskRun, runID string, status taskpkg.RunStatus) bool {
	for _, item := range rows {
		if item.RunID == runID && item.Status == status {
			return true
		}
	}
	return false
}

func containsDashboardBreakdown(
	rows []TaskDashboardStatusBreakdown,
	status taskpkg.Status,
	count int,
	sharePercent int,
) bool {
	for _, item := range rows {
		if item.Status == status && item.Count == count && item.SharePercent == sharePercent {
			return true
		}
	}
	return false
}

func activeRunIDs(rows []TaskDashboardActiveRun) []string {
	ids := make([]string, 0, len(rows))
	for _, item := range rows {
		ids = append(ids, item.RunID)
	}
	return ids
}

func requireInboxGroup(t *testing.T, groups []TaskInboxLaneGroup, lane TaskInboxLane) TaskInboxLaneGroup {
	t.Helper()
	for _, group := range groups {
		if group.Lane == lane {
			return group
		}
	}
	t.Fatalf("missing inbox group %q in %#v", lane, groups)
	return TaskInboxLaneGroup{}
}

func inboxItemTaskIDs(items []TaskInboxItem) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.Task.ID)
	}
	return ids
}

func equalStringSlices(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for idx := range left {
		if left[idx] != right[idx] {
			return false
		}
	}
	return true
}
