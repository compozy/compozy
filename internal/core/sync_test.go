package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/core/workpackages"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
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

func TestSyncTaskMetadataRootScanPrunesDeletedWorkflowRows(t *testing.T) {
	t.Run("Should prune deleted workflow rows and their review issues", func(t *testing.T) {
		workspaceRoot := t.TempDir()
		setSyncTestHome(t)

		rootDir := filepath.Join(workspaceRoot, ".compozy", "tasks")
		alphaDir := filepath.Join(rootDir, "alpha")
		betaDir := filepath.Join(rootDir, "beta")
		writeSyncWorkflowFile(t, alphaDir, "task_01.md", taskBody("pending", "Alpha"))
		writeSyncWorkflowFile(t, betaDir, "task_01.md", taskBody("completed", "Beta"))
		writeSyncWorkflowFile(
			t,
			betaDir,
			filepath.Join("reviews-001", "issue_001.md"),
			reviewIssueBody("resolved", "medium"),
		)

		firstResult, err := Sync(context.Background(), SyncConfig{RootDir: rootDir})
		if err != nil {
			t.Fatalf("Sync(first): %v", err)
		}
		if firstResult.WorkflowsScanned != 2 || firstResult.WorkflowsPruned != 0 {
			t.Fatalf("unexpected first sync result: %#v", firstResult)
		}

		sqlDB := openSyncSQLite(t)
		defer func() {
			_ = sqlDB.Close()
		}()
		var (
			betaWorkflowID string
			betaRoundID    string
		)
		if err := sqlDB.QueryRowContext(
			context.Background(),
			`SELECT id FROM workflows WHERE slug = ? AND archived_at IS NULL`,
			"beta",
		).Scan(&betaWorkflowID); err != nil {
			t.Fatalf("query beta workflow id: %v", err)
		}
		if err := sqlDB.QueryRowContext(
			context.Background(),
			`SELECT id FROM review_rounds WHERE workflow_id = ?`,
			betaWorkflowID,
		).Scan(&betaRoundID); err != nil {
			t.Fatalf("query beta review round id: %v", err)
		}

		if err := os.RemoveAll(betaDir); err != nil {
			t.Fatalf("remove beta workflow dir: %v", err)
		}
		secondResult, err := Sync(context.Background(), SyncConfig{RootDir: rootDir})
		if err != nil {
			t.Fatalf("Sync(second): %v", err)
		}
		if secondResult.WorkflowsScanned != 1 || secondResult.WorkflowsPruned != 1 {
			t.Fatalf("unexpected second sync result: %#v", secondResult)
		}
		if !reflect.DeepEqual(secondResult.PrunedWorkflows, []string{"beta"}) {
			t.Fatalf("PrunedWorkflows = %#v, want [beta]", secondResult.PrunedWorkflows)
		}
		if got := queryCount(t, sqlDB, "SELECT COUNT(1) FROM workflows WHERE archived_at IS NULL"); got != 1 {
			t.Fatalf("active workflow count = %d, want 1", got)
		}
		if got := queryCount(t, sqlDB, "SELECT COUNT(1) FROM workflows WHERE slug = 'beta'"); got != 0 {
			t.Fatalf("beta workflow count = %d, want 0", got)
		}
		if got := queryCount(
			t,
			sqlDB,
			"SELECT COUNT(1) FROM task_items WHERE workflow_id = ?",
			betaWorkflowID,
		); got != 0 {
			t.Fatalf("beta task_items count = %d, want 0", got)
		}
		if got := queryCount(
			t,
			sqlDB,
			"SELECT COUNT(1) FROM review_rounds WHERE workflow_id = ?",
			betaWorkflowID,
		); got != 0 {
			t.Fatalf("beta review_rounds count = %d, want 0", got)
		}
		if got := queryCount(t, sqlDB, "SELECT COUNT(1) FROM review_issues WHERE round_id = ?", betaRoundID); got != 0 {
			t.Fatalf("beta review_issues count = %d, want 0", got)
		}
	})
}

