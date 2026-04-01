package core

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/tasks"
)

func TestSyncTaskMetadataScansWorkflowRoot(t *testing.T) {
	t.Parallel()

	rootDir := filepath.Join(t.TempDir(), ".compozy", "tasks")
	alphaDir := filepath.Join(rootDir, "alpha")
	betaDir := filepath.Join(rootDir, "beta")
	gammaDir := filepath.Join(rootDir, "gamma")
	for _, dir := range []string{alphaDir, betaDir, gammaDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	writeSyncTaskFile(t, alphaDir, "task_01.md", "pending")
	writeSyncTaskFile(t, betaDir, "task_01.md", "completed")
	if err := tasks.WriteTaskMeta(betaDir, model.TaskMeta{
		CreatedAt: time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 4, 1, 12, 5, 0, 0, time.UTC),
		Total:     99,
		Completed: 99,
		Pending:   0,
	}); err != nil {
		t.Fatalf("write stale beta meta: %v", err)
	}

	result, err := Sync(context.Background(), SyncConfig{RootDir: rootDir})
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if result.WorkflowsScanned != 3 {
		t.Fatalf("expected 3 workflows scanned, got %d", result.WorkflowsScanned)
	}
	if result.MetaCreated != 2 || result.MetaUpdated != 1 {
		t.Fatalf("unexpected sync counts: %#v", result)
	}

	wantPaths := []string{alphaDir, betaDir, gammaDir}
	if !reflect.DeepEqual(result.SyncedPaths, wantPaths) {
		t.Fatalf("unexpected synced paths\nwant: %#v\ngot:  %#v", wantPaths, result.SyncedPaths)
	}

	alphaMeta, err := tasks.ReadTaskMeta(alphaDir)
	if err != nil {
		t.Fatalf("read alpha meta: %v", err)
	}
	if alphaMeta.Total != 1 || alphaMeta.Completed != 0 || alphaMeta.Pending != 1 {
		t.Fatalf("unexpected alpha meta: %#v", alphaMeta)
	}

	betaMeta, err := tasks.ReadTaskMeta(betaDir)
	if err != nil {
		t.Fatalf("read beta meta: %v", err)
	}
	if betaMeta.Total != 1 || betaMeta.Completed != 1 || betaMeta.Pending != 0 {
		t.Fatalf("unexpected beta meta: %#v", betaMeta)
	}

	gammaMeta, err := tasks.ReadTaskMeta(gammaDir)
	if err != nil {
		t.Fatalf("read gamma meta: %v", err)
	}
	if gammaMeta.Total != 0 || gammaMeta.Completed != 0 || gammaMeta.Pending != 0 {
		t.Fatalf("unexpected gamma meta: %#v", gammaMeta)
	}
}

func TestSyncTaskMetadataRestrictsToSingleWorkflow(t *testing.T) {
	t.Parallel()

	rootDir := filepath.Join(t.TempDir(), ".compozy", "tasks")
	alphaDir := filepath.Join(rootDir, "alpha")
	betaDir := filepath.Join(rootDir, "beta")
	for _, dir := range []string{alphaDir, betaDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	writeSyncTaskFile(t, alphaDir, "task_01.md", "pending")
	writeSyncTaskFile(t, betaDir, "task_01.md", "pending")

	result, err := Sync(context.Background(), SyncConfig{Name: "beta", RootDir: rootDir})
	if err != nil {
		t.Fatalf("sync by name: %v", err)
	}
	if result.WorkflowsScanned != 1 || result.MetaCreated != 1 || result.MetaUpdated != 0 {
		t.Fatalf("unexpected sync result: %#v", result)
	}
	if _, err := os.Stat(tasks.MetaPath(alphaDir)); !os.IsNotExist(err) {
		t.Fatalf("expected alpha meta to remain absent, got err=%v", err)
	}
	if _, err := os.Stat(tasks.MetaPath(betaDir)); err != nil {
		t.Fatalf("expected beta meta to exist: %v", err)
	}
}

func TestSyncTaskMetadataRejectsConflictingTargets(t *testing.T) {
	t.Parallel()

	_, err := Sync(context.Background(), SyncConfig{
		Name:     "alpha",
		TasksDir: ".compozy/tasks/alpha",
	})
	if err == nil {
		t.Fatal("expected sync to reject conflicting targets")
	}
	if !strings.Contains(err.Error(), "--name or --tasks-dir") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func writeSyncTaskFile(t *testing.T, workflowDir, name, status string) {
	t.Helper()

	content := strings.Join([]string{
		"---",
		"status: " + status,
		"domain: backend",
		"type: feature",
		"scope: small",
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
