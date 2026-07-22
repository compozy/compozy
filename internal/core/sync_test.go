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
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/taskgroups"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
)

type completionStoreFunc func(
	context.Context,
	string,
	string,
	taskgroups.CompletionValidator,
) (taskgroups.CompletionResult, error)

func (fn completionStoreFunc) MarkCompleteValidated(
	ctx context.Context,
	initiativeDir string,
	taskGroupID string,
	validator taskgroups.CompletionValidator,
) (taskgroups.CompletionResult, error) {
	return fn(ctx, initiativeDir, taskGroupID, validator)
}

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

func TestSyncTaskGroupInitiativePreservesParentChildOwnership(t *testing.T) {
	// Suite boundary
	// IN: root sync, real Task Group manifest parsing, and SQLite projection
	// OUT: task execution and API transport, owned by later workflow tasks
	// Invariant: valid Task Group sync owns mutable artifacts through child workflow IDs only.
	workspaceRoot := t.TempDir()
	setSyncTestHome(t)
	initiativeDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "initiative")
	writeTaskGroupFixture(t, initiativeDir, map[string]string{
		"TG-001": "completed",
		"TG-002": "pending",
	})

	result, err := Sync(context.Background(), SyncConfig{TasksDir: initiativeDir})
	if err != nil {
		t.Fatalf("Sync(valid task group initiative): %v", err)
	}
	if result.WorkflowsScanned != 3 {
		t.Fatalf("WorkflowsScanned = %d, want parent plus two children", result.WorkflowsScanned)
	}
	if result.Partial {
		t.Fatalf("valid task group sync marked partial: %#v", result)
	}
	if len(result.TaskGroupChildIDs) != 2 {
		t.Fatalf("TaskGroupChildIDs = %#v, want two stable child IDs", result.TaskGroupChildIDs)
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
		`SELECT COUNT(1) FROM workflows WHERE parent_workflow_id = ? AND kind = 'task_group'`,
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
		`SELECT COUNT(1) FROM artifact_snapshots WHERE workflow_id = ? AND relative_path LIKE '_task_groups/%'`,
		parentID,
	); got != 0 {
		t.Fatalf("parent task group snapshot count = %d, want 0", got)
	}
	if got := queryCount(
		t,
		sqlDB,
		`SELECT COUNT(1) FROM task_items WHERE workflow_id IN (SELECT id FROM workflows WHERE parent_workflow_id = ?)`,
		parentID,
	); got != 2 {
		t.Fatalf("child task projection count = %d, want 2", got)
	}

	planPath := filepath.Join(initiativeDir, "_task_groups.md")
	reopenedPlan := strings.Replace(
		mustReadFile(t, planPath),
		"## [x] TG-001 — Persistence",
		"## [ ] TG-001 — Persistence",
		1,
	)
	writeSyncWorkflowFile(t, initiativeDir, "_task_groups.md", reopenedPlan)
	reopened, err := Sync(context.Background(), SyncConfig{TasksDir: initiativeDir})
	if err != nil {
		t.Fatalf("Sync(reopened task group): %v", err)
	}
	if !reflect.DeepEqual(reopened.TaskGroupChildIDs, result.TaskGroupChildIDs) {
		t.Fatalf(
			"reopened child ids = %#v, want stable %#v",
			reopened.TaskGroupChildIDs,
			result.TaskGroupChildIDs,
		)
	}
	var lifecycleCompleted bool
	if err := sqlDB.QueryRowContext(
		context.Background(),
		`SELECT lifecycle_completed FROM workflows WHERE slug = ?`,
		"initiative/TG-001",
	).Scan(&lifecycleCompleted); err != nil {
		t.Fatalf("query reopened task group lifecycle: %v", err)
	}
	if lifecycleCompleted {
		t.Fatal("reopened task group lifecycle_completed = true, want false")
	}
}

func TestSyncTaskGroupExecutionScopeReconcilesOnlySelectedChild(t *testing.T) {
	// INVARIANT: a task group lifecycle refresh reads root specifications and one
	// selected child without inspecting sibling operational artifacts.
	// OWNING_LAYER: service-integration. EXISTING_SUITE: internal/core/sync_test.go.
	workspaceRoot := t.TempDir()
	setSyncTestHome(t)
	initiativeDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "initiative")
	writeTaskGroupFixture(t, initiativeDir, map[string]string{
		"TG-001": "completed",
		"TG-002": "pending",
	})
	writeSyncWorkflowFile(
		t,
		filepath.Join(initiativeDir, "_task_groups", "TG-002"),
		"task_01.md",
		"this sibling artifact is deliberately invalid and must not be read\n",
	)

	target, err := (taskgroups.TargetResolver{}).ResolveTaskGroup(
		context.Background(),
		workspaceRoot,
		"initiative/TG-001",
	)
	if err != nil {
		t.Fatalf("ResolveTaskGroup() error = %v", err)
	}
	scope, err := taskgroups.BuildExecutionScope(target)
	if err != nil {
		t.Fatalf("BuildExecutionScope() error = %v", err)
	}
	homePaths, err := compozyconfig.ResolveHomePaths()
	if err != nil {
		t.Fatalf("ResolveHomePaths() error = %v", err)
	}
	db, err := globaldb.Open(context.Background(), homePaths.GlobalDBPath)
	if err != nil {
		t.Fatalf("globaldb.Open() error = %v", err)
	}
	defer func() { _ = db.Close() }()
	workspace, err := db.ResolveOrRegister(context.Background(), workspaceRoot)
	if err != nil {
		t.Fatalf("ResolveOrRegister() error = %v", err)
	}

	result, err := SyncWithDB(context.Background(), db, workspace, SyncConfig{ExecutionScope: &scope})
	if err != nil {
		t.Fatalf("SyncWithDB(scoped task group): %v", err)
	}
	if result.WorkflowsScanned != 2 ||
		!reflect.DeepEqual(result.SyncedPaths, []string{scope.SpecDir, scope.OperationalDir}) {
		t.Fatalf("scoped sync result = %#v, want root plus TG-001 only", result)
	}

	sqlDB := openSyncSQLite(t)
	defer func() { _ = sqlDB.Close() }()
	if got := queryCount(t, sqlDB, `SELECT COUNT(1) FROM workflows WHERE slug = 'initiative/TG-001'`); got != 1 {
		t.Fatalf("TG-001 workflow count = %d, want 1", got)
	}
	if got := queryCount(t, sqlDB, `SELECT COUNT(1) FROM workflows WHERE slug = 'initiative/TG-002'`); got != 0 {
		t.Fatalf("IT-019 scoped sync created or read TG-002 child count = %d, want 0", got)
	}
	if err := os.Remove(filepath.Join(initiativeDir, "_techspec.md")); err != nil {
		t.Fatalf("remove canonical techspec: %v", err)
	}
	if _, err := SyncWithDB(
		context.Background(),
		db,
		workspace,
		SyncConfig{ExecutionScope: &scope},
	); err == nil ||
		!strings.Contains(err.Error(), "_techspec.md") {
		t.Fatalf("IT-038 scoped sync error = %v, want inaccessible canonical techspec", err)
	}
}