func TestSyncTaskMetadataSingleWorkflowSyncDoesNotPruneDeletedSiblings(t *testing.T) {
	t.Run("Should avoid pruning deleted siblings during a single-workflow sync", func(t *testing.T) {
		workspaceRoot := t.TempDir()
		setSyncTestHome(t)

		rootDir := filepath.Join(workspaceRoot, ".compozy", "tasks")
		alphaDir := filepath.Join(rootDir, "alpha")
		betaDir := filepath.Join(rootDir, "beta")
		writeSyncWorkflowFile(t, alphaDir, "task_01.md", taskBody("pending", "Alpha"))
		writeSyncWorkflowFile(t, betaDir, "task_01.md", taskBody("completed", "Beta"))

		if _, err := Sync(context.Background(), SyncConfig{RootDir: rootDir}); err != nil {
			t.Fatalf("Sync(root): %v", err)
		}
		if err := os.RemoveAll(betaDir); err != nil {
			t.Fatalf("remove beta workflow dir: %v", err)
		}

		result, err := Sync(context.Background(), SyncConfig{TasksDir: alphaDir})
		if err != nil {
			t.Fatalf("Sync(single): %v", err)
		}
		if result.WorkflowsScanned != 1 || result.WorkflowsPruned != 0 || len(result.PrunedWorkflows) != 0 {
			t.Fatalf("unexpected single sync result: %#v", result)
		}

		sqlDB := openSyncSQLite(t)
		defer func() {
			_ = sqlDB.Close()
		}()
		if got := queryCount(t, sqlDB, "SELECT COUNT(1) FROM workflows WHERE archived_at IS NULL"); got != 2 {
			t.Fatalf("active workflow count = %d, want 2", got)
		}
	})
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

func TestSyncTaskMetadataSkipsEmptyReviewDirectories(t *testing.T) {
	t.Run("Should skip empty review directories", func(t *testing.T) {
		workspaceRoot := t.TempDir()
		setSyncTestHome(t)

		workflowDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "empty-review-demo")
		writeSyncWorkflowFile(t, workflowDir, "task_01.md", taskBody("pending", "Demo task"))
		if err := os.MkdirAll(filepath.Join(workflowDir, "reviews-002"), 0o755); err != nil {
			t.Fatalf("mkdir empty reviews dir: %v", err)
		}

		result, err := Sync(context.Background(), SyncConfig{TasksDir: workflowDir})
		if err != nil {
			t.Fatalf("Sync(): %v", err)
		}
		if result.WorkflowsScanned != 1 || result.ReviewRoundsUpserted != 0 || result.ReviewIssuesUpserted != 0 {
			t.Fatalf("unexpected sync result: %#v", result)
		}

		sqlDB := openSyncSQLite(t)
		defer func() {
			_ = sqlDB.Close()
		}()
		if got := queryCount(t, sqlDB, "SELECT COUNT(1) FROM review_rounds"); got != 0 {
			t.Fatalf("review_rounds count = %d, want 0", got)
		}
	})
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

func TestCleanupLegacyWorkflowMetadataPreservesTaskGraphManifest(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		body string
	}{
		{
			name: "Should preserve canonical task graph manifest",
			body: canonicalTaskGraphManifestBody("demo"),
		},
		{
			name: "Should preserve forward version task graph manifest",
			body: strings.Replace(canonicalTaskGraphManifestBody("demo"), "compozy.tasks/v2", "compozy.tasks/v3", 1),
		},
		{
			name: "Should preserve malformed task graph manifest frontmatter",
			body: strings.Join([]string{
				"---",
				"schema_version: [",
				"---",
				"# Task Graph",
				"",
			}, "\n"),
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			workflowDir := t.TempDir()
			writeSyncWorkflowFile(t, workflowDir, "_meta.md", legacyMetaBody())
			writeSyncWorkflowFile(t, workflowDir, "_tasks.md", tc.body)

			removed, err := cleanupLegacyWorkflowMetadata(workflowDir)
			if err != nil {
				t.Fatalf("cleanupLegacyWorkflowMetadata(): %v", err)
			}
			if !reflect.DeepEqual(removed, []string{"_meta.md"}) {
				t.Fatalf("removed legacy files = %#v, want only _meta.md", removed)
			}
			if got := mustReadFile(t, filepath.Join(workflowDir, "_tasks.md")); got != tc.body {
				t.Fatalf("expected task graph manifest to remain unchanged")
			}
		})
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

func TestCollectReviewRoundsProjectsIssueFilesAndSkipsEmptyDirs(t *testing.T) {
	t.Parallel()

	t.Run("Should project issue files and skip empty review directories", func(t *testing.T) {
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
			t.Fatalf("collectReviewRounds(): %v", err)
		}
		if len(rounds) != 1 || rounds[0].ResolvedCount != 1 || rounds[0].UnresolvedCount != 0 {
			t.Fatalf("unexpected review round projection: %#v", rounds)
		}
		if rounds[0].Provider != "" || rounds[0].PRRef != "" {
			t.Fatalf(
				"expected legacy _meta.md to be ignored, got provider=%q pr=%q",
				rounds[0].Provider,
				rounds[0].PRRef,
			)
		}

		if err := os.MkdirAll(filepath.Join(workflowDir, "reviews-002"), 0o755); err != nil {
			t.Fatalf("mkdir empty reviews dir: %v", err)
		}
		rounds, err = collectReviewRounds(workflowDir)
		if err != nil {
			t.Fatalf("collectReviewRounds(with empty dir): %v", err)
		}
		if len(rounds) != 1 || rounds[0].RoundNumber != 1 {
			t.Fatalf("expected empty reviews dir to be skipped, got %#v", rounds)
		}
	})
}

