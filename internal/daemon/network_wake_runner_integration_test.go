//go:build integration

package daemon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	taskpkg "github.com/compozy/agh/internal/task"
	aghworkspace "github.com/compozy/agh/internal/workspace"
)

func TestNetworkWakeRunnerWithDurableTaskService(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	rootDir := t.TempDir()
	databasePath := filepath.Join(rootDir, store.GlobalDatabaseName)
	workspaceDir := filepath.Join(rootDir, "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(workspace) error = %v", err)
	}

	acceptedAt := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	seedDB := openNetworkWakeIntegrationDB(t, databasePath)
	seedNetworkWakeIntegrationScope(t, seedDB, workspaceDir, acceptedAt)
	accepted, err := seedDB.AcceptNetworkMessage(
		ctx,
		networkWakeIntegrationAcceptance(t, acceptedAt),
	)
	if err != nil {
		t.Fatalf("AcceptNetworkMessage() error = %v", err)
	}
	if len(accepted.Notify) != 1 {
		t.Fatalf("AcceptNetworkMessage().Notify = %#v, want one committed wake", accepted.Notify)
	}
	if err := seedDB.Close(context.Background()); err != nil {
		t.Fatalf("GlobalDB.Close(seed) error = %v", err)
	}

	reopenedDB := openNetworkWakeIntegrationDB(t, databasePath)
	taskService, err := taskpkg.NewManager(taskpkg.WithStore(reopenedDB))
	if err != nil {
		t.Fatalf("task.NewManager() error = %v", err)
	}
	actor, err := taskpkg.DeriveDaemonActorContext("network-wake-runner", "daemon.network_wake")
	if err != nil {
		t.Fatalf("DeriveDaemonActorContext() error = %v", err)
	}

	queued, err := reopenedDB.ListQueuedNetworkWakes(ctx, 10)
	if err != nil {
		t.Fatalf("ListQueuedNetworkWakes() error = %v", err)
	}
	if len(queued) != 1 || queued[0] != accepted.Notify[0] {
		t.Fatalf("durable queued wakes = %#v, want %#v", queued, accepted.Notify)
	}
	if _, err := taskService.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
		RunID:            queued[0].TaskRunID,
		RunKind:          taskpkg.RunKindNetworkWake,
		Scope:            taskpkg.ScopeWorkspace,
		WorkspaceID:      queued[0].WorkspaceID,
		TargetSessionID:  "session-foreign",
		ClaimerSessionID: "session-foreign",
		LeaseDuration:    taskpkg.DefaultRunLeaseDuration,
		Now:              acceptedAt.Add(time.Millisecond),
	}, actor); !errors.Is(err, taskpkg.ErrNoClaimableRun) {
		t.Fatalf("ClaimNextRun(foreign session) error = %v, want %v", err, taskpkg.ErrNoClaimableRun)
	}
	run, err := reopenedDB.GetTaskRun(ctx, queued[0].TaskRunID)
	if err != nil {
		t.Fatalf("GetTaskRun(after foreign claim) error = %v", err)
	}
	if run.Status != taskpkg.TaskRunStatusQueued || run.TaskID != "" {
		t.Fatalf("run after foreign claim = %#v, want queued taskless wake", run)
	}

	inputTokens := int64(21)
	outputTokens := int64(8)
	prompter := &networkWakePrompterStub{events: []acp.AgentEvent{{
		Type: acp.EventTypeDone,
		Usage: &acp.TokenUsage{
			InputTokens:  &inputTokens,
			OutputTokens: &outputTokens,
		},
	}}}
	runner, err := newNetworkWakeRunner(taskService, prompter, reopenedDB, nil, 2)
	if err != nil {
		t.Fatalf("newNetworkWakeRunner() error = %v", err)
	}
	if _, err := runner.processNotification(ctx, actor, queued[0]); err != nil {
		t.Fatalf("processNotification() error = %v", err)
	}
	if prompter.calls != 1 {
		t.Fatalf("PromptNetwork calls = %d, want 1", prompter.calls)
	}

	completed, err := reopenedDB.GetTaskRun(ctx, queued[0].TaskRunID)
	if err != nil {
		t.Fatalf("GetTaskRun(completed) error = %v", err)
	}
	if completed.Status != taskpkg.TaskRunStatusCompleted || completed.TaskID != "" ||
		completed.TokensUsed != inputTokens+outputTokens {
		t.Fatalf("completed network wake = %#v", completed)
	}
	tasks, err := taskService.ListTasks(ctx, taskpkg.Query{WorkspaceID: "workspace-network"}, actor)
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("ListTasks() = %#v, want no kanban task for a network wake", tasks)
	}
	usage, err := reopenedDB.GetNetworkUsage(ctx, store.NetworkUsageQuery{
		WorkspaceID: "workspace-network",
		Channel:     "builders",
		Owner: &participation.OwnerRef{
			WorkspaceID: "workspace-network",
			Kind:        participation.OwnerKindSession,
			ID:          "session-target",
		},
	})
	if err != nil {
		t.Fatalf("GetNetworkUsage() error = %v", err)
	}
	if len(usage.Details) != 1 || usage.Details[0].State != store.NetworkWakeStateSucceeded ||
		usage.Details[0].UsageState != store.NetworkWakeUsageActual ||
		usage.Total.InputTokens != inputTokens || usage.Total.OutputTokens != outputTokens {
		t.Fatalf("settled network usage = %#v", usage)
	}
}

func openNetworkWakeIntegrationDB(t *testing.T, databasePath string) *globaldb.GlobalDB {
	t.Helper()

	db, err := globaldb.OpenGlobalDB(t.Context(), databasePath)
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(context.Background()); err != nil {
			t.Errorf("GlobalDB.Close() error = %v", err)
		}
	})
	return db
}

func seedNetworkWakeIntegrationScope(
	t *testing.T,
	db *globaldb.GlobalDB,
	workspaceDir string,
	now time.Time,
) {
	t.Helper()

	if err := db.InsertWorkspace(t.Context(), aghworkspace.Workspace{
		ID: "workspace-network", RootDir: workspaceDir, Name: "network-runner",
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("InsertWorkspace() error = %v", err)
	}
	for _, sessionID := range []string{"session-sender", "session-target", "session-foreign"} {
		if err := db.RegisterSession(t.Context(), store.SessionInfo{
			ID: sessionID, AgentName: "coder", Provider: "test",
			WorkspaceID: "workspace-network", State: "active",
			CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			t.Fatalf("RegisterSession(%q) error = %v", sessionID, err)
		}
	}
}
