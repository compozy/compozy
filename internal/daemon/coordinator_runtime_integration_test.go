//go:build integration

package daemon

import (
	"context"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestCoordinatorBootstrapStartsOnceForUserTaskRunsIntegration(t *testing.T) {
	t.Run("Should start one builtin coordinator with configured limits", func(t *testing.T) {
		runCoordinatorBootstrapStartsOnceForUserTaskRunsIntegration(t)
	})
}

func runCoordinatorBootstrapStartsOnceForUserTaskRunsIntegration(t *testing.T) {
	t.Helper()

	ctx := testutil.Context(t)
	manager, sessions := newCoordinatorTaskManagerIntegration(t, ctx)
	actor := coordinatorTaskActor()

	created, err := manager.CreateTask(ctx, taskpkg.CreateTask{
		Scope:       taskpkg.ScopeWorkspace,
		WorkspaceID: "ws-int",
		Title:       "First executable task",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if got := sessions.createCount(); got != 0 {
		t.Fatalf("Create count after task creation = %d, want 0", got)
	}

	execution, err := manager.StartTask(ctx, created.ID, taskpkg.ExecutionRequest{}, actor)
	if err != nil {
		t.Fatalf("StartTask(first) error = %v", err)
	}
	if got, want := execution.Run.NetworkSpecSnapshot(), participation.LocalSpec(); got != want {
		t.Fatalf("StartTask(first) participation = %#v, want %#v", got, want)
	}
	if got := sessions.createCount(); got != 1 {
		t.Fatalf("Create count after first start = %d, want 1", got)
	}
	createdCoordinator := sessions.createCall(0)
	if createdCoordinator.AgentName != aghconfig.BuiltinCoordinatorAgentName ||
		createdCoordinator.Type != session.SessionTypeCoordinator {
		t.Fatalf("coordinator Create() opts = %#v, want bundled coordinator identity and type", createdCoordinator)
	}
	if createdCoordinator.Lineage == nil ||
		createdCoordinator.Lineage.SpawnBudget.MaxChildren != 5 ||
		createdCoordinator.Lineage.SpawnBudget.MaxActivePerWorkspace != 5 ||
		createdCoordinator.Lineage.TTLExpiresAt == nil ||
		!createdCoordinator.Lineage.TTLExpiresAt.Equal(time.Date(2026, 4, 26, 14, 0, 0, 0, time.UTC)) {
		t.Fatalf("coordinator Create() lineage = %#v, want configured caps and TTL", createdCoordinator.Lineage)
	}

	second, err := manager.CreateTask(ctx, taskpkg.CreateTask{
		Scope:       taskpkg.ScopeWorkspace,
		WorkspaceID: "ws-int",
		Title:       "Second executable task",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask(second) error = %v", err)
	}
	if _, err := manager.StartTask(ctx, second.ID, taskpkg.ExecutionRequest{}, actor); err != nil {
		t.Fatalf("StartTask(second) error = %v", err)
	}
	if got := sessions.createCount(); got != 1 {
		t.Fatalf("Create count after second start = %d, want singleton reuse", got)
	}
}

func TestCoordinatorRecoveryRestartsAfterStoppedCoordinatorIntegration(t *testing.T) {
	ctx := testutil.Context(t)
	manager, sessions := newCoordinatorTaskManagerIntegration(t, ctx)
	actor := coordinatorTaskActor()

	created, err := manager.CreateTask(ctx, taskpkg.CreateTask{
		Scope:       taskpkg.ScopeWorkspace,
		WorkspaceID: "ws-int",
		Title:       "Recoverable task",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if _, err := manager.StartTask(ctx, created.ID, taskpkg.ExecutionRequest{}, actor); err != nil {
		t.Fatalf("StartTask() error = %v", err)
	}
	if got := sessions.createCount(); got != 1 {
		t.Fatalf("Create count after start = %d, want 1", got)
	}

	stopped := sessions.stopCoordinatorForTest(t, "coord-1")
	runtime := sessions.runtime
	runtime.OnSessionStopped(ctx, stopped)
	if got := sessions.createCount(); got != 2 {
		t.Fatalf("Create count after stopped coordinator recovery = %d, want 2", got)
	}
}

func newCoordinatorTaskManagerIntegration(
	t *testing.T,
	ctx context.Context,
) (*taskpkg.Service, *coordinatorRuntimeSessionsWithRuntime) {
	t.Helper()

	db := openDaemonTestGlobalDB(t)
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	if err := db.InsertWorkspace(ctx, workspacepkg.Workspace{
		ID:        "ws-int",
		Name:      "Integration Workspace",
		RootDir:   t.TempDir(),
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("InsertWorkspace() error = %v", err)
	}
	notifier := newHooksNotifier(discardLogger(), func() time.Time { return now })
	sessions := &coordinatorRuntimeSessionsWithRuntime{
		coordinatorRuntimeSessions: coordinatorRuntimeSessions{
			infos: []*session.Info{{
				ID:          "manual-1",
				Type:        session.SessionTypeUser,
				WorkspaceID: "ws-int",
				State:       session.StateActive,
			}},
		},
	}
	runtime, err := newCoordinatorRuntime(
		ctx,
		db,
		sessions,
		&staticCoordinatorRoleResolver{cfg: coordinatorRuntimeConfig()},
		notifier,
		discardLogger(),
		func() time.Time { return now },
	)
	if err != nil {
		t.Fatalf("newCoordinatorRuntime() error = %v", err)
	}
	sessions.runtime = runtime
	notifier.AddTaskRunEnqueuedObserver(runtime)

	manager, err := taskpkg.NewManager(
		taskpkg.WithStore(db),
		taskpkg.WithTaskRunHooks(notifier),
		taskpkg.WithManagerNow(func() time.Time { return now }),
	)
	if err != nil {
		t.Fatalf("task.NewManager() error = %v", err)
	}
	return manager, sessions
}

func coordinatorTaskActor() taskpkg.ActorContext {
	return taskpkg.ActorContext{
		Actor: taskpkg.ActorIdentity{
			Kind: taskpkg.ActorKindHuman,
			Ref:  "operator",
		},
		Origin: taskpkg.Origin{
			Kind: taskpkg.OriginKindCLI,
			Ref:  "integration",
		},
		Scope:     taskpkg.CallerScope{Operator: true},
		Authority: taskpkg.FullAccessAuthority(),
	}
}

type coordinatorRuntimeSessionsWithRuntime struct {
	coordinatorRuntimeSessions
	runtime *coordinatorRuntime
}

func (s *coordinatorRuntimeSessionsWithRuntime) stopCoordinatorForTest(
	t *testing.T,
	id string,
) *session.Session {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, info := range s.infos {
		if info == nil || info.ID != id {
			continue
		}
		info.State = session.StateStopped
		return &session.Session{
			ID:                   info.ID,
			Name:                 info.Name,
			AgentName:            info.AgentName,
			Provider:             info.Provider,
			WorkspaceID:          info.WorkspaceID,
			Workspace:            info.Workspace,
			NetworkParticipation: info.NetworkParticipation,
			Type:                 info.Type,
			Lineage:              info.Lineage,
			State:                session.StateStopped,
		}
	}
	t.Fatalf("coordinator session %q not found", id)
	return nil
}
