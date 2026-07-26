package task

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	diagnosticcontract "github.com/compozy/agh/internal/diagnosticcontract"
	storepkg "github.com/compozy/agh/internal/store"
)

type inspectReaderForTest struct {
	sessions  []storepkg.SessionInfo
	events    []storepkg.EventSummary
	scheduler InspectSchedulerState
}

func (r inspectReaderForTest) ListSessions(
	_ context.Context,
	query storepkg.SessionListQuery,
) ([]storepkg.SessionInfo, error) {
	items := make([]storepkg.SessionInfo, 0, len(r.sessions))
	for _, session := range r.sessions {
		if strings.TrimSpace(query.ID) != "" && session.ID != strings.TrimSpace(query.ID) {
			continue
		}
		if strings.TrimSpace(query.WorkspaceID) != "" && session.WorkspaceID != strings.TrimSpace(query.WorkspaceID) {
			continue
		}
		items = append(items, session)
	}
	if query.Limit > 0 && len(items) > query.Limit {
		items = items[:query.Limit]
	}
	return items, nil
}

func (r inspectReaderForTest) ListEventSummaries(
	_ context.Context,
	query storepkg.EventSummaryQuery,
) ([]storepkg.EventSummary, error) {
	items := make([]storepkg.EventSummary, 0, len(r.events))
	for _, event := range r.events {
		if strings.TrimSpace(query.TaskID) != "" && event.TaskID != strings.TrimSpace(query.TaskID) {
			continue
		}
		if strings.TrimSpace(query.RunID) != "" && event.RunID != strings.TrimSpace(query.RunID) {
			continue
		}
		items = append(items, event)
	}
	if query.Limit > 0 && len(items) > query.Limit {
		items = items[:query.Limit]
	}
	return items, nil
}

func (r inspectReaderForTest) GetSchedulerPauseState(context.Context) (InspectSchedulerState, error) {
	return r.scheduler, nil
}

