package daemon

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/store/rundb"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

func TestStartReconcilesInterruptedRunsBeforeReady(t *testing.T) {
	paths := mustHomePaths(t)
	t.Setenv("HOME", filepath.Dir(paths.HomeDir))
	if err := compozyconfig.EnsureHomeLayout(paths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	db := openDaemonGlobalDB(t, paths)
	workspace := registerDaemonWorkspace(t, db)
	now := time.Date(2026, 4, 17, 21, 0, 0, 0, time.UTC)

	for _, runID := range []string{"run-starting", "run-running"} {
		seedInterruptedRun(t, db, workspace.ID, runID, map[string]string{
			"run-starting": "starting",
			"run-running":  "running",
		}[runID], now)
		createRecoverableRunDB(t, runID)
	}

	result, err := Start(context.Background(), StartOptions{
		HomePaths: paths,
		PID:       4242,
		Now:       func() time.Time { return now },
		ProcessAlive: func(pid int) bool {
			return pid == 4242
		},
		Prepare: func(ctx context.Context, host *Host) error {
			reconcileResult, reconcileErr := ReconcileStartup(ctx, ReconcileConfig{
				HomePaths: host.Paths(),
				Now:       func() time.Time { return now.Add(2 * time.Minute) },
			})
			if reconcileErr != nil {
				return reconcileErr
			}
			if reconcileResult.ReconciledRuns != 2 {
				t.Fatalf("ReconcileStartup() reconciled %d runs, want 2", reconcileResult.ReconciledRuns)
			}

			info, readErr := ReadInfo(host.Paths().InfoPath)
			if readErr != nil {
				t.Fatalf("ReadInfo(starting) error = %v", readErr)
			}
			if info.State != ReadyStateStarting {
				t.Fatalf("info.State during prepare = %q, want starting", info.State)
			}

			for _, runID := range []string{"run-starting", "run-running"} {
				row, getErr := db.GetRun(context.Background(), runID)
				if getErr != nil {
					t.Fatalf("GetRun(%q) during prepare error = %v", runID, getErr)
				}
				if row.Status != "crashed" {
					t.Fatalf("row.Status during prepare = %q, want crashed", row.Status)
				}
				lastEvent := lastRunDBEvent(t, runID)
				if lastEvent == nil || lastEvent.Kind != eventspkg.EventKindRunCrashed {
					t.Fatalf("last event for %q = %#v, want run.crashed", runID, lastEvent)
				}
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() {
		_ = result.Host.Close(context.Background())
	}()

	if result.Info.State != ReadyStateReady {
		t.Fatalf("result.Info.State = %q, want ready", result.Info.State)
	}
}

func TestStartRemainsHealthyWhenInterruptedRunDBIsMissingOrCorrupt(t *testing.T) {
	paths := mustHomePaths(t)
	t.Setenv("HOME", filepath.Dir(paths.HomeDir))
	if err := compozyconfig.EnsureHomeLayout(paths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	db := openDaemonGlobalDB(t, paths)
	workspace := registerDaemonWorkspace(t, db)
	now := time.Date(2026, 4, 17, 22, 0, 0, 0, time.UTC)

	seedInterruptedRun(t, db, workspace.ID, "run-missing-db", "running", now)
	seedInterruptedRun(t, db, workspace.ID, "run-corrupt-db", "starting", now.Add(time.Second))
	writeCorruptRunDB(t, "run-corrupt-db")

	result, err := Start(context.Background(), StartOptions{
		HomePaths: paths,
		PID:       5252,
		Now:       func() time.Time { return now },
		ProcessAlive: func(pid int) bool {
			return pid == 5252
		},
		Prepare: func(ctx context.Context, host *Host) error {
			_, reconcileErr := ReconcileStartup(ctx, ReconcileConfig{
				HomePaths: host.Paths(),
				Now:       func() time.Time { return now.Add(5 * time.Minute) },
			})
			return reconcileErr
		},
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() {
		_ = result.Host.Close(context.Background())
	}()

	status, err := QueryStatus(context.Background(), paths, ProbeOptions{
		ProcessAlive: func(pid int) bool { return pid == 5252 },
	})
	if err != nil {
		t.Fatalf("QueryStatus() error = %v", err)
	}
	if !status.Healthy || status.State != ReadyStateReady {
		t.Fatalf("status = %#v, want ready and healthy", status)
	}

	for _, runID := range []string{"run-missing-db", "run-corrupt-db"} {
		row, err := db.GetRun(context.Background(), runID)
		if err != nil {
			t.Fatalf("GetRun(%q) error = %v", runID, err)
		}
		if row.Status != "crashed" {
			t.Fatalf("row.Status(%q) = %q, want crashed", runID, row.Status)
		}
		if !strings.Contains(row.ErrorText, "daemon stopped before run reached terminal state") {
			t.Fatalf("row.ErrorText(%q) = %q, want crash summary", runID, row.ErrorText)
		}
		if !strings.Contains(row.ErrorText, "synthetic crash event unavailable") {
			t.Fatalf("row.ErrorText(%q) = %q, want append failure summary", runID, row.ErrorText)
		}
	}
}

func TestLoadRunLifecycleSettingsPopulatesWorktreesRoot(t *testing.T) {
	t.Run("Should keep lifecycle settings on the captured home layout", func(t *testing.T) {
		paths := mustHomePaths(t)
		t.Setenv("HOME", t.TempDir())
		if err := compozyconfig.EnsureHomeLayout(paths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}
		if err := os.WriteFile(
			paths.ConfigFile,
			[]byte("[runs]\nkeep_max = 17\n"),
			0o600,
		); err != nil {
			t.Fatalf("write captured config: %v", err)
		}

		settings, configPath, err := LoadRunLifecycleSettingsForHome(context.Background(), paths)
		if err != nil {
			t.Fatalf("LoadRunLifecycleSettingsForHome() error = %v", err)
		}
		if settings.WorktreesRoot != paths.WorktreesDir {
			t.Fatalf("WorktreesRoot = %q, want %q", settings.WorktreesRoot, paths.WorktreesDir)
		}
		if settings.RunsDir != paths.RunsDir {
			t.Fatalf("RunsDir = %q, want %q", settings.RunsDir, paths.RunsDir)
		}
		if settings.KeepMax != 17 {
			t.Fatalf("KeepMax = %d, want 17 from captured config", settings.KeepMax)
		}
		if configPath != paths.ConfigFile {
			t.Fatalf("config path = %q, want %q", configPath, paths.ConfigFile)
		}
	})
}

func TestReconcileStartupCleansSafeWorktreesAndSurfacesPreviousHome(t *testing.T) {
	t.Run("Should clean safe worktrees and surface previous home", func(t *testing.T) {
		requireGitForTaskMulti(t)
		paths := mustHomePaths(t)
		if err := compozyconfig.EnsureHomeLayout(paths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}
		db := openDaemonGlobalDB(t, paths)
		workspace := registerDaemonWorkspace(t, db)
		writeFileForTest(t, filepath.Join(workspace.RootDir, "README.md"), "seed\n")
		commitTaskMultiGitWorkspace(t, workspace.RootDir)
		base := taskMultiWorktreeBase{
			Branch: "main",
			Commit: runGitOutput(t, workspace.RootDir, "rev-parse", "HEAD"),
		}
		allocator := newTaskMultiWorktreeAllocator(paths.WorktreesDir)

		cleanRunID := "reconcile-clean-worktree"
		seedInterruptedRun(t, db, workspace.ID, cleanRunID, runStatusRunning, time.Now().UTC())
		branch, err := taskMultiResultBranch(cleanRunID, 0, "alpha")
		if err != nil {
			t.Fatalf("taskMultiResultBranch() error = %v", err)
		}
		allocation, err := allocator.Allocate(context.Background(), taskMultiWorktreeSpec{
			WorkspaceRoot: workspace.RootDir,
			ParentRunID:   cleanRunID,
			Slug:          "alpha",
			Index:         0,
			ResultBranch:  branch,
			Base:          base,
		})
		if err != nil {
			t.Fatalf("Allocate() error = %v", err)
		}
		appendReconcileTaskMultiEvent(t, paths, cleanRunID, allocation)

		oldHomeRunID := "reconcile-old-home-worktree"
		seedInterruptedRun(t, db, workspace.ID, oldHomeRunID, runStatusRunning, time.Now().UTC())
		appendReconcileTaskMultiEvent(t, paths, oldHomeRunID, taskMultiWorktreeAllocation{
			Path:           filepath.Join(t.TempDir(), ".compozy", "state", "worktrees", "old", "00-alpha"),
			BaseBranch:     "main",
			BaseCommit:     base.Commit,
			WorktreeStatus: taskMultiWorktreeStatusActive,
		})

		result, err := ReconcileStartup(context.Background(), ReconcileConfig{
			HomePaths: paths,
			Now:       func() time.Time { return time.Now().UTC() },
		})
		if err != nil {
			t.Fatalf("ReconcileStartup() error = %v", err)
		}
		if result.WorktreesRemoved != 1 {
			t.Fatalf("WorktreesRemoved = %d, want 1", result.WorktreesRemoved)
		}
		if result.WorktreeCleanupDeferred != 1 || len(result.WorktreeCleanupWarnings) != 1 {
			t.Fatalf("worktree cleanup result = %#v", result)
		}
		if !strings.Contains(result.WorktreeCleanupWarnings[0], "previous COMPOZY_HOME") {
			t.Fatalf("cleanup warning = %q, want previous COMPOZY_HOME", result.WorktreeCleanupWarnings[0])
		}
		if _, err := os.Stat(allocation.Path); !os.IsNotExist(err) {
			t.Fatalf("safe worktree stat error = %v, want removed", err)
		}
		if got := runGitOutput(t, workspace.RootDir, "branch", "--list", branch); got != "" {
			t.Fatalf("empty result branch remains: %q", got)
		}
	})
}

func appendReconcileTaskMultiEvent(
	t *testing.T,
	paths compozyconfig.HomePaths,
	runID string,
	allocation taskMultiWorktreeAllocation,
) {
	t.Helper()
	artifacts := model.NewRunArtifactsForRunsDir(paths.RunsDir, runID)
	if err := os.MkdirAll(artifacts.RunDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	runDB, err := rundb.Open(context.Background(), artifacts.RunDBPath)
	if err != nil {
		t.Fatalf("rundb.Open(%q) error = %v", artifacts.RunDBPath, err)
	}
	defer func() { _ = runDB.Close() }()
	if _, err := runDB.AppendSyntheticEvent(
		context.Background(),
		eventspkg.EventKindTaskRunMultipleItemQueued,
		kinds.TaskRunMultiplePayload{
			Slug:           "alpha",
			WorktreePath:   allocation.Path,
			BaseBranch:     allocation.BaseBranch,
			BaseCommit:     allocation.BaseCommit,
			WorktreeStatus: allocation.WorktreeStatus,
			ResultBranch:   allocation.ResultBranch,
		},
	); err != nil {
		t.Fatalf("AppendSyntheticEvent() error = %v", err)
	}
}

func openDaemonGlobalDB(t *testing.T, paths compozyconfig.HomePaths) *globaldb.GlobalDB {
	t.Helper()

	db, err := globaldb.Open(context.Background(), paths.GlobalDBPath)
	if err != nil {
		t.Fatalf("globaldb.Open(%s) error = %v", paths.GlobalDBPath, err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

func registerDaemonWorkspace(t *testing.T, db *globaldb.GlobalDB) globaldb.Workspace {
	t.Helper()

	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(filepath.Join(workspaceRoot, ".compozy"), 0o755); err != nil {
		t.Fatalf("mkdir workspace marker: %v", err)
	}
	workspace, err := db.Register(context.Background(), workspaceRoot, "daemon-reconcile")
	if err != nil {
		t.Fatalf("Register(%q) error = %v", workspaceRoot, err)
	}
	return workspace
}

func seedInterruptedRun(
	t *testing.T,
	db *globaldb.GlobalDB,
	workspaceID string,
	runID string,
	status string,
	startedAt time.Time,
) {
	t.Helper()

	if _, err := db.PutRun(context.Background(), globaldb.Run{
		RunID:            runID,
		WorkspaceID:      workspaceID,
		Mode:             "task",
		Status:           status,
		PresentationMode: "stream",
		StartedAt:        startedAt,
	}); err != nil {
		t.Fatalf("PutRun(%q) error = %v", runID, err)
	}
}

func createRecoverableRunDB(t *testing.T, runID string) {
	t.Helper()

	runArtifacts, err := model.ResolveHomeRunArtifacts(runID)
	if err != nil {
		t.Fatalf("ResolveHomeRunArtifacts(%q) error = %v", runID, err)
	}
	if err := os.MkdirAll(runArtifacts.RunDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	runDB, err := rundb.Open(context.Background(), runArtifacts.RunDBPath)
	if err != nil {
		t.Fatalf("rundb.Open(%q) error = %v", runArtifacts.RunDBPath, err)
	}
	_ = runDB.Close()
}

func writeCorruptRunDB(t *testing.T, runID string) {
	t.Helper()

	runArtifacts, err := model.ResolveHomeRunArtifacts(runID)
	if err != nil {
		t.Fatalf("ResolveHomeRunArtifacts(%q) error = %v", runID, err)
	}
	if err := os.MkdirAll(runArtifacts.RunDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	if err := os.WriteFile(runArtifacts.RunDBPath, []byte("not-a-sqlite-database"), 0o600); err != nil {
		t.Fatalf("write corrupt run db: %v", err)
	}
}

func lastRunDBEvent(t *testing.T, runID string) *eventspkg.Event {
	t.Helper()

	runArtifacts, err := model.ResolveHomeRunArtifacts(runID)
	if err != nil {
		t.Fatalf("ResolveHomeRunArtifacts(%q) error = %v", runID, err)
	}
	runDB, err := rundb.Open(context.Background(), runArtifacts.RunDBPath)
	if err != nil {
		t.Fatalf("rundb.Open(%q) error = %v", runArtifacts.RunDBPath, err)
	}
	defer func() {
		_ = runDB.Close()
	}()

	event, err := runDB.LastEvent(context.Background())
	if err != nil {
		t.Fatalf("LastEvent(%q) error = %v", runID, err)
	}
	return event
}