func TestCollectReviewRoundsUsesIssueRoundMetadataAndRejectsConflicts(t *testing.T) {
	t.Parallel()

	t.Run("Should project metadata when provider and PR are consistent", func(t *testing.T) {
		t.Parallel()

		workflowDir := t.TempDir()
		writeSyncWorkflowFile(
			t,
			workflowDir,
			filepath.Join("reviews-002", "issue_001.md"),
			reviewIssueBodyWithRoundMetadata("pending", "medium", "coderabbit", "123", 2),
		)
		writeSyncWorkflowFile(
			t,
			workflowDir,
			filepath.Join("reviews-002", "issue_002.md"),
			reviewIssueBodyWithRoundMetadata("resolved", "high", "coderabbit", "123", 2),
		)

		rounds, err := collectReviewRounds(workflowDir)
		if err != nil {
			t.Fatalf("collectReviewRounds(): %v", err)
		}
		if len(rounds) != 1 || rounds[0].Provider != "coderabbit" || rounds[0].PRRef != "123" {
			t.Fatalf("unexpected round metadata projection: %#v", rounds)
		}
		if rounds[0].ResolvedCount != 1 || rounds[0].UnresolvedCount != 1 {
			t.Fatalf("unexpected counts: %#v", rounds[0])
		}
	})

	t.Run("Should reject review issues whose declared round mismatches the directory", func(t *testing.T) {
		t.Parallel()

		workflowDir := t.TempDir()
		writeSyncWorkflowFile(
			t,
			workflowDir,
			filepath.Join("reviews-002", "issue_001.md"),
			reviewIssueBodyWithRoundMetadata("pending", "medium", "coderabbit", "123", 3),
		)

		_, err := collectReviewRounds(workflowDir)
		if err == nil || !strings.Contains(err.Error(), "declares round=3") {
			t.Fatalf("collectReviewRounds() error = %v, want round mismatch", err)
		}
	})

	t.Run("Should reject mixed providers within a review round", func(t *testing.T) {
		t.Parallel()

		workflowDir := t.TempDir()
		writeSyncWorkflowFile(
			t,
			workflowDir,
			filepath.Join("reviews-001", "issue_001.md"),
			reviewIssueBodyWithRoundMetadata("pending", "medium", "coderabbit", "123", 1),
		)
		writeSyncWorkflowFile(
			t,
			workflowDir,
			filepath.Join("reviews-001", "issue_002.md"),
			reviewIssueBodyWithRoundMetadata("pending", "medium", "other", "123", 1),
		)

		_, err := collectReviewRounds(workflowDir)
		if err == nil || !strings.Contains(err.Error(), "already uses provider") {
			t.Fatalf("collectReviewRounds() error = %v, want provider conflict", err)
		}
	})

	t.Run("Should reject mixed PR references within a review round", func(t *testing.T) {
		t.Parallel()

		workflowDir := t.TempDir()
		writeSyncWorkflowFile(
			t,
			workflowDir,
			filepath.Join("reviews-001", "issue_001.md"),
			reviewIssueBodyWithRoundMetadata("pending", "medium", "coderabbit", "123", 1),
		)
		writeSyncWorkflowFile(
			t,
			workflowDir,
			filepath.Join("reviews-001", "issue_002.md"),
			reviewIssueBodyWithRoundMetadata("pending", "medium", "coderabbit", "456", 1),
		)

		_, err := collectReviewRounds(workflowDir)
		if err == nil || !strings.Contains(err.Error(), "already uses pr") {
			t.Fatalf("collectReviewRounds() error = %v, want pr conflict", err)
		}
	})
}

