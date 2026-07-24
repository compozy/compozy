package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/reviews"
	"github.com/compozy/compozy/internal/core/taskgroups"
	"github.com/compozy/compozy/internal/core/tasks"
	"github.com/compozy/compozy/internal/store/globaldb"
)

func TestArchiveWithDBUsesInjectedWorkspaceBoundary(t *testing.T) {
	rootDir := archiveTestRoot(t)
	workflowDir := filepath.Join(rootDir, "injected-db")
	writeArchiveTaskFile(t, workflowDir, "task_001.md", "completed")
	mustSyncArchiveWorkflow(t, workflowDir)
	db, workspace := openArchiveWorkflowDB(t, rootDir)
	t.Cleanup(func() {
		_ = db.Close()
	})

	t.Run("Should reject a nil archive database", func(t *testing.T) {
		if _, err := ArchiveWithDB(
			context.Background(),
			nil,
			workspace,
			ArchiveConfig{TasksDir: workflowDir},
		); !errors.Is(err, ErrArchiveDatabaseRequired) {
			t.Fatalf("ArchiveWithDB(nil database) error = %v, want %v", err, ErrArchiveDatabaseRequired)
		}
	})

	t.Run("Should reject a mismatched workspace and archive target", func(t *testing.T) {
		mismatched := workspace
		mismatched.RootDir = t.TempDir()
		_, err := ArchiveWithDB(
			context.Background(),
			db,
			mismatched,
			ArchiveConfig{TasksDir: workflowDir},
		)
		var mismatch ArchiveWorkspaceMismatchError
		if !errors.As(err, &mismatch) {
			t.Fatalf("ArchiveWithDB(mismatched workspace) error = %v, want ArchiveWorkspaceMismatchError", err)
		}
		if mismatch.WorkspaceRoot != mismatched.RootDir {
			t.Fatalf(
				"ArchiveWorkspaceMismatchError.WorkspaceRoot = %q, want %q",
				mismatch.WorkspaceRoot,
				mismatched.RootDir,
			)
		}
	})

	t.Run("Should archive when the injected workspace boundary matches", func(t *testing.T) {
		result, err := ArchiveWithDB(
			context.Background(),
			db,
			workspace,
			ArchiveConfig{TasksDir: workflowDir},
		)
		if err != nil {
			t.Fatalf("ArchiveWithDB() error = %v", err)
		}
		if result.Archived != 1 || len(result.ArchivedPaths) != 1 {
			t.Fatalf("ArchiveWithDB() result = %#v, want one archived workflow", result)
		}
	})
}

func TestArchiveTaskWorkflowRequiresForceForPendingStateFromSyncedDBEvenWithStaleMeta(t *testing.T) {
	rootDir := archiveTestRoot(t)
	workflowDir := filepath.Join(rootDir, "beta")
	writeArchiveTaskFile(t, workflowDir, "task_001.md", "pending")
	mustSyncArchiveWorkflow(t, workflowDir)

	// Reintroduce stale metadata that claims the workflow is complete. Archive must ignore it.
	writeArchiveTaskMeta(t, workflowDir, strings.Join([]string{
		"---",
		"created_at: 2026-04-01T12:00:00Z",
		"updated_at: 2026-04-01T12:00:00Z",
		"---",
		"",
		"## Summary",
		"- Total: 1",
		"- Completed: 1",
		"- Pending: 0",
		"",
	}, "\n"))

	result, err := Archive(context.Background(), ArchiveConfig{TasksDir: workflowDir})
	if !errors.Is(err, ErrWorkflowForceRequired) {
		t.Fatalf("Archive() error = %v, want ErrWorkflowForceRequired", err)
	}
	if result == nil {
		t.Fatal("expected archive result")
	}
	if result.WorkflowsScanned != 1 || result.Archived != 0 || result.Skipped != 0 {
		t.Fatalf("unexpected archive result: %#v", result)
	}
	var forceRequired WorkflowArchiveForceRequiredError
	if !errors.As(err, &forceRequired) {
		t.Fatalf("expected typed force-required error, got %T", err)
	}
	if forceRequired.TaskNonTerminal != 1 || forceRequired.ReviewUnresolved != 0 {
		t.Fatalf("unexpected force-required details: %#v", forceRequired)
	}
	if _, statErr := os.Stat(workflowDir); statErr != nil {
		t.Fatalf("expected workflow dir to remain in place: %v", statErr)
	}
}