func TestTaskGroupCompletionBridgeSeparatesReviewAndCompletionOutcomes(t *testing.T) {
	// INVARIANT: clean review evidence, plan mutation, and catalog sync have
	// independent truthful outcomes.
	// OWNING_LAYER: service-integration. EXISTING_SUITE: internal/core/sync_test.go.
	t.Run("IT-076 records a clean task group and is idempotent", func(t *testing.T) {
		workspaceRoot := t.TempDir()
		setSyncTestHome(t)
		initiativeDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "initiative")
		writeTaskGroupFixture(t, initiativeDir, map[string]string{"TG-001": "pending", "TG-002": "pending"})
		writeSyncWorkflowFile(
			t,
			filepath.Join(initiativeDir, "_task_groups", "TG-001"),
			"task_01.md",
			taskBody("completed", "TG-001 task"),
		)
		writeSyncWorkflowFile(
			t,
			filepath.Join(initiativeDir, "_task_groups", "TG-002"),
			"task_01.md",
			"sibling mutable artifact must not be read during completion\n",
		)

		request := TaskGroupCompletionRequest{
			WorkspaceRoot:      workspaceRoot,
			Reference:          "initiative/TG-001",
			VerificationPassed: true,
		}
		first, err := CompleteTaskGroup(context.Background(), request)
		if err != nil {
			t.Fatalf("CompleteTaskGroup(first): %v", err)
		}
		if !first.ReviewClean || !first.CompletionRecorded || first.AlreadyCompleted || first.SyncPending {
			t.Fatalf("first completion result = %#v", first)
		}
		second, err := CompleteTaskGroup(context.Background(), request)
		if err != nil {
			t.Fatalf("CompleteTaskGroup(second): %v", err)
		}
		if !second.ReviewClean || second.CompletionRecorded || !second.AlreadyCompleted || second.SyncPending {
			t.Fatalf("second completion result = %#v", second)
		}
		plan, err := taskgroups.NewStore().Load(context.Background(), initiativeDir)
		if err != nil {
			t.Fatalf("Load(completed plan): %v", err)
		}
		if !plan.IsComplete("TG-001") {
			t.Fatal("IT-076 completed task group checkbox was not recorded")
		}

		sqlDB := openSyncSQLite(t)
		defer func() { _ = sqlDB.Close() }()
		var complete bool
		if err := sqlDB.QueryRowContext(
			context.Background(),
			`SELECT lifecycle_completed FROM workflows WHERE slug = 'initiative/TG-001'`,
		).Scan(&complete); err != nil {
			t.Fatalf("query completed task group projection: %v", err)
		}
		if !complete {
			t.Fatal("IT-076 child lifecycle projection = false, want true")
		}
		if got := queryCount(t, sqlDB, `SELECT COUNT(1) FROM workflows WHERE slug = 'initiative/TG-002'`); got != 0 {
			t.Fatalf("IT-024 completion sync read sibling TG-002 count = %d, want 0", got)
		}
	})

	t.Run("IT-025 preserves a clean review when the current plan is malformed", func(t *testing.T) {
		workspaceRoot := t.TempDir()
		initiativeDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "initiative")
		writeTaskGroupFixture(t, initiativeDir, map[string]string{"TG-001": "pending", "TG-002": "pending"})
		writeSyncWorkflowFile(
			t,
			filepath.Join(initiativeDir, "_task_groups", "TG-001"),
			"task_01.md",
			taskBody("completed", "TG-001 task"),
		)
		planPath := filepath.Join(initiativeDir, taskgroups.ManifestFileName)
		malformed := "---\ninvalid"
		writeSyncWorkflowFile(t, initiativeDir, taskgroups.ManifestFileName, malformed)

		result, err := NewTaskGroupCompletionService().Complete(context.Background(), TaskGroupCompletionRequest{
			WorkspaceRoot: workspaceRoot, Reference: "initiative/TG-001", VerificationPassed: true,
		})
		if !errors.Is(err, taskgroups.ErrInvalidPlan) {
			t.Fatalf("Complete(malformed plan) error = %v, want invalid plan", err)
		}
		if !result.ReviewClean || result.CompletionRecorded || result.SyncPending {
			t.Fatalf("IT-025 completion result = %#v", result)
		}
		if got := mustReadFile(t, planPath); got != malformed {
			t.Fatalf("malformed plan bytes changed\nwant: %q\ngot:  %q", malformed, got)
		}
	})

	t.Run("IT-028 and sync failure keep completion outcomes distinct", func(t *testing.T) {
		workspaceRoot := t.TempDir()
		initiativeDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "initiative")
		writeTaskGroupFixture(t, initiativeDir, map[string]string{"TG-001": "pending", "TG-002": "pending"})
		writeSyncWorkflowFile(
			t,
			filepath.Join(initiativeDir, "_task_groups", "TG-001"),
			"task_01.md",
			taskBody("completed", "TG-001 task"),
		)
		service := NewTaskGroupCompletionService()
		service.sync = func(context.Context, string, model.ExecutionScope) error {
			return errors.New("catalog temporarily unavailable")
		}

		result, err := service.Complete(context.Background(), TaskGroupCompletionRequest{
			WorkspaceRoot: workspaceRoot, Reference: "initiative/TG-001", VerificationPassed: true,
		})
		if err == nil || !strings.Contains(err.Error(), "catalog temporarily unavailable") {
			t.Fatalf("Complete(sync failure) error = %v", err)
		}
		if !result.ReviewClean || !result.CompletionRecorded || !result.SyncPending {
			t.Fatalf("sync-pending completion result = %#v", result)
		}
		plan, loadErr := taskgroups.NewStore().Load(context.Background(), initiativeDir)
		if loadErr != nil || !plan.IsComplete("TG-001") {
			t.Fatalf("completion record after sync failure plan=%#v error=%v", plan, loadErr)
		}
	})

	t.Run("IT-028 preserves a clean review when the plan is read only", func(t *testing.T) {
		workspaceRoot := t.TempDir()
		initiativeDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "initiative")
		writeTaskGroupFixture(t, initiativeDir, map[string]string{"TG-001": "pending", "TG-002": "pending"})
		writeSyncWorkflowFile(
			t,
			filepath.Join(initiativeDir, "_task_groups", "TG-001"),
			"task_01.md",
			taskBody("completed", "TG-001 task"),
		)
		planPath := filepath.Join(initiativeDir, taskgroups.ManifestFileName)
		before := mustReadFile(t, planPath)
		service := NewTaskGroupCompletionService()
		service.store = completionStoreFunc(
			func(
				context.Context,
				string,
				string,
				taskgroups.CompletionValidator,
			) (taskgroups.CompletionResult, error) {
				return taskgroups.CompletionResult{}, fmt.Errorf(
					"record completion: %w",
					taskgroups.ErrPlanReadOnly,
				)
			},
		)
		service.sync = func(context.Context, string, model.ExecutionScope) error {
			t.Fatal("sync must not run after a read-only completion failure")
			return nil
		}

		result, err := service.Complete(context.Background(), TaskGroupCompletionRequest{
			WorkspaceRoot: workspaceRoot, Reference: "initiative/TG-001", VerificationPassed: true,
		})
		if !errors.Is(err, taskgroups.ErrPlanReadOnly) {
			t.Fatalf("Complete(read-only plan) error = %v", err)
		}
		if !result.ReviewClean || result.CompletionRecorded || result.SyncPending {
			t.Fatalf("IT-028 completion result = %#v", result)
		}
		if got := mustReadFile(t, planPath); got != before {
			t.Fatal("read-only completion failure changed plan bytes")
		}
	})

	t.Run("IT-027 rechecks reopened dependencies before recording completion", func(t *testing.T) {
		workspaceRoot := t.TempDir()
		initiativeDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "initiative")
		writeTaskGroupFixture(t, initiativeDir, map[string]string{"TG-001": "pending", "TG-002": "pending"})
		writeSyncWorkflowFile(
			t,
			filepath.Join(initiativeDir, "_task_groups", "TG-002"),
			"task_01.md",
			taskBody("completed", "TG-002 task"),
		)
		before := mustReadFile(t, filepath.Join(initiativeDir, taskgroups.ManifestFileName))

		result, err := NewTaskGroupCompletionService().Complete(context.Background(), TaskGroupCompletionRequest{
			WorkspaceRoot: workspaceRoot, Reference: "initiative/TG-002", VerificationPassed: true,
		})
		if !errors.Is(err, taskgroups.ErrDependenciesUnmet) {
			t.Fatalf("Complete(reopened dependency) error = %v, want dependency error", err)
		}
		if !result.ReviewClean || result.CompletionRecorded {
			t.Fatalf("IT-027 completion result = %#v", result)
		}
		if got := mustReadFile(t, filepath.Join(initiativeDir, taskgroups.ManifestFileName)); got != before {
			t.Fatal("dependency-blocked completion changed plan bytes")
		}
	})

	t.Run("IT-021 rejects unresolved review evidence without changing the plan", func(t *testing.T) {
		workspaceRoot := t.TempDir()
		initiativeDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "initiative")
		writeTaskGroupFixture(t, initiativeDir, map[string]string{"TG-001": "pending", "TG-002": "pending"})
		taskGroupDir := filepath.Join(initiativeDir, "_task_groups", "TG-001")
		writeSyncWorkflowFile(t, taskGroupDir, "task_01.md", taskBody("completed", "TG-001 task"))
		writeSyncWorkflowFile(
			t,
			taskGroupDir,
			filepath.Join("reviews-001", "issue_001.md"),
			reviewIssueBody("pending", "high"),
		)
		before := mustReadFile(t, filepath.Join(initiativeDir, taskgroups.ManifestFileName))

		result, err := NewTaskGroupCompletionService().Complete(context.Background(), TaskGroupCompletionRequest{
			WorkspaceRoot: workspaceRoot, Reference: "initiative/TG-001", VerificationPassed: true,
		})
		if err == nil || result.ReviewClean || result.CompletionRecorded {
			t.Fatalf("IT-021 unresolved review result=%#v error=%v", result, err)
		}
		if got := mustReadFile(t, filepath.Join(initiativeDir, taskgroups.ManifestFileName)); got != before {
			t.Fatal("completion changed plan despite unresolved review")
		}
	})

	t.Run("keeps a clean resolved review distinct from a nonterminal task", func(t *testing.T) {
		workspaceRoot := t.TempDir()
		initiativeDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "initiative")
		writeTaskGroupFixture(t, initiativeDir, map[string]string{"TG-001": "pending", "TG-002": "pending"})
		taskGroupDir := filepath.Join(initiativeDir, "_task_groups", "TG-001")
		writeSyncWorkflowFile(t, taskGroupDir, "task_01.md", taskBody("pending", "TG-001 task"))
		writeSyncWorkflowFile(
			t,
			taskGroupDir,
			filepath.Join("reviews-001", "issue_001.md"),
			reviewIssueBody("resolved", "high"),
		)
		before := mustReadFile(t, filepath.Join(initiativeDir, taskgroups.ManifestFileName))

		result, err := NewTaskGroupCompletionService().Complete(context.Background(), TaskGroupCompletionRequest{
			WorkspaceRoot: workspaceRoot, Reference: "initiative/TG-001", VerificationPassed: true,
		})
		if err == nil || !strings.Contains(err.Error(), "all task group tasks to be terminal") {
			t.Fatalf("nonterminal-task completion error = %v, want terminal-task block", err)
		}
		// The review is verified and fully resolved, so review_clean must stay true even though
		// the nonterminal task blocks the checkbox mutation. The two outcomes must diverge.
		if !result.ReviewClean {
			t.Fatalf("clean resolved review reported review_clean=false: %#v", result)
		}
		if result.CompletionRecorded || result.AlreadyCompleted || result.SyncPending {
			t.Fatalf("nonterminal task recorded completion: %#v", result)
		}
		if got := mustReadFile(t, filepath.Join(initiativeDir, taskgroups.ManifestFileName)); got != before {
			t.Fatal("nonterminal-task completion changed plan bytes")
		}
	})

	t.Run("preserves a clean resolved review when task evidence fails to parse", func(t *testing.T) {
		workspaceRoot := t.TempDir()
		initiativeDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "initiative")
		writeTaskGroupFixture(t, initiativeDir, map[string]string{"TG-001": "pending", "TG-002": "pending"})
		taskGroupDir := filepath.Join(initiativeDir, "_task_groups", "TG-001")
		// A malformed task file makes tasks.SnapshotTaskMeta fail. The review scan must
		// still run independently so a fully resolved review history is not erased.
		writeSyncWorkflowFile(t, taskGroupDir, "task_01.md", "---\nstatus: [unterminated\n---\n# broken\n")
		writeSyncWorkflowFile(
			t,
			taskGroupDir,
			filepath.Join("reviews-001", "issue_001.md"),
			reviewIssueBody("resolved", "high"),
		)
		before := mustReadFile(t, filepath.Join(initiativeDir, taskgroups.ManifestFileName))

		result, err := NewTaskGroupCompletionService().Complete(context.Background(), TaskGroupCompletionRequest{
			WorkspaceRoot: workspaceRoot, Reference: "initiative/TG-001", VerificationPassed: true,
		})
		if err == nil {
			t.Fatal("task evidence parse failure must block completion")
		}
		// The task read failure blocks the checkbox mutation, but verification passed and
		// every review is resolved, so review_clean must remain accurate.
		if !result.ReviewClean {
			t.Fatalf("task-evidence error erased a clean resolved review: %#v", result)
		}
		if result.CompletionRecorded || result.AlreadyCompleted || result.SyncPending {
			t.Fatalf("task-evidence failure recorded completion: %#v", result)
		}
		// The blocker must name task evidence, not the review scan, so it stays actionable.
		if !strings.Contains(err.Error(), "task evidence") {
			t.Fatalf("completion error = %v, want an actionable task-evidence error", err)
		}
		if got := mustReadFile(t, filepath.Join(initiativeDir, taskgroups.ManifestFileName)); got != before {
			t.Fatal("task-evidence failure changed plan bytes")
		}
	})
}

