package observe

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
)

type countingTaskRegistry struct {
	Registry

	mu             sync.Mutex
	listTasksCalls int
	listRunsCalls  int
	listEventCalls int
	listAuditCalls int
}

func (r *countingTaskRegistry) ListTasks(ctx context.Context, query taskpkg.Query) ([]taskpkg.Summary, error) {
	r.mu.Lock()
	r.listTasksCalls++
	r.mu.Unlock()
	return r.Registry.ListTasks(ctx, query)
}

func (r *countingTaskRegistry) ListTaskRuns(
	ctx context.Context,
	query taskpkg.RunQuery,
) ([]taskpkg.Run, error) {
	r.mu.Lock()
	r.listRunsCalls++
	r.mu.Unlock()
	return r.Registry.ListTaskRuns(ctx, query)
}

func (r *countingTaskRegistry) ListTaskEvents(
	ctx context.Context,
	query taskpkg.EventQuery,
) ([]taskpkg.Event, error) {
	r.mu.Lock()
	r.listEventCalls++
	r.mu.Unlock()
	return r.Registry.ListTaskEvents(ctx, query)
}

func (r *countingTaskRegistry) ListNetworkAudit(
	ctx context.Context,
	query store.NetworkAuditQuery,
) ([]store.NetworkAuditEntry, error) {
	r.mu.Lock()
	r.listAuditCalls++
	r.mu.Unlock()
	return r.Registry.ListNetworkAudit(ctx, query)
}

func TestHealthLoadsTaskDataOncePerSnapshot(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	wrapped := &countingTaskRegistry{Registry: h.registry}
	h.observer.registry = wrapped

	createObserveTask(t, h, taskpkg.Task{
		ID:          "task-health-once",
		Scope:       taskpkg.ScopeWorkspace,
		WorkspaceID: h.workspaceID,
		Title:       "Health once",
		Status:      taskpkg.TaskStatusInProgress,
		CreatedBy:   taskActor(taskpkg.ActorKindHuman, "user"),
		Origin:      taskOrigin(taskpkg.OriginKindCLI, "agh task"),
		CreatedAt:   h.now,
		UpdatedAt:   h.now,
	})
	createObserveRun(t, h, taskpkg.Run{
		ID:        "run-health-once",
		TaskID:    "task-health-once",
		Status:    taskpkg.TaskRunStatusClaimed,
		Attempt:   1,
		Origin:    taskOrigin(taskpkg.OriginKindCLI, "agh task"),
		QueuedAt:  h.now.Add(-10 * time.Minute),
		ClaimedAt: h.now.Add(-6 * time.Minute),
	})

	if _, err := h.observer.Health(testutil.Context(t)); err != nil {
		t.Fatalf("Health() error = %v", err)
	}

	if got, want := wrapped.listTasksCalls, 1; got != want {
		t.Fatalf("ListTasks calls = %d, want %d", got, want)
	}
	if got, want := wrapped.listRunsCalls, 1; got != want {
		t.Fatalf("ListTaskRuns calls = %d, want %d", got, want)
	}
	if got, want := wrapped.listEventCalls, 1; got != want {
		t.Fatalf("ListTaskEvents calls = %d, want %d", got, want)
	}
	if got, want := wrapped.listAuditCalls, 1; got != want {
		t.Fatalf("ListNetworkAudit calls = %d, want %d", got, want)
	}
}
