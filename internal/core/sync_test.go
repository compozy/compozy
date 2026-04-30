package core

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
)

func TestSyncTaskMetadataSyncsSingleWorkflowIntoGlobalDBWithoutMutatingArtifacts(t *testing.T) {
	workspaceRoot := t.TempDir()
	setSyncTestHome(t)

	workflowDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "demo")
	writeSyncWorkflowFile(t, workflowDir, "task_01.md", taskBody("pending", "Demo task"))
	writeSyncWorkflowFile(t, workflowDir, "_tasks.md", canonicalTaskListBody())
	writeSyncWorkflowFile(t, workflowDir, "_techspec.md", "# Techspec\n")
	writeSyncWorkflowFile(t, workflowDir, filepath.Join("adrs", "adr-001.md"), "# ADR 001\n")
	writeSyncWorkflowFile(t, workflowDir, filepath.Join("memory", "MEMORY.md"), "# Workflow Memory\n")

	originalBodies := map[string]string{
		"task_01.md":       mustReadFile(t, filepath.Join(workflowDir, "task_01.md")),
		"_tasks.md":        mustReadFile(t, filepath.Join(workflowDir, "_tasks.md")),
		"_techspec.md":     mustReadFile(t, filepath.Join(workflowDir, "_techspec.md")),
		"adrs/adr-001.md":  mustReadFile(t, filepath.Join(workflowDir, "adrs", "adr-001.md")),
		"memory/MEMORY.md": mustReadFile(t, filepath.Join(workflowDir, "memory", "MEMORY.md")),
	}

	result, err := Sync(context.Background(), SyncConfig{TasksDir: workflowDir})
	if err != nil {
		t.Fatalf("Sync(): %v", err)
	}
	if result.WorkflowsScanned != 1 {
		t.Fatalf("WorkflowsScanned = %d, want 1", result.WorkflowsScanned)
	}
	if result.TaskItemsUpserted != 1 || result.CheckpointsUpdated != 1 {
		t.Fatalf("unexpected sync result counts: %#v", result)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %#v", result.Warnings)
	}

	for relativePath, want := range originalBodies {
		path := filepath.Join(workflowDir, filepath.FromSlash(relativePath))
		if got := mustReadFile(t, path); got != want {
			t.Fatalf("artifact mutated during sync: %s", relativePath)
		}
	}
	if _, err := os.Stat(filepath.Join(workflowDir, "_meta.md")); !os.IsNotExist(err) {
		t.Fatalf("expected workflow _meta.md to remain absent, got err=%v", err)
	}

	sqlDB := openSyncSQLite(t)
	defer func() {
		_ = sqlDB.Close()
	}()

	if got := queryCount(t, sqlDB, "SELECT COUNT(1) FROM workflows"); got != 1 {
		t.Fatalf("workflows count = %d, want 1", got)
	}
	if got := queryCount(t, sqlDB, "SELECT COUNT(1) FROM artifact_snapshots"); got != 5 {
		t.Fatalf("artifact_snapshots count = %d, want 5", got)
	}
	if got := queryCount(t, sqlDB, "SELECT COUNT(1) FROM task_items"); got != 1 {
		t.Fatalf("task_items count = %d, want 1", got)
	}
}

func TestSyncTaskMetadataScansWorkflowRootIntoGlobalDB(t *testing.T) {
	workspaceRoot := t.TempDir()
	setSyncTestHome(t)

	rootDir := filepath.Join(workspaceRoot, ".compozy", "tasks")
	alphaDir := filepath.Join(rootDir, "alpha")
	betaDir := filepath.Join(rootDir, "beta")
	archivedDir := filepath.Join(rootDir, "_archived")

	writeSyncWorkflowFile(t, alphaDir, "task_01.md", taskBody("pending", "Alpha"))
	writeSyncWorkflowFile(t, betaDir, "task_01.md", taskBody("completed", "Beta"))
	if err := os.MkdirAll(archivedDir, 0o755); err != nil {
		t.Fatalf("mkdir archived dir: %v", err)
	}

	result, err := Sync(context.Background(), SyncConfig{RootDir: rootDir})
	if err != nil {
		t.Fatalf("Sync(): %v", err)
	}
	if result.WorkflowsScanned != 2 {
		t.Fatalf("WorkflowsScanned = %d, want 2", result.WorkflowsScanned)
	}
	if !reflect.DeepEqual(result.SyncedPaths, []string{alphaDir, betaDir}) {
		t.Fatalf("unexpected synced paths: %#v", result.SyncedPaths)
	}

	sqlDB := openSyncSQLite(t)
	defer func() {
		_ = sqlDB.Close()
	}()

	if got := queryCount(t, sqlDB, "SELECT COUNT(1) FROM workflows"); got != 2 {
		t.Fatalf("workflows count = %d, want 2", got)
	}
	if got := queryCount(t, sqlDB, "SELECT COUNT(1) FROM task_items"); got != 2 {
		t.Fatalf("task_items count = %d, want 2", got)
	}
}