// completionResolverSpy simulates a concurrent task, review, or plan writer by
// running a one-shot mutation immediately after the completion service resolves
// the current plan. The mutation therefore lands after the initial evidence gate
// but before the checkbox write, deterministically reproducing the stale-evidence
// race without goroutines or timing hacks.
type completionResolverSpy struct {
	inner   taskGroupTargetResolver
	mutate  func()
	mutated bool
}

func (r *completionResolverSpy) ResolveTaskGroup(
	ctx context.Context,
	workspaceRoot string,
	reference string,
) (taskgroups.Target, error) {
	target, err := r.inner.ResolveTaskGroup(ctx, workspaceRoot, reference)
	if err == nil && !r.mutated && r.mutate != nil {
		r.mutated = true
		r.mutate()
	}
	return target, err
}

func TestTaskGroupCompletionBridgeRejectsStaleEvidenceRaces(t *testing.T) {
	// INVARIANT: completion records a checkbox only from evidence that is still
	// current at the moment of the write. A task, review, or dependency mutation
	// that lands after the initial gate but before MarkComplete must abort the
	// record, because Store.MarkComplete locks only _task_groups.md and never
	// re-checks task or review artifacts.
	// OWNING_LAYER: service-integration. EXISTING_SUITE: internal/core/sync_test.go.
	newBlockingService := func(t *testing.T, mutate func()) *TaskGroupCompletionService {
		t.Helper()
		service := NewTaskGroupCompletionService()
		service.resolver = &completionResolverSpy{inner: taskgroups.TargetResolver{}, mutate: mutate}
		service.sync = func(context.Context, string, model.ExecutionScope) error {
			t.Fatal("sync must not run when a stale-evidence race blocks completion")
			return nil
		}
		return service
	}

	t.Run("refuses completion when a task turns nonterminal after the initial gate", func(t *testing.T) {
		workspaceRoot := t.TempDir()
		initiativeDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "initiative")
		writeTaskGroupFixture(t, initiativeDir, map[string]string{"TG-001": "pending", "TG-002": "pending"})
		taskGroupDir := filepath.Join(initiativeDir, "_task_groups", "TG-001")
		writeSyncWorkflowFile(t, taskGroupDir, "task_01.md", taskBody("completed", "TG-001 task"))
		before := mustReadFile(t, filepath.Join(initiativeDir, taskgroups.ManifestFileName))

		service := newBlockingService(t, func() {
			writeSyncWorkflowFile(t, taskGroupDir, "task_01.md", taskBody("pending", "TG-001 task"))
		})
		result, err := service.Complete(context.Background(), TaskGroupCompletionRequest{
			WorkspaceRoot: workspaceRoot, Reference: "initiative/TG-001", VerificationPassed: true,
		})
		if err == nil || !strings.Contains(err.Error(), "all task group tasks to be terminal") {
			t.Fatalf("stale-task completion error = %v, want terminal-task block", err)
		}
		// The review was verified and clean, so the review outcome must stay true
		// even though the reopened task blocks the checkbox: the two must diverge.
		if !result.ReviewClean {
			t.Fatalf("verified clean review reported review_clean=false: %#v", result)
		}
		if result.CompletionRecorded || result.AlreadyCompleted || result.SyncPending {
			t.Fatalf("stale-task race recorded completion: %#v", result)
		}
		if got := mustReadFile(t, filepath.Join(initiativeDir, taskgroups.ManifestFileName)); got != before {
			t.Fatal("stale-task race changed plan bytes")
		}
	})

	t.Run("refuses completion when a review reopens after the initial gate", func(t *testing.T) {
		workspaceRoot := t.TempDir()
		initiativeDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "initiative")
		writeTaskGroupFixture(t, initiativeDir, map[string]string{"TG-001": "pending", "TG-002": "pending"})
		taskGroupDir := filepath.Join(initiativeDir, "_task_groups", "TG-001")
		writeSyncWorkflowFile(t, taskGroupDir, "task_01.md", taskBody("completed", "TG-001 task"))
		writeSyncWorkflowFile(
			t,
			taskGroupDir,
			filepath.Join("reviews-001", "issue_001.md"),
			reviewIssueBody("resolved", "high"),
		)
		before := mustReadFile(t, filepath.Join(initiativeDir, taskgroups.ManifestFileName))

		service := newBlockingService(t, func() {
			writeSyncWorkflowFile(
				t,
				taskGroupDir,
				filepath.Join("reviews-001", "issue_002.md"),
				reviewIssueBody("pending", "high"),
			)
		})
		result, err := service.Complete(context.Background(), TaskGroupCompletionRequest{
			WorkspaceRoot: workspaceRoot, Reference: "initiative/TG-001", VerificationPassed: true,
		})
		if err == nil || !strings.Contains(err.Error(), "prior_issues_unresolved") {
			t.Fatalf("reopened-review completion error = %v, want prior-issues block", err)
		}
		if result.ReviewClean {
			t.Fatalf("reopened review reported review_clean=true: %#v", result)
		}
		if result.CompletionRecorded || result.AlreadyCompleted || result.SyncPending {
			t.Fatalf("reopened-review race recorded completion: %#v", result)
		}
		if got := mustReadFile(t, filepath.Join(initiativeDir, taskgroups.ManifestFileName)); got != before {
			t.Fatal("reopened-review race changed plan bytes")
		}
	})

	t.Run("Should refuse completion when a task reopens after the final service gate", func(t *testing.T) {
		workspaceRoot := t.TempDir()
		initiativeDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "initiative")
		writeTaskGroupFixture(t, initiativeDir, map[string]string{"TG-001": "pending", "TG-002": "pending"})
		taskGroupDir := filepath.Join(initiativeDir, "_task_groups", "TG-001")
		writeSyncWorkflowFile(t, taskGroupDir, "task_01.md", taskBody("completed", "TG-001 task"))
		before := mustReadFile(t, filepath.Join(initiativeDir, taskgroups.ManifestFileName))
		realStore := taskgroups.NewStore()
		service := NewTaskGroupCompletionService()
		service.store = completionStoreFunc(
			func(
				ctx context.Context,
				initiativeDir, taskGroupID string,
				validator taskgroups.CompletionValidator,
			) (taskgroups.CompletionResult, error) {
				writeSyncWorkflowFile(t, taskGroupDir, "task_01.md", taskBody("pending", "TG-001 task"))
				return realStore.MarkCompleteValidated(ctx, initiativeDir, taskGroupID, validator)
			},
		)
		service.sync = func(context.Context, string, model.ExecutionScope) error {
			t.Fatal("sync must not run after evidence changes at the store boundary")
			return nil
		}

		result, err := service.Complete(context.Background(), TaskGroupCompletionRequest{
			WorkspaceRoot: workspaceRoot, Reference: "initiative/TG-001", VerificationPassed: true,
		})
		if err == nil || !strings.Contains(err.Error(), "all task group tasks to be terminal") {
			t.Fatalf("store-boundary completion error = %v, want terminal-task block", err)
		}
		if result.CompletionRecorded || result.AlreadyCompleted || result.SyncPending {
			t.Fatalf("store-boundary stale evidence recorded completion: %#v", result)
		}
		if got := mustReadFile(t, filepath.Join(initiativeDir, taskgroups.ManifestFileName)); got != before {
			t.Fatal("store-boundary stale evidence changed plan bytes")
		}
	})
}

