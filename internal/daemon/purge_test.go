package daemon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/store/globaldb"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
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

func TestRunManagerPurgeRemovesTerminalRunTaskWorktrees(t *testing.T) {
	t.Run("Should remove terminal run task worktrees", func(t *testing.T) {
		requireGitForTaskMulti(t)
		env := newRunManagerTestEnv(t, runManagerTestDeps{})
		writeFileForTest(t, filepath.Join(env.workspaceRoot, "README.md"), "seed\n")
		commitTaskMultiGitWorkspace(t, env.workspaceRoot)

		workspace, err := env.globalDB.ResolveOrRegister(context.Background(), env.workspaceRoot)
		if err != nil {
			t.Fatalf("ResolveOrRegister(%q) error = %v", env.workspaceRoot, err)
		}
		runID := "purge-task-worktrees-clean"
		seedTerminalRunForPurge(
			t,
			env.globalDB,
			workspace.ID,
			runID,
			runStatusCompleted,
			time.Now().UTC().Add(-time.Hour),
		)
		allocation := allocatePurgeTaskWorktree(t, env, runID, "alpha", 1)
		appendPurgeRunEvent(t, runID, eventspkg.EventKindTaskRunMultipleChildCompleted, kinds.TaskRunMultiplePayload{
			RunID:          runID,
			Slug:           "alpha",
			Index:          0,
			WorktreePath:   allocation.Path,
			BaseBranch:     allocation.BaseBranch,
			BaseCommit:     allocation.BaseCommit,
			WorktreeStatus: allocation.WorktreeStatus,
		})

		result, err := env.manager.Purge(context.Background(), RunLifecycleSettings{
			KeepTerminalDays: 0,
			KeepMax:          0,
		})
		if err != nil {
			t.Fatalf("Purge() error = %v", err)
		}
		if got, want := result.PurgedRunIDs, []string{runID}; !equalStrings(got, want) {
			t.Fatalf("purged run ids = %v, want %v", got, want)
		}
		if got, want := result.PurgedWorktreePaths, []string{allocation.Path}; !equalStrings(got, want) {
			t.Fatalf("purged worktree paths = %v, want %v", got, want)
		}
		assertPathMissing(t, allocation.Path)
		if _, err := env.globalDB.GetRun(context.Background(), runID); !errors.Is(err, globaldb.ErrRunNotFound) {
			t.Fatalf("GetRun(%q) error = %v, want ErrRunNotFound", runID, err)
		}
	})
}

func TestRunManagerPurgeRemovesCrashedParallelTaskWorktree(t *testing.T) {
	t.Run("Should remove crashed parallel task worktree", func(t *testing.T) {
		requireGitForTaskMulti(t)
		env := newRunManagerTestEnv(t, runManagerTestDeps{})
		writeFileForTest(t, filepath.Join(env.workspaceRoot, "README.md"), "seed\n")
		commitTaskMultiGitWorkspace(t, env.workspaceRoot)

		workspace, err := env.globalDB.ResolveOrRegister(context.Background(), env.workspaceRoot)
		if err != nil {
			t.Fatalf("ResolveOrRegister(%q) error = %v", env.workspaceRoot, err)
		}
		runID := "purge-crashed-parallel-task"
		seedTerminalRunForPurge(
			t,
			env.globalDB,
			workspace.ID,
			runID,
			runStatusCrashed,
			time.Now().UTC().Add(-time.Hour),
		)
		allocation := allocatePurgeTaskWorktree(t, env, runID, "parallel", 1)
		appendPurgeRunEvent(t, runID, eventspkg.EventKindTaskParallelTaskStarted, kinds.TaskParallelPayload{
			RunID:        runID,
			TaskID:       "task_01",
			ChildRunID:   "child-task-01",
			WorktreePath: allocation.Path,
		})

		result, err := env.manager.Purge(context.Background(), RunLifecycleSettings{
			KeepTerminalDays: 0,
			KeepMax:          0,
		})
		if err != nil {
			t.Fatalf("Purge() error = %v", err)
		}
		if got, want := result.PurgedRunIDs, []string{runID}; !equalStrings(got, want) {
			t.Fatalf("purged run ids = %v, want %v", got, want)
		}
		if got, want := result.PurgedWorktreePaths, []string{allocation.Path}; !equalStrings(got, want) {
			t.Fatalf("purged worktree paths = %v, want %v", got, want)
		}
		assertPathMissing(t, allocation.Path)
	})
}

