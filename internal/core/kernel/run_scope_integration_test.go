package kernel_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/internal/core/agent"
	extensions "github.com/compozy/compozy/internal/core/extension"
	"github.com/compozy/compozy/internal/core/kernel"
	"github.com/compozy/compozy/internal/core/kernel/commands"
	"github.com/compozy/compozy/internal/core/model"
)

func TestRunStartHandleWithoutExecutableExtensionsPreservesArtifactsWithoutAuditLog(t *testing.T) {
	result, workspaceRoot := dispatchRunStartForTest(t, false)

	if result.Status != "succeeded" {
		t.Fatalf("result.Status = %q, want succeeded", result.Status)
	}
	if result.RunID == "" {
		t.Fatal("expected run id")
	}
	if result.ArtifactsDir == "" {
		t.Fatal("expected artifacts dir")
	}
	if _, err := os.Stat(result.ArtifactsDir); err != nil {
		t.Fatalf("expected artifacts dir to exist: %v", err)
	}
	if _, err := os.Stat(
		filepath.Join(result.ArtifactsDir, extensions.AuditLogFileName),
	); !errors.Is(
		err,
		os.ErrNotExist,
	) {
		t.Fatalf("expected disabled run to skip audit log, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspaceRoot, ".compozy", "tasks", "demo", "_meta.md")); err != nil {
		t.Fatalf("expected task metadata refresh artifact: %v", err)
	}
}

func TestRunStartHandleWithExecutableExtensionsProducesAuditLog(t *testing.T) {
	result, _ := dispatchRunStartForTest(t, true)

	if result.Status != "succeeded" {
		t.Fatalf("result.Status = %q, want succeeded", result.Status)
	}
	if _, err := os.Stat(filepath.Join(result.ArtifactsDir, extensions.AuditLogFileName)); err != nil {
		t.Fatalf("expected enabled run to create audit log: %v", err)
	}
}

func dispatchRunStartForTest(
	t *testing.T,
	enableExecutableExtensions bool,
) (commands.RunStartResult, string) {
	t.Helper()

	workspaceRoot := t.TempDir()
	t.Setenv("HOME", t.TempDir())

	tasksDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "demo")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("mkdir tasks dir: %v", err)
	}

	taskFile := filepath.Join(tasksDir, "task_01.md")
	taskContent := `---
status: pending
title: Demo task
type: backend
complexity: low
---

# Task 01: Demo task

Implement the run-scope bootstrap refactor.
`
	if err := os.WriteFile(taskFile, []byte(taskContent), 0o600); err != nil {
		t.Fatalf("write task file: %v", err)
	}

	dispatcher := kernel.BuildDefault(kernel.KernelDeps{
		AgentRegistry: agent.DefaultRegistry(),
		OpenRunScopeOptions: model.OpenRunScopeOptions{
			EnableExecutableExtensions: enableExecutableExtensions,
		},
	})
	if err := kernel.ValidateDefaultRegistry(dispatcher); err != nil {
		t.Fatalf("ValidateDefaultRegistry() error = %v", err)
	}

	result, err := kernel.Dispatch[commands.RunStartCommand, commands.RunStartResult](
		context.Background(),
		dispatcher,
		commands.RunStartCommand{
			Runtime: model.RuntimeConfig{
				WorkspaceRoot:          workspaceRoot,
				Name:                   "demo",
				TasksDir:               tasksDir,
				Mode:                   model.ExecutionModePRDTasks,
				IDE:                    model.IDECodex,
				DryRun:                 true,
				BatchSize:              1,
				Concurrent:             1,
				RetryBackoffMultiplier: 1.5,
			},
		},
	)
	if err != nil {
		t.Fatalf("Dispatch(run start) error = %v", err)
	}
	return result, workspaceRoot
}