func TestSyncTaskGroupInitiativeFailsClosedAndPreservesChildren(t *testing.T) {
	// Suite boundary
	// IN: real filesystem marker validation and aggregate SQLite reconciliation
	// OUT: CLI/API error mapping, owned by transport task work
	// Invariant: a malformed marker never flattens task group artifacts or deletes a valid child projection.
	workspaceRoot := t.TempDir()
	setSyncTestHome(t)
	initiativeDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "initiative")
	writeTaskGroupFixture(t, initiativeDir, map[string]string{
		"TG-001": "completed",
		"TG-002": "completed",
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
		`SELECT COUNT(1) FROM workflows WHERE kind = 'task_group' AND archived_at IS NULL`,
	)
	writeSyncWorkflowFile(t, initiativeDir, "_task_groups.md", "---\ninvalid")
	writeSyncWorkflowFile(t, initiativeDir, "task_99.md", taskBody("pending", "must not flatten"))

	_, err := Sync(context.Background(), SyncConfig{TasksDir: initiativeDir})
	if !errors.Is(err, taskgroups.ErrInvalidPlan) {
		t.Fatalf("Sync(malformed marker) error = %v, want task group invalid plan", err)
	}
	if got := queryCount(
		t,
		sqlDB,
		`SELECT COUNT(1) FROM workflows WHERE kind = 'task_group' AND archived_at IS NULL`,
	); got != beforeChildren {
		t.Fatalf("child rows after invalid sync = %d, want preserved %d", got, beforeChildren)
	}
	if got := queryCount(t, sqlDB, `SELECT COUNT(1) FROM task_items WHERE task_id = 'task_99'`); got != 0 {
		t.Fatalf("flattened parent task rows = %d, want 0", got)
	}
	if err := os.Remove(filepath.Join(initiativeDir, "_task_groups.md")); err != nil {
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
		`SELECT COUNT(1) FROM workflows WHERE kind = 'task_group' AND archived_at IS NULL`,
	); got != 0 {
		t.Fatalf("marker-absent child rows = %d, want 0", got)
	}
	if got := queryCount(t, sqlDB, `SELECT COUNT(1) FROM task_items WHERE task_id = 'task_99'`); got != 1 {
		t.Fatalf("legacy task projection count = %d, want 1", got)
	}
}