func TestArchiveTaskWorkflowsRootScanUsesDBStateAndSortsSkippedPaths(t *testing.T) {
	rootDir := archiveTestRoot(t)
	alphaDir := filepath.Join(rootDir, "alpha")
	betaDir := filepath.Join(rootDir, "beta")
	gammaDir := filepath.Join(rootDir, "gamma")
	deltaDir := filepath.Join(rootDir, "delta")
	for _, dir := range []string{alphaDir, betaDir, gammaDir, deltaDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	writeArchiveTaskFile(t, alphaDir, "task_001.md", "completed")
	writeArchiveTaskFile(t, betaDir, "task_001.md", "pending")
	writeArchiveTaskFile(t, gammaDir, "task_001.md", "completed")
	writeArchiveTaskFile(t, deltaDir, "task_001.md", "completed")
	writeArchiveReviewRound(t, gammaDir, 1, []string{"pending"}, true)

	mustSyncArchiveRoot(t, rootDir)

	// Stale filesystem metadata must not affect DB-backed eligibility.
	writeArchiveTaskMeta(t, betaDir, strings.Join([]string{
		"---",
		"created_at: 2026-04-01T12:00:00Z",
		"updated_at: 2026-04-01T12:00:00Z",
		"---",
		"",
		"## Summary",
		"- Total: 1",
		"- Completed: 1",
		"- Pending: 0",
		"",
	}, "\n"))
	if err := os.Remove(reviews.MetaPath(reviews.ReviewDirectory(gammaDir, 1))); err != nil {
		t.Fatalf("remove stale review meta: %v", err)
	}
	insertActiveArchiveRun(t, deltaDir, "delta", "run-delta-active")

	db, workspace := openArchiveWorkflowDB(t, rootDir)
	alphaWorkflow, err := db.GetActiveWorkflowBySlug(context.Background(), workspace.ID, "alpha")
	if err != nil {
		t.Fatalf("GetActiveWorkflowBySlug(alpha): %v", err)
	}
	shortID := model.ArchivedWorkflowShortID(alphaWorkflow.ID)
	if err := db.Close(); err != nil {
		t.Fatalf("Close(): %v", err)
	}

	result, err := Archive(context.Background(), ArchiveConfig{RootDir: rootDir})
	if err != nil {
		t.Fatalf("Archive(root): %v", err)
	}
	if result.WorkflowsScanned != 4 {
		t.Fatalf("WorkflowsScanned = %d, want 4", result.WorkflowsScanned)
	}
	if result.Archived != 1 || result.Skipped != 3 {
		t.Fatalf("unexpected archive counts: %#v", result)
	}
	if got := result.SkippedReasons[betaDir]; got != "task workflow not fully completed" {
		t.Fatalf("skip reason for beta = %q, want task workflow not fully completed", got)
	}
	if got := result.SkippedReasons[gammaDir]; got != "review rounds not fully resolved" {
		t.Fatalf("skip reason for gamma = %q, want review rounds not fully resolved", got)
	}
	if got := result.SkippedReasons[deltaDir]; got != "workflow has active runs" {
		t.Fatalf("skip reason for delta = %q, want workflow has active runs", got)
	}

	wantSkipped := []string{betaDir, deltaDir, gammaDir}
	sort.Strings(wantSkipped)
	if !equalStrings(result.SkippedPaths, wantSkipped) {
		t.Fatalf("SkippedPaths = %#v, want %#v", result.SkippedPaths, wantSkipped)
	}

	if len(result.ArchivedPaths) != 1 {
		t.Fatalf("ArchivedPaths = %#v, want one entry", result.ArchivedPaths)
	}
	archivedPath := result.ArchivedPaths[0]
	if filepath.Dir(archivedPath) != filepath.Join(rootDir, model.ArchivedWorkflowDirName) {
		t.Fatalf(
			"archive parent = %q, want %q",
			filepath.Dir(archivedPath),
			filepath.Join(rootDir, model.ArchivedWorkflowDirName),
		)
	}
	pattern := fmt.Sprintf(`^\d{13}-%s-alpha$`, shortID)
	if matched, err := regexp.MatchString(pattern, filepath.Base(archivedPath)); err != nil || !matched {
		t.Fatalf("archived path %q does not match %q", archivedPath, pattern)
	}
	if _, statErr := os.Stat(alphaDir); !os.IsNotExist(statErr) {
		t.Fatalf("expected archived workflow to leave active root, got err=%v", statErr)
	}

	db, workspace = openArchiveWorkflowDB(t, rootDir)
	defer func() {
		_ = db.Close()
	}()
	activeRows, err := db.ListWorkflows(context.Background(), globaldb.ListWorkflowsOptions{WorkspaceID: workspace.ID})
	if err != nil {
		t.Fatalf("ListWorkflows(active): %v", err)
	}
	for _, row := range activeRows {
		if row.Slug == "alpha" {
			t.Fatalf("expected alpha to disappear from active workflow list: %#v", activeRows)
		}
	}
	allRows, err := db.ListWorkflows(context.Background(), globaldb.ListWorkflowsOptions{
		WorkspaceID:     workspace.ID,
		IncludeArchived: true,
	})
	if err != nil {
		t.Fatalf("ListWorkflows(all): %v", err)
	}
	if len(allRows) != 4 {
		t.Fatalf("ListWorkflows(all) len = %d, want 4", len(allRows))
	}
}

func TestArchiveTaskWorkflowRejectsActiveRunConflict(t *testing.T) {
	rootDir := archiveTestRoot(t)
	workflowDir := filepath.Join(rootDir, "delta")
	writeArchiveTaskFile(t, workflowDir, "task_001.md", "completed")
	mustSyncArchiveWorkflow(t, workflowDir)
	insertActiveArchiveRun(t, workflowDir, "delta", "run-delta-active")

	result, err := Archive(context.Background(), ArchiveConfig{TasksDir: workflowDir})
	if !errors.Is(err, globaldb.ErrWorkflowHasActiveRuns) {
		t.Fatalf("Archive() error = %v, want ErrWorkflowHasActiveRuns", err)
	}
	if result == nil {
		t.Fatal("expected archive result")
	}
	if result.WorkflowsScanned != 1 || result.Archived != 0 || result.Skipped != 0 {
		t.Fatalf("unexpected archive result: %#v", result)
	}
}

func TestArchiveTaskWorkflowForceDoesNotBypassActiveRunConflict(t *testing.T) {
	rootDir := archiveTestRoot(t)
	workflowDir := filepath.Join(rootDir, "delta")
	writeArchiveTaskFile(t, workflowDir, "task_001.md", "completed")
	mustSyncArchiveWorkflow(t, workflowDir)
	insertActiveArchiveRun(t, workflowDir, "delta", "run-delta-active")

	result, err := Archive(context.Background(), ArchiveConfig{TasksDir: workflowDir, Force: true})
	if !errors.Is(err, globaldb.ErrWorkflowHasActiveRuns) {
		t.Fatalf("Archive(force) error = %v, want ErrWorkflowHasActiveRuns", err)
	}
	if result == nil {
		t.Fatal("expected archive result")
	}
	if result.Archived != 0 || result.Forced {
		t.Fatalf("unexpected forced active-run result: %#v", result)
	}
}

func TestArchiveTaskGroupInitiativeMovesOnlyRootAndArchivesChildren(t *testing.T) {
	// Suite boundary
	// IN: real initiative filesystem, aggregate sync, archive move, and SQLite hierarchy rows
	// OUT: HTTP transport and CLI confirmation rendering
	// Invariant: archiving a Task Group initiative moves one root and archives its children as one durable unit.
	rootDir := archiveTestRoot(t)
	initiativeDir := filepath.Join(rootDir, "initiative")
	writeTaskGroupFixture(t, initiativeDir, map[string]string{
		"TG-001": "completed",
		"TG-002": "completed",
	})

	syncResult, err := Sync(context.Background(), SyncConfig{TasksDir: initiativeDir})
	if err != nil {
		t.Fatalf("Sync(task group initiative): %v", err)
	}
	if syncResult.WorkflowsScanned != 3 {
		t.Fatalf("Sync workflows scanned = %d, want parent plus two children", syncResult.WorkflowsScanned)
	}
	_, err = Archive(
		context.Background(),
		ArchiveConfig{TasksDir: filepath.Join(initiativeDir, "_task_groups", "TG-001")},
	)
	if !errors.Is(err, ErrTaskGroupRootOnly) {
		t.Fatalf("Archive(task group target) error = %v, want ErrTaskGroupRootOnly", err)
	}

	result, err := Archive(context.Background(), ArchiveConfig{TasksDir: initiativeDir})
	if err != nil {
		t.Fatalf("Archive(task group initiative): %v", err)
	}
	if result.Archived != 1 || result.WorkflowsScanned != 1 || len(result.TaskGroupChildIDs) != 2 {
		t.Fatalf("Archive(task group initiative) result = %#v, want one root and two archived children", result)
	}
	if len(result.ArchivedPaths) != 1 {
		t.Fatalf("ArchivedPaths = %#v, want one root move", result.ArchivedPaths)
	}
	if _, statErr := os.Stat(filepath.Join(result.ArchivedPaths[0], "_task_groups", "TG-001")); statErr != nil {
		t.Fatalf("archived root did not retain TG-001 directory: %v", statErr)
	}
	if _, statErr := os.Stat(initiativeDir); !os.IsNotExist(statErr) {
		t.Fatalf("initiative root remains active after archive: %v", statErr)
	}

	db, workspace := openArchiveWorkflowDB(t, rootDir)
	defer func() {
		_ = db.Close()
	}()
	rows, err := db.ListWorkflows(context.Background(), globaldb.ListWorkflowsOptions{
		WorkspaceID:     workspace.ID,
		IncludeArchived: true,
	})
	if err != nil {
		t.Fatalf("ListWorkflows(archived hierarchy): %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("archived hierarchy rows = %#v, want initiative and two children", rows)
	}
	var parent globaldb.Workflow
	children := 0
	for _, row := range rows {
		if row.Kind == globaldb.WorkflowKindInitiative {
			parent = row
		}
		if row.Kind == globaldb.WorkflowKindTaskGroup {
			children++
			if row.ArchivedAt == nil {
				t.Fatalf("child %q was not marked archived", row.TaskGroupID)
			}
		}
	}
	if parent.ID == "" || parent.ArchivedAt == nil || children != 2 {
		t.Fatalf("archived parent/children = parent %#v children %d", parent, children)
	}

	_, err = Archive(context.Background(), ArchiveConfig{TasksDir: initiativeDir})
	if !errors.Is(err, globaldb.ErrWorkflowArchived) {
		t.Fatalf("second Archive() error = %v, want archived identity conflict", err)
	}
}

func TestArchiveTaskGroupInitiativeForceArchivesPendingPlanAsOneRoot(t *testing.T) {
	// Suite boundary
	// IN: aggregate archive eligibility, task completion, and root archive move
	// OUT: interactive force confirmation rendering
	// Invariant: force archive completes child mutable state but never moves a task group independently.
	rootDir := archiveTestRoot(t)
	initiativeDir := filepath.Join(rootDir, "initiative")
	writeTaskGroupFixture(t, initiativeDir, map[string]string{
		"TG-001": "completed",
		"TG-002": "pending",
	})

	result, err := Archive(context.Background(), ArchiveConfig{TasksDir: initiativeDir})
	if !errors.Is(err, ErrWorkflowForceRequired) {
		t.Fatalf("Archive(pending task group) error = %v, want force required", err)
	}
	if result == nil || !equalStrings(result.PendingTaskGroups, []string{"TG-002"}) {
		t.Fatalf("pending task group archive result = %#v, want TG-002", result)
	}

	result, err = Archive(context.Background(), ArchiveConfig{TasksDir: initiativeDir, Force: true})
	if err != nil {
		t.Fatalf("Archive(force pending task group): %v", err)
	}
	if !result.Forced || result.Archived != 1 || result.CompletedTasks != 1 || len(result.ArchivedPaths) != 1 {
		t.Fatalf("forced initiative archive result = %#v", result)
	}
	if _, statErr := os.Stat(
		filepath.Join(result.ArchivedPaths[0], "_task_groups", "TG-002", "task_01.md"),
	); statErr != nil {
		t.Fatalf("forced archive did not preserve child artifact: %v", statErr)
	}
}

func TestArchiveTaskGroupInitiativeForceRollsBackArtifactsWhenChildRunStarts(t *testing.T) {
	// Suite boundary
	// IN: real initiative files, aggregate sync, durable child run insertion, and forced archive
	// OUT: daemon scheduling and transport rendering
	// Invariant: a forced archive rejected by a child run that starts after preflight leaves every
	// task and review artifact byte-for-byte unchanged.
	rootDir := archiveTestRoot(t)
	initiativeDir := filepath.Join(rootDir, "initiative")
	writeTaskGroupFixture(t, initiativeDir, map[string]string{
		"TG-001": "pending",
		"TG-002": "completed",
	})
	taskGroupDir := filepath.Join(initiativeDir, "_task_groups", "TG-001")
	writeArchiveReviewRound(t, taskGroupDir, 1, []string{"pending"}, true)
	if _, err := Sync(context.Background(), SyncConfig{TasksDir: initiativeDir}); err != nil {
		t.Fatalf("Sync(task group initiative): %v", err)
	}

	taskPath := filepath.Join(taskGroupDir, "task_01.md")
	issuePath := filepath.Join(reviews.ReviewDirectory(taskGroupDir, 1), "issue_001.md")
	metaPath := tasks.MetaPath(taskGroupDir)
	taskBefore, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("read task before archive: %v", err)
	}
	issueBefore, err := os.ReadFile(issuePath)
	if err != nil {
		t.Fatalf("read review issue before archive: %v", err)
	}
	if _, err := os.Stat(metaPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("task metadata before archive error = %v, want not exist", err)
	}

	originalForceArchiveInitiative := forceArchiveInitiative
	forceArchiveInitiative = func(
		ctx context.Context,
		workspaceRoot string,
		target taskgroups.Target,
	) (initiativeArchiveMutation, error) {
		insertActiveArchiveRun(t, target.InitiativeDir, "initiative/TG-001", "run-started-during-force")
		return originalForceArchiveInitiative(ctx, workspaceRoot, target)
	}
	t.Cleanup(func() {
		forceArchiveInitiative = originalForceArchiveInitiative
	})

	result, err := Archive(context.Background(), ArchiveConfig{TasksDir: initiativeDir, Force: true})
	if !errors.Is(err, globaldb.ErrWorkflowHasActiveRuns) {
		t.Fatalf("Archive(force) error = %v, want ErrWorkflowHasActiveRuns", err)
	}
	if result == nil || result.Archived != 0 || result.Forced || result.CompletedTasks != 0 ||
		result.ResolvedReviewIssues != 0 {
		t.Fatalf("unexpected archive result: %#v", result)
	}
	assertArchiveFileUnchanged(t, taskPath, taskBefore)
	assertArchiveFileUnchanged(t, issuePath, issueBefore)
	if _, err := os.Stat(metaPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("task metadata after blocked archive error = %v, want not exist", err)
	}
}

func TestArchiveTaskGroupInitiativeRejectsActiveParentRun(t *testing.T) {
	// Suite boundary
	// IN: real initiative filesystem, aggregate sync, an active run on the initiative parent row
	// OUT: HTTP transport and CLI confirmation rendering
	// Invariant: an ordinary workflow promoted to an initiative in place keeps its own workflow ID
	// and any run linked to the parent row; archiving must refuse while that parent run is active,
	// and --force must not bypass it. The plan-declared task groups carry no runs, so a plan-scoped
	// guard would wrongly let the parent be archived mid-flight.
	for _, force := range []bool{false, true} {
		force := force
		name := "Should reject archive when the initiative parent has an active run"
		if force {
			name = "Should reject forced archive when the initiative parent has an active run"
		}
		t.Run(name, func(t *testing.T) {
			rootDir := archiveTestRoot(t)
			initiativeDir := filepath.Join(rootDir, "initiative")
			writeTaskGroupFixture(t, initiativeDir, map[string]string{
				"TG-001": "completed",
				"TG-002": "completed",
			})
			if _, err := Sync(context.Background(), SyncConfig{TasksDir: initiativeDir}); err != nil {
				t.Fatalf("Sync(task group initiative): %v", err)
			}
			insertActiveArchiveRun(t, initiativeDir, "initiative", "run-initiative-parent-active")

			result, err := Archive(context.Background(), ArchiveConfig{TasksDir: initiativeDir, Force: force})
			if !errors.Is(err, globaldb.ErrWorkflowHasActiveRuns) {
				t.Fatalf("Archive(force=%v) error = %v, want ErrWorkflowHasActiveRuns", force, err)
			}
			if result == nil || result.Archived != 0 || result.Forced {
				t.Fatalf("unexpected active parent-run archive result: %#v", result)
			}
			if _, statErr := os.Stat(initiativeDir); statErr != nil {
				t.Fatalf("initiative root must remain active after blocked archive: %v", statErr)
			}
		})
	}
}

func TestArchiveTaskGroupInitiativeRejectsRetainedChildActiveRun(t *testing.T) {
	// Suite boundary
	// IN: real initiative filesystem, aggregate sync, plan pruning that retains a child with a live run
	// OUT: HTTP transport and CLI confirmation rendering
	// Invariant: a child dropped from _task_groups.md but retained by pruning because its run is
	// active remains a non-archived direct child while leaving target.Plan.TaskGroups; archiving must
	// refuse (default and --force) so the retained child is never archived mid-run.
	for _, force := range []bool{false, true} {
		force := force
		name := "Should reject archive when a retained child has an active run"
		if force {
			name = "Should reject forced archive when a retained child has an active run"
		}
		t.Run(name, func(t *testing.T) {
			rootDir := archiveTestRoot(t)
			initiativeDir := filepath.Join(rootDir, "initiative")
			writeTaskGroupFixture(t, initiativeDir, map[string]string{
				"TG-001": "completed",
				"TG-002": "completed",
			})
			if _, err := Sync(context.Background(), SyncConfig{TasksDir: initiativeDir}); err != nil {
				t.Fatalf("Sync(task group initiative): %v", err)
			}
			insertActiveArchiveRun(t, initiativeDir, "initiative/TG-002", "run-task-group-002-active")

			// Drop TG-002 from the plan while its run is active; pruning retains the child row.
			writeSingleTaskGroupInitiativePlan(t, initiativeDir)
			if err := os.RemoveAll(filepath.Join(initiativeDir, "_task_groups", "TG-002")); err != nil {
				t.Fatalf("remove pruned task group dir: %v", err)
			}

			result, err := Archive(context.Background(), ArchiveConfig{TasksDir: initiativeDir, Force: force})
			if !errors.Is(err, globaldb.ErrWorkflowHasActiveRuns) {
				t.Fatalf("Archive(force=%v) error = %v, want ErrWorkflowHasActiveRuns", force, err)
			}
			if result == nil || result.Archived != 0 || result.Forced {
				t.Fatalf("unexpected retained-child active-run archive result: %#v", result)
			}
			if _, statErr := os.Stat(initiativeDir); statErr != nil {
				t.Fatalf("initiative root must remain active after blocked archive: %v", statErr)
			}
		})
	}
}

func TestArchiveTaskGroupInitiativeRejectsMissingCompletedTaskGroup(t *testing.T) {
	// Suite boundary
	// IN: real initiative filesystem, aggregate sync that flags a vanished completed
	//     task group's directory Missing while retaining its projection, and the archive command
	// OUT: HTTP transport and CLI confirmation rendering
	// Invariant: a declared task group whose directory disappeared after completion is treated as
	// missing (matching the daemon read model's Missing archive-ineligibility), so both default
	// and forced initiative archive refuse before any mutation, report the missing task group, and
	// preserve the active hierarchy on disk and in the durable store.
	for _, force := range []bool{false, true} {
		force := force
		name := "Should reject archive when a completed task group directory is missing"
		if force {
			name = "Should reject forced archive when a completed task group directory is missing"
		}
		t.Run(name, func(t *testing.T) {
			rootDir := archiveTestRoot(t)
			initiativeDir := filepath.Join(rootDir, "initiative")
			writeTaskGroupFixture(t, initiativeDir, map[string]string{
				"TG-001": "completed",
				"TG-002": "completed",
			})
			if _, err := Sync(context.Background(), SyncConfig{TasksDir: initiativeDir}); err != nil {
				t.Fatalf("Sync(task group initiative): %v", err)
			}

			// Delete a completed task group directory while leaving it declared in the plan.
			if err := os.RemoveAll(filepath.Join(initiativeDir, "_task_groups", "TG-001")); err != nil {
				t.Fatalf("remove completed task group dir: %v", err)
			}
			// Re-sync: the durable row is flagged Missing but its completed task/review
			// projection is deliberately retained, reproducing the state where the row still
			// looks terminal even though the directory is gone.
			syncResult, err := Sync(context.Background(), SyncConfig{TasksDir: initiativeDir})
			if err != nil {
				t.Fatalf("Sync(after task group removal): %v", err)
			}
			if !equalStrings(syncResult.MissingTaskGroups, []string{"TG-001"}) {
				t.Fatalf("sync missing task groups = %#v, want TG-001", syncResult.MissingTaskGroups)
			}

			result, err := Archive(context.Background(), ArchiveConfig{TasksDir: initiativeDir, Force: force})
			if !errors.Is(err, ErrWorkflowForceRequired) {
				t.Fatalf("Archive(force=%v) error = %v, want ErrWorkflowForceRequired", force, err)
			}
			var conflict WorkflowArchiveForceRequiredError
			if !errors.As(err, &conflict) {
				t.Fatalf("Archive(force=%v) error = %v, want WorkflowArchiveForceRequiredError", force, err)
			}
			if !strings.Contains(conflict.Reason, "TG-001") || !strings.Contains(conflict.Reason, "missing") {
				t.Fatalf("conflict reason = %q, want it to report TG-001 missing", conflict.Reason)
			}
			if result == nil || result.Archived != 0 || result.Forced {
				t.Fatalf("unexpected missing-task-group archive result: %#v", result)
			}
			// The active hierarchy stays on disk: the initiative root and the present sibling.
			if _, statErr := os.Stat(initiativeDir); statErr != nil {
				t.Fatalf("initiative root must remain active after blocked archive: %v", statErr)
			}
			if _, statErr := os.Stat(filepath.Join(initiativeDir, "_task_groups", "TG-002")); statErr != nil {
				t.Fatalf("present sibling task group must remain after blocked archive: %v", statErr)
			}
			// The parent workflow stays active (unarchived) in the durable store.
			db, workspace := openArchiveWorkflowDB(t, rootDir)
			defer func() {
				_ = db.Close()
			}()
			parent, err := db.GetActiveWorkflowBySlug(context.Background(), workspace.ID, "initiative")
			if err != nil {
				t.Fatalf("GetActiveWorkflowBySlug(initiative) after blocked archive: %v", err)
			}
			if parent.ArchivedAt != nil {
				t.Fatalf("initiative parent must remain active, got archived at %v", parent.ArchivedAt)
			}
		})
	}
}

func TestArchiveTaskWorkflowForceArchivesAfterLocalReviewResolutionWithoutManualResync(t *testing.T) {
	rootDir := archiveTestRoot(t)
	workflowDir := filepath.Join(rootDir, "gamma")
	writeArchiveTaskFile(t, workflowDir, "task_001.md", "completed")
	writeArchiveReviewRound(t, workflowDir, 1, []string{"pending"}, true)
	mustSyncArchiveWorkflow(t, workflowDir)

	issuePath := filepath.Join(reviews.ReviewDirectory(workflowDir, 1), "issue_001.md")
	if err := rewriteArchiveIssueStatus(issuePath, "pending", "resolved"); err != nil {
		t.Fatalf("rewrite issue status: %v", err)
	}
	if err := os.Remove(reviews.MetaPath(reviews.ReviewDirectory(workflowDir, 1))); err != nil {
		t.Fatalf("remove review meta: %v", err)
	}

	result, err := Archive(context.Background(), ArchiveConfig{TasksDir: workflowDir})
	if !errors.Is(err, ErrWorkflowForceRequired) {
		t.Fatalf("Archive(without resync) error = %v, want ErrWorkflowForceRequired", err)
	}
	if result == nil || result.Archived != 0 {
		t.Fatalf("unexpected archive result before resync: %#v", result)
	}

	result, err = Archive(context.Background(), ArchiveConfig{TasksDir: workflowDir, Force: true})
	if err != nil {
		t.Fatalf("Archive(force after local resolve): %v", err)
	}
	if result == nil || result.Archived != 1 || len(result.ArchivedPaths) != 1 {
		t.Fatalf("unexpected archive result after forced resync: %#v", result)
	}
	if result.Forced {
		t.Fatalf("expected forced flag to remain false when no local rewrite was needed: %#v", result)
	}
}

func TestArchiveTaskWorkflowHandlesReviewOnlyWorkflows(t *testing.T) {
	testCases := []struct {
		name                     string
		reviewStatus             []string
		force                    bool
		wantErr                  error
		wantArchived             int
		wantSkipped              int
		wantRemoved              bool
		wantForced               bool
		wantResolvedReviewIssues int
	}{
		{
			name:         "Should archive resolved review-only workflow",
			reviewStatus: []string{"resolved", "resolved"},
			wantArchived: 1,
			wantSkipped:  0,
			wantRemoved:  true,
		},
		{
			name:         "Should require force for unresolved review-only workflow",
			reviewStatus: []string{"resolved", "pending"},
			wantErr:      ErrWorkflowForceRequired,
			wantArchived: 0,
			wantSkipped:  0,
			wantRemoved:  false,
		},
		{
			name:                     "Should force archive unresolved review-only workflow",
			reviewStatus:             []string{"resolved", "pending"},
			force:                    true,
			wantArchived:             1,
			wantSkipped:              0,
			wantRemoved:              true,
			wantForced:               true,
			wantResolvedReviewIssues: 1,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			rootDir := archiveTestRoot(t)
			workflowDir := filepath.Join(rootDir, "review-only")
			writeArchiveReviewRound(t, workflowDir, 1, tc.reviewStatus, false)
			mustSyncArchiveWorkflow(t, workflowDir)

			result, err := Archive(context.Background(), ArchiveConfig{TasksDir: workflowDir, Force: tc.force})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Archive(review-only) error = %v, want %v", err, tc.wantErr)
			}
			if result == nil || result.WorkflowsScanned != 1 || result.Archived != tc.wantArchived ||
				result.Skipped != tc.wantSkipped {
				t.Fatalf("unexpected archive result: %#v", result)
			}
			if result.Forced != tc.wantForced || result.ResolvedReviewIssues != tc.wantResolvedReviewIssues {
				t.Fatalf("unexpected force result details: %#v", result)
			}
			if tc.wantRemoved {
				if len(result.ArchivedPaths) != 1 {
					t.Fatalf("archived paths = %#v, want one archived path", result.ArchivedPaths)
				}
				if _, statErr := os.Stat(workflowDir); !os.IsNotExist(statErr) {
					t.Fatalf("expected review-only workflow to leave active root, got err=%v", statErr)
				}
				body, readErr := os.ReadFile(filepath.Join(result.ArchivedPaths[0], "reviews-001", "issue_002.md"))
				if readErr != nil {
					t.Fatalf("read archived issue: %v", readErr)
				}
				if tc.wantResolvedReviewIssues > 0 && !strings.Contains(string(body), "status: resolved") {
					t.Fatalf("expected archived issue to be resolved, got:\n%s", string(body))
				}
				return
			}
			if _, statErr := os.Stat(workflowDir); statErr != nil {
				t.Fatalf("expected unresolved review-only workflow dir to remain: %v", statErr)
			}
		})
	}
}