func TestRunManagerPurgeRemovesTerminalRunIntegrationWorktree(t *testing.T) {
	t.Run("Should remove terminal run integration worktree", func(t *testing.T) {
		requireGitForTaskMulti(t)
		env := newRunManagerTestEnv(t, runManagerTestDeps{})
		writeFileForTest(t, filepath.Join(env.workspaceRoot, "README.md"), "seed\n")
		commitTaskMultiGitWorkspace(t, env.workspaceRoot)

		workspace, err := env.globalDB.ResolveOrRegister(context.Background(), env.workspaceRoot)
		if err != nil {
			t.Fatalf("ResolveOrRegister(%q) error = %v", env.workspaceRoot, err)
		}
		runID := "purge-parallel-integration"
		seedTerminalRunForPurge(
			t,
			env.globalDB,
			workspace.ID,
			runID,
			runStatusFailed,
			time.Now().UTC().Add(-time.Hour),
		)
		base, err := env.manager.worktreeAllocator.ResolveBase(context.Background(), env.workspaceRoot)
		if err != nil {
			t.Fatalf("ResolveBase() error = %v", err)
		}
		integrationPath, err := planParallelIntegrationPath(env.paths.WorktreesDir, env.workspaceRoot, runID)
		if err != nil {
			t.Fatalf("planParallelIntegrationPath() error = %v", err)
		}
		integrationBranch := parallelIntegrationBranch(runID)
		if err := env.manager.worktreeAllocator.CreateIntegrationBranch(
			context.Background(),
			env.workspaceRoot,
			integrationPath,
			integrationBranch,
			base.Commit,
		); err != nil {
			t.Fatalf("CreateIntegrationBranch() error = %v", err)
		}
		appendPurgeRunEvent(t, runID, eventspkg.EventKindTaskParallelTaskStarted, kinds.TaskParallelPayload{
			RunID:             runID,
			IntegrationBranch: integrationBranch,
			Phase:             "task_started",
		})

		result, err := env.manager.Purge(context.Background(), RunLifecycleSettings{
			KeepTerminalDays: 0,
			KeepMax:          0,
		})
		if err != nil {
			t.Fatalf("Purge() error = %v", err)
		}
		if len(result.PurgedWorktreePaths) != 1 {
			t.Fatalf("purged worktree paths = %v, want one integration worktree", result.PurgedWorktreePaths)
		}
		parallelParent := sanitizeTaskMultiWorktreeSegment(runID, taskMultiWorktreeParentShortLen) +
			"-" + taskMultiShortHash(runID, taskMultiWorktreeParentHashLen)
		if wantSuffix := filepath.Join(
			taskMultiWorkspaceHash(env.workspaceRoot),
			parallelParent,
			"integration",
		); !strings.HasSuffix(result.PurgedWorktreePaths[0], wantSuffix) {
			t.Fatalf("purged worktree path = %q, want suffix %q", result.PurgedWorktreePaths[0], wantSuffix)
		}
		assertPathMissing(t, result.PurgedWorktreePaths[0])
		if branches := strings.TrimSpace(runGitOutput(
			t,
			env.workspaceRoot,
			"branch",
			"--list",
			integrationBranch,
			"--format=%(refname:short)",
		)); branches != "" {
			t.Fatalf("integration branch still exists: %q", branches)
		}
	})
}