func TestSyncTaskGroupInitiativeReportsMissingChildWithoutPruning(t *testing.T) {
	// Suite boundary
	// IN: root aggregate sync against filesystem and SQLite
	// OUT: watcher event delivery, owned by daemon watcher tests
	// Invariant: a vanished declared task group preserves its prior child row and its
	// last-known task projection while flipping missing=1 so read models stop
	// advertising it as runnable, and clears missing on re-materialization; readable
	// siblings update normally.
	workspaceRoot := t.TempDir()
	setSyncTestHome(t)
	initiativeDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "initiative")
	writeTaskGroupFixture(t, initiativeDir, map[string]string{
		"TG-001": "pending",
		"TG-002": "pending",
	})
	if _, err := Sync(context.Background(), SyncConfig{TasksDir: initiativeDir}); err != nil {
		t.Fatalf("Sync(initial): %v", err)
	}

	sqlDB := openSyncSQLite(t)
	defer func() {
		_ = sqlDB.Close()
	}()
	var tg002ID string
	if err := sqlDB.QueryRowContext(context.Background(), `SELECT id FROM workflows WHERE slug = ?`, "initiative/TG-002").
		Scan(&tg002ID); err != nil {
		t.Fatalf("query TG-002 child: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(initiativeDir, "_task_groups", "TG-002")); err != nil {
		t.Fatalf("remove TG-002: %v", err)
	}
	writeSyncWorkflowFile(
		t,
		filepath.Join(initiativeDir, "_task_groups", "TG-001"),
		"task_01.md",
		taskBody("completed", "TG-001 task"),
	)

	result, err := Sync(context.Background(), SyncConfig{TasksDir: initiativeDir})
	if err != nil {
		t.Fatalf("Sync(missing child): %v", err)
	}
	if !result.Partial || !reflect.DeepEqual(result.MissingTaskGroups, []string{"TG-002"}) {
		t.Fatalf("missing child result = %#v, want partial TG-002", result)
	}
	var preservedID string
	if err := sqlDB.QueryRowContext(context.Background(), `SELECT id FROM workflows WHERE slug = ?`, "initiative/TG-002").
		Scan(&preservedID); err != nil {
		t.Fatalf("query preserved TG-002 child: %v", err)
	}
	if preservedID != tg002ID {
		t.Fatalf("TG-002 child id changed after partial sync: got %q want %q", preservedID, tg002ID)
	}
	var tg001Status string
	if err := sqlDB.QueryRowContext(
		context.Background(),
		`SELECT status FROM task_items WHERE workflow_id = (SELECT id FROM workflows WHERE slug = ?)`,
		"initiative/TG-001",
	).Scan(&tg001Status); err != nil {
		t.Fatalf("query TG-001 task state: %v", err)
	}
	if tg001Status != "completed" {
		t.Fatalf("TG-001 task status = %q, want completed", tg001Status)
	}
	// The vanished directory must flip the durable row to missing so read models
	// stop advertising it as runnable, while a metadata-only reconcile retains its
	// last-known task projection rather than erasing it.
	if got := queryCount(t, sqlDB, `SELECT missing FROM workflows WHERE slug = ?`, "initiative/TG-002"); got != 1 {
		t.Fatalf("removed TG-002 missing = %d, want 1 to mark the absent directory", got)
	}
	if got := queryCount(
		t,
		sqlDB,
		`SELECT COUNT(1) FROM task_items WHERE workflow_id = (SELECT id FROM workflows WHERE slug = ?)`,
		"initiative/TG-002",
	); got != 1 {
		t.Fatalf("removed TG-002 retained task count = %d, want 1 preserved projection", got)
	}
	var tg002TaskStatus string
	if err := sqlDB.QueryRowContext(
		context.Background(),
		`SELECT status FROM task_items WHERE workflow_id = (SELECT id FROM workflows WHERE slug = ?)`,
		"initiative/TG-002",
	).Scan(&tg002TaskStatus); err != nil {
		t.Fatalf("query retained TG-002 task: %v", err)
	}
	if tg002TaskStatus != "pending" {
		t.Fatalf("removed TG-002 retained task status = %q, want pending last-known state", tg002TaskStatus)
	}

	// Re-materializing the directory clears the missing flag in place without
	// changing the row identity, so the read model resumes treating it as runnable.
	writeSyncWorkflowFile(
		t,
		filepath.Join(initiativeDir, "_task_groups", "TG-002"),
		"task_01.md",
		taskBody("pending", "TG-002 task"),
	)
	if _, err := Sync(context.Background(), SyncConfig{TasksDir: initiativeDir}); err != nil {
		t.Fatalf("Sync(rematerialized child): %v", err)
	}
	var rematerializedID string
	var rematerializedMissing bool
	if err := sqlDB.QueryRowContext(
		context.Background(),
		`SELECT id, missing FROM workflows WHERE slug = ?`,
		"initiative/TG-002",
	).Scan(&rematerializedID, &rematerializedMissing); err != nil {
		t.Fatalf("query rematerialized TG-002: %v", err)
	}
	if rematerializedID != tg002ID {
		t.Fatalf("rematerialized TG-002 id changed: got %q want %q", rematerializedID, tg002ID)
	}
	if rematerializedMissing {
		t.Fatal("rematerialized TG-002 missing = true, want cleared once the directory exists")
	}
}

func TestSyncTaskGroupInitiativeReportsMissingRootWithoutAborting(t *testing.T) {
	// Suite boundary
	// IN: root aggregate sync against filesystem and SQLite
	// OUT: watcher event delivery, owned by daemon watcher tests
	// Invariant: an entirely absent _task_groups root degrades every declared task group
	// to a Missing placeholder so aggregate sync completes partially instead of
	// hard-aborting (which also unblocks archive, whose refresh runs the same
	// reconcile), and the root returning clears every placeholder in place.
	workspaceRoot := t.TempDir()
	setSyncTestHome(t)
	initiativeDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "initiative")
	writeTaskGroupFixture(t, initiativeDir, map[string]string{
		"TG-001": "pending",
		"TG-002": "pending",
	})
	if _, err := Sync(context.Background(), SyncConfig{TasksDir: initiativeDir}); err != nil {
		t.Fatalf("Sync(initial): %v", err)
	}

	if err := os.RemoveAll(filepath.Join(initiativeDir, "_task_groups")); err != nil {
		t.Fatalf("remove _task_groups root: %v", err)
	}

	result, err := Sync(context.Background(), SyncConfig{TasksDir: initiativeDir})
	if err != nil {
		t.Fatalf("Sync(missing root): %v", err)
	}
	if !result.Partial || !reflect.DeepEqual(result.MissingTaskGroups, []string{"TG-001", "TG-002"}) {
		t.Fatalf("missing root result = %#v, want partial TG-001+TG-002", result)
	}

	sqlDB := openSyncSQLite(t)
	defer func() {
		_ = sqlDB.Close()
	}()
	for _, slug := range []string{"initiative/TG-001", "initiative/TG-002"} {
		if got := queryCount(t, sqlDB, `SELECT missing FROM workflows WHERE slug = ?`, slug); got != 1 {
			t.Fatalf("%s missing = %d, want 1 with the root absent", slug, got)
		}
	}

	// The root returning must clear every placeholder without a hard abort so read
	// models resume treating the task groups as runnable.
	writeSyncWorkflowFile(
		t,
		filepath.Join(initiativeDir, "_task_groups", "TG-001"),
		"task_01.md",
		taskBody("pending", "TG-001 task"),
	)
	writeSyncWorkflowFile(
		t,
		filepath.Join(initiativeDir, "_task_groups", "TG-002"),
		"task_01.md",
		taskBody("pending", "TG-002 task"),
	)
	if _, err := Sync(context.Background(), SyncConfig{TasksDir: initiativeDir}); err != nil {
		t.Fatalf("Sync(rematerialized root): %v", err)
	}
	for _, slug := range []string{"initiative/TG-001", "initiative/TG-002"} {
		if got := queryCount(t, sqlDB, `SELECT missing FROM workflows WHERE slug = ?`, slug); got != 0 {
			t.Fatalf("%s missing = %d, want 0 once the root returns", slug, got)
		}
	}
}