func TestArchiveTaskWorkflowRefreshesStaleEmptyCatalogForReviewOnlyWorkflows(t *testing.T) {
	testCases := []struct {
		name                 string
		reviewStatus         string
		wantErr              error
		wantArchived         int
		wantForceReviewTotal int
		wantForceUnresolved  int
	}{
		{
			name:         "Should archive resolved review-only workflow after stale empty sync",
			reviewStatus: "resolved",
			wantArchived: 1,
		},
		{
			name:                 "Should require force for unresolved review-only workflow after stale empty sync",
			reviewStatus:         "pending",
			wantErr:              ErrWorkflowForceRequired,
			wantForceReviewTotal: 1,
			wantForceUnresolved:  1,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			rootDir := archiveTestRoot(t)
			workflowDir := filepath.Join(rootDir, "action-gaps")
			if err := os.MkdirAll(workflowDir, 0o755); err != nil {
				t.Fatalf("mkdir workflow dir: %v", err)
			}
			mustSyncArchiveWorkflow(t, workflowDir)

			writeArchiveReviewRound(t, workflowDir, 1, []string{tc.reviewStatus}, false)

			result, err := Archive(context.Background(), ArchiveConfig{TasksDir: workflowDir})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Archive(stale review-only) error = %v, want %v", err, tc.wantErr)
			}
			if result == nil || result.WorkflowsScanned != 1 || result.Archived != tc.wantArchived {
				t.Fatalf("unexpected archive result: %#v", result)
			}

			if tc.wantErr == nil {
				if len(result.ArchivedPaths) != 1 {
					t.Fatalf("ArchivedPaths = %#v, want one path", result.ArchivedPaths)
				}
				if _, statErr := os.Stat(workflowDir); !os.IsNotExist(statErr) {
					t.Fatalf("expected review-only workflow to leave active root, got err=%v", statErr)
				}
				return
			}

			var forceRequired WorkflowArchiveForceRequiredError
			if !errors.As(err, &forceRequired) {
				t.Fatalf("Archive() error = %T, want WorkflowArchiveForceRequiredError", err)
			}
			if forceRequired.ReviewTotal != tc.wantForceReviewTotal ||
				forceRequired.ReviewUnresolved != tc.wantForceUnresolved {
				t.Fatalf("unexpected force-required details: %#v", forceRequired)
			}
			if errors.Is(err, globaldb.ErrWorkflowNotArchivable) {
				t.Fatalf("Archive() error = %v, should not report stale no-task eligibility", err)
			}
			if _, statErr := os.Stat(workflowDir); statErr != nil {
				t.Fatalf("expected unresolved review-only workflow dir to remain: %v", statErr)
			}
		})
	}
}

