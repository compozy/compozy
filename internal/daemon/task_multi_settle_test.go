package daemon

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/store/globaldb"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
)

// TestWaitForSettledTaskMultiChild covers the settle gate that both parallel
// failure paths rely on before touching a child's worktree (issues 004 and 006):
// removal must never race a live child subprocess.
func TestWaitForSettledTaskMultiChild(t *testing.T) {
	// newBackstopEnv relies on t.Setenv, which is incompatible with t.Parallel.

	t.Run("Should treat an empty run id as settled", func(t *testing.T) {
		env := newBackstopEnv(t, nil)
		if !env.manager.waitForSettledTaskMultiChild(context.Background(), "  ", time.Second) {
			t.Fatal("an empty run id has no live child and must settle immediately")
		}
	})

	t.Run("Should report an already terminal child as settled", func(t *testing.T) {
		const runID = "settle-terminal-child"
		env := newBackstopEnv(t, nil, runID)
		env.settle(runID, runStatusCompleted)
		if !env.manager.waitForSettledTaskMultiChild(context.Background(), runID, time.Second) {
			t.Fatal("a terminal child must be reported settled")
		}
	})

	t.Run("Should report a still-running child as not settled within the bound", func(t *testing.T) {
		const runID = "settle-running-child"
		env := newBackstopEnv(t, nil, runID)
		// activate registers an active child that only settles on cancellation; the
		// wait detaches the context, so nothing cancels it and it stays running.
		env.activate(t, runID, runStatusCancelled)
		if env.manager.waitForSettledTaskMultiChild(context.Background(), runID, 150*time.Millisecond) {
			t.Fatal("a child that never settles must not be reported settled")
		}
	})

	t.Run("Should report settled once the child closes its done channel", func(t *testing.T) {
		const runID = "settle-done-child"
		env := newBackstopEnv(t, nil, runID)
		active := env.activate(t, runID, runStatusCancelled)
		// A real child closes active.done after it writes its terminal status; closing
		// it here releases the wait through the event-driven path.
		close(active.done)
		if !env.manager.waitForSettledTaskMultiChild(context.Background(), runID, 5*time.Second) {
			t.Fatal("a child that closed its done channel must be reported settled")
		}
	})
}

// TestCleanupSettledTaskMultiChildWorktree proves the issue-004 regression fix:
// a plain-parallel child (whose policy would otherwise remove the worktree) keeps
// its worktree while the child has not settled, so cleanup never deletes a
// directory out from under a live agent subprocess.
func TestCleanupSettledTaskMultiChildWorktree(t *testing.T) {
	// newBackstopEnv relies on t.Setenv, which is incompatible with t.Parallel.

	t.Run("Should preserve a plain-parallel worktree while its child is still running", func(t *testing.T) {
		const runID = "cleanup-running-child"
		env := newBackstopEnv(t, nil, runID)
		env.activate(t, runID, runStatusCancelled)
		prepared := &preparedTaskMulti{
			// Plain parallel sets preserve=false, so a settled child WOULD be removed;
			// the unsettled child must still be preserved.
			executionKind: apicore.ExecutionKindTaskMultiParallel,
			workspace:     globaldb.Workspace{RootDir: t.TempDir()},
		}
		worktreePath := filepath.Join(t.TempDir(), "worktree")
		child := taskWorktreeChildRun{
			Run: apicore.Run{RunID: runID},
			Allocation: taskMultiWorktreeAllocation{
				Path:           worktreePath,
				ResultBranch:   "compozy/result",
				WorktreeStatus: taskMultiWorktreeStatusActive,
			},
		}
		got := env.manager.cleanupSettledTaskMultiChildWorktree(
			context.Background(),
			prepared,
			child,
			150*time.Millisecond,
		)
		if got.WorktreeStatus != taskMultiWorktreeStatusPreserved {
			t.Fatalf("WorktreeStatus = %q, want %q", got.WorktreeStatus, taskMultiWorktreeStatusPreserved)
		}
		if got.Path != worktreePath {
			t.Fatalf("Path = %q, want unchanged %q", got.Path, worktreePath)
		}
		if got.ResultBranch != "compozy/result" {
			t.Fatalf("ResultBranch = %q, want retained result branch", got.ResultBranch)
		}
		if !strings.Contains(got.WorktreeReason, "did not settle") {
			t.Fatalf("WorktreeReason = %q, want the unsettled preservation reason", got.WorktreeReason)
		}
	})

	t.Run("Should return the allocation unchanged when no worktree was allocated", func(t *testing.T) {
		env := newBackstopEnv(t, nil)
		prepared := &preparedTaskMulti{
			executionKind: apicore.ExecutionKindTaskMultiParallel,
			workspace:     globaldb.Workspace{RootDir: t.TempDir()},
		}
		child := taskWorktreeChildRun{Run: apicore.Run{RunID: ""}}
		got := env.manager.cleanupSettledTaskMultiChildWorktree(
			context.Background(),
			prepared,
			child,
			time.Second,
		)
		if got.Path != "" || got.WorktreeStatus != "" || got.WorktreeReason != "" {
			t.Fatalf("allocation = %#v, want the zero allocation left untouched", got)
		}
	})
}