func TestSyncTaskGroupInitiativeRepeatedPartialSyncRetainsChildHistory(t *testing.T) {
	// Suite boundary
	// IN: repeated root aggregate sync against filesystem and SQLite while a
	// materialized task group directory stays absent across consecutive scans
	// OUT: daemon read-model projection, exercised by internal/daemon integration tests
	// Invariant: every partial sync after a materialized child vanishes preserves the
	// child identity, completion state, artifacts, tasks, reviews, and checkpoint via a
	// metadata-only reconcile; the second consecutive partial sync must not erase the
	// retained projection just because the durable row already reads missing=1.
	workspaceRoot := t.TempDir()
	setSyncTestHome(t)
	initiativeDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "initiative")
	writeTaskGroupFixture(t, initiativeDir, map[string]string{
		"TG-001": "pending",
		"TG-002": "completed",
	})
	// Give TG-002 a review round so the metadata-only reconcile has artifacts, tasks,
	// reviews, and a checkpoint to retain across repeated partial syncs.
	tg002Dir := filepath.Join(initiativeDir, "_task_groups", "TG-002")
	writeSyncWorkflowFile(t, tg002Dir, filepath.Join("reviews-001", "_meta.md"), reviewRoundMetaBody("manual", "", 1))
	writeSyncWorkflowFile(t, tg002Dir, filepath.Join("reviews-001", "issue_001.md"), reviewIssueBody("pending", "high"))

	if _, err := Sync(context.Background(), SyncConfig{TasksDir: initiativeDir}); err != nil {
		t.Fatalf("Sync(initial materialized): %v", err)
	}

	sqlDB := openSyncSQLite(t)
	defer func() {
		_ = sqlDB.Close()
	}()

	const tg002Slug = "initiative/TG-002"
	baseline := readChildProjection(t, sqlDB, tg002Slug)
	if baseline.missing {
		t.Fatal("materialized TG-002 missing = true, want false before the directory is removed")
	}
	if !baseline.lifecycleCompleted {
		t.Fatal("materialized TG-002 lifecycle_completed = false, want true for a completed task group")
	}
	if baseline.artifacts == 0 || baseline.tasks == 0 || baseline.reviewRounds == 0 ||
		baseline.reviewIssues == 0 || baseline.checkpointChecksum == "" {
		t.Fatalf("materialized TG-002 baseline is incomplete: %#v", baseline)
	}

	// The directory vanishes and never returns: two consecutive partial syncs must
	// both preserve the retained projection. The first sync flips missing=1 in the
	// durable row; the second must not treat that flag as license to reseed empty
	// projections and delete the retained history.
	if err := os.RemoveAll(tg002Dir); err != nil {
		t.Fatalf("remove TG-002: %v", err)
	}
	for _, pass := range []string{"first", "second"} {
		result, err := Sync(context.Background(), SyncConfig{TasksDir: initiativeDir})
		if err != nil {
			t.Fatalf("Sync(%s partial): %v", pass, err)
		}
		if !result.Partial || !reflect.DeepEqual(result.MissingTaskGroups, []string{"TG-002"}) {
			t.Fatalf("%s partial sync result = %#v, want partial TG-002", pass, result)
		}
		got := readChildProjection(t, sqlDB, tg002Slug)
		if got.id != baseline.id {
			t.Fatalf("%s partial sync changed TG-002 id: got %q want %q", pass, got.id, baseline.id)
		}
		if !got.missing {
			t.Fatalf("%s partial sync TG-002 missing = false, want true while the directory stays absent", pass)
		}
		if got.lifecycleCompleted != baseline.lifecycleCompleted {
			t.Fatalf(
				"%s partial sync TG-002 lifecycle_completed = %v, want %v",
				pass,
				got.lifecycleCompleted,
				baseline.lifecycleCompleted,
			)
		}
		if got.artifacts != baseline.artifacts {
			t.Fatalf("%s partial sync TG-002 artifacts = %d, want %d retained", pass, got.artifacts, baseline.artifacts)
		}
		if got.tasks != baseline.tasks {
			t.Fatalf("%s partial sync TG-002 tasks = %d, want %d retained", pass, got.tasks, baseline.tasks)
		}
		if got.reviewRounds != baseline.reviewRounds || got.reviewIssues != baseline.reviewIssues {
			t.Fatalf(
				"%s partial sync TG-002 reviews = %d rounds / %d issues, want %d / %d retained",
				pass,
				got.reviewRounds,
				got.reviewIssues,
				baseline.reviewRounds,
				baseline.reviewIssues,
			)
		}
		if got.checkpointChecksum != baseline.checkpointChecksum {
			t.Fatalf(
				"%s partial sync TG-002 checkpoint checksum = %q, want %q retained",
				pass,
				got.checkpointChecksum,
				baseline.checkpointChecksum,
			)
		}
	}
}

func TestSyncTaskGroupInitiativePrunesRemovedChildDespiteMissingSibling(t *testing.T) {
	// Suite boundary
	// IN: root aggregate sync against filesystem and SQLite where one declared task group
	// directory is absent while another task group is dropped from the plan in the same edit
	// OUT: daemon read-model projection, exercised by internal/daemon integration tests
	// Invariant: a full-initiative sync always emits a complete child set (real
	// projections plus Missing placeholders), so it must prune children the plan no
	// longer declares even while a declared sibling directory is absent. The missing
	// sibling survives as a retained missing=1 placeholder; the plan-removed child is
	// pruned unless it still owns an active run, in which case pruning is skipped and
	// reported rather than silently stranding a ghost child.
	t.Run("Should prune a removed child while retaining a missing declared sibling", func(t *testing.T) {
		workspaceRoot := t.TempDir()
		setSyncTestHome(t)
		initiativeDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "initiative")
		writeInitiativeFixtureWithTaskGroups(t, initiativeDir, []string{"TG-001", "TG-002", "TG-003"})
		if _, err := Sync(context.Background(), SyncConfig{TasksDir: initiativeDir}); err != nil {
			t.Fatalf("Sync(initial three-task-group plan): %v", err)
		}

		sqlDB := openSyncSQLite(t)
		defer func() {
			_ = sqlDB.Close()
		}()
		tg002ID := querySyncWorkflowID(t, sqlDB, "initiative/TG-002")

		// TG-002's directory vanishes but the plan still declares it; the same edit
		// drops TG-003 from the plan entirely.
		if err := os.RemoveAll(filepath.Join(initiativeDir, "_task_groups", "TG-002")); err != nil {
			t.Fatalf("remove TG-002 directory: %v", err)
		}
		writeInitiativePlanWithTaskGroups(t, initiativeDir, []string{"TG-001", "TG-002"})

		result, err := Sync(context.Background(), SyncConfig{TasksDir: initiativeDir})
		if err != nil {
			t.Fatalf("Sync(missing sibling + removed child): %v", err)
		}
		if !result.Partial || !reflect.DeepEqual(result.MissingTaskGroups, []string{"TG-002"}) {
			t.Fatalf("result = %#v, want partial with missing TG-002", result)
		}
		if result.WorkflowsPruned != 1 || !reflect.DeepEqual(result.PrunedWorkflows, []string{"TG-003"}) {
			t.Fatalf("prune result = %d %#v, want 1 [TG-003]", result.WorkflowsPruned, result.PrunedWorkflows)
		}
		// The still-declared but absent sibling keeps its row identity and flips to
		// missing=1 rather than being pruned alongside the removed child.
		if preservedID := querySyncWorkflowID(t, sqlDB, "initiative/TG-002"); preservedID != tg002ID {
			t.Fatalf("TG-002 child id changed: got %q want %q", preservedID, tg002ID)
		}
		if got := queryCount(t, sqlDB, `SELECT missing FROM workflows WHERE slug = ?`, "initiative/TG-002"); got != 1 {
			t.Fatalf("TG-002 missing = %d, want 1 to mark the absent directory", got)
		}
		// The plan-removed child is gone from the initiative entirely.
		if got := queryCount(
			t,
			sqlDB,
			`SELECT COUNT(1) FROM workflows WHERE slug = ? AND archived_at IS NULL`,
			"initiative/TG-003",
		); got != 0 {
			t.Fatalf("TG-003 row count = %d, want 0 after pruning", got)
		}
	})

	t.Run("Should skip pruning a removed child that still owns an active run", func(t *testing.T) {
		workspaceRoot := t.TempDir()
		setSyncTestHome(t)
		initiativeDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "initiative")
		writeInitiativeFixtureWithTaskGroups(t, initiativeDir, []string{"TG-001", "TG-002", "TG-003"})
		if _, err := Sync(context.Background(), SyncConfig{TasksDir: initiativeDir}); err != nil {
			t.Fatalf("Sync(initial three-task-group plan): %v", err)
		}
		insertActiveArchiveRun(t, initiativeDir, "initiative/TG-003", "run-task-group-003-active")

		sqlDB := openSyncSQLite(t)
		defer func() {
			_ = sqlDB.Close()
		}()

		if err := os.RemoveAll(filepath.Join(initiativeDir, "_task_groups", "TG-002")); err != nil {
			t.Fatalf("remove TG-002 directory: %v", err)
		}
		writeInitiativePlanWithTaskGroups(t, initiativeDir, []string{"TG-001", "TG-002"})

		result, err := Sync(context.Background(), SyncConfig{TasksDir: initiativeDir})
		if err != nil {
			t.Fatalf("Sync(active-run removed child): %v", err)
		}
		if result.WorkflowsPruned != 0 || len(result.PrunedWorkflows) != 0 {
			t.Fatalf(
				"prune result = %d %#v, want no prune while the run is active",
				result.WorkflowsPruned,
				result.PrunedWorkflows,
			)
		}
		if !hasWarningContaining(result.Warnings, "skipped stale task group prune") {
			t.Fatalf("warnings = %#v, want a skipped stale task group prune notice", result.Warnings)
		}
		// The retained child keeps its row so the in-flight run is never orphaned.
		if got := queryCount(
			t,
			sqlDB,
			`SELECT COUNT(1) FROM workflows WHERE slug = ? AND archived_at IS NULL`,
			"initiative/TG-003",
		); got != 1 {
			t.Fatalf("TG-003 row count = %d, want 1 retained for the active run", got)
		}
	})
}

