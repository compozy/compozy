package test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/compozy/looper"
	"github.com/compozy/looper/command"
)

func TestPrepareAndRunExposePublicAPI(t *testing.T) {
	t.Parallel()

	tasksDir := filepath.Join(t.TempDir(), "tasks", "demo")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("mkdir tasks dir: %v", err)
	}

	taskFile := filepath.Join(tasksDir, "task_1.md")
	taskContent := `## status: pending
<task_context>
  <domain>backend</domain>
  <type>feature</type>
  <scope>small</scope>
  <complexity>low</complexity>
</task_context>
`
	if err := os.WriteFile(taskFile, []byte(taskContent), 0o600); err != nil {
		t.Fatalf("write task file: %v", err)
	}

	cfg := looper.Config{
		Name:     "demo",
		TasksDir: tasksDir,
		Mode:     looper.ModePRDTasks,
		DryRun:   true,
	}

	prep, err := looper.Prepare(context.Background(), cfg)
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

	if err := looper.Run(context.Background(), cfg); err != nil {
		t.Fatalf("run: %v", err)
	}
}

func TestCommandNewUsesLooperRootCommand(t *testing.T) {
	t.Parallel()

	cmd := command.New()
	if cmd == nil {
		t.Fatal("expected command")
	}
	if cmd.Use != "looper" {
		t.Fatalf("expected use looper, got %q", cmd.Use)
	}
}
