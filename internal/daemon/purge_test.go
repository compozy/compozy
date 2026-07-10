package daemon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
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
		seedTerminalRunForPurge(t, env.manager, env.globalDB, workspace.ID, item.runID, item.status, item.endedAt)
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
		runArtifacts := env.manager.runArtifacts(runID)
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

	runArtifacts := env.manager.runArtifacts(activeRun.RunID)
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
		env.manager,
		env.globalDB,
		workspace.ID,
		runID,
		runStatusCompleted,
		time.Now().UTC().AddDate(0, 0, -30),
	)

	result, err := PurgeTerminalRuns(context.Background(), env.globalDB, RunLifecycleSettings{
		KeepTerminalDays: 0,
		KeepMax:          0,
		RunsDir:          env.manager.homePaths.RunsDir,
	})
	if err != nil {
		t.Fatalf("PurgeTerminalRuns() error = %v", err)
	}
	if got, want := result.PurgedRunIDs, []string{runID}; !equalStrings(got, want) {
		t.Fatalf("purged run ids = %v, want %v", got, want)
	}
	if _, err := os.Stat(env.manager.runArtifacts(runID).RunDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("purged run dir stat error = %v, want not exist", err)
	}
}

func TestPurgeTerminalRunsRejectsUnsafeRunsDirectory(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		runsDir string
		want    string
	}{
		{name: "missing", want: "captured runs directory is required"},
		{name: "relative", runsDir: "relative/runs", want: "must be absolute"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := PurgeTerminalRuns(context.Background(), nil, RunLifecycleSettings{RunsDir: tc.runsDir})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("PurgeTerminalRuns() error = %v, want %q", err, tc.want)
			}
		})
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
			env.manager,
			env.globalDB,
			workspace.ID,
			runID,
			runStatusCompleted,
			time.Now().UTC().Add(-time.Hour),
		)
		allocation := allocatePurgeTaskWorktree(t, env, runID, "alpha", 1)
		appendPurgeRunEvent(
			t,
			env.manager,
			runID,
			eventspkg.EventKindTaskRunMultipleChildCompleted,
			kinds.TaskRunMultiplePayload{
				RunID:          runID,
				Slug:           "alpha",
				Index:          0,
				WorktreePath:   allocation.Path,
				BaseBranch:     allocation.BaseBranch,
				BaseCommit:     allocation.BaseCommit,
				WorktreeStatus: allocation.WorktreeStatus,
			},
		)

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
			env.manager,
			env.globalDB,
			workspace.ID,
			runID,
			runStatusCrashed,
			time.Now().UTC().Add(-time.Hour),
		)
		allocation := allocatePurgeTaskWorktree(t, env, runID, "parallel", 1)
		appendPurgeRunEvent(
			t,
			env.manager,
			runID,
			eventspkg.EventKindTaskParallelTaskStarted,
			kinds.TaskParallelPayload{
				RunID:        runID,
				TaskID:       "task_01",
				ChildRunID:   "child-task-01",
				WorktreePath: allocation.Path,
			},
		)

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
			env.manager,
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
		appendPurgeRunEvent(
			t,
			env.manager,
			runID,
			eventspkg.EventKindTaskParallelTaskStarted,
			kinds.TaskParallelPayload{
				RunID:             runID,
				IntegrationBranch: integrationBranch,
				Phase:             "task_started",
			},
		)

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
			env.manager,
			env.globalDB,
			workspace.ID,
			runID,
			runStatusFailed,
			time.Now().UTC().Add(-time.Hour),
		)
		allocation := allocatePurgeTaskWorktree(t, env, runID, "dirty", 1)
		writeFileForTest(t, filepath.Join(allocation.Path, "dirty.txt"), "dirty\n")
		appendPurgeRunEvent(
			t,
			env.manager,
			runID,
			eventspkg.EventKindTaskRunMultipleChildFailed,
			kinds.TaskRunMultiplePayload{
				RunID:          runID,
				Slug:           "dirty",
				WorktreePath:   allocation.Path,
				BaseBranch:     allocation.BaseBranch,
				BaseCommit:     allocation.BaseCommit,
				WorktreeStatus: allocation.WorktreeStatus,
			},
		)

		result, err := env.manager.Purge(context.Background(), RunLifecycleSettings{
			KeepTerminalDays: 0,
			KeepMax:          0,
		})
		assertErrorContains(t, err, "uncommitted changes")
		assertErrorContains(t, err, runID)
		if len(result.PurgedRunIDs) != 0 {
			t.Fatalf("purged run ids = %v, want none", result.PurgedRunIDs)
		}
		assertPathPresent(t, allocation.Path)
		if _, err := env.globalDB.GetRun(context.Background(), runID); err != nil {
			t.Fatalf("GetRun(%q) error = %v, want metadata preserved", runID, err)
		}
		runArtifacts := env.manager.runArtifacts(runID)
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
			env.manager,
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
		appendPurgeRunEvent(
			t,
			env.manager,
			runID,
			eventspkg.EventKindTaskRunMultipleChildCompleted,
			kinds.TaskRunMultiplePayload{
				RunID:          runID,
				Slug:           "committed",
				WorktreePath:   allocation.Path,
				BaseBranch:     allocation.BaseBranch,
				BaseCommit:     allocation.BaseCommit,
				WorktreeStatus: allocation.WorktreeStatus,
			},
		)

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
		runArtifacts := env.manager.runArtifacts(runID)
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
			env.manager,
			env.globalDB,
			workspace.ID,
			runID,
			runStatusFailed,
			time.Now().UTC().Add(-time.Hour),
		)
		cleanAllocation := allocatePurgeTaskWorktree(t, env, runID, "clean", 1)
		appendPurgeRunEvent(
			t,
			env.manager,
			runID,
			eventspkg.EventKindTaskRunMultipleChildCompleted,
			kinds.TaskRunMultiplePayload{
				RunID:          runID,
				Slug:           "clean",
				WorktreePath:   cleanAllocation.Path,
				BaseBranch:     cleanAllocation.BaseBranch,
				BaseCommit:     cleanAllocation.BaseCommit,
				WorktreeStatus: cleanAllocation.WorktreeStatus,
			},
		)
		dirtyAllocation := allocatePurgeTaskWorktree(t, env, runID, "dirty", 2)
		writeFileForTest(t, filepath.Join(dirtyAllocation.Path, "dirty.txt"), "dirty\n")
		appendPurgeRunEvent(
			t,
			env.manager,
			runID,
			eventspkg.EventKindTaskRunMultipleChildFailed,
			kinds.TaskRunMultiplePayload{
				RunID:          runID,
				Slug:           "dirty",
				WorktreePath:   dirtyAllocation.Path,
				BaseBranch:     dirtyAllocation.BaseBranch,
				BaseCommit:     dirtyAllocation.BaseCommit,
				WorktreeStatus: dirtyAllocation.WorktreeStatus,
			},
		)

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
		runArtifacts := env.manager.runArtifacts(runID)
		assertPathPresent(t, runArtifacts.RunDir)
	})
}