func TestArchiveTaskWorkflowKeepsTrulyEmptyWorkflowNotArchivableAfterRefresh(t *testing.T) {
	t.Run("Should keep truly empty workflow not archivable after refresh", func(t *testing.T) {
		rootDir := archiveTestRoot(t)
		workflowDir := filepath.Join(rootDir, "action-gaps")
		if err := os.MkdirAll(workflowDir, 0o755); err != nil {
			t.Fatalf("mkdir workflow dir: %v", err)
		}
		mustSyncArchiveWorkflow(t, workflowDir)

		result, err := Archive(context.Background(), ArchiveConfig{TasksDir: workflowDir})
		if !errors.Is(err, globaldb.ErrWorkflowNotArchivable) {
			t.Fatalf("Archive(empty workflow) error = %v, want ErrWorkflowNotArchivable", err)
		}
		var notArchivable globaldb.WorkflowNotArchivableError
		if !errors.As(err, &notArchivable) {
			t.Fatalf("Archive(empty workflow) error = %T, want WorkflowNotArchivableError", err)
		}
		if result == nil || result.WorkflowsScanned != 1 || result.Archived != 0 {
			t.Fatalf("unexpected archive result for empty workflow: %#v", result)
		}
		if _, statErr := os.Stat(workflowDir); statErr != nil {
			t.Fatalf("expected empty workflow dir to remain: %v", statErr)
		}
	})
}