func TestInspectTaskDiagnostics(t *testing.T) {
	t.Parallel()

	t.Run("Should emit stuck diagnostic for stale claimed heartbeat", func(t *testing.T) {
		t.Parallel()

		manager, now := newInspectManagerForTest(t, inspectReaderForTest{})
		seedInspectTaskRun(t, manager.store.(*inMemoryManagerStore), inspectSeed{
			TaskID:         "task-inspect-stuck",
			RunID:          "run-inspect-stuck",
			Status:         TaskRunStatusClaimed,
			ClaimTokenHash: "sha256:abcdef1234567890",
			LeaseUntil:     now.Add(10 * time.Minute),
			HeartbeatAt:    now.Add(-10 * time.Minute),
			ClaimedAt:      now.Add(-12 * time.Minute),
		})

		view, err := manager.InspectTask(context.Background(), "task-inspect-stuck", validActorContext())
		if err != nil {
			t.Fatalf("InspectTask() error = %v", err)
		}
		if !inspectCodesContain(view.Diagnostics, diagnosticcontract.CodeTaskRunStuck) {
			t.Fatalf("diagnostics = %#v, want %s", view.Diagnostics, diagnosticcontract.CodeTaskRunStuck)
		}
		if view.CurrentRun == nil || view.CurrentRun.ClaimTokenHashTruncated != "abcdef12" {
			t.Fatalf("current run = %#v, want truncated claim token hash", view.CurrentRun)
		}
	})

	t.Run("Should emit stranded diagnostic for old queued run without eligible sessions", func(t *testing.T) {
		t.Parallel()

		manager, now := newInspectManagerForTest(t, inspectReaderForTest{})
		seedInspectTaskRun(t, manager.store.(*inMemoryManagerStore), inspectSeed{
			TaskID:    "task-inspect-stranded",
			RunID:     "run-inspect-stranded",
			Status:    TaskRunStatusQueued,
			QueuedAt:  now.Add(-10 * time.Minute),
			ClaimedAt: time.Time{},
		})

		view, err := manager.InspectTask(context.Background(), "task-inspect-stranded", validActorContext())
		if err != nil {
			t.Fatalf("InspectTask() error = %v", err)
		}
		if !inspectCodesContain(view.Diagnostics, diagnosticcontract.CodeTaskRunStranded) {
			t.Fatalf("diagnostics = %#v, want %s", view.Diagnostics, diagnosticcontract.CodeTaskRunStranded)
		}
		if view.NextAction != InspectNextActionStranded {
			t.Fatalf("NextAction = %q, want %q", view.NextAction, InspectNextActionStranded)
		}
	})

	t.Run("Should not emit stranded diagnostic while scheduler is paused", func(t *testing.T) {
		t.Parallel()

		reader := inspectReaderForTest{scheduler: InspectSchedulerState{Paused: true}}
		manager, now := newInspectManagerForTest(t, reader)
		seedInspectTaskRun(t, manager.store.(*inMemoryManagerStore), inspectSeed{
			TaskID:    "task-inspect-paused-scheduler",
			RunID:     "run-inspect-paused-scheduler",
			Status:    TaskRunStatusQueued,
			QueuedAt:  now.Add(-10 * time.Minute),
			ClaimedAt: time.Time{},
		})

		view, err := manager.InspectTask(context.Background(), "task-inspect-paused-scheduler", validActorContext())
		if err != nil {
			t.Fatalf("InspectTask() error = %v", err)
		}
		if inspectCodesContain(view.Diagnostics, diagnosticcontract.CodeTaskRunStranded) {
			t.Fatalf("diagnostics = %#v, want no %s", view.Diagnostics, diagnosticcontract.CodeTaskRunStranded)
		}
		if view.NextAction != InspectNextActionWaitingForSession {
			t.Fatalf("NextAction = %q, want %q", view.NextAction, InspectNextActionWaitingForSession)
		}
	})

	t.Run("Should emit orphan diagnostic for terminal bound session", func(t *testing.T) {
		t.Parallel()

		reader := inspectReaderForTest{
			sessions: []storepkg.SessionInfo{{
				ID:          "sess-terminal",
				AgentName:   "coder",
				WorkspaceID: "ws-inspect",
				State:       "stopped",
			}},
		}
		manager, now := newInspectManagerForTest(t, reader)
		seedInspectTaskRun(t, manager.store.(*inMemoryManagerStore), inspectSeed{
			TaskID:         "task-inspect-orphan",
			RunID:          "run-inspect-orphan",
			Status:         TaskRunStatusClaimed,
			SessionID:      "sess-terminal",
			ClaimTokenHash: "sha256:fedcba9876543210",
			LeaseUntil:     now.Add(10 * time.Minute),
			HeartbeatAt:    now.Add(-30 * time.Second),
			ClaimedAt:      now.Add(-time.Minute),
		})

		view, err := manager.InspectRun(context.Background(), "run-inspect-orphan", validActorContext())
		if err != nil {
			t.Fatalf("InspectRun() error = %v", err)
		}
		if !inspectCodesContain(view.Diagnostics, diagnosticcontract.CodeTaskRunOrphan) {
			t.Fatalf("diagnostics = %#v, want %s", view.Diagnostics, diagnosticcontract.CodeTaskRunOrphan)
		}
		if view.BoundSession == nil || view.BoundSession.State != "stopped" {
			t.Fatalf("BoundSession = %#v, want stopped session", view.BoundSession)
		}
	})

	t.Run("Should not emit orphan diagnostic for terminal run statuses", func(t *testing.T) {
		t.Parallel()

		reader := inspectReaderForTest{
			sessions: []storepkg.SessionInfo{{
				ID:          "sess-terminal-history",
				AgentName:   "coder",
				WorkspaceID: "ws-inspect",
				State:       "stopped",
			}},
		}
		manager, now := newInspectManagerForTest(t, reader)
		for _, status := range []RunStatus{
			TaskRunStatusCompleted,
			TaskRunStatusFailed,
			TaskRunStatusCanceled,
		} {
			runID := "run-inspect-terminal-" + status.String()
			seedInspectTaskRun(t, manager.store.(*inMemoryManagerStore), inspectSeed{
				TaskID:         "task-inspect-terminal-" + status.String(),
				RunID:          runID,
				Status:         status,
				SessionID:      "sess-terminal-history",
				ClaimTokenHash: "sha256:0123456789abcdef",
				LeaseUntil:     time.Time{},
				HeartbeatAt:    time.Time{},
				ClaimedAt:      now.Add(-time.Minute),
			})

			view, err := manager.InspectRun(context.Background(), runID, validActorContext())
			if err != nil {
				t.Fatalf("InspectRun(%s) error = %v", status, err)
			}
			if inspectCodesContain(view.Diagnostics, diagnosticcontract.CodeTaskRunOrphan) {
				t.Fatalf(
					"InspectRun(%s) diagnostics = %#v, want no %s",
					status,
					view.Diagnostics,
					diagnosticcontract.CodeTaskRunOrphan,
				)
			}
		}
	})

	t.Run("Should not emit crashed diagnostic when a later retry exists", func(t *testing.T) {
		t.Parallel()

		manager, now := newInspectManagerForTest(t, inspectReaderForTest{})
		store := manager.store.(*inMemoryManagerStore)
		seedInspectTaskRun(t, store, inspectSeed{
			TaskID:   "task-inspect-crashed-retry",
			RunID:    "run-inspect-crashed",
			Status:   TaskRunStatusFailed,
			QueuedAt: now.Add(-10 * time.Minute),
		})
		failedRun := store.runs["run-inspect-crashed"]
		failedRun.Error = "provider exited before completion"
		store.runs["run-inspect-crashed"] = failedRun
		store.runs["run-inspect-retry"] = Run{
			ID:            "run-inspect-retry",
			TaskID:        "task-inspect-crashed-retry",
			Status:        TaskRunStatusQueued,
			Attempt:       2,
			PreviousRunID: "run-inspect-crashed",
			Origin:        Origin{Kind: OriginKindCLI, Ref: "task.inspect.test"},
			QueuedAt:      now.Add(-time.Minute),
		}

		view, err := manager.InspectTask(context.Background(), "task-inspect-crashed-retry", validActorContext())
		if err != nil {
			t.Fatalf("InspectTask() error = %v", err)
		}
		if inspectCodesContain(view.Diagnostics, diagnosticcontract.CodeTaskRunCrashed) {
			t.Fatalf("diagnostics = %#v, want no %s", view.Diagnostics, diagnosticcontract.CodeTaskRunCrashed)
		}
		if view.NextAction != InspectNextActionTerminal {
			t.Fatalf("NextAction = %q, want %q", view.NextAction, InspectNextActionTerminal)
		}
	})
}