type childProjectionSnapshot struct {
	id                 string
	missing            bool
	lifecycleCompleted bool
	artifacts          int
	tasks              int
	reviewRounds       int
	reviewIssues       int
	checkpointChecksum string
}

func readChildProjection(t *testing.T, db *sql.DB, slug string) childProjectionSnapshot {
	t.Helper()

	var snap childProjectionSnapshot
	if err := db.QueryRowContext(
		context.Background(),
		`SELECT id, missing, lifecycle_completed FROM workflows WHERE slug = ? AND archived_at IS NULL`,
		slug,
	).Scan(&snap.id, &snap.missing, &snap.lifecycleCompleted); err != nil {
		t.Fatalf("query workflow row %q: %v", slug, err)
	}
	snap.artifacts = queryCount(t, db, `SELECT COUNT(1) FROM artifact_snapshots WHERE workflow_id = ?`, snap.id)
	snap.tasks = queryCount(t, db, `SELECT COUNT(1) FROM task_items WHERE workflow_id = ?`, snap.id)
	snap.reviewRounds = queryCount(t, db, `SELECT COUNT(1) FROM review_rounds WHERE workflow_id = ?`, snap.id)
	snap.reviewIssues = queryCount(
		t,
		db,
		`SELECT COUNT(1) FROM review_issues WHERE round_id IN (SELECT id FROM review_rounds WHERE workflow_id = ?)`,
		snap.id,
	)
	if err := db.QueryRowContext(
		context.Background(),
		`SELECT checksum FROM sync_checkpoints WHERE workflow_id = ?`,
		snap.id,
	).Scan(&snap.checkpointChecksum); err != nil {
		t.Fatalf("query checkpoint for %q: %v", slug, err)
	}
	return snap
}

