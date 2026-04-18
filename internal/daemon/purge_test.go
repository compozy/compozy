package daemon

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/store/globaldb"
)

func TestRunManagerPurgeRemovesTerminalRunsOldestFirstWithoutTouchingActiveRuns(t *testing.T) {
	now := time.Date(2026, 4, 17, 23, 0, 0, 0, time.UTC)
	started := make(chan string, 1)
	env := newRunManagerTestEnv(t, runManagerTestDeps{
		now: func() time.Time { return now },
		prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
			return &model.SolvePreparation{}, nil
		},
		execute: func(ctx context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
			started <- cfg.RunID
			<-ctx.Done()
			return ctx.Err()
		},
	})

	workspace, err := env.globalDB.ResolveOrRegister(context.Background(), env.workspaceRoot)
	if err != nil {
		t.Fatalf("ResolveOrRegister(%q) error = %v", env.workspaceRoot, err)
	}
	for _, item := range []struct {
		runID   string
		status  string
		endedAt time.Time
	}{
		{runID: "run-oldest", status: "completed", endedAt: now.AddDate(0, 0, -30)},
		{runID: "run-old-age", status: "failed", endedAt: now.AddDate(0, 0, -20)},
		{runID: "run-recent", status: "crashed", endedAt: now.AddDate(0, 0, -1)},
	} {
		seedTerminalRunForPurge(t, env.globalDB, workspace.ID, item.runID, item.status, item.endedAt)
	}

	activeRun := env.startTaskRun(t, "run-active", nil)
	waitForString(t, started, activeRun.RunID)

	result, err := env.manager.Purge(context.Background(), RunLifecycleSettings{
		KeepTerminalDays:     14,
		KeepMax:              1,
		ShutdownDrainTimeout: defaultShutdownDrainTimeout,
	})
	if err != nil {
		t.Fatalf("Purge() error = %v", err)
	}
	if got, want := result.PurgedRunIDs, []string{"run-oldest", "run-old-age"}; !equalStrings(got, want) {
		t.Fatalf("purged run ids = %v, want %v", got, want)
	}

	for _, runID := range result.PurgedRunIDs {
		if _, err := env.globalDB.GetRun(context.Background(), runID); !errors.Is(err, globaldb.ErrRunNotFound) {
			t.Fatalf("GetRun(%q) error = %v, want ErrRunNotFound", runID, err)
		}
		runArtifacts, err := model.ResolveHomeRunArtifacts(runID)
		if err != nil {
			t.Fatalf("ResolveHomeRunArtifacts(%q) error = %v", runID, err)
		}
		if _, err := os.Stat(runArtifacts.RunDir); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Stat(%q) error = %v, want os.ErrNotExist", runArtifacts.RunDir, err)
		}
	}

	activeRow, err := env.globalDB.GetRun(context.Background(), activeRun.RunID)
	if err != nil {
		t.Fatalf("GetRun(active) error = %v", err)
	}
	if activeRow.Status != runStatusRunning {
		t.Fatalf("active row status = %q, want running", activeRow.Status)
	}

	runArtifacts, err := model.ResolveHomeRunArtifacts(activeRun.RunID)
	if err != nil {
		t.Fatalf("ResolveHomeRunArtifacts(active) error = %v", err)
	}
	if _, err := os.Stat(runArtifacts.RunDir); err != nil {
		t.Fatalf("Stat(active run dir) error = %v", err)
	}

	if err := env.manager.Shutdown(context.Background(), true); err != nil {
		t.Fatalf("Shutdown(force cleanup) error = %v", err)
	}
}

func TestPurgeTerminalRunsDelegatesToManagerPurge(t *testing.T) {
	env := newRunManagerTestEnv(t, runManagerTestDeps{})

	workspace, err := env.globalDB.ResolveOrRegister(context.Background(), env.workspaceRoot)
	if err != nil {
		t.Fatalf("ResolveOrRegister(%q) error = %v", env.workspaceRoot, err)
	}

	runID := "purge-wrapper-old"
	seedTerminalRunForPurge(
		t,
		env.globalDB,
		workspace.ID,
		runID,
		runStatusCompleted,
		time.Now().UTC().AddDate(0, 0, -30),
	)

	result, err := PurgeTerminalRuns(context.Background(), env.globalDB, RunLifecycleSettings{
		KeepTerminalDays: 0,
		KeepMax:          0,
	})
	if err != nil {
		t.Fatalf("PurgeTerminalRuns() error = %v", err)
	}
	if got, want := result.PurgedRunIDs, []string{runID}; !equalStrings(got, want) {
		t.Fatalf("purged run ids = %v, want %v", got, want)
	}
}

func seedTerminalRunForPurge(
	t *testing.T,
	db *globaldb.GlobalDB,
	workspaceID string,
	runID string,
	status string,
	endedAt time.Time,
) {
	t.Helper()

	runArtifacts, err := model.ResolveHomeRunArtifacts(runID)
	if err != nil {
		t.Fatalf("ResolveHomeRunArtifacts(%q) error = %v", runID, err)
	}
	if err := os.MkdirAll(runArtifacts.RunDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir %q: %v", runArtifacts.RunDir, err)
	}
	if _, err := db.PutRun(context.Background(), globaldb.Run{
		RunID:            runID,
		WorkspaceID:      workspaceID,
		Mode:             "task",
		Status:           status,
		PresentationMode: "stream",
		StartedAt:        endedAt.Add(-time.Minute),
		EndedAt:          &endedAt,
		ErrorText:        status,
	}); err != nil {
		t.Fatalf("PutRun(%q) error = %v", runID, err)
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for idx := range got {
		if got[idx] != want[idx] {
			return false
		}
	}
	return true
}