// TestSettleTaskMultiParallelStartFailureRecordsLaunchedChildRunID proves the
// issue-006 fix: a start failure that already launched a child records that
// child's run id in the ChildFailed payload instead of an empty string, so
// operators, cancellation, and the recovery summary can still find the run.
func TestSettleTaskMultiParallelStartFailureRecordsLaunchedChildRunID(t *testing.T) {
	t.Parallel()

	parentRunID := "task-multi-start-failure-runid"
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseAll := func() { releaseOnce.Do(func() { close(release) }) }
	started := make(chan struct{}, 1)
	env := newRunManagerTestEnv(t, runManagerTestDeps{
		buildRunID: taskMultiRunIDBuilder(parentRunID),
		prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
			return &model.SolvePreparation{}, nil
		},
		execute: func(ctx context.Context, _ *model.SolvePreparation, _ *model.RuntimeConfig) error {
			select {
			case started <- struct{}{}:
			default:
			}
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	})
	t.Cleanup(releaseAll)
	writeTaskMultiWorkflow(t, env, "alpha", "pending")

	parent := startTaskMultiRun(t, env, parentRunID, []string{"alpha"})
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("child alpha never started")
	}
	active := env.manager.getActive(parent.RunID)
	if active == nil {
		t.Fatalf("parent run %s is not active", parent.RunID)
	}

	prepared := &preparedTaskMulti{
		mode:          "parallel",
		executionKind: apicore.ExecutionKindTaskMultiParallel,
		workspace:     globaldb.Workspace{RootDir: env.workspaceRoot},
	}
	// The launched-but-unacknowledged child: startTaskMultiWorktreeChild returns a
	// populated Run together with the emit error, and settle must not drop its id.
	child := taskWorktreeChildRun{Run: apicore.Run{RunID: "phantom-child"}}
	err := env.manager.settleTaskMultiParallelStartFailure(
		active,
		prepared,
		preparedTaskMultiItem{slug: "phantom"},
		1,
		2,
		child,
		errors.New("child_started emit failed"),
	)
	if err == nil || !strings.Contains(err.Error(), "child_started emit failed") {
		t.Fatalf("settleTaskMultiParallelStartFailure() error = %v, want the start failure", err)
	}

	// Let the batch finish so the parent journal is fully flushed before it is read
	// back through a fresh run-DB handle.
	releaseAll()
	waitForRun(t, env.globalDB, parent.RunID, func(row globaldb.Run) bool {
		return isTerminalRunStatus(row.Status)
	})

	found := false
	for _, event := range allRunEvents(t, env.manager, parent.RunID) {
		if event.Kind != eventspkg.EventKindTaskRunMultipleChildFailed {
			continue
		}
		payload, decodeErr := decodeTaskMultiPayload(event)
		if decodeErr != nil {
			t.Fatalf("decode child_failed payload: %v", decodeErr)
		}
		if payload.Slug != "phantom" {
			continue
		}
		found = true
		if payload.ChildRunID != "phantom-child" {
			t.Fatalf("child_failed ChildRunID = %q, want phantom-child", payload.ChildRunID)
		}
		if payload.Status != taskMultiItemStatusFailed {
			t.Fatalf("child_failed Status = %q, want %q", payload.Status, taskMultiItemStatusFailed)
		}
	}
	if !found {
		t.Fatal("no child_failed event recorded the launched child's run id")
	}
}
