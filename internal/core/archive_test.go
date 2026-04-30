package core

import (
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
	"github.com/compozy/compozy/internal/core/tasks"
	"github.com/compozy/compozy/internal/store/globaldb"
)

func TestArchiveTaskWorkflowRejectsPendingStateFromSyncedDBEvenWithStaleMeta(t *testing.T) {
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
	if !errors.Is(err, globaldb.ErrWorkflowNotArchivable) {
		t.Fatalf("Archive() error = %v, want ErrWorkflowNotArchivable", err)
	}
	if result == nil {
		t.Fatal("expected archive result")
	}
	if result.WorkflowsScanned != 1 || result.Archived != 0 || result.Skipped != 0 {
		t.Fatalf("unexpected archive result: %#v", result)
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

func TestArchiveTaskWorkflowRequiresResyncAfterReviewResolution(t *testing.T) {
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
	if !errors.Is(err, globaldb.ErrWorkflowNotArchivable) {
		t.Fatalf("Archive(without resync) error = %v, want ErrWorkflowNotArchivable", err)
	}
	if result == nil || result.Archived != 0 {
		t.Fatalf("unexpected archive result before resync: %#v", result)
	}

	writeArchiveReviewRound(t, workflowDir, 1, []string{"resolved"}, true)
	mustSyncArchiveWorkflow(t, workflowDir)

	result, err = Archive(context.Background(), ArchiveConfig{TasksDir: workflowDir})
	if err != nil {
		t.Fatalf("Archive(after resync): %v", err)
	}
	if result == nil || result.Archived != 1 || len(result.ArchivedPaths) != 1 {
		t.Fatalf("unexpected archive result after resync: %#v", result)
	}
}

func TestArchiveTaskWorkflowAllowsResolvedReviewOnlyWorkflow(t *testing.T) {
	rootDir := archiveTestRoot(t)
	workflowDir := filepath.Join(rootDir, "review-only")
	writeArchiveReviewRound(t, workflowDir, 1, []string{"resolved", "resolved"}, false)
	mustSyncArchiveWorkflow(t, workflowDir)

	result, err := Archive(context.Background(), ArchiveConfig{TasksDir: workflowDir})
	if err != nil {
		t.Fatalf("Archive(review-only resolved): %v", err)
	}
	if result == nil || result.WorkflowsScanned != 1 || result.Archived != 1 ||
		len(result.ArchivedPaths) != 1 {
		t.Fatalf("unexpected archive result: %#v", result)
	}
	if _, statErr := os.Stat(workflowDir); !os.IsNotExist(statErr) {
		t.Fatalf("expected review-only workflow to leave active root, got err=%v", statErr)
	}
}

func TestArchiveTaskWorkflowRejectsUnresolvedReviewOnlyWorkflow(t *testing.T) {
	rootDir := archiveTestRoot(t)
	workflowDir := filepath.Join(rootDir, "review-only")
	writeArchiveReviewRound(t, workflowDir, 1, []string{"resolved", "pending"}, false)
	mustSyncArchiveWorkflow(t, workflowDir)

	result, err := Archive(context.Background(), ArchiveConfig{TasksDir: workflowDir})
	if !errors.Is(err, globaldb.ErrWorkflowNotArchivable) {
		t.Fatalf("Archive(review-only unresolved) error = %v, want ErrWorkflowNotArchivable", err)
	}
	if result == nil || result.WorkflowsScanned != 1 || result.Archived != 0 ||
		result.Skipped != 0 {
		t.Fatalf("unexpected archive result: %#v", result)
	}
	if _, statErr := os.Stat(workflowDir); statErr != nil {
		t.Fatalf("expected unresolved review-only workflow dir to remain: %v", statErr)
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