func TestSyncTaskMetadataResyncUpdatesExistingWorkflowAndTaskIdentity(t *testing.T) {
	workspaceRoot := t.TempDir()
	setSyncTestHome(t)

	workflowDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "identity-demo")
	taskPath := filepath.Join(workflowDir, "task_01.md")
	writeSyncWorkflowFile(t, workflowDir, "task_01.md", taskBody("pending", "Original"))

	if _, err := Sync(context.Background(), SyncConfig{TasksDir: workflowDir}); err != nil {
		t.Fatalf("Sync(first): %v", err)
	}

	sqlDB := openSyncSQLite(t)
	defer func() {
		_ = sqlDB.Close()
	}()

	var (
		workflowID string
		taskRowID  string
		taskID     string
		sourcePath string
	)
	if err := sqlDB.QueryRowContext(
		context.Background(),
		`SELECT w.id, t.id, t.task_id, t.source_path
		 FROM workflows w
		 JOIN task_items t ON t.workflow_id = w.id
		 WHERE w.slug = ? AND t.task_number = 1`,
		"identity-demo",
	).Scan(&workflowID, &taskRowID, &taskID, &sourcePath); err != nil {
		t.Fatalf("query first sync identity rows: %v", err)
	}

	if err := os.WriteFile(taskPath, []byte(taskBody("completed", "Updated title")), 0o600); err != nil {
		t.Fatalf("rewrite task: %v", err)
	}
	if _, err := Sync(context.Background(), SyncConfig{TasksDir: workflowDir}); err != nil {
		t.Fatalf("Sync(second): %v", err)
	}

	var (
		workflowIDAfter string
		taskRowIDAfter  string
		taskTitleAfter  string
		taskStatusAfter string
	)
	if err := sqlDB.QueryRowContext(
		context.Background(),
		`SELECT w.id, t.id, t.title, t.status
		 FROM workflows w
		 JOIN task_items t ON t.workflow_id = w.id
		 WHERE w.slug = ? AND t.task_number = 1`,
		"identity-demo",
	).Scan(&workflowIDAfter, &taskRowIDAfter, &taskTitleAfter, &taskStatusAfter); err != nil {
		t.Fatalf("query second sync identity rows: %v", err)
	}

	if workflowIDAfter != workflowID {
		t.Fatalf("workflow id changed across resync: before=%q after=%q", workflowID, workflowIDAfter)
	}
	if taskRowIDAfter != taskRowID {
		t.Fatalf("task row id changed across resync: before=%q after=%q", taskRowID, taskRowIDAfter)
	}
	if taskID != "task_01" {
		t.Fatalf("task_id = %q, want task_01", taskID)
	}
	if sourcePath != "task_01.md" {
		t.Fatalf("source_path = %q, want task_01.md", sourcePath)
	}
	if taskTitleAfter != "Updated title" || taskStatusAfter != "completed" {
		t.Fatalf("unexpected task row after resync: title=%q status=%q", taskTitleAfter, taskStatusAfter)
	}
	if got := queryCount(t, sqlDB, "SELECT COUNT(1) FROM task_items"); got != 1 {
		t.Fatalf("task_items count = %d, want 1", got)
	}
}