func TestCollectReviewRoundsRejectsInvalidReviewIssue(t *testing.T) {
	t.Parallel()

	t.Run("Should reject review issues without a status", func(t *testing.T) {
		t.Parallel()

		workflowDir := t.TempDir()
		writeSyncWorkflowFile(
			t,
			workflowDir,
			filepath.Join("reviews-001", "issue_001.md"),
			strings.Join([]string{
				"---",
				"file: internal/app/service.go",
				"---",
				"",
				"# Issue 001",
				"",
			}, "\n"),
		)

		if _, err := collectReviewRounds(workflowDir); err == nil ||
			!strings.Contains(err.Error(), "review front matter missing status") {
			t.Fatalf("collectReviewRounds() error = %v, want missing review status validation", err)
		}
	})
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

func TestSyncWorkPackageInitiativePreservesParentChildOwnership(t *testing.T) {
	// Suite boundary
	// IN: root sync, real Work Package manifest parsing, and SQLite projection
	// OUT: task execution and API transport, owned by later workflow tasks
	// Invariant: valid Work Package sync owns mutable artifacts through child workflow IDs only.
	workspaceRoot := t.TempDir()
	setSyncTestHome(t)
	initiativeDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "initiative")
	writeWorkPackageFixture(t, initiativeDir, map[string]string{
		"WP-001": "completed",
		"WP-002": "pending",
	})

	result, err := Sync(context.Background(), SyncConfig{TasksDir: initiativeDir})
	if err != nil {
		t.Fatalf("Sync(valid work package initiative): %v", err)
	}
	if result.WorkflowsScanned != 3 {
		t.Fatalf("WorkflowsScanned = %d, want parent plus two children", result.WorkflowsScanned)
	}
	if result.Partial {
		t.Fatalf("valid work package sync marked partial: %#v", result)
	}
	if len(result.WorkPackageChildIDs) != 2 {
		t.Fatalf("WorkPackageChildIDs = %#v, want two stable child IDs", result.WorkPackageChildIDs)
	}

	sqlDB := openSyncSQLite(t)
	defer func() {
		_ = sqlDB.Close()
	}()
	var parentID string
	if err := sqlDB.QueryRowContext(
		context.Background(),
		`SELECT id FROM workflows WHERE slug = ? AND kind = 'initiative'`,
		"initiative",
	).Scan(&parentID); err != nil {
		t.Fatalf("query initiative parent: %v", err)
	}
	if got := queryCount(
		t,
		sqlDB,
		`SELECT COUNT(1) FROM workflows WHERE parent_workflow_id = ? AND kind = 'work_package'`,
		parentID,
	); got != 2 {
		t.Fatalf("child workflow count = %d, want 2", got)
	}
	if got := queryCount(t, sqlDB, `SELECT COUNT(1) FROM task_items WHERE workflow_id = ?`, parentID); got != 0 {
		t.Fatalf("parent task projection count = %d, want 0", got)
	}
	if got := queryCount(
		t,
		sqlDB,
		`SELECT COUNT(1) FROM artifact_snapshots WHERE workflow_id = ? AND relative_path LIKE '_packages/%'`,
		parentID,
	); got != 0 {
		t.Fatalf("parent package snapshot count = %d, want 0", got)
	}
	if got := queryCount(
		t,
		sqlDB,
		`SELECT COUNT(1) FROM task_items WHERE workflow_id IN (SELECT id FROM workflows WHERE parent_workflow_id = ?)`,
		parentID,
	); got != 2 {
		t.Fatalf("child task projection count = %d, want 2", got)
	}

	planPath := filepath.Join(initiativeDir, "_work_packages.md")
	reopenedPlan := strings.Replace(
		mustReadFile(t, planPath),
		"## [x] WP-001 — Persistence",
		"## [ ] WP-001 — Persistence",
		1,
	)
	writeSyncWorkflowFile(t, initiativeDir, "_work_packages.md", reopenedPlan)
	reopened, err := Sync(context.Background(), SyncConfig{TasksDir: initiativeDir})
	if err != nil {
		t.Fatalf("Sync(reopened package): %v", err)
	}
	if !reflect.DeepEqual(reopened.WorkPackageChildIDs, result.WorkPackageChildIDs) {
		t.Fatalf(
			"reopened child ids = %#v, want stable %#v",
			reopened.WorkPackageChildIDs,
			result.WorkPackageChildIDs,
		)
	}
	var lifecycleCompleted bool
	if err := sqlDB.QueryRowContext(
		context.Background(),
		`SELECT lifecycle_completed FROM workflows WHERE slug = ?`,
		"initiative/WP-001",
	).Scan(&lifecycleCompleted); err != nil {
		t.Fatalf("query reopened package lifecycle: %v", err)
	}
	if lifecycleCompleted {
		t.Fatal("reopened package lifecycle_completed = true, want false")
	}
}