func TestRunManagerPurgePreservesWorktreeHostingActiveNestedRun(t *testing.T) {
	t.Run("Should preserve worktree and parent metadata while nested run is active", func(t *testing.T) {
		requireGitForTaskMulti(t)
		started := make(chan string, 1)
		release := make(chan struct{})
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
				return &model.SolvePreparation{}, nil
			},
			execute: func(ctx context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
				started <- cfg.RunID
				select {
				case <-release:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			},
		})
		defer close(release)
		env.writeWorkflowFile(t, env.workflowSlug, "task_01.md", validPurgeTaskMarkdown())
		writeFileForTest(t, filepath.Join(env.workspaceRoot, "README.md"), "seed\n")
		commitTaskMultiGitWorkspace(t, env.workspaceRoot)

		workspace, err := env.globalDB.ResolveOrRegister(context.Background(), env.workspaceRoot)
		if err != nil {
			t.Fatalf("ResolveOrRegister(%q) error = %v", env.workspaceRoot, err)
		}
		runID := "purge-parent-with-live-nested"
		seedTerminalRunForPurge(
			t,
			env.manager,
			env.globalDB,
			workspace.ID,
			runID,
			runStatusCompleted,
			time.Now().UTC().Add(-time.Hour),
		)
		allocation := allocatePurgeTaskWorktree(t, env, runID, "nested", 1)
		appendPurgeRunEvent(
			t,
			env.manager,
			runID,
			eventspkg.EventKindTaskRunMultipleChildCompleted,
			kinds.TaskRunMultiplePayload{
				RunID:          runID,
				Slug:           "nested",
				WorktreePath:   allocation.Path,
				BaseBranch:     allocation.BaseBranch,
				BaseCommit:     allocation.BaseCommit,
				WorktreeStatus: allocation.WorktreeStatus,
			},
		)

		nestedRun, err := env.manager.StartTaskRun(
			context.Background(),
			allocation.Path,
			env.workflowSlug,
			apicore.TaskRunRequest{
				Workspace:        allocation.Path,
				PresentationMode: defaultPresentationMode,
				RuntimeOverrides: rawJSON(t, `{"run_id":"nested-live-run"}`),
			},
		)
		if err != nil {
			t.Fatalf("StartTaskRun(nested) error = %v", err)
		}
		waitForString(t, started, nestedRun.RunID)

		result, err := env.manager.Purge(context.Background(), RunLifecycleSettings{
			KeepTerminalDays: 0,
			KeepMax:          0,
		})
		if err != nil {
			t.Fatalf("Purge() error = %v", err)
		}
		if len(result.PurgedRunIDs) != 0 {
			t.Fatalf("purged run ids = %v, want none while nested run is active", result.PurgedRunIDs)
		}
		if len(result.PurgedWorktreePaths) != 0 {
			t.Fatalf("purged worktree paths = %v, want none while nested run is active", result.PurgedWorktreePaths)
		}
		assertPathPresent(t, allocation.Path)
		if _, err := env.globalDB.GetRun(context.Background(), runID); err != nil {
			t.Fatalf("GetRun(%q) error = %v, want parent metadata preserved", runID, err)
		}
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
		seedTerminalRunForPurge(
			t,
			env.manager,
			env.globalDB,
			workspace.ID,
			cleanRunID,
			runStatusCompleted,
			now.Add(-2*time.Hour),
		)
		seedTerminalRunForPurge(
			t,
			env.manager,
			env.globalDB,
			workspace.ID,
			dirtyRunID,
			runStatusFailed,
			now.Add(-time.Hour),
		)

		cleanAllocation := allocatePurgeTaskWorktree(t, env, cleanRunID, "clean", 1)
		appendPurgeRunEvent(
			t,
			env.manager,
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
		appendPurgeRunEvent(
			t,
			env.manager,
			dirtyRunID,
			eventspkg.EventKindTaskRunMultipleChildFailed,
			kinds.TaskRunMultiplePayload{
				RunID:          dirtyRunID,
				Slug:           "dirty",
				WorktreePath:   dirtyAllocation.Path,
				BaseBranch:     dirtyAllocation.BaseBranch,
				BaseCommit:     dirtyAllocation.BaseCommit,
				WorktreeStatus: dirtyAllocation.WorktreeStatus,
			},
		)

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
		cleanArtifacts := env.manager.runArtifacts(cleanRunID)
		dirtyArtifacts := env.manager.runArtifacts(dirtyRunID)
		assertPathMissing(t, cleanAllocation.Path)
		assertPathMissing(t, cleanArtifacts.RunDir)
		assertPathPresent(t, dirtyAllocation.Path)
		assertPathPresent(t, dirtyArtifacts.RunDir)
	})
}

