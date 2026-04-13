package test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/compozy"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/pkg/compozy/runs"
	"github.com/compozy/compozy/pkg/compozy/runs/layout"
)

func TestPrepareAndRunExposePublicAPI(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	tasksDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "demo")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("mkdir tasks dir: %v", err)
	}

	taskFile := filepath.Join(tasksDir, "task_1.md")
	taskContent := `---
status: pending
title: Demo
type: backend
complexity: low
---

# Task 1: Demo
`
	if err := os.WriteFile(taskFile, []byte(taskContent), 0o600); err != nil {
		t.Fatalf("write task file: %v", err)
	}

	cfg := compozy.Config{
		Name:          "demo",
		TasksDir:      tasksDir,
		WorkspaceRoot: workspaceRoot,
		Mode:          compozy.ModePRDTasks,
		DryRun:        true,
	}

	prep, err := compozy.Prepare(context.Background(), cfg)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if prep == nil {
		t.Fatal("expected preparation result")
	}
	if len(prep.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(prep.Jobs))
	}
	if prep.Jobs[0].PromptPath == "" {
		t.Fatal("expected prompt path to be populated")
	}

	if err := compozy.Run(context.Background(), cfg); err != nil {
		t.Fatalf("run: %v", err)
	}
}

func TestNewCommandUsesCompozyRootCommand(t *testing.T) {
	t.Parallel()

	cmd := compozy.NewCommand()
	if cmd == nil {
		t.Fatal("expected command")
	}
	if cmd.Use != "compozy" {
		t.Fatalf("expected use compozy, got %q", cmd.Use)
	}
}

func TestMigrateExposePublicAPI(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	workflowDir := filepath.Join(tmpDir, ".compozy", "tasks", "demo")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workflowDir, "task_1.md"), []byte(strings.Join([]string{
		"## status: pending",
		"<task_context><domain>backend</domain><type>feature</type><scope>small</scope><complexity>low</complexity></task_context>",
		"# Task 1: Demo",
		"",
	}, "\n")), 0o600); err != nil {
		t.Fatalf("write legacy task: %v", err)
	}

	result, err := compozy.Migrate(context.Background(), compozy.MigrationConfig{DryRun: true})
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if result == nil {
		t.Fatal("expected migration result")
	}
	if result.FilesMigrated != 1 {
		t.Fatalf("expected 1 planned migration, got %d", result.FilesMigrated)
	}
}

func TestSyncExposePublicAPI(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	workflowDir := filepath.Join(tmpDir, ".compozy", "tasks", "demo")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workflowDir, "task_1.md"), []byte(strings.Join([]string{
		"---",
		"status: pending",
		"title: Demo",
		"type: backend",
		"complexity: low",
		"---",
		"",
		"# Task 1: Demo",
		"",
	}, "\n")), 0o600); err != nil {
		t.Fatalf("write task file: %v", err)
	}

	result, err := compozy.Sync(context.Background(), compozy.SyncConfig{TasksDir: workflowDir})
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if result == nil {
		t.Fatal("expected sync result")
	}
	if result.WorkflowsScanned != 1 || result.MetaCreated != 1 || result.MetaUpdated != 0 {
		t.Fatalf("unexpected sync result: %#v", result)
	}
}

func TestArchiveExposePublicAPI(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	workflowDir := filepath.Join(tmpDir, ".compozy", "tasks", "demo")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workflowDir, "task_001.md"), []byte(strings.Join([]string{
		"---",
		"status: completed",
		"title: Demo",
		"type: backend",
		"complexity: low",
		"---",
		"",
		"# Task 1: Demo",
		"",
	}, "\n")), 0o600); err != nil {
		t.Fatalf("write task file: %v", err)
	}

	metaContent := strings.Join([]string{
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
	}, "\n")
	if err := os.WriteFile(filepath.Join(workflowDir, "_meta.md"), []byte(metaContent), 0o600); err != nil {
		t.Fatalf("write task meta: %v", err)
	}

	result, err := compozy.Archive(context.Background(), compozy.ArchiveConfig{TasksDir: workflowDir})
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if result == nil {
		t.Fatal("expected archive result")
	}
	if result.Archived != 1 || result.Skipped != 0 {
		t.Fatalf("unexpected archive result: %#v", result)
	}
	if len(result.ArchivedPaths) != 1 {
		t.Fatalf("expected one archived path, got %#v", result.ArchivedPaths)
	}
	if _, err := os.Stat(result.ArchivedPaths[0]); err != nil {
		t.Fatalf("expected archived path to exist: %v", err)
	}
}

// TestRunsLayoutAgreesAcrossWriterAndReader proves that the canonical writer
// (model.NewRunArtifacts) and the public reader (runs.Open) agree on the
// on-disk layout via the shared pkg/compozy/runs/layout constants. If anyone
// changes a literal on only one side, this test fails before the change is
// merged.
func TestRunsLayoutAgreesAcrossWriterAndReader(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	const runID = "agree-test"

	artifacts := model.NewRunArtifacts(workspaceRoot, runID)
	if err := os.MkdirAll(artifacts.RunDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	meta := []byte(
		`{"version":1,"run_id":"agree-test","status":"completed","mode":"exec","created_at":"2026-04-13T12:00:00Z","updated_at":"2026-04-13T12:00:00Z"}`,
	)
	if err := os.WriteFile(artifacts.RunMetaPath, meta, 0o600); err != nil {
		t.Fatalf("write run meta: %v", err)
	}

	run, err := runs.Open(workspaceRoot, runID)
	if err != nil {
		t.Fatalf("runs.Open after model.NewRunArtifacts: %v", err)
	}
	if got := run.Summary().RunID; got != runID {
		t.Fatalf("Summary.RunID = %q, want %q", got, runID)
	}

	cases := []struct {
		name   string
		writer string
		reader string
	}{
		{"run meta", artifacts.RunMetaPath, layout.RunMetaPath(artifacts.RunDir)},
		{"events log", artifacts.EventsPath, layout.EventsLogPath(artifacts.RunDir)},
		{"result", artifacts.ResultPath, layout.ResultPath(artifacts.RunDir)},
		{"jobs dir", artifacts.JobsDir, layout.JobsDir(artifacts.RunDir)},
		{"turns dir", artifacts.TurnsDir, layout.TurnsDir(artifacts.RunDir)},
	}
	for _, tc := range cases {
		if tc.writer != tc.reader {
			t.Errorf("%s: writer=%q reader=%q (writer/reader disagree on layout)", tc.name, tc.writer, tc.reader)
		}
	}
}