func TestRunManagerPurgePreservesDirtyTaskWorktreeAndMetadata(t *testing.T) {
	t.Run("Should preserve dirty task worktree and metadata", func(t *testing.T) {
		requireGitForTaskMulti(t)
		env := newRunManagerTestEnv(t, runManagerTestDeps{})
		writeFileForTest(t, filepath.Join(env.workspaceRoot, "README.md"), "seed\n")
		commitTaskMultiGitWorkspace(t, env.workspaceRoot)

		workspace, err := env.globalDB.ResolveOrRegister(context.Background(), env.workspaceRoot)
		if err != nil {
			t.Fatalf("ResolveOrRegister(%q) error = %v", env.workspaceRoot, err)
		}
		runID := "purge-dirty-worktree"
		seedTerminalRunForPurge(
			t,
			env.globalDB,
			workspace.ID,
			runID,
			runStatusFailed,
			time.Now().UTC().Add(-time.Hour),
		)
		allocation := allocatePurgeTaskWorktree(t, env, runID, "dirty", 1)
		writeFileForTest(t, filepath.Join(allocation.Path, "dirty.txt"), "dirty\n")
		appendPurgeRunEvent(t, runID, eventspkg.EventKindTaskRunMultipleChildFailed, kinds.TaskRunMultiplePayload{
			RunID:          runID,
			Slug:           "dirty",
			WorktreePath:   allocation.Path,
			BaseBranch:     allocation.BaseBranch,
			BaseCommit:     allocation.BaseCommit,
			WorktreeStatus: allocation.WorktreeStatus,
		})

		result, err := env.manager.Purge(context.Background(), RunLifecycleSettings{
			KeepTerminalDays: 0,
			KeepMax:          0,
		})
		assertErrorContains(t, err, "uncommitted changes")
		if len(result.PurgedRunIDs) != 0 {
			t.Fatalf("purged run ids = %v, want none", result.PurgedRunIDs)
		}
		assertPathPresent(t, allocation.Path)
		if _, err := env.globalDB.GetRun(context.Background(), runID); err != nil {
			t.Fatalf("GetRun(%q) error = %v, want metadata preserved", runID, err)
		}
		runArtifacts, err := model.ResolveHomeRunArtifacts(runID)
		if err != nil {
			t.Fatalf("ResolveHomeRunArtifacts(%q) error = %v", runID, err)
		}
		assertPathPresent(t, runArtifacts.RunDir)
	})
}

func TestRunManagerPurgePreservesCommittedTaskWorktreeAndMetadata(t *testing.T) {
	t.Run("Should preserve committed task worktree and metadata", func(t *testing.T) {
		requireGitForTaskMulti(t)
		env := newRunManagerTestEnv(t, runManagerTestDeps{})
		writeFileForTest(t, filepath.Join(env.workspaceRoot, "README.md"), "seed\n")
		commitTaskMultiGitWorkspace(t, env.workspaceRoot)

		workspace, err := env.globalDB.ResolveOrRegister(context.Background(), env.workspaceRoot)
		if err != nil {
			t.Fatalf("ResolveOrRegister(%q) error = %v", env.workspaceRoot, err)
		}
		runID := "purge-committed-worktree"
		seedTerminalRunForPurge(
			t,
			env.globalDB,
			workspace.ID,
			runID,
			runStatusFailed,
			time.Now().UTC().Add(-time.Hour),
		)
		allocation := allocatePurgeTaskWorktree(t, env, runID, "committed", 1)
		writeFileForTest(t, filepath.Join(allocation.Path, "task-output.txt"), "important output\n")
		runGitOutput(t, allocation.Path, "add", "-A")
		runGitOutput(t, allocation.Path, "commit", "-q", "-m", "task output")
		appendPurgeRunEvent(t, runID, eventspkg.EventKindTaskRunMultipleChildCompleted, kinds.TaskRunMultiplePayload{
			RunID:          runID,
			Slug:           "committed",
			WorktreePath:   allocation.Path,
			BaseBranch:     allocation.BaseBranch,
			BaseCommit:     allocation.BaseCommit,
			WorktreeStatus: allocation.WorktreeStatus,
		})

		result, err := env.manager.Purge(context.Background(), RunLifecycleSettings{
			KeepTerminalDays: 0,
			KeepMax:          0,
		})
		assertErrorContains(t, err, "committed changes")
		if len(result.PurgedRunIDs) != 0 {
			t.Fatalf("purged run ids = %v, want none", result.PurgedRunIDs)
		}
		if len(result.PurgedWorktreePaths) != 0 {
			t.Fatalf("purged worktree paths = %v, want none", result.PurgedWorktreePaths)
		}
		assertPathPresent(t, allocation.Path)
		if _, err := env.globalDB.GetRun(context.Background(), runID); err != nil {
			t.Fatalf("GetRun(%q) error = %v, want metadata preserved", runID, err)
		}
		runArtifacts, err := model.ResolveHomeRunArtifacts(runID)
		if err != nil {
			t.Fatalf("ResolveHomeRunArtifacts(%q) error = %v", runID, err)
		}
		assertPathPresent(t, runArtifacts.RunDir)
	})
}