type inspectSeed struct {
	TaskID         string
	RunID          string
	Status         RunStatus
	SessionID      string
	ClaimTokenHash string
	LeaseUntil     time.Time
	HeartbeatAt    time.Time
	QueuedAt       time.Time
	ClaimedAt      time.Time
}

func newInspectManagerForTest(t *testing.T, reader InspectStateReader) (*Service, time.Time) {
	t.Helper()

	store := newInMemoryManagerStore()
	manager := newTaskManagerForTestWithOptions(t, store, WithInspectStateReader(reader))
	now := manager.now()
	return manager, now
}

func seedInspectTaskRun(t *testing.T, store *inMemoryManagerStore, seed inspectSeed) {
	t.Helper()

	queuedAt := seed.QueuedAt
	if queuedAt.IsZero() {
		queuedAt = time.Date(2026, 4, 14, 14, 0, 0, 0, time.UTC)
	}
	store.tasks[seed.TaskID] = Task{
		ID:             seed.TaskID,
		Scope:          ScopeWorkspace,
		WorkspaceID:    "ws-inspect",
		Title:          "Inspect task",
		Priority:       PriorityMedium,
		MaxAttempts:    3,
		Status:         TaskStatusReady,
		ApprovalPolicy: ApprovalPolicyNone,
		ApprovalState:  ApprovalStateNotRequired,
		CurrentRunID:   seed.RunID,
		CreatedBy:      ActorIdentity{Kind: ActorKindHuman, Ref: "operator"},
		Origin:         Origin{Kind: OriginKindCLI, Ref: "task.inspect.test"},
		CreatedAt:      queuedAt.Add(-time.Minute),
		UpdatedAt:      queuedAt,
	}
	store.runs[seed.RunID] = Run{
		ID:             seed.RunID,
		TaskID:         seed.TaskID,
		Status:         seed.Status,
		Attempt:        1,
		SessionID:      seed.SessionID,
		Origin:         Origin{Kind: OriginKindCLI, Ref: "task.inspect.test"},
		ClaimTokenHash: seed.ClaimTokenHash,
		LeaseUntil:     seed.LeaseUntil,
		HeartbeatAt:    seed.HeartbeatAt,
		QueuedAt:       queuedAt,
		ClaimedAt:      seed.ClaimedAt,
	}
}

func inspectCodesContain(items []diagnosticcontract.DiagnosticItem, code string) bool {
	codes := make([]string, 0, len(items))
	for _, item := range items {
		codes = append(codes, item.Code)
	}
	return slices.Contains(codes, code)
}