func TestArchiveTaskWorkflowForceCompletesTasksAndResolvesReviewsBeforeArchiving(t *testing.T) {
	rootDir := archiveTestRoot(t)
	workflowDir := filepath.Join(rootDir, "daemon")
	writeArchiveTaskFile(t, workflowDir, "task_001.md", "pending")
	writeArchiveTaskFile(t, workflowDir, "task_002.md", "in_progress")
	writeArchiveTaskFile(t, workflowDir, "task_003.md", "completed")
	writeArchiveReviewRound(t, workflowDir, 1, []string{"pending", "valid", "resolved"}, true)
	mustSyncArchiveWorkflow(t, workflowDir)

	if _, err := Archive(
		context.Background(),
		ArchiveConfig{TasksDir: workflowDir},
	); !errors.Is(
		err,
		ErrWorkflowForceRequired,
	) {
		t.Fatalf("Archive() error = %v, want ErrWorkflowForceRequired", err)
	}

	result, err := Archive(context.Background(), ArchiveConfig{TasksDir: workflowDir, Force: true})
	if err != nil {
		t.Fatalf("Archive(force): %v", err)
	}
	if result == nil {
		t.Fatal("expected archive result")
	}
	if result.Archived != 1 || !result.Forced {
		t.Fatalf("unexpected forced archive result: %#v", result)
	}
	if result.CompletedTasks != 2 {
		t.Fatalf("CompletedTasks = %d, want 2", result.CompletedTasks)
	}
	if result.ResolvedReviewIssues != 2 {
		t.Fatalf("ResolvedReviewIssues = %d, want 2", result.ResolvedReviewIssues)
	}
	if result.ArchivedAt == nil || len(result.ArchivedPaths) != 1 {
		t.Fatalf("expected archived path and timestamp, got %#v", result)
	}

	archivedDir := result.ArchivedPaths[0]
	for _, name := range []string{"task_001.md", "task_002.md", "task_003.md"} {
		body, err := os.ReadFile(filepath.Join(archivedDir, name))
		if err != nil {
			t.Fatalf("read archived task %s: %v", name, err)
		}
		if !strings.Contains(string(body), "status: completed") {
			t.Fatalf("expected archived task %s to be completed, got:\n%s", name, string(body))
		}
	}

	for _, name := range []string{"issue_001.md", "issue_002.md", "issue_003.md"} {
		body, err := os.ReadFile(filepath.Join(archivedDir, "reviews-001", name))
		if err != nil {
			t.Fatalf("read archived issue %s: %v", name, err)
		}
		if !strings.Contains(string(body), "status: resolved") {
			t.Fatalf("expected archived issue %s to be resolved, got:\n%s", name, string(body))
		}
	}
}