func TestRunManagerPurgePreflightsAllTaskWorktreesBeforeRemovingAny(t *testing.T) {
	t.Run("Should preflight all task worktrees before removing any", func(t *testing.T) {
		requireGitForTaskMulti(t)
		env := newRunManagerTestEnv(t, runManagerTestDeps{})
		writeFileForTest(t, filepath.Join(env.workspaceRoot, "README.md"), "seed\n")
		commitTaskMultiGitWorkspace(t, env.workspaceRoot)

		workspace, err := env.globalDB.ResolveOrRegister(context.Background(), env.workspaceRoot)
		if err != nil {
			t.Fatalf("ResolveOrRegister(%q) error = %v", env.workspaceRoot, err)
		}
		runID := "purge-preflight-siblings"
		seedTerminalRunForPurge(
			t,
			env.globalDB,
			workspace.ID,
			runID,
			runStatusFailed,
			time.Now().UTC().Add(-time.Hour),
		)
		cleanAllocation := allocatePurgeTaskWorktree(t, env, runID, "clean", 1)
		appendPurgeRunEvent(t, runID, eventspkg.EventKindTaskRunMultipleChildCompleted, kinds.TaskRunMultiplePayload{
			RunID:          runID,
			Slug:           "clean",
			WorktreePath:   cleanAllocation.Path,
			BaseBranch:     cleanAllocation.BaseBranch,
			BaseCommit:     cleanAllocation.BaseCommit,
			WorktreeStatus: cleanAllocation.WorktreeStatus,
		})
		dirtyAllocation := allocatePurgeTaskWorktree(t, env, runID, "dirty", 2)
		writeFileForTest(t, filepath.Join(dirtyAllocation.Path, "dirty.txt"), "dirty\n")
		appendPurgeRunEvent(t, runID, eventspkg.EventKindTaskRunMultipleChildFailed, kinds.TaskRunMultiplePayload{
			RunID:          runID,
			Slug:           "dirty",
			WorktreePath:   dirtyAllocation.Path,
			BaseBranch:     dirtyAllocation.BaseBranch,
			BaseCommit:     dirtyAllocation.BaseCommit,
			WorktreeStatus: dirtyAllocation.WorktreeStatus,
		})

		result, err := env.manager.Purge(context.Background(), RunLifecycleSettings{
			KeepTerminalDays: 0,
			KeepMax:          0,
		})
		assertErrorContains(t, err, "uncommitted changes")
		if len(result.PurgedRunIDs) != 0 {
			t.Fatalf("purged run ids = %v, want none", result.PurgedRunIDs)
		}
		if len(result.PurgedWorktreePaths) != 0 {
			t.Fatalf("purged worktree paths = %v, want none", result.PurgedWorktreePaths)
		}
		assertPathPresent(t, cleanAllocation.Path)
		assertPathPresent(t, dirtyAllocation.Path)
		if _, err := env.globalDB.GetRun(context.Background(), runID); err != nil {
			t.Fatalf("GetRun(%q) error = %v, want metadata preserved", runID, err)
		}
		runArtifacts, err := model.ResolveHomeRunArtifacts(runID)
		if err != nil {
			t.Fatalf("ResolveHomeRunArtifacts(%q) error = %v", runID, err)
		}
		assertPathPresent(t, runArtifacts.RunDir)
	})
}

