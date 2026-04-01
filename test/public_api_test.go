package test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/compozy"
	"github.com/compozy/compozy/command"
)

func TestPrepareAndRunExposePublicAPI(t *testing.T) {
	t.Parallel()

	tasksDir := filepath.Join(t.TempDir(), ".compozy", "tasks", "demo")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("mkdir tasks dir: %v", err)
	}

	taskFile := filepath.Join(tasksDir, "task_1.md")
	taskContent := `---
status: pending
domain: backend
type: feature
scope: small
complexity: low
---

# Task 1: Demo
`
	if err := os.WriteFile(taskFile, []byte(taskContent), 0o600); err != nil {
		t.Fatalf("write task file: %v", err)
	}

	cfg := compozy.Config{
		Name:     "demo",
		TasksDir: tasksDir,
		Mode:     compozy.ModePRDTasks,
		DryRun:   true,
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

func TestCommandNewUsesCompozyRootCommand(t *testing.T) {
	t.Parallel()

	cmd := command.New()
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