func TestSyncTaskMetadataSyncsMixedWorkflowArtifacts(t *testing.T) {
	workspaceRoot := t.TempDir()
	setSyncTestHome(t)

	workflowDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "mixed-demo")
	writeSyncWorkflowFile(t, workflowDir, "task_01.md", taskBody("pending", "Mixed task"))
	writeSyncWorkflowFile(t, workflowDir, filepath.Join("memory", "MEMORY.md"), "# Workflow Memory\n")
	writeSyncWorkflowFile(t, workflowDir, filepath.Join("prompts", "task-run.md"), "# Prompt\n")
	writeSyncWorkflowFile(t, workflowDir, filepath.Join("protocol", "handoff.md"), "# Protocol\n")
	writeSyncWorkflowFile(t, workflowDir, filepath.Join("qa", "verification-report.md"), "# QA\n")
	writeSyncWorkflowFile(t, workflowDir, filepath.Join("adrs", "adr-001.md"), "# ADR 001\n")
	writeSyncWorkflowFile(
		t,
		workflowDir,
		filepath.Join("reviews-001", "_meta.md"),
		reviewRoundMetaBody("coderabbit", "456", 1),
	)
	writeSyncWorkflowFile(
		t,
		workflowDir,
		filepath.Join("reviews-001", "issue_001.md"),
		reviewIssueBody("pending", "medium"),
	)

	result, err := Sync(context.Background(), SyncConfig{TasksDir: workflowDir})
	if err != nil {
		t.Fatalf("Sync(): %v", err)
	}
	if result.WorkflowsScanned != 1 || result.ReviewRoundsUpserted != 1 || result.ReviewIssuesUpserted != 1 {
		t.Fatalf("unexpected sync result: %#v", result)
	}

	sqlDB := openSyncSQLite(t)
	defer func() {
		_ = sqlDB.Close()
	}()

	if got := queryCount(t, sqlDB, "SELECT COUNT(1) FROM artifact_snapshots"); got != 7 {
		t.Fatalf("artifact_snapshots count = %d, want 7", got)
	}
	if got := queryCount(t, sqlDB, "SELECT COUNT(1) FROM review_rounds"); got != 1 {
		t.Fatalf("review_rounds count = %d, want 1", got)
	}
	if got := queryCount(t, sqlDB, "SELECT COUNT(1) FROM review_issues"); got != 1 {
		t.Fatalf("review_issues count = %d, want 1", got)
	}
}

func TestSyncTaskMetadataRemovesLegacyGeneratedMetadataOnce(t *testing.T) {
	workspaceRoot := t.TempDir()
	setSyncTestHome(t)

	workflowDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "legacy-demo")
	writeSyncWorkflowFile(t, workflowDir, "task_01.md", taskBody("pending", "Legacy task"))
	writeSyncWorkflowFile(t, workflowDir, "_meta.md", legacyMetaBody())
	writeSyncWorkflowFile(t, workflowDir, "_tasks.md", "Legacy generated summary\n")

	firstResult, err := Sync(context.Background(), SyncConfig{TasksDir: workflowDir})
	if err != nil {
		t.Fatalf("Sync(first): %v", err)
	}
	if firstResult.LegacyArtifactsRemoved != 2 {
		t.Fatalf("LegacyArtifactsRemoved = %d, want 2", firstResult.LegacyArtifactsRemoved)
	}
	if len(firstResult.Warnings) != 1 {
		t.Fatalf("warnings len = %d, want 1", len(firstResult.Warnings))
	}
	if !strings.Contains(firstResult.Warnings[0], "_meta.md, _tasks.md") {
		t.Fatalf("unexpected cleanup warning: %#v", firstResult.Warnings)
	}
	if _, err := os.Stat(filepath.Join(workflowDir, "_meta.md")); !os.IsNotExist(err) {
		t.Fatalf("expected workflow _meta.md to be removed, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(workflowDir, "_tasks.md")); !os.IsNotExist(err) {
		t.Fatalf("expected legacy _tasks.md to be removed, got err=%v", err)
	}

	secondResult, err := Sync(context.Background(), SyncConfig{TasksDir: workflowDir})
	if err != nil {
		t.Fatalf("Sync(second): %v", err)
	}
	if secondResult.LegacyArtifactsRemoved != 0 {
		t.Fatalf("LegacyArtifactsRemoved(second) = %d, want 0", secondResult.LegacyArtifactsRemoved)
	}
	if len(secondResult.Warnings) != 0 {
		t.Fatalf("expected no repeat cleanup warning, got %#v", secondResult.Warnings)
	}
}

func TestResolveSyncTargetRejectsConflictingTargets(t *testing.T) {
	t.Parallel()

	_, _, err := resolveSyncTarget(SyncConfig{
		Name:     "alpha",
		TasksDir: ".compozy/tasks/alpha",
	})
	if err == nil {
		t.Fatal("expected conflicting sync target selectors to fail")
	}
	if !strings.Contains(err.Error(), "--name or --tasks-dir") {
		t.Fatalf("unexpected conflicting target error: %v", err)
	}
}