func TestArchiveTaskWorkflowRejectsArchivedTargetsAndArchivedIdentities(t *testing.T) {
	rootDir := archiveTestRoot(t)
	workflowDir := filepath.Join(rootDir, "alpha")
	writeArchiveTaskFile(t, workflowDir, "task_001.md", "completed")
	mustSyncArchiveWorkflow(t, workflowDir)

	firstResult, err := Archive(context.Background(), ArchiveConfig{TasksDir: workflowDir})
	if err != nil {
		t.Fatalf("Archive(first): %v", err)
	}
	if firstResult == nil || len(firstResult.ArchivedPaths) != 1 {
		t.Fatalf("unexpected first archive result: %#v", firstResult)
	}

	if _, err := Archive(context.Background(), ArchiveConfig{TasksDir: firstResult.ArchivedPaths[0]}); err == nil {
		t.Fatal("expected archive to reject already archived directory paths")
	}

	if _, err := Archive(
		context.Background(),
		ArchiveConfig{RootDir: rootDir, Name: "alpha"},
	); !errors.Is(
		err,
		globaldb.ErrWorkflowArchived,
	) {
		t.Fatalf("Archive(archived identity) error = %v, want ErrWorkflowArchived", err)
	}
}

func TestArchiveTaskWorkflowRejectsArchivedIdentityFromCatalogWithoutArchivedDir(t *testing.T) {
	rootDir := archiveTestRoot(t)
	workflowDir := filepath.Join(rootDir, "alpha")
	writeArchiveTaskFile(t, workflowDir, "task_001.md", "completed")
	mustSyncArchiveWorkflow(t, workflowDir)

	firstResult, err := Archive(context.Background(), ArchiveConfig{TasksDir: workflowDir})
	if err != nil {
		t.Fatalf("Archive(first): %v", err)
	}
	if firstResult == nil || len(firstResult.ArchivedPaths) != 1 {
		t.Fatalf("unexpected first archive result: %#v", firstResult)
	}
	if err := os.RemoveAll(firstResult.ArchivedPaths[0]); err != nil {
		t.Fatalf("RemoveAll(archived path): %v", err)
	}

	if _, err := Archive(
		context.Background(),
		ArchiveConfig{RootDir: rootDir, Name: "alpha"},
	); !errors.Is(
		err,
		globaldb.ErrWorkflowArchived,
	) {
		t.Fatalf("Archive(archived identity without archived dir) error = %v, want ErrWorkflowArchived", err)
	}
}

