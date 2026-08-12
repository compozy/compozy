package globaldb

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	worktreepkg "github.com/compozy/compozy/internal/worktree"
)

// Canonical suite: workspace-predicated worktree storage and SQL constraints.
func TestGlobalDBWorktreeStore(t *testing.T) {
	t.Parallel()

	t.Run("Should isolate every repository read and side-table mutation by workspace", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceA := registerWorkspaceForGlobalTests(t, globalDB, "worktree-a", filepath.Join(t.TempDir(), "a"))
		workspaceB := registerWorkspaceForGlobalTests(t, globalDB, "worktree-b", filepath.Join(t.TempDir(), "b"))
		now := time.Date(2026, 8, 12, 17, 0, 0, 0, time.UTC)
		item := worktreepkg.Worktree{
			ID: "wt_store_a", WorkspaceID: workspaceA, Name: "store-a", Branch: "feature/a",
			Path: filepath.Join(t.TempDir(), "store-a"), State: worktreepkg.StateReady,
			Origin: worktreepkg.OriginManual, SetupState: worktreepkg.SetupNone,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := globalDB.Worktrees.Insert(ctx, item); err != nil {
			t.Fatalf("Insert() error = %v", err)
		}
		if _, err := globalDB.Worktrees.Get(ctx, workspaceB, item.ID); !errors.Is(err, worktreepkg.ErrNotFound) {
			t.Fatalf("Get(cross-workspace) error = %v, want ErrNotFound", err)
		}
		if _, err := globalDB.Worktrees.GetByPath(ctx, workspaceB, item.Path); !errors.Is(err, worktreepkg.ErrNotFound) {
			t.Fatalf("GetByPath(cross-workspace) error = %v, want ErrNotFound", err)
		}
		if rows, err := globalDB.Worktrees.List(ctx, workspaceB); err != nil || len(rows) != 0 {
			t.Fatalf("List(cross-workspace) = %#v, %v, want empty", rows, err)
		}
		if swapped, err := globalDB.Worktrees.CompareAndSwapState(
			ctx, workspaceB, item.ID, worktreepkg.StateReady, worktreepkg.StateRemoving, now,
		); err != nil || swapped {
			t.Fatalf("CompareAndSwapState(cross-workspace) = %v, %v, want false nil", swapped, err)
		}
		if err := globalDB.Worktrees.SetState(
			ctx, workspaceB, item.ID, worktreepkg.StateRemoving, now,
		); !errors.Is(err, worktreepkg.ErrNotFound) {
			t.Fatalf("SetState(cross-workspace) error = %v, want ErrNotFound", err)
		}
		if err := globalDB.Worktrees.UpdatePending(
			ctx,
			workspaceB,
			item.ID,
			worktreepkg.PendingUpdate{Phase: worktreepkg.PhaseCheckout},
		); !errors.Is(err, worktreepkg.ErrNotFound) {
			t.Fatalf("UpdatePending(cross-workspace) error = %v, want ErrNotFound", err)
		}
		if err := globalDB.Worktrees.CompleteCreate(
			ctx, workspaceB, item.ID, worktreepkg.SetupNone, "", now,
		); !errors.Is(err, worktreepkg.ErrNotFound) {
			t.Fatalf("CompleteCreate(cross-workspace) error = %v, want ErrNotFound", err)
		}
		if err := globalDB.Worktrees.DeletePending(ctx, workspaceB, item.ID); !errors.Is(
			err,
			worktreepkg.ErrNotFound,
		) {
			t.Fatalf("DeletePending(cross-workspace) error = %v, want ErrNotFound", err)
		}
		if err := globalDB.Worktrees.SaveStatus(ctx, workspaceB, item.ID, worktreepkg.Status{
			WorktreeID: item.ID,
		}); !errors.Is(err, worktreepkg.ErrNotFound) {
			t.Fatalf("SaveStatus(cross-workspace) error = %v, want ErrNotFound", err)
		}
		if status, err := globalDB.Worktrees.GetStatus(ctx, workspaceB, item.ID); err != nil || status != nil {
			t.Fatalf("GetStatus(cross-workspace) = %#v, %v, want nil", status, err)
		}
		if err := globalDB.Worktrees.SaveForgeStatus(ctx, workspaceB, item.ID, worktreepkg.ForgeStatus{
			WorktreeID: item.ID,
		}); !errors.Is(err, worktreepkg.ErrNotFound) {
			t.Fatalf("SaveForgeStatus(cross-workspace) error = %v, want ErrNotFound", err)
		}
		if forge, err := globalDB.Worktrees.GetForgeStatus(ctx, workspaceB, item.ID); err != nil || forge != nil {
			t.Fatalf("GetForgeStatus(cross-workspace) = %#v, %v, want nil", forge, err)
		}
		exitOperation := worktreepkg.ExitOperation{
			ID: "op-isolation", WorkspaceID: workspaceA, WorktreeID: item.ID,
			Action: "push", State: "running", StartedAt: now,
		}
		if err := globalDB.Worktrees.InsertExitOperation(ctx, exitOperation); err != nil {
			t.Fatalf("InsertExitOperation() error = %v", err)
		}
		if finished, err := globalDB.Worktrees.FinishExitOperation(
			ctx, workspaceB, item.ID, exitOperation.ID, "failed", now,
		); err != nil || finished {
			t.Fatalf("FinishExitOperation(cross-workspace) = %v, %v, want false nil", finished, err)
		}
		branch, dirty, aheadOfBase, refreshed := "feature/a", 2, 4, now
		hasUpstream := false
		if err := globalDB.Worktrees.SaveStatus(ctx, workspaceA, item.ID, worktreepkg.Status{
			WorktreeID: item.ID, Branch: &branch, DirtyFiles: &dirty, HasUpstream: &hasUpstream,
			AheadOfBase: &aheadOfBase, RefreshedAt: &refreshed,
		}); err != nil {
			t.Fatalf("SaveStatus() error = %v", err)
		}
		status, err := globalDB.Worktrees.GetStatus(ctx, workspaceA, item.ID)
		if err != nil || status == nil || status.DirtyFiles == nil || *status.DirtyFiles != 2 ||
			status.Ahead != nil || status.AheadOfBase == nil || *status.AheadOfBase != aheadOfBase {
			t.Fatalf("GetStatus() = %#v, %v, want dirty_files=2 and ahead_of_base=4", status, err)
		}
		provider, prState, merged, fetchedAt := "forge-test", "open", false, now.Add(time.Minute)
		if err := globalDB.Worktrees.SaveForgeStatus(ctx, workspaceA, item.ID, worktreepkg.ForgeStatus{
			WorktreeID: item.ID, Provider: provider, PRState: &prState, Merged: &merged, FetchedAt: &fetchedAt,
		}); err != nil {
			t.Fatalf("SaveForgeStatus() error = %v", err)
		}
		forge, err := globalDB.Worktrees.GetForgeStatus(ctx, workspaceA, item.ID)
		if err != nil || forge == nil || forge.Provider != provider || forge.PRState == nil ||
			*forge.PRState != prState || forge.FetchedAt == nil || !forge.FetchedAt.Equal(fetchedAt) {
			t.Fatalf("GetForgeStatus() = %#v, %v, want durable forge timestamp", forge, err)
		}
	})

	t.Run("Should persist lifecycle events with workspace and worktree correlation", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceA := registerWorkspaceForGlobalTests(t, globalDB, "event-a", filepath.Join(t.TempDir(), "a"))
		workspaceB := registerWorkspaceForGlobalTests(t, globalDB, "event-b", filepath.Join(t.TempDir(), "b"))
		event := worktreepkg.LifecycleEvent{
			Name: worktreepkg.EventCreated, WorkspaceID: workspaceA, WorktreeID: "wt_event_a",
			Payload: json.RawMessage(`{"worktree_id":"wt_event_a"}`),
		}
		if err := globalDB.PublishWorktreeEvent(ctx, event); err != nil {
			t.Fatalf("PublishWorktreeEvent() error = %v", err)
		}
		summaries, err := globalDB.ListEventSummaries(ctx, store.EventSummaryQuery{
			WorkspaceID: workspaceA, WorktreeID: event.WorktreeID, Type: event.Name,
		})
		if err != nil || len(summaries) != 1 {
			t.Fatalf("ListEventSummaries(owner) = %#v, %v, want one", summaries, err)
		}
		if summaries[0].WorkspaceID != workspaceA || summaries[0].WorktreeID != event.WorktreeID {
			t.Fatalf("event projections = %#v, want workspace/worktree correlation", summaries[0])
		}
		crossWorkspace, err := globalDB.ListEventSummaries(ctx, store.EventSummaryQuery{
			WorkspaceID: workspaceB, WorktreeID: event.WorktreeID,
		})
		if err != nil || len(crossWorkspace) != 0 {
			t.Fatalf("ListEventSummaries(cross-workspace) = %#v, %v, want empty", crossWorkspace, err)
		}
	})

	t.Run("Should commit an exit terminal state and replay event atomically", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "terminal-event", t.TempDir())
		now := time.Date(2026, 8, 12, 17, 30, 0, 0, time.UTC)
		item := worktreepkg.Worktree{
			ID: "wt_terminal_event", WorkspaceID: workspaceID, Name: "terminal-event",
			Branch: "feature/terminal", Path: filepath.Join(t.TempDir(), "terminal"),
			State: worktreepkg.StateReady, Origin: worktreepkg.OriginManual,
			SetupState: worktreepkg.SetupNone, CreatedAt: now, UpdatedAt: now,
		}
		if err := globalDB.Worktrees.Insert(ctx, item); err != nil {
			t.Fatalf("Insert() error = %v", err)
		}
		operation := worktreepkg.ExitOperation{
			ID: "op-terminal", WorkspaceID: workspaceID, WorktreeID: item.ID,
			Action: "push", State: "running", StartedAt: now,
		}
		if err := globalDB.Worktrees.InsertExitOperation(ctx, operation); err != nil {
			t.Fatalf("InsertExitOperation() error = %v", err)
		}
		event := worktreepkg.LifecycleEvent{
			Name: worktreepkg.EventExitActionCompleted, WorkspaceID: workspaceID, WorktreeID: item.ID,
			Payload: json.RawMessage(`{"op_id":"op-terminal","state":"completed"}`),
		}
		if _, err := globalDB.FinishExitOperationWithEvent(
			ctx, workspaceID, item.ID, operation.ID, "invalid", now, event,
		); err == nil {
			t.Fatal("FinishExitOperationWithEvent(invalid state) error = nil")
		}
		running, err := globalDB.Worktrees.ListRunningExitOperations(ctx)
		if err != nil || len(running) != 1 {
			t.Fatalf("running after rollback = %#v, %v, want original operation", running, err)
		}
		summaries, err := globalDB.ListEventSummaries(ctx, store.EventSummaryQuery{
			WorkspaceID: workspaceID, WorktreeID: item.ID, Type: event.Name,
		})
		if err != nil || len(summaries) != 0 {
			t.Fatalf("events after rollback = %#v, %v, want none", summaries, err)
		}
		finished, err := globalDB.FinishExitOperationWithEvent(
			ctx, workspaceID, item.ID, operation.ID, "completed", now, event,
		)
		if err != nil || !finished {
			t.Fatalf("FinishExitOperationWithEvent() = %t, %v", finished, err)
		}
		running, err = globalDB.Worktrees.ListRunningExitOperations(ctx)
		if err != nil || len(running) != 0 {
			t.Fatalf("running after commit = %#v, %v, want none", running, err)
		}
		summaries, err = globalDB.ListEventSummaries(ctx, store.EventSummaryQuery{
			WorkspaceID: workspaceID, WorktreeID: item.ID, Type: event.Name,
		})
		if err != nil || len(summaries) != 1 {
			t.Fatalf("events after commit = %#v, %v, want one", summaries, err)
		}
	})

	t.Run("Should enforce profile run binding and one-active-exit constraints in SQLite", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceA := registerWorkspaceForGlobalTests(t, globalDB, "constraint-a", filepath.Join(t.TempDir(), "a"))
		workspaceB := registerWorkspaceForGlobalTests(t, globalDB, "constraint-b", filepath.Join(t.TempDir(), "b"))
		now := time.Date(2026, 8, 12, 18, 0, 0, 0, time.UTC)
		item := worktreepkg.Worktree{
			ID: "wt_constraints", WorkspaceID: workspaceB, Name: "constraints", Branch: "feature/c",
			Path: filepath.Join(t.TempDir(), "constraints"), State: worktreepkg.StateReady,
			Origin: worktreepkg.OriginManual, SetupState: worktreepkg.SetupNone,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := globalDB.Worktrees.Insert(ctx, item); err != nil {
			t.Fatalf("Insert() error = %v", err)
		}

		task := workspaceTaskRecordForTest("task-worktree-constraints", workspaceA)
		if err := globalDB.CreateTask(ctx, task); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run := taskRunForTest("run-worktree-constraints", task.ID)
		if err := globalDB.CreateTaskRun(ctx, run); err != nil {
			t.Fatalf("CreateTaskRun() error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx, `UPDATE task_runs SET worktree_id = ? WHERE id = ?`, item.ID, run.ID,
		); err == nil {
			t.Fatal("cross-workspace task run binding succeeded, want composite FK rejection")
		}
		if _, err := globalDB.db.ExecContext(
			ctx, `UPDATE task_runs SET resolved_worktree_mode = 'none', resolved_worktree_ref = 'unexpected' WHERE id = ?`,
			run.ID,
		); err == nil {
			t.Fatal("invalid resolved mode/ref succeeded, want CHECK rejection")
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`INSERT INTO task_execution_profiles (
			  task_id, worktree_mode, worktree_ref, created_at, updated_at
			) VALUES (?, 'none', 'unexpected', ?, ?)`,
			task.ID, store.FormatTimestamp(now), store.FormatTimestamp(now),
		); err == nil {
			t.Fatal("invalid profile mode/ref succeeded, want CHECK rejection")
		}

		first := worktreepkg.ExitOperation{
			ID: "op-one", WorkspaceID: workspaceB, WorktreeID: item.ID,
			Action: "push", State: "running", StartedAt: now,
		}
		if err := globalDB.Worktrees.InsertExitOperation(ctx, first); err != nil {
			t.Fatalf("InsertExitOperation(first) error = %v", err)
		}
		second := first
		second.ID = "op-two"
		if err := globalDB.Worktrees.InsertExitOperation(ctx, second); !errors.Is(
			err, worktreepkg.ErrOperationInProgress,
		) {
			t.Fatalf("InsertExitOperation(second) error = %v, want ErrOperationInProgress", err)
		}
	})

	t.Run("Should reject cross-workspace session bindings at the database boundary", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		globalDB := openTestGlobalDB(t)
		workspaceA := registerSessionForGlobalTests(t, globalDB, "session-worktree-constraints")
		workspaceB := registerWorkspaceForGlobalTests(t, globalDB, "session-worktree-b", filepath.Join(t.TempDir(), "b"))
		now := time.Date(2026, 8, 12, 19, 0, 0, 0, time.UTC)
		item := worktreepkg.Worktree{
			ID: "wt_session_constraints", WorkspaceID: workspaceB, Name: "session-constraints",
			Path: filepath.Join(t.TempDir(), "session"), State: worktreepkg.StateReady,
			Origin: worktreepkg.OriginManual, SetupState: worktreepkg.SetupNone,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := globalDB.Worktrees.Insert(ctx, item); err != nil {
			t.Fatalf("Insert() error = %v", err)
		}
		if workspaceA == workspaceB {
			t.Fatal("test fixture workspaces unexpectedly match")
		}
		if _, err := globalDB.db.ExecContext(
			ctx, `UPDATE sessions SET worktree_id = ? WHERE id = ?`, item.ID, "session-worktree-constraints",
		); err == nil {
			t.Fatal("cross-workspace session binding succeeded, want composite FK rejection")
		}
	})

	t.Run("Should make exit start and removal mutually exclusive at the SQLite fence", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(
			t, globalDB, "exit-removal-fence", filepath.Join(t.TempDir(), "workspace"),
		)
		now := time.Date(2026, 8, 12, 19, 10, 0, 0, time.UTC)
		insertReady := func(id string) {
			t.Helper()
			if err := globalDB.Worktrees.Insert(ctx, worktreepkg.Worktree{
				ID: id, WorkspaceID: workspaceID, Name: id, Branch: "feature/" + id,
				Path: filepath.Join(t.TempDir(), id), State: worktreepkg.StateReady,
				Origin: worktreepkg.OriginManual, SetupState: worktreepkg.SetupNone,
				CreatedAt: now, UpdatedAt: now,
			}); err != nil {
				t.Fatalf("Insert(%s) error = %v", id, err)
			}
		}

		insertReady("wt_exit_first")
		exitOperation := worktreepkg.ExitOperation{
			ID: "op-exit-first", WorkspaceID: workspaceID, WorktreeID: "wt_exit_first",
			Action: "push", State: "running", StartedAt: now,
		}
		if err := globalDB.Worktrees.InsertExitOperation(ctx, exitOperation); err != nil {
			t.Fatalf("InsertExitOperation(exit first) error = %v", err)
		}
		if swapped, err := globalDB.Worktrees.CompareAndSwapState(
			ctx, workspaceID, "wt_exit_first", worktreepkg.StateReady, worktreepkg.StateRemoving, now,
		); err != nil || swapped {
			t.Fatalf("CompareAndSwapState(after exit) = %t, %v, want false nil", swapped, err)
		}
		running, err := globalDB.Worktrees.ListRunningExitOperations(ctx)
		if err != nil || len(running) != 1 || running[0].ID != exitOperation.ID ||
			running[0].WorkspaceID != workspaceID || running[0].WorktreeID != "wt_exit_first" {
			t.Fatalf("ListRunningExitOperations() = %#v, %v", running, err)
		}

		insertReady("wt_remove_first")
		if swapped, err := globalDB.Worktrees.CompareAndSwapState(
			ctx, workspaceID, "wt_remove_first", worktreepkg.StateReady, worktreepkg.StateRemoving, now,
		); err != nil || !swapped {
			t.Fatalf("CompareAndSwapState(removal first) = %t, %v, want true nil", swapped, err)
		}
		if err := globalDB.Worktrees.InsertExitOperation(ctx, worktreepkg.ExitOperation{
			ID: "op-remove-first", WorkspaceID: workspaceID, WorktreeID: "wt_remove_first",
			Action: "commit", State: "running", StartedAt: now,
		}); !errors.Is(err, worktreepkg.ErrNotReady) {
			t.Fatalf("InsertExitOperation(after removal) error = %v, want ErrNotReady", err)
		}
	})

	t.Run("Should fence new session bindings after removal starts without blocking bound session updates", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"session-removal-fence",
			filepath.Join(t.TempDir(), "workspace"),
		)
		now := time.Date(2026, 8, 12, 19, 15, 0, 0, time.UTC)
		item := worktreepkg.Worktree{
			ID: "wt_session_removal_fence", WorkspaceID: workspaceID, Name: "session-removal-fence",
			Path: filepath.Join(t.TempDir(), "worktree"), State: worktreepkg.StateReady,
			Origin: worktreepkg.OriginManual, SetupState: worktreepkg.SetupNone,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := globalDB.Worktrees.Insert(ctx, item); err != nil {
			t.Fatalf("Insert() error = %v", err)
		}
		bound := store.SessionInfo{
			ID: "session-before-removal", AgentName: "coder", Provider: "claude",
			RuntimeStatus: store.SessionRuntimeUnbound, WorkspaceID: workspaceID, WorktreeID: item.ID,
			State: "active", CreatedAt: now, UpdatedAt: now,
		}
		if err := globalDB.RegisterSession(ctx, bound); err != nil {
			t.Fatalf("RegisterSession(bound) error = %v", err)
		}
		if err := globalDB.Worktrees.SetState(
			ctx, workspaceID, item.ID, worktreepkg.StateRemoving, now.Add(time.Minute),
		); err != nil {
			t.Fatalf("SetState(removing) error = %v", err)
		}

		bound.State = "stopped"
		bound.UpdatedAt = now.Add(2 * time.Minute)
		if err := globalDB.RegisterSession(ctx, bound); err != nil {
			t.Fatalf("RegisterSession(existing bound update) error = %v", err)
		}
		newBinding := bound
		newBinding.ID = "session-after-removal"
		newBinding.State = "active"
		if err := globalDB.RegisterSession(ctx, newBinding); !errors.Is(err, worktreepkg.ErrNotReady) {
			t.Fatalf("RegisterSession(new binding) error = %v, want %v", err, worktreepkg.ErrNotReady)
		}
		rows, err := globalDB.ListSessions(ctx, store.SessionListQuery{ID: newBinding.ID})
		if err != nil {
			t.Fatalf("ListSessions(new binding) error = %v", err)
		}
		if len(rows) != 0 {
			t.Fatalf("new binding rows = %#v, want none", rows)
		}
	})

	t.Run("Should dismiss a tombstone without cascading session or task-run history", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		globalDB := openTestGlobalDB(t)
		sessionID := "session-worktree-history"
		workspaceID := registerSessionForGlobalTests(t, globalDB, sessionID)
		now := time.Date(2026, 8, 12, 19, 30, 0, 0, time.UTC)
		item := worktreepkg.Worktree{
			ID: "wt_history", WorkspaceID: workspaceID, Name: "history",
			Path: filepath.Join(t.TempDir(), "history"), State: worktreepkg.StateMissing,
			Origin: worktreepkg.OriginManual, SetupState: worktreepkg.SetupNone,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := globalDB.Worktrees.Insert(ctx, item); err != nil {
			t.Fatalf("Insert() error = %v", err)
		}
		task := workspaceTaskRecordForTest("task-worktree-history", workspaceID)
		if err := globalDB.CreateTask(ctx, task); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run := taskRunForTest("run-worktree-history", task.ID)
		if err := globalDB.CreateTaskRun(ctx, run); err != nil {
			t.Fatalf("CreateTaskRun() error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx, `UPDATE sessions SET worktree_id = ? WHERE id = ?`, item.ID, sessionID,
		); err != nil {
			t.Fatalf("bind session history: %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx, `UPDATE task_runs SET worktree_id = ? WHERE id = ?`, item.ID, run.ID,
		); err != nil {
			t.Fatalf("bind task-run history: %v", err)
		}
		service := worktreepkg.NewService(globalDB.Worktrees, nil, worktreepkg.WithClock(func() time.Time { return now }))
		if err := service.Dismiss(ctx, workspaceID, item.ID); err != nil {
			t.Fatalf("Dismiss() error = %v", err)
		}
		var sessionWorktreeID string
		if err := globalDB.db.QueryRowContext(
			ctx, `SELECT worktree_id FROM sessions WHERE id = ?`, sessionID,
		).Scan(&sessionWorktreeID); err != nil {
			t.Fatalf("read session history: %v", err)
		}
		var runWorktreeID string
		if err := globalDB.db.QueryRowContext(
			ctx, `SELECT worktree_id FROM task_runs WHERE id = ?`, run.ID,
		).Scan(&runWorktreeID); err != nil {
			t.Fatalf("read task-run history: %v", err)
		}
		if sessionWorktreeID != item.ID || runWorktreeID != item.ID {
			t.Fatalf(
				"dismissed bindings = session %q run %q, want preserved %q",
				sessionWorktreeID, runWorktreeID, item.ID,
			)
		}
	})
}