func TestSnapshotArtifactContentHandlesPlainMarkdownAndInvalidFrontmatter(t *testing.T) {
	t.Parallel()

	frontmatterJSON, body, err := snapshotArtifactContent("# Plain markdown\n")
	if err != nil {
		t.Fatalf("snapshotArtifactContent(plain markdown): %v", err)
	}
	if frontmatterJSON != "{}" || body != "# Plain markdown\n" {
		t.Fatalf("unexpected plain markdown snapshot: frontmatter=%q body=%q", frontmatterJSON, body)
	}

	if _, _, err := snapshotArtifactContent(strings.Join([]string{
		"---",
		"status: pending",
		"# missing footer",
	}, "\n")); err == nil {
		t.Fatal("expected invalid front matter to fail")
	}
}

func TestCleanupLegacyWorkflowMetadataPreservesCanonicalTaskList(t *testing.T) {
	t.Parallel()

	workflowDir := t.TempDir()
	writeSyncWorkflowFile(t, workflowDir, "_meta.md", legacyMetaBody())
	writeSyncWorkflowFile(t, workflowDir, "_tasks.md", canonicalTaskListBody())

	removed, err := cleanupLegacyWorkflowMetadata(workflowDir)
	if err != nil {
		t.Fatalf("cleanupLegacyWorkflowMetadata(): %v", err)
	}
	if !reflect.DeepEqual(removed, []string{"_meta.md"}) {
		t.Fatalf("removed legacy files = %#v, want only _meta.md", removed)
	}
	if _, err := os.Stat(filepath.Join(workflowDir, "_tasks.md")); err != nil {
		t.Fatalf("expected canonical _tasks.md to remain: %v", err)
	}

	writeSyncWorkflowFile(t, workflowDir, "_tasks.md", "Legacy generated summary\n")
	removed, err = cleanupLegacyWorkflowMetadata(workflowDir)
	if err != nil {
		t.Fatalf("cleanupLegacyWorkflowMetadata(noncanonical): %v", err)
	}
	if !reflect.DeepEqual(removed, []string{"_tasks.md"}) {
		t.Fatalf("removed legacy files on second pass = %#v, want only _tasks.md", removed)
	}
}

func TestCollectArtifactSnapshotsSkipsHiddenDirsAndClassifiesAuthoredTaskList(t *testing.T) {
	t.Parallel()

	workflowDir := t.TempDir()
	writeSyncWorkflowFile(t, workflowDir, "_tasks.md", canonicalTaskListBody())
	writeSyncWorkflowFile(t, workflowDir, filepath.Join(".tmp", "ignored.md"), "# Ignore me\n")
	writeSyncWorkflowFile(t, workflowDir, filepath.Join("qa", "verification-report.md"), "# QA\n")

	snapshots, checkpointChecksum, err := collectArtifactSnapshots(workflowDir)
	if err != nil {
		t.Fatalf("collectArtifactSnapshots(): %v", err)
	}
	if checkpointChecksum == "" {
		t.Fatal("expected non-empty checkpoint checksum")
	}
	if len(snapshots) != 2 {
		t.Fatalf("snapshot count = %d, want 2", len(snapshots))
	}
	kindsByPath := map[string]string{
		snapshots[0].RelativePath: snapshots[0].ArtifactKind,
		snapshots[1].RelativePath: snapshots[1].ArtifactKind,
	}
	if kindsByPath["_tasks.md"] != "tasks_index" {
		t.Fatalf("_tasks.md artifact kind = %q, want tasks_index", kindsByPath["_tasks.md"])
	}
	if kindsByPath["qa/verification-report.md"] != "qa" {
		t.Fatalf(
			"qa/verification-report.md artifact kind = %q, want qa",
			kindsByPath["qa/verification-report.md"],
		)
	}
}

func TestCollectTaskItemsRejectsInvalidTaskArtifacts(t *testing.T) {
	t.Parallel()

	workflowDir := t.TempDir()
	writeSyncWorkflowFile(t, workflowDir, "task_01.md", strings.Join([]string{
		"---",
		"status: pending",
		"domain: backend",
		"type: backend",
		"scope: small",
		"complexity: low",
		"---",
		"",
		"# Task 01",
		"",
	}, "\n"))

	if _, err := collectTaskItems(workflowDir); err == nil {
		t.Fatal("expected invalid task artifact to fail parsing")
	}
}

