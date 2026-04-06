package runs

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestWatchWorkspaceEmitsRunCreated(t *testing.T) {
	workspaceRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspaceRoot, ".compozy", "runs"), 0o755); err != nil {
		t.Fatalf("mkdir runs dir: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventsCh, errsCh := WatchWorkspace(ctx, workspaceRoot)
	writeRunFixture(t, workspaceRoot, "run-created", runFixture{
		runJSON: map[string]any{
			"run_id":         "run-created",
			"mode":           "exec",
			"status":         "running",
			"workspace_root": workspaceRoot,
			"created_at":     time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC),
		},
	})

	event := awaitRunEvent(t, eventsCh, errsCh, time.Second)
	if event.Kind != RunEventCreated || event.RunID != "run-created" {
		t.Fatalf("RunEvent = %#v, want created for run-created", event)
	}
	if event.Summary == nil || event.Summary.Status != "running" {
		t.Fatalf("RunEvent summary = %#v, want running summary", event.Summary)
	}
}

func TestWatchWorkspaceEmitsStatusChanged(t *testing.T) {
	workspaceRoot := t.TempDir()
	writeRunFixture(t, workspaceRoot, "run-status", runFixture{
		runJSON: map[string]any{
			"run_id":         "run-status",
			"mode":           "exec",
			"status":         "running",
			"workspace_root": workspaceRoot,
			"created_at":     time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC),
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventsCh, errsCh := WatchWorkspace(ctx, workspaceRoot)
	writeRunFixture(t, workspaceRoot, "run-status", runFixture{
		runJSON: map[string]any{
			"run_id":         "run-status",
			"mode":           "exec",
			"status":         "failed",
			"workspace_root": workspaceRoot,
			"created_at":     time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC),
			"updated_at":     time.Date(2026, 4, 6, 12, 1, 0, 0, time.UTC),
		},
	},
	)

	event := awaitRunEvent(t, eventsCh, errsCh, time.Second)
	if event.Kind != RunEventStatusChanged || event.RunID != "run-status" {
		t.Fatalf("RunEvent = %#v, want status_changed for run-status", event)
	}
	if event.Summary == nil || event.Summary.Status != "failed" {
		t.Fatalf("RunEvent summary = %#v, want failed summary", event.Summary)
	}
}

func TestWatchWorkspaceEmitsRemoved(t *testing.T) {
	workspaceRoot := t.TempDir()
	runDir := writeRunFixture(t, workspaceRoot, "run-removed", runFixture{
		runJSON: map[string]any{
			"run_id":         "run-removed",
			"mode":           "exec",
			"status":         "running",
			"workspace_root": workspaceRoot,
			"created_at":     time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC),
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventsCh, errsCh := WatchWorkspace(ctx, workspaceRoot)
	if err := os.RemoveAll(runDir); err != nil {
		t.Fatalf("RemoveAll() error = %v", err)
	}

	event := awaitRunEvent(t, eventsCh, errsCh, time.Second)
	if event.Kind != RunEventRemoved || event.RunID != "run-removed" {
		t.Fatalf("RunEvent = %#v, want removed for run-removed", event)
	}
	if event.Summary != nil {
		t.Fatalf("RunEvent summary = %#v, want nil", event.Summary)
	}
}

func TestWatchWorkspaceCancelsCleanly(t *testing.T) {
	workspaceRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspaceRoot, ".compozy", "runs"), 0o755); err != nil {
		t.Fatalf("mkdir runs dir: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	eventsCh, errsCh := WatchWorkspace(ctx, workspaceRoot)
	cancelAndAwaitClose(t, cancel, eventsCh, errsCh)
}

func TestWatchWorkspaceDetectsExternalRunCreationWithinOneSecond(t *testing.T) {
	workspaceRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspaceRoot, ".compozy", "runs"), 0o755); err != nil {
		t.Fatalf("mkdir runs dir: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventsCh, errsCh := WatchWorkspace(ctx, workspaceRoot)

	runDir := filepath.Join(workspaceRoot, ".compozy", "runs", "external-run")
	runJSONPath := filepath.Join(runDir, "run.json")
	script := "mkdir -p '" + runDir + "' && cat > '" + runJSONPath + "' <<'EOF'\n" +
		"{\"run_id\":\"external-run\",\"mode\":\"exec\",\"status\":\"running\",\"workspace_root\":\"" + workspaceRoot + "\",\"created_at\":\"2026-04-06T12:00:00Z\"}\n" +
		"EOF\n"
	cmd := exec.CommandContext(ctx, "sh", "-c", script)
	if err := cmd.Run(); err != nil {
		t.Fatalf("external creator failed: %v", err)
	}

	event := awaitRunEvent(t, eventsCh, errsCh, time.Second)
	if event.Kind != RunEventCreated || event.RunID != "external-run" {
		t.Fatalf("RunEvent = %#v, want created for external-run", event)
	}
}