func TestArchiveTaskWorkflowsRootScanSkipsUnsyncedWorkflowState(t *testing.T) {
	rootDir := archiveTestRoot(t)
	completedDir := filepath.Join(rootDir, "alpha")
	unsyncedDir := filepath.Join(rootDir, "beta")

	writeArchiveTaskFile(t, completedDir, "task_001.md", "completed")
	writeArchiveTaskFile(t, unsyncedDir, "task_001.md", "completed")
	mustSyncArchiveWorkflow(t, completedDir)

	result, err := Archive(context.Background(), ArchiveConfig{RootDir: rootDir})
	if err != nil {
		t.Fatalf("Archive(root): %v", err)
	}
	if result.Skipped != 1 {
		t.Fatalf("Skipped = %d, want 1", result.Skipped)
	}
	if got := result.SkippedReasons[unsyncedDir]; got != workflowStateNotSyncedReason {
		t.Fatalf("skip reason for unsynced workflow = %q, want %q", got, workflowStateNotSyncedReason)
	}
}

func archiveTestRoot(t *testing.T) string {
	t.Helper()
	t.Setenv("HOME", filepath.Join(t.TempDir(), "home"))

	rootDir := filepath.Join(t.TempDir(), ".compozy", "tasks")
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		t.Fatalf("mkdir tasks root: %v", err)
	}
	return rootDir
}