func TestCollectReviewRoundsBuildsResolvedCountsAndRejectsRoundMismatch(t *testing.T) {
	t.Parallel()

	workflowDir := t.TempDir()
	writeSyncWorkflowFile(
		t,
		workflowDir,
		filepath.Join("reviews-001", "_meta.md"),
		reviewRoundMetaBody("coderabbit", "123", 1),
	)
	writeSyncWorkflowFile(
		t,
		workflowDir,
		filepath.Join("reviews-001", "issue_001.md"),
		reviewIssueBody("resolved", "high"),
	)

	rounds, err := collectReviewRounds(workflowDir)
	if err != nil {
		t.Fatalf("collectReviewRounds(valid): %v", err)
	}
	if len(rounds) != 1 || rounds[0].ResolvedCount != 1 || rounds[0].UnresolvedCount != 0 {
		t.Fatalf("unexpected review round projection: %#v", rounds)
	}

	writeSyncWorkflowFile(
		t,
		workflowDir,
		filepath.Join("reviews-002", "_meta.md"),
		reviewRoundMetaBody("coderabbit", "123", 3),
	)
	writeSyncWorkflowFile(
		t,
		workflowDir,
		filepath.Join("reviews-002", "issue_001.md"),
		reviewIssueBody("pending", "medium"),
	)
	if _, err := collectReviewRounds(workflowDir); err == nil {
		t.Fatal("expected mismatched review round metadata to fail")
	}
}

func TestSyncHelpersClassifyKindsAndSortResults(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"_prd.md":                   "prd",
		"_techspec.md":              "techspec",
		"_tasks.md":                 "tasks_index",
		"task_01.md":                "task",
		"adrs/adr-001.md":           "adr",
		"memory/MEMORY.md":          "memory",
		"reviews-001/_meta.md":      "artifact",
		"reviews-001/issue_001.md":  "review_issue",
		"prompts/task-run.md":       "prompt",
		"protocol/handoff.md":       "protocol",
		"qa/verification-report.md": "qa",
		"notes.md":                  "artifact",
	}
	for relativePath, wantKind := range cases {
		if got := classifyArtifactKind(relativePath); got != wantKind {
			t.Fatalf("classifyArtifactKind(%q) = %q, want %q", relativePath, got, wantKind)
		}
	}

	result := &SyncResult{
		SyncedPaths: []string{"b", "a"},
		Warnings:    []string{"warning-b", "warning-a"},
	}
	sortSyncResult(result)
	if !reflect.DeepEqual(result.SyncedPaths, []string{"a", "b"}) {
		t.Fatalf("SyncedPaths not sorted: %#v", result.SyncedPaths)
	}
	if !reflect.DeepEqual(result.Warnings, []string{"warning-a", "warning-b"}) {
		t.Fatalf("Warnings not sorted: %#v", result.Warnings)
	}
	sortSyncResult(nil)
}

func TestOpenWorkflowGlobalDBRegistersWorkspaceAndRejectsMissingTargets(t *testing.T) {
	workspaceRoot := t.TempDir()
	setSyncTestHome(t)

	workflowDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "demo")
	writeSyncWorkflowFile(t, workflowDir, "task_01.md", taskBody("pending", "Demo"))

	db, workspace, err := openWorkflowGlobalDB(context.Background(), workflowDir)
	if err != nil {
		t.Fatalf("openWorkflowGlobalDB(valid): %v", err)
	}
	resolvedWorkspaceRoot, err := filepath.EvalSymlinks(workspaceRoot)
	if err != nil {
		t.Fatalf("EvalSymlinks(workspaceRoot): %v", err)
	}
	if workspace.RootDir != resolvedWorkspaceRoot {
		t.Fatalf("workspace root = %q, want %q", workspace.RootDir, resolvedWorkspaceRoot)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close(): %v", err)
	}

	if _, _, err := openWorkflowGlobalDB(context.Background(), filepath.Join(workspaceRoot, "missing")); err == nil {
		t.Fatal("expected missing sync target to fail workspace resolution")
	}
}