func TestRunManagerPurgeSkipsDeletedWorkspaceRootAndContinues(t *testing.T) {
	t.Run("Should skip missing registered workspace root without blocking other purges", func(t *testing.T) {
		requireGitForTaskMulti(t)
		env := newRunManagerTestEnv(t, runManagerTestDeps{})
		env.writeWorkflowFile(t, env.workflowSlug, "task_01.md", validPurgeTaskMarkdown())
		writeFileForTest(t, filepath.Join(env.workspaceRoot, "README.md"), "seed\n")
		commitTaskMultiGitWorkspace(t, env.workspaceRoot)

		missingWorkspace, err := env.globalDB.ResolveOrRegister(context.Background(), env.workspaceRoot)
		if err != nil {
			t.Fatalf("ResolveOrRegister(%q) error = %v", env.workspaceRoot, err)
		}
		missingRunID := "purge-missing-workspace-root"
		seedTerminalRunForPurge(
			t,
			env.manager,
			env.globalDB,
			missingWorkspace.ID,
			missingRunID,
			runStatusCompleted,
			time.Now().UTC().Add(-2*time.Hour),
		)
		missingAllocation := allocatePurgeTaskWorktree(t, env, missingRunID, "missing", 1)
		appendPurgeRunEvent(
			t,
			env.manager,
			missingRunID,
			eventspkg.EventKindTaskRunMultipleChildCompleted,
			kinds.TaskRunMultiplePayload{
				RunID:          missingRunID,
				Slug:           "missing",
				WorktreePath:   missingAllocation.Path,
				BaseBranch:     missingAllocation.BaseBranch,
				BaseCommit:     missingAllocation.BaseCommit,
				WorktreeStatus: missingAllocation.WorktreeStatus,
			},
		)

		otherRoot := filepath.Join(t.TempDir(), "other-workspace")
		writeFileForTest(
			t,
			filepath.Join(otherRoot, ".compozy", "tasks", env.workflowSlug, "task_01.md"),
			validPurgeTaskMarkdown(),
		)
		writeFileForTest(t, filepath.Join(otherRoot, "README.md"), "other\n")
		commitTaskMultiGitWorkspace(t, otherRoot)
		otherWorkspace, err := env.globalDB.ResolveOrRegister(context.Background(), otherRoot)
		if err != nil {
			t.Fatalf("ResolveOrRegister(%q) error = %v", otherRoot, err)
		}
		otherRunID := "purge-other-workspace-root"
		seedTerminalRunForPurge(
			t,
			env.manager,
			env.globalDB,
			otherWorkspace.ID,
			otherRunID,
			runStatusCompleted,
			time.Now().UTC().Add(-time.Hour),
		)
		otherAllocation := allocatePurgeTaskWorktreeForRoot(t, env, otherRoot, otherRunID, "other", 1)
		appendPurgeRunEvent(
			t,
			env.manager,
			otherRunID,
			eventspkg.EventKindTaskRunMultipleChildCompleted,
			kinds.TaskRunMultiplePayload{
				RunID:          otherRunID,
				Slug:           "other",
				WorktreePath:   otherAllocation.Path,
				BaseBranch:     otherAllocation.BaseBranch,
				BaseCommit:     otherAllocation.BaseCommit,
				WorktreeStatus: otherAllocation.WorktreeStatus,
			},
		)

		if err := os.RemoveAll(env.workspaceRoot); err != nil {
			t.Fatalf("remove missing workspace root: %v", err)
		}
		result, err := env.manager.Purge(context.Background(), RunLifecycleSettings{
			KeepTerminalDays: 0,
			KeepMax:          0,
		})
		if err != nil {
			t.Fatalf("Purge() error = %v", err)
		}
		if got, want := result.PurgedRunIDs, []string{otherRunID}; !equalStrings(got, want) {
			t.Fatalf("purged run ids = %v, want %v", got, want)
		}
		if got, want := result.PurgedWorktreePaths, []string{otherAllocation.Path}; !equalStrings(got, want) {
			t.Fatalf("purged worktree paths = %v, want %v", got, want)
		}
		assertPathPresent(t, missingAllocation.Path)
		if _, err := env.globalDB.GetRun(context.Background(), missingRunID); err != nil {
			t.Fatalf("GetRun(%q) error = %v, want missing-root run metadata preserved", missingRunID, err)
		}
		assertPathMissing(t, otherAllocation.Path)
		if _, err := env.globalDB.GetRun(context.Background(), otherRunID); !errors.Is(err, globaldb.ErrRunNotFound) {
			t.Fatalf("GetRun(%q) error = %v, want ErrRunNotFound", otherRunID, err)
		}
	})
}