func TestSyncWorkPackageInitiativeFailsClosedAndPreservesChildren(t *testing.T) {
	// Suite boundary
	// IN: real filesystem marker validation and aggregate SQLite reconciliation
	// OUT: CLI/API error mapping, owned by transport task work
	// Invariant: a malformed marker never flattens package artifacts or deletes a valid child projection.
	workspaceRoot := t.TempDir()
	setSyncTestHome(t)
	initiativeDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "initiative")
	writeWorkPackageFixture(t, initiativeDir, map[string]string{
		"WP-001": "completed",
		"WP-002": "completed",
	})
	if _, err := Sync(context.Background(), SyncConfig{TasksDir: initiativeDir}); err != nil {
		t.Fatalf("Sync(initial): %v", err)
	}

	sqlDB := openSyncSQLite(t)
	defer func() {
		_ = sqlDB.Close()
	}()
	beforeChildren := queryCount(
		t,
		sqlDB,
		`SELECT COUNT(1) FROM workflows WHERE kind = 'work_package' AND archived_at IS NULL`,
	)
	writeSyncWorkflowFile(t, initiativeDir, "_work_packages.md", "---\ninvalid")
	writeSyncWorkflowFile(t, initiativeDir, "task_99.md", taskBody("pending", "must not flatten"))

	_, err := Sync(context.Background(), SyncConfig{TasksDir: initiativeDir})
	if !errors.Is(err, workpackages.ErrInvalidPlan) {
		t.Fatalf("Sync(malformed marker) error = %v, want work package invalid plan", err)
	}
	if got := queryCount(
		t,
		sqlDB,
		`SELECT COUNT(1) FROM workflows WHERE kind = 'work_package' AND archived_at IS NULL`,
	); got != beforeChildren {
		t.Fatalf("child rows after invalid sync = %d, want preserved %d", got, beforeChildren)
	}
	if got := queryCount(t, sqlDB, `SELECT COUNT(1) FROM task_items WHERE task_id = 'task_99'`); got != 0 {
		t.Fatalf("flattened parent task rows = %d, want 0", got)
	}
	if err := os.Remove(filepath.Join(initiativeDir, "_work_packages.md")); err != nil {
		t.Fatalf("remove malformed marker: %v", err)
	}
	legacyResult, err := Sync(context.Background(), SyncConfig{TasksDir: initiativeDir})
	if err != nil {
		t.Fatalf("Sync(marker removed): %v", err)
	}
	if legacyResult.WorkflowsScanned != 1 {
		t.Fatalf("legacy marker-absent sync workflows = %d, want 1", legacyResult.WorkflowsScanned)
	}
	var kind string
	if err := sqlDB.QueryRowContext(
		context.Background(),
		`SELECT kind FROM workflows WHERE slug = ? AND archived_at IS NULL`,
		"initiative",
	).Scan(&kind); err != nil {
		t.Fatalf("query marker-absent parent: %v", err)
	}
	if kind != string(globaldb.WorkflowKindOrdinary) {
		t.Fatalf("marker-absent parent kind = %q, want ordinary", kind)
	}
	if got := queryCount(
		t,
		sqlDB,
		`SELECT COUNT(1) FROM workflows WHERE kind = 'work_package' AND archived_at IS NULL`,
	); got != 0 {
		t.Fatalf("marker-absent child rows = %d, want 0", got)
	}
	if got := queryCount(t, sqlDB, `SELECT COUNT(1) FROM task_items WHERE task_id = 'task_99'`); got != 1 {
		t.Fatalf("legacy task projection count = %d, want 1", got)
	}
}