func mustSyncArchiveWorkflow(t *testing.T, workflowDir string) {
	t.Helper()

	result, err := Sync(context.Background(), SyncConfig{TasksDir: workflowDir})
	if err != nil {
		t.Fatalf("Sync(%s) error = %v", workflowDir, err)
	}
	if result == nil || result.WorkflowsScanned != 1 {
		t.Fatalf("unexpected sync result for %s: %#v", workflowDir, result)
	}
}

func mustSyncArchiveRoot(t *testing.T, rootDir string) {
	t.Helper()

	result, err := Sync(context.Background(), SyncConfig{RootDir: rootDir})
	if err != nil {
		t.Fatalf("Sync(root=%s) error = %v", rootDir, err)
	}
	if result == nil {
		t.Fatalf("expected sync result for %s", rootDir)
	}
}

func openArchiveWorkflowDB(t *testing.T, target string) (*globaldb.GlobalDB, globaldb.Workspace) {
	t.Helper()

	db, workspace, err := openWorkflowGlobalDB(context.Background(), target)
	if err != nil {
		t.Fatalf("openWorkflowGlobalDB(%s): %v", target, err)
	}
	return db, workspace
}

func insertActiveArchiveRun(t *testing.T, target string, slug string, runID string) {
	t.Helper()

	db, workspace := openArchiveWorkflowDB(t, target)
	defer func() {
		_ = db.Close()
	}()

	workflow, err := db.GetActiveWorkflowBySlug(context.Background(), workspace.ID, slug)
	if err != nil {
		t.Fatalf("GetActiveWorkflowBySlug(%s): %v", slug, err)
	}
	if _, err := db.PutRun(context.Background(), globaldb.Run{
		RunID:            runID,
		WorkspaceID:      workspace.ID,
		WorkflowID:       &workflow.ID,
		Mode:             "task",
		Status:           "running",
		PresentationMode: "stream",
		StartedAt:        time.Date(2026, 4, 17, 19, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("PutRun(%s): %v", runID, err)
	}
}

func assertArchiveFileUnchanged(t *testing.T, path string, before []byte) {
	t.Helper()

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read artifact after blocked archive: %v", err)
	}
	if !bytes.Equal(after, before) {
		t.Fatalf("artifact %s changed after blocked archive\nbefore:\n%s\nafter:\n%s", path, before, after)
	}
}

func writeSingleTaskGroupInitiativePlan(t *testing.T, initiativeDir string) {
	t.Helper()

	body := strings.Join([]string{
		"---",
		"schema_version: compozy.task-groups/v1",
		"initiative: initiative",
		"graph:",
		"  nodes:",
		"    - id: TG-001",
		"      directory: _task_groups/TG-001",
		"  edges: []",
		"---",
		"",
		"# Initiative Task Groups",
		"",
		"## [x] TG-001 — Persistence",
		"",
		"- Reference: `initiative/TG-001`",
		"- Outcome: Persist the parent workflow.",
		"- Owns:",
		"  - persistence",
		"- Dependencies: None",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(initiativeDir, "_task_groups.md"), []byte(body), 0o600); err != nil {
		t.Fatalf("rewrite initiative plan: %v", err)
	}
}

func writeArchiveTaskMeta(t *testing.T, workflowDir string, content string) {
	t.Helper()
	if err := os.WriteFile(tasks.MetaPath(workflowDir), []byte(content), 0o600); err != nil {
		t.Fatalf("write task meta: %v", err)
	}
}

func rewriteArchiveIssueStatus(path string, from string, to string) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read issue file: %w", err)
	}
	rewritten := strings.Replace(string(body), "status: "+from, "status: "+to, 1)
	if err := os.WriteFile(path, []byte(rewritten), 0o600); err != nil {
		return fmt.Errorf("write issue file: %w", err)
	}
	return nil
}

func equalStrings(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for idx := range left {
		if left[idx] != right[idx] {
			return false
		}
	}
	return true
}

func writeArchiveTaskFile(t *testing.T, workflowDir string, name string, status string) {
	t.Helper()

	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}

	content := strings.Join([]string{
		"---",
		"status: " + status,
		"title: " + name,
		"type: backend",
		"complexity: low",
		"---",
		"",
		"# " + name,
		"",
	}, "\n")

	if err := os.WriteFile(filepath.Join(workflowDir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func writeArchiveReviewRound(t *testing.T, workflowDir string, round int, statuses []string, withMeta bool) {
	t.Helper()

	reviewDir := reviews.ReviewDirectory(workflowDir, round)
	if err := os.MkdirAll(reviewDir, 0o755); err != nil {
		t.Fatalf("mkdir review dir: %v", err)
	}

	resolvedCount := 0
	for idx, status := range statuses {
		if status == "resolved" {
			resolvedCount++
		}
		content := strings.Join([]string{
			"---",
			"status: " + status,
			"file: internal/app/service.go",
			"line: 42",
			"severity: medium",
			"author: coderabbitai[bot]",
			"provider_ref: thread:PRT_1,comment:RC_1",
			"---",
			"",
			"Review body",
			"",
		}, "\n")
		name := filepath.Join(reviewDir, "issue_"+formatArchiveIssueNumber(idx+1)+".md")
		if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
			t.Fatalf("write review issue: %v", err)
		}
	}

	if !withMeta {
		return
	}

	if err := reviews.WriteRoundMeta(reviewDir, model.RoundMeta{
		Provider:   "coderabbit",
		PR:         "259",
		Round:      round,
		CreatedAt:  time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
		Total:      len(statuses),
		Resolved:   resolvedCount,
		Unresolved: len(statuses) - resolvedCount,
	}); err != nil {
		t.Fatalf("write review meta: %v", err)
	}
}

func formatArchiveIssueNumber(n int) string {
	return fmt.Sprintf("%03d", n)
}