func TestRunManagerPurgeDefersWorktreePathsOutsideOwnedRoot(t *testing.T) {
	t.Run("Should preserve run metadata when a worktree belongs to another home root", func(t *testing.T) {
		env := newRunManagerTestEnv(t, runManagerTestDeps{})

		workspace, err := env.globalDB.ResolveOrRegister(context.Background(), env.workspaceRoot)
		if err != nil {
			t.Fatalf("ResolveOrRegister(%q) error = %v", env.workspaceRoot, err)
		}
		runID := "purge-unsafe-worktree-path"
		seedTerminalRunForPurge(
			t,
			env.manager,
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
		appendPurgeRunEvent(
			t,
			env.manager,
			runID,
			eventspkg.EventKindTaskRunMultipleChildCompleted,
			kinds.TaskRunMultiplePayload{
				RunID:        runID,
				Slug:         "unsafe",
				WorktreePath: outsidePath,
			},
		)

		result, err := env.manager.Purge(context.Background(), RunLifecycleSettings{
			KeepTerminalDays: 0,
			KeepMax:          0,
		})
		if err != nil {
			t.Fatalf("Purge() error = %v", err)
		}
		if got, want := result.PurgedRunIDs, []string{}; !equalStrings(got, want) {
			t.Fatalf("purged run ids = %v, want %v", got, want)
		}
		if len(result.PurgedWorktreePaths) != 0 {
			t.Fatalf("purged worktree paths = %v, want none", result.PurgedWorktreePaths)
		}
		assertPathPresent(t, outsidePath)
		if _, err := env.globalDB.GetRun(context.Background(), runID); err != nil {
			t.Fatalf("GetRun(%q) error = %v, want deferred run metadata preserved", runID, err)
		}
	})
}

func TestResolveIntegrationPurgePathRejectsFallbackOutsideWorktreeRoot(t *testing.T) {
	t.Parallel()
	worktreesRoot := filepath.Join(t.TempDir(), "worktrees")
	if err := os.MkdirAll(worktreesRoot, 0o755); err != nil {
		t.Fatalf("mkdir worktrees root: %v", err)
	}
	allocator := &taskMultiWorktreeAllocator{
		run: func(_ context.Context, _ string, args ...string) (string, error) {
			if strings.Join(args, " ") != "worktree list --porcelain" {
				t.Fatalf("unexpected git command: %v", args)
			}
			return "", nil
		},
	}

	outsidePath := filepath.Join(t.TempDir(), "integration")
	_, err := allocator.resolveIntegrationPurgePath(
		context.Background(),
		t.TempDir(),
		worktreesRoot,
		outsidePath,
		"compozy/integration/run-1",
	)
	assertErrorContains(t, err, "outside worktree root")
}

func TestCleanOwnedWorktreePathRejectsSymlinkEscapesWorktreeRoot(t *testing.T) {
	t.Parallel()
	worktreesRoot := filepath.Join(t.TempDir(), "worktrees")
	outsideRoot := filepath.Join(t.TempDir(), "outside")
	if err := os.MkdirAll(worktreesRoot, 0o755); err != nil {
		t.Fatalf("mkdir worktrees root: %v", err)
	}
	if err := os.MkdirAll(outsideRoot, 0o755); err != nil {
		t.Fatalf("mkdir outside root: %v", err)
	}
	if err := os.Symlink(outsideRoot, filepath.Join(worktreesRoot, "link")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	_, ok, err := cleanOwnedWorktreePath(worktreesRoot, filepath.Join(worktreesRoot, "link", "repo"))
	if err != nil {
		t.Fatalf("cleanOwnedWorktreePath() error = %v", err)
	}
	if ok {
		t.Fatal("cleanOwnedWorktreePath() ok = true, want false for symlink escape")
	}
}

func TestCleanOwnedWorktreePathAcceptsMissingChildUnderOwnedRoot(t *testing.T) {
	t.Parallel()
	worktreesRoot := filepath.Join(t.TempDir(), "worktrees")
	if err := os.MkdirAll(worktreesRoot, 0o755); err != nil {
		t.Fatalf("mkdir worktrees root: %v", err)
	}
	path := filepath.Join(worktreesRoot, "missing", "repo")

	got, ok, err := cleanOwnedWorktreePath(worktreesRoot, path)
	if err != nil {
		t.Fatalf("cleanOwnedWorktreePath() error = %v", err)
	}
	if !ok {
		t.Fatal("cleanOwnedWorktreePath() ok = false, want true for missing child under root")
	}
	if got != path {
		t.Fatalf("cleanOwnedWorktreePath() path = %q, want %q", got, path)
	}
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
	manager *RunManager,
	db *globaldb.GlobalDB,
	workspaceID string,
	runID string,
	status string,
	endedAt time.Time,
) {
	t.Helper()

	runArtifacts := manager.runArtifacts(runID)
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
	return allocatePurgeTaskWorktreeForRoot(t, env, env.workspaceRoot, runID, slug, taskNumber)
}

func allocatePurgeTaskWorktreeForRoot(
	t *testing.T,
	env *runManagerTestEnv,
	workspaceRoot string,
	runID string,
	slug string,
	taskNumber int,
) taskMultiWorktreeAllocation {
	t.Helper()

	base, err := env.manager.worktreeAllocator.ResolveBase(context.Background(), workspaceRoot)
	if err != nil {
		t.Fatalf("ResolveBase() error = %v", err)
	}
	allocation, err := env.manager.worktreeAllocator.Allocate(context.Background(), taskMultiWorktreeSpec{
		WorkspaceRoot: workspaceRoot,
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

func validPurgeTaskMarkdown() string {
	return strings.Join([]string{
		"---",
		"status: pending",
		"title: Demo Task",
		"type: backend",
		"complexity: low",
		"---",
		"",
		"# Task 1: Demo Task",
		"",
	}, "\n")
}

func appendPurgeRunEvent(
	t *testing.T,
	manager *RunManager,
	runID string,
	kind eventspkg.EventKind,
	payload any,
) {
	t.Helper()

	runDB, err := manager.openRunDB(context.Background(), runID)
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