func TestSyncWorkPackageInitiativeReportsMissingChildWithoutPruning(t *testing.T) {
	// Suite boundary
	// IN: root aggregate sync against filesystem and SQLite
	// OUT: watcher event delivery, owned by daemon watcher tests
	// Invariant: one missing declared package preserves its prior child while readable siblings update.
	workspaceRoot := t.TempDir()
	setSyncTestHome(t)
	initiativeDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "initiative")
	writeWorkPackageFixture(t, initiativeDir, map[string]string{
		"WP-001": "pending",
		"WP-002": "pending",
	})
	if _, err := Sync(context.Background(), SyncConfig{TasksDir: initiativeDir}); err != nil {
		t.Fatalf("Sync(initial): %v", err)
	}

	sqlDB := openSyncSQLite(t)
	defer func() {
		_ = sqlDB.Close()
	}()
	var wp002ID string
	if err := sqlDB.QueryRowContext(context.Background(), `SELECT id FROM workflows WHERE slug = ?`, "initiative/WP-002").
		Scan(&wp002ID); err != nil {
		t.Fatalf("query WP-002 child: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(initiativeDir, "_packages", "WP-002")); err != nil {
		t.Fatalf("remove WP-002: %v", err)
	}
	writeSyncWorkflowFile(
		t,
		filepath.Join(initiativeDir, "_packages", "WP-001"),
		"task_01.md",
		taskBody("completed", "WP-001 task"),
	)

	result, err := Sync(context.Background(), SyncConfig{TasksDir: initiativeDir})
	if err != nil {
		t.Fatalf("Sync(missing child): %v", err)
	}
	if !result.Partial || !reflect.DeepEqual(result.MissingWorkPackages, []string{"WP-002"}) {
		t.Fatalf("missing child result = %#v, want partial WP-002", result)
	}
	var preservedID string
	if err := sqlDB.QueryRowContext(context.Background(), `SELECT id FROM workflows WHERE slug = ?`, "initiative/WP-002").
		Scan(&preservedID); err != nil {
		t.Fatalf("query preserved WP-002 child: %v", err)
	}
	if preservedID != wp002ID {
		t.Fatalf("WP-002 child id changed after partial sync: got %q want %q", preservedID, wp002ID)
	}
	var wp001Status string
	if err := sqlDB.QueryRowContext(
		context.Background(),
		`SELECT status FROM task_items WHERE workflow_id = (SELECT id FROM workflows WHERE slug = ?)`,
		"initiative/WP-001",
	).Scan(&wp001Status); err != nil {
		t.Fatalf("query WP-001 task state: %v", err)
	}
	if wp001Status != "completed" {
		t.Fatalf("WP-001 task status = %q, want completed", wp001Status)
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

func writeWorkPackageFixture(t *testing.T, initiativeDir string, states map[string]string) {
	t.Helper()

	planStatus := func(packageID string) string {
		if states[packageID] == "completed" {
			return "x"
		}
		return " "
	}
	writeSyncWorkflowFile(t, initiativeDir, "_prd.md", "# Initiative\n")
	writeSyncWorkflowFile(t, initiativeDir, "_techspec.md", "# Initiative Techspec\n")
	writeSyncWorkflowFile(t, initiativeDir, "_tasks.md", canonicalTaskListBody())
	writeSyncWorkflowFile(t, initiativeDir, "_work_packages.md", strings.Join([]string{
		"---",
		"schema_version: compozy.work-packages/v1",
		"initiative: initiative",
		"graph:",
		"  nodes:",
		"    - id: WP-001",
		"      directory: _packages/WP-001",
		"    - id: WP-002",
		"      directory: _packages/WP-002",
		"  edges:",
		"    - from: WP-001",
		"      to: WP-002",
		"      rationale: WP-002 consumes the WP-001 contract.",
		"---",
		"",
		"# Initiative Work Packages",
		"",
		"## [" + planStatus("WP-001") + "] WP-001 — Persistence",
		"",
		"- Reference: `initiative/WP-001`",
		"- Outcome: Persist the parent workflow.",
		"- Owns:",
		"  - persistence",
		"- Dependencies: None",
		"",
		"## [" + planStatus("WP-002") + "] WP-002 — Archive",
		"",
		"- Reference: `initiative/WP-002`",
		"- Outcome: Archive the aggregate workflow.",
		"- Owns:",
		"  - archive",
		"- Dependencies:",
		"  - `WP-001` — WP-002 consumes the WP-001 contract.",
		"",
	}, "\n"))

	for _, packageID := range []string{"WP-001", "WP-002"} {
		packageDir := filepath.Join(initiativeDir, "_packages", packageID)
		writeSyncWorkflowFile(t, packageDir, "_tasks.md", canonicalTaskListBody())
		writeSyncWorkflowFile(t, packageDir, "task_01.md", taskBody(states[packageID], packageID+" task"))
	}
}

func canonicalTaskGraphManifestBody(workflow string) string {
	return strings.Join([]string{
		"---",
		"schema_version: \"compozy.tasks/v2\"",
		"workflow: " + workflow,
		"graph:",
		"  nodes:",
		"    - id: task_01",
		"      file: task_01.md",
		"  edges: []",
		"---",
		"",
		"# " + workflow + " Tasks",
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

func reviewIssueBodyWithRoundMetadata(status string, severity string, provider string, pr string, round int) string {
	lines := []string{
		"---",
		"status: " + status,
		"file: internal/app/service.go",
		"line: 42",
		"severity: " + severity,
		"author: review-bot",
		"provider_ref: thread:1",
	}
	if provider != "" {
		lines = append(lines, "provider: "+provider)
	}
	if pr != "" {
		lines = append(lines, "pr: "+fmt.Sprintf("%q", pr))
	}
	if round > 0 {
		lines = append(lines,
			fmt.Sprintf("round: %d", round),
			"round_created_at: 2026-04-17T12:00:00Z",
		)
	}
	lines = append(lines,
		"---",
		"",
		"# Issue 001",
		"",
		"Review body.",
		"",
	)
	return strings.Join(lines, "\n")
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