func TestSyncWithDBRejectsMismatchedWorkspaceAndTarget(t *testing.T) {
	setSyncTestHome(t)

	workspaceRootA := t.TempDir()
	workspaceRootB := t.TempDir()
	workflowDirA := filepath.Join(workspaceRootA, ".compozy", "tasks", "alpha")
	workflowDirB := filepath.Join(workspaceRootB, ".compozy", "tasks", "beta")
	writeSyncWorkflowFile(t, workflowDirA, "task_01.md", taskBody("pending", "Alpha"))
	writeSyncWorkflowFile(t, workflowDirB, "task_01.md", taskBody("pending", "Beta"))

	db, workspaceA, err := openWorkflowGlobalDB(context.Background(), workflowDirA)
	if err != nil {
		t.Fatalf("openWorkflowGlobalDB(workspace A): %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	_, err = SyncWithDB(context.Background(), db, workspaceA, SyncConfig{TasksDir: workflowDirB})
	if err == nil {
		t.Fatal("SyncWithDB() error = nil, want mismatched workspace error")
	}
	if !strings.Contains(err.Error(), "mismatched workspace and sync target") {
		t.Fatalf("SyncWithDB() error = %v, want mismatch context", err)
	}
}

func TestSyncWorkflowRejectsNilInputs(t *testing.T) {
	if err := syncWorkflow(context.Background(), nil, "ws-1", t.TempDir(), &SyncResult{}); err == nil {
		t.Fatal("expected nil sync database to fail")
	}

	setSyncTestHome(t)
	workspaceRoot := t.TempDir()
	workflowDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "demo")
	writeSyncWorkflowFile(t, workflowDir, "task_01.md", taskBody("pending", "Demo"))
	db, _, err := openWorkflowGlobalDB(context.Background(), workflowDir)
	if err != nil {
		t.Fatalf("openWorkflowGlobalDB(): %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	if err := syncWorkflow(context.Background(), db, "ws-1", workflowDir, nil); err == nil {
		t.Fatal("expected nil sync result to fail")
	}
}

func setSyncTestHome(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
}

func openSyncSQLite(t *testing.T) *sql.DB {
	t.Helper()

	homePaths, err := compozyconfig.ResolveHomePaths()
	if err != nil {
		t.Fatalf("ResolveHomePaths(): %v", err)
	}
	db, err := store.OpenSQLiteDatabase(context.Background(), homePaths.GlobalDBPath, nil)
	if err != nil {
		t.Fatalf("OpenSQLiteDatabase(): %v", err)
	}
	return db
}

func queryCount(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()

	var count int
	if err := db.QueryRowContext(context.Background(), query, args...).Scan(&count); err != nil {
		t.Fatalf("query count %q: %v", query, err)
	}
	return count
}

func writeSyncWorkflowFile(t *testing.T, workflowDir, relativePath, content string) {
	t.Helper()

	path := filepath.Join(workflowDir, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func taskBody(status string, title string) string {
	return strings.Join([]string{
		"---",
		"status: " + status,
		"title: " + title,
		"type: backend",
		"complexity: low",
		"dependencies: []",
		"---",
		"",
		"# " + title,
		"",
	}, "\n")
}

func canonicalTaskListBody() string {
	return strings.Join([]string{
		"# Demo — Task List",
		"",
		"## Tasks",
		"",
		authoredTaskListHeader,
		"|---|-------|--------|------------|--------------|",
		"| 01 | Demo task | pending | low | — |",
		"",
	}, "\n")
}

func reviewRoundMetaBody(provider string, pr string, round int) string {
	return strings.Join([]string{
		"---",
		"provider: " + provider,
		"pr: " + pr,
		fmt.Sprintf("round: %d", round),
		"created_at: 2026-04-17T12:00:00Z",
		"---",
		"",
		"## Summary",
		"- Total: 1",
		"- Resolved: 0",
		"- Unresolved: 1",
		"",
	}, "\n")
}

func reviewIssueBody(status string, severity string) string {
	return strings.Join([]string{
		"---",
		"status: " + status,
		"file: internal/app/service.go",
		"line: 42",
		"severity: " + severity,
		"author: review-bot",
		"provider_ref: thread:1",
		"---",
		"",
		"# Issue 001",
		"",
		"Review body.",
		"",
	}, "\n")
}

func legacyMetaBody() string {
	return strings.Join([]string{
		"---",
		"created_at: 2026-04-01T12:00:00Z",
		"updated_at: 2026-04-01T12:05:00Z",
		"---",
		"",
		"## Summary",
		"- Total: 1",
		"- Completed: 0",
		"- Pending: 1",
		"",
	}, "\n")
}