func TestRunManagerPurgeCommitsEarlierRunsBeforeLaterDirtyWorktreeFails(t *testing.T) {
	t.Run("Should commit earlier runs before later dirty worktree fails", func(t *testing.T) {
		requireGitForTaskMulti(t)
		env := newRunManagerTestEnv(t, runManagerTestDeps{})
		writeFileForTest(t, filepath.Join(env.workspaceRoot, "README.md"), "seed\n")
		commitTaskMultiGitWorkspace(t, env.workspaceRoot)

		workspace, err := env.globalDB.ResolveOrRegister(context.Background(), env.workspaceRoot)
		if err != nil {
			t.Fatalf("ResolveOrRegister(%q) error = %v", env.workspaceRoot, err)
		}
		now := time.Now().UTC()
		cleanRunID := "purge-clean-before-dirty"
		dirtyRunID := "purge-dirty-after-clean"
		seedTerminalRunForPurge(t, env.globalDB, workspace.ID, cleanRunID, runStatusCompleted, now.Add(-2*time.Hour))
		seedTerminalRunForPurge(t, env.globalDB, workspace.ID, dirtyRunID, runStatusFailed, now.Add(-time.Hour))

		cleanAllocation := allocatePurgeTaskWorktree(t, env, cleanRunID, "clean", 1)
		appendPurgeRunEvent(
			t,
			cleanRunID,
			eventspkg.EventKindTaskRunMultipleChildCompleted,
			kinds.TaskRunMultiplePayload{
				RunID:          cleanRunID,
				Slug:           "clean",
				WorktreePath:   cleanAllocation.Path,
				BaseBranch:     cleanAllocation.BaseBranch,
				BaseCommit:     cleanAllocation.BaseCommit,
				WorktreeStatus: cleanAllocation.WorktreeStatus,
			},
		)
		dirtyAllocation := allocatePurgeTaskWorktree(t, env, dirtyRunID, "dirty", 1)
		writeFileForTest(t, filepath.Join(dirtyAllocation.Path, "dirty.txt"), "dirty\n")
		appendPurgeRunEvent(t, dirtyRunID, eventspkg.EventKindTaskRunMultipleChildFailed, kinds.TaskRunMultiplePayload{
			RunID:          dirtyRunID,
			Slug:           "dirty",
			WorktreePath:   dirtyAllocation.Path,
			BaseBranch:     dirtyAllocation.BaseBranch,
			BaseCommit:     dirtyAllocation.BaseCommit,
			WorktreeStatus: dirtyAllocation.WorktreeStatus,
		})

		result, err := env.manager.Purge(context.Background(), RunLifecycleSettings{
			KeepTerminalDays: 0,
			KeepMax:          0,
		})
		assertErrorContains(t, err, "uncommitted changes")
		if got, want := result.PurgedRunIDs, []string{cleanRunID}; !equalStrings(got, want) {
			t.Fatalf("purged run ids = %v, want %v", got, want)
		}
		if got, want := result.PurgedWorktreePaths, []string{cleanAllocation.Path}; !equalStrings(got, want) {
			t.Fatalf("purged worktree paths = %v, want %v", got, want)
		}
		if _, err := env.globalDB.GetRun(context.Background(), cleanRunID); !errors.Is(err, globaldb.ErrRunNotFound) {
			t.Fatalf("GetRun(%q) error = %v, want ErrRunNotFound", cleanRunID, err)
		}
		if _, err := env.globalDB.GetRun(context.Background(), dirtyRunID); err != nil {
			t.Fatalf("GetRun(%q) error = %v, want preserved metadata", dirtyRunID, err)
		}
		cleanArtifacts, err := model.ResolveHomeRunArtifacts(cleanRunID)
		if err != nil {
			t.Fatalf("ResolveHomeRunArtifacts(%q) error = %v", cleanRunID, err)
		}
		dirtyArtifacts, err := model.ResolveHomeRunArtifacts(dirtyRunID)
		if err != nil {
			t.Fatalf("ResolveHomeRunArtifacts(%q) error = %v", dirtyRunID, err)
		}
		assertPathMissing(t, cleanAllocation.Path)
		assertPathMissing(t, cleanArtifacts.RunDir)
		assertPathPresent(t, dirtyAllocation.Path)
		assertPathPresent(t, dirtyArtifacts.RunDir)
	})
}