func TestSyncTaskGroupInitiativeFirstSyncSeedsMissingDependencyPlaceholder(t *testing.T) {
	// Suite boundary
	// IN: first-ever root aggregate sync against filesystem and SQLite
	// OUT: daemon read-model projection, exercised by internal/daemon integration tests
	// Invariant: a first-ever partial sync persists the complete declared graph so a
	// present task group never stores an edge to an unpersisted prerequisite node.
	workspaceRoot := t.TempDir()
	setSyncTestHome(t)
	initiativeDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "initiative")
	writeTaskGroupFixture(t, initiativeDir, map[string]string{
		"TG-001": "pending",
		"TG-002": "pending",
	})
	// Remove the prerequisite task group directory before the first sync: TG-002
	// (present) declares a dependency on TG-001 (now missing), reproducing the
	// first-ever partial sync that previously left a dangling graph edge.
	if err := os.RemoveAll(filepath.Join(initiativeDir, "_task_groups", "TG-001")); err != nil {
		t.Fatalf("remove TG-001: %v", err)
	}

	result, err := Sync(context.Background(), SyncConfig{TasksDir: initiativeDir})
	if err != nil {
		t.Fatalf("Sync(first partial): %v", err)
	}
	if !result.Partial || !reflect.DeepEqual(result.MissingTaskGroups, []string{"TG-001"}) {
		t.Fatalf("first partial sync result = %#v, want partial TG-001", result)
	}

	sqlDB := openSyncSQLite(t)
	defer func() {
		_ = sqlDB.Close()
	}()

	var firstID, kind, dependenciesJSON string
	var lifecycleCompleted bool
	if err := sqlDB.QueryRowContext(
		context.Background(),
		`SELECT id, kind, lifecycle_completed, dependencies_json FROM workflows WHERE slug = ?`,
		"initiative/TG-001",
	).Scan(&firstID, &kind, &lifecycleCompleted, &dependenciesJSON); err != nil {
		t.Fatalf("query seeded TG-001 placeholder: %v", err)
	}
	if kind != string(globaldb.WorkflowKindTaskGroup) {
		t.Fatalf("placeholder kind = %q, want task_group", kind)
	}
	if lifecycleCompleted {
		t.Fatal("placeholder lifecycle_completed = true, want false to block start and archive")
	}
	// The durable missing discriminator distinguishes an unavailable placeholder
	// from a real, taskless task group so the read model can block start with a
	// clear reason rather than advertising the absent directory as runnable.
	if got := queryCount(t, sqlDB, `SELECT missing FROM workflows WHERE slug = ?`, "initiative/TG-001"); got != 1 {
		t.Fatalf("placeholder missing = %d, want 1 to mark the absent directory", got)
	}
	if got := queryCount(t, sqlDB, `SELECT missing FROM workflows WHERE slug = ?`, "initiative/TG-002"); got != 0 {
		t.Fatalf("materialized TG-002 missing = %d, want 0", got)
	}
	if got := queryCount(
		t,
		sqlDB,
		`SELECT COUNT(1) FROM task_items WHERE workflow_id = (SELECT id FROM workflows WHERE slug = ?)`,
		"initiative/TG-001",
	); got != 0 {
		t.Fatalf("placeholder task projection count = %d, want 0 fabricated rows", got)
	}

	// The present dependent keeps its declared edge; with the prerequisite now
	// persisted the stored graph is complete instead of pointing at a missing node.
	var dependentDeps string
	if err := sqlDB.QueryRowContext(
		context.Background(),
		`SELECT dependencies_json FROM workflows WHERE slug = ?`,
		"initiative/TG-002",
	).Scan(&dependentDeps); err != nil {
		t.Fatalf("query dependent TG-002: %v", err)
	}
	if !strings.Contains(dependentDeps, "TG-001") {
		t.Fatalf("dependent TG-002 dependencies = %q, want edge to TG-001", dependentDeps)
	}

	// Re-syncing while the directory stays absent must preserve the same placeholder
	// row rather than duplicating or overwriting it.
	if _, err := Sync(context.Background(), SyncConfig{TasksDir: initiativeDir}); err != nil {
		t.Fatalf("Sync(second partial): %v", err)
	}
	if got := queryCount(t, sqlDB, `SELECT COUNT(1) FROM workflows WHERE slug = ?`, "initiative/TG-001"); got != 1 {
		t.Fatalf("TG-001 placeholder rows after re-sync = %d, want 1", got)
	}
	var secondID string
	if err := sqlDB.QueryRowContext(
		context.Background(),
		`SELECT id FROM workflows WHERE slug = ?`,
		"initiative/TG-001",
	).Scan(&secondID); err != nil {
		t.Fatalf("query preserved TG-001 placeholder: %v", err)
	}
	if secondID != firstID {
		t.Fatalf("placeholder id changed after re-sync: got %q want %q", secondID, firstID)
	}
	if got := queryCount(t, sqlDB, `SELECT missing FROM workflows WHERE slug = ?`, "initiative/TG-001"); got != 1 {
		t.Fatalf("placeholder missing after re-sync = %d, want 1 while directory stays absent", got)
	}

	// Materializing the directory replaces the placeholder in place: the same row
	// clears its missing flag so the read model stops blocking start.
	writeSyncWorkflowFile(
		t,
		filepath.Join(initiativeDir, "_task_groups", "TG-001"),
		"task_01.md",
		taskBody("pending", "TG-001 task"),
	)
	if _, err := Sync(context.Background(), SyncConfig{TasksDir: initiativeDir}); err != nil {
		t.Fatalf("Sync(materialized): %v", err)
	}
	var materializedID string
	var materializedMissing bool
	if err := sqlDB.QueryRowContext(
		context.Background(),
		`SELECT id, missing FROM workflows WHERE slug = ?`,
		"initiative/TG-001",
	).Scan(&materializedID, &materializedMissing); err != nil {
		t.Fatalf("query materialized TG-001: %v", err)
	}
	if materializedID != firstID {
		t.Fatalf("materialized TG-001 id changed: got %q want %q", materializedID, firstID)
	}
	if materializedMissing {
		t.Fatal("materialized TG-001 missing = true, want cleared once the directory exists")
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

func writeTaskGroupFixture(t *testing.T, initiativeDir string, states map[string]string) {
	t.Helper()

	planStatus := func(taskGroupID string) string {
		if states[taskGroupID] == "completed" {
			return "x"
		}
		return " "
	}
	writeSyncWorkflowFile(t, initiativeDir, "_prd.md", "# Initiative\n")
	writeSyncWorkflowFile(t, initiativeDir, "_techspec.md", "# Initiative Techspec\n")
	writeSyncWorkflowFile(t, initiativeDir, "_tasks.md", canonicalTaskListBody())
	writeSyncWorkflowFile(t, initiativeDir, "_task_groups.md", strings.Join([]string{
		"---",
		"schema_version: compozy.task-groups/v1",
		"initiative: initiative",
		"graph:",
		"  nodes:",
		"    - id: TG-001",
		"      directory: _task_groups/TG-001",
		"    - id: TG-002",
		"      directory: _task_groups/TG-002",
		"  edges:",
		"    - from: TG-001",
		"      to: TG-002",
		"      rationale: TG-002 consumes the TG-001 contract.",
		"---",
		"",
		"# Initiative Task Groups",
		"",
		"## [" + planStatus("TG-001") + "] TG-001 — Persistence",
		"",
		"- Reference: `initiative/TG-001`",
		"- Outcome: Persist the parent workflow.",
		"- Owns:",
		"  - persistence",
		"- Dependencies: None",
		"",
		"## [" + planStatus("TG-002") + "] TG-002 — Archive",
		"",
		"- Reference: `initiative/TG-002`",
		"- Outcome: Archive the aggregate workflow.",
		"- Owns:",
		"  - archive",
		"- Dependencies:",
		"  - `TG-001` — TG-002 consumes the TG-001 contract.",
		"",
	}, "\n"))

	for _, taskGroupID := range []string{"TG-001", "TG-002"} {
		taskGroupDir := filepath.Join(initiativeDir, "_task_groups", taskGroupID)
		writeSyncWorkflowFile(t, taskGroupDir, "_tasks.md", canonicalTaskListBody())
		writeSyncWorkflowFile(t, taskGroupDir, "task_01.md", taskBody(states[taskGroupID], taskGroupID+" task"))
	}
}

// writeInitiativeFixtureWithTaskGroups writes a valid initiative whose plan declares
// the given task groups, materializing a task projection for each so a first sync
// projects every child as present.
func writeInitiativeFixtureWithTaskGroups(t *testing.T, initiativeDir string, taskGroupIDs []string) {
	t.Helper()

	writeSyncWorkflowFile(t, initiativeDir, "_prd.md", "# Initiative\n")
	writeSyncWorkflowFile(t, initiativeDir, "_techspec.md", "# Initiative Techspec\n")
	writeSyncWorkflowFile(t, initiativeDir, "_tasks.md", canonicalTaskListBody())
	writeInitiativePlanWithTaskGroups(t, initiativeDir, taskGroupIDs)
	for _, taskGroupID := range taskGroupIDs {
		taskGroupDir := filepath.Join(initiativeDir, "_task_groups", taskGroupID)
		writeSyncWorkflowFile(t, taskGroupDir, "_tasks.md", canonicalTaskListBody())
		writeSyncWorkflowFile(t, taskGroupDir, "task_01.md", taskBody("pending", taskGroupID+" task"))
	}
}

// writeInitiativePlanWithTaskGroups writes a dependency-free _task_groups.md that
// declares exactly the given task groups, so rewriting it with a subset simulates a
// plan edit that drops a task group.
func writeInitiativePlanWithTaskGroups(t *testing.T, initiativeDir string, taskGroupIDs []string) {
	t.Helper()

	lines := []string{
		"---",
		"schema_version: compozy.task-groups/v1",
		"initiative: initiative",
		"graph:",
		"  nodes:",
	}
	for _, taskGroupID := range taskGroupIDs {
		lines = append(lines, "    - id: "+taskGroupID, "      directory: _task_groups/"+taskGroupID)
	}
	lines = append(lines, "  edges: []", "---", "", "# Initiative Task Groups", "")
	for _, taskGroupID := range taskGroupIDs {
		lines = append(lines,
			"## [ ] "+taskGroupID+" — "+taskGroupID+" work",
			"",
			"- Reference: `initiative/"+taskGroupID+"`",
			"- Outcome: Deliver "+taskGroupID+".",
			"- Owns:",
			"  - "+strings.ToLower(taskGroupID),
			"- Dependencies: None",
			"",
		)
	}
	writeSyncWorkflowFile(t, initiativeDir, "_task_groups.md", strings.Join(lines, "\n"))
}

func querySyncWorkflowID(t *testing.T, db *sql.DB, slug string) string {
	t.Helper()

	var id string
	if err := db.QueryRowContext(
		context.Background(),
		`SELECT id FROM workflows WHERE slug = ? AND archived_at IS NULL`,
		slug,
	).Scan(&id); err != nil {
		t.Fatalf("query workflow id %q: %v", slug, err)
	}
	return id
}

func hasWarningContaining(warnings []string, substr string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, substr) {
			return true
		}
	}
	return false
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