func TestRunManagerPurgeIgnoresWorktreePathsOutsideOwnedRoot(t *testing.T) {
	t.Run("Should ignore worktree paths outside owned root", func(t *testing.T) {
		env := newRunManagerTestEnv(t, runManagerTestDeps{})

		workspace, err := env.globalDB.ResolveOrRegister(context.Background(), env.workspaceRoot)
		if err != nil {
			t.Fatalf("ResolveOrRegister(%q) error = %v", env.workspaceRoot, err)
		}
		runID := "purge-unsafe-worktree-path"
		seedTerminalRunForPurge(
			t,
			env.globalDB,
			workspace.ID,
			runID,
			runStatusCompleted,
			time.Now().UTC().Add(-time.Hour),
		)
		outsidePath := filepath.Join(t.TempDir(), "outside-worktree")
		if err := os.MkdirAll(outsidePath, 0o755); err != nil {
			t.Fatalf("mkdir outside path: %v", err)
		}
		appendPurgeRunEvent(t, runID, eventspkg.EventKindTaskRunMultipleChildCompleted, kinds.TaskRunMultiplePayload{
			RunID:        runID,
			Slug:         "unsafe",
			WorktreePath: outsidePath,
		})

		result, err := env.manager.Purge(context.Background(), RunLifecycleSettings{
			KeepTerminalDays: 0,
			KeepMax:          0,
		})
		if err != nil {
			t.Fatalf("Purge() error = %v", err)
		}
		if got, want := result.PurgedRunIDs, []string{runID}; !equalStrings(got, want) {
			t.Fatalf("purged run ids = %v, want %v", got, want)
		}
		if len(result.PurgedWorktreePaths) != 0 {
			t.Fatalf("purged worktree paths = %v, want none", result.PurgedWorktreePaths)
		}
		assertPathPresent(t, outsidePath)
		if _, err := env.globalDB.GetRun(context.Background(), runID); !errors.Is(err, globaldb.ErrRunNotFound) {
			t.Fatalf("GetRun(%q) error = %v, want ErrRunNotFound", runID, err)
		}
	})
}

func assertErrorContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want substring %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want substring %q", err, want)
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

func allocatePurgeTaskWorktree(
	t *testing.T,
	env *runManagerTestEnv,
	runID string,
	slug string,
	taskNumber int,
) taskMultiWorktreeAllocation {
	t.Helper()

	base, err := env.manager.worktreeAllocator.ResolveBase(context.Background(), env.workspaceRoot)
	if err != nil {
		t.Fatalf("ResolveBase() error = %v", err)
	}
	allocation, err := env.manager.worktreeAllocator.Allocate(context.Background(), taskMultiWorktreeSpec{
		WorkspaceRoot: env.workspaceRoot,
		ParentRunID:   runID,
		Slug:          slug,
		Index:         taskNumber - 1,
		TaskNumber:    taskNumber,
		Base:          base,
	})
	if err != nil {
		t.Fatalf("Allocate(%q) error = %v", slug, err)
	}
	return allocation
}

func appendPurgeRunEvent(t *testing.T, runID string, kind eventspkg.EventKind, payload any) {
	t.Helper()

	runDB, err := openRunDBForRunID(context.Background(), runID)
	if err != nil {
		t.Fatalf("openRunDBForRunID(%q) error = %v", runID, err)
	}
	defer func() {
		_ = runDB.Close()
	}()
	if _, err := runDB.AppendSyntheticEvent(context.Background(), kind, payload); err != nil {
		t.Fatalf("AppendSyntheticEvent(%q) error = %v", kind, err)
	}
}

func assertPathMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Stat(%q) error = %v, want os.ErrNotExist", path, err)
	}
}

func assertPathPresent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("Stat(%q) error = %v, want present", path, err)
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
