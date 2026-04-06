package runs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestListReturnsAllRunsSortedByStartedAtDescending(t *testing.T) {
	workspaceRoot := t.TempDir()
	baseTime := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)

	writeRunFixture(t, workspaceRoot, "run-early", runFixture{
		runJSON: map[string]any{
			"run_id":         "run-early",
			"mode":           "prd-tasks",
			"ide":            "codex",
			"model":          "gpt-5.4",
			"artifacts_dir":  filepath.Join(workspaceRoot, ".compozy", "runs", "run-early"),
			"workspace_root": workspaceRoot,
			"created_at":     baseTime,
		},
		resultJSON: map[string]any{"status": "failed"},
	})
	writeRunFixture(t, workspaceRoot, "run-late", runFixture{
		runJSON: map[string]any{
			"run_id":         "run-late",
			"mode":           "exec",
			"status":         "succeeded",
			"ide":            "codex",
			"model":          "gpt-5.4",
			"workspace_root": workspaceRoot,
			"created_at":     baseTime.Add(2 * time.Hour),
			"updated_at":     baseTime.Add(2*time.Hour + time.Minute),
		},
	})
	writeRunFixture(t, workspaceRoot, "run-middle", runFixture{
		runJSON: map[string]any{
			"run_id":         "run-middle",
			"mode":           "pr-review",
			"workspace_root": workspaceRoot,
			"created_at":     baseTime.Add(time.Hour),
		},
	})

	got, err := List(workspaceRoot, ListOptions{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("List() returned %d runs, want 3", len(got))
	}
	if got[0].RunID != "run-late" || got[1].RunID != "run-middle" || got[2].RunID != "run-early" {
		t.Fatalf(
			"List() order = [%s %s %s], want [run-late run-middle run-early]",
			got[0].RunID,
			got[1].RunID,
			got[2].RunID,
		)
	}
	if got[0].Status != "completed" {
		t.Fatalf("List() normalized status = %q, want completed", got[0].Status)
	}
}

func TestListFiltersByStatus(t *testing.T) {
	workspaceRoot := t.TempDir()
	now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)

	writeRunFixture(t, workspaceRoot, "run-failed", runFixture{
		runJSON: map[string]any{
			"run_id":         "run-failed",
			"mode":           "prd-tasks",
			"workspace_root": workspaceRoot,
			"created_at":     now,
		},
		resultJSON: map[string]any{"status": "failed"},
	})
	writeRunFixture(t, workspaceRoot, "run-completed", runFixture{
		runJSON: map[string]any{
			"run_id":         "run-completed",
			"mode":           "prd-tasks",
			"status":         "succeeded",
			"workspace_root": workspaceRoot,
			"created_at":     now.Add(time.Minute),
		},
	})

	got, err := List(workspaceRoot, ListOptions{Status: []string{"failed"}})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 1 || got[0].RunID != "run-failed" {
		t.Fatalf("List() = %#v, want only run-failed", got)
	}
}

func TestListFiltersByMode(t *testing.T) {
	workspaceRoot := t.TempDir()
	now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)

	writeRunFixture(t, workspaceRoot, "run-exec", runFixture{
		runJSON: map[string]any{
			"run_id":         "run-exec",
			"mode":           "exec",
			"status":         "running",
			"workspace_root": workspaceRoot,
			"created_at":     now,
		},
	})
	writeRunFixture(t, workspaceRoot, "run-batch", runFixture{
		runJSON: map[string]any{
			"run_id":         "run-batch",
			"mode":           "prd-tasks",
			"status":         "running",
			"workspace_root": workspaceRoot,
			"created_at":     now.Add(time.Minute),
		},
	})

	got, err := List(workspaceRoot, ListOptions{Mode: []string{"exec"}})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 1 || got[0].RunID != "run-exec" {
		t.Fatalf("List() = %#v, want only run-exec", got)
	}
}

func TestListAppliesTimeBounds(t *testing.T) {
	workspaceRoot := t.TempDir()
	baseTime := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)

	for index := range 3 {
		runID := []string{"run-1", "run-2", "run-3"}[index]
		writeRunFixture(t, workspaceRoot, runID, runFixture{
			runJSON: map[string]any{
				"run_id":         runID,
				"mode":           "prd-tasks",
				"workspace_root": workspaceRoot,
				"created_at":     baseTime.Add(time.Duration(index) * time.Hour),
			},
		})
	}

	got, err := List(workspaceRoot, ListOptions{
		Since: baseTime.Add(30 * time.Minute),
		Until: baseTime.Add(90 * time.Minute),
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 1 || got[0].RunID != "run-2" {
		t.Fatalf("List() = %#v, want only run-2", got)
	}
}

func TestListAppliesLimit(t *testing.T) {
	workspaceRoot := t.TempDir()
	baseTime := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)

	for index := range 3 {
		runID := []string{"run-1", "run-2", "run-3"}[index]
		writeRunFixture(t, workspaceRoot, runID, runFixture{
			runJSON: map[string]any{
				"run_id":         runID,
				"mode":           "prd-tasks",
				"workspace_root": workspaceRoot,
				"created_at":     baseTime.Add(time.Duration(index) * time.Hour),
			},
		})
	}

	got, err := List(workspaceRoot, ListOptions{Limit: 2})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("List() returned %d runs, want 2", len(got))
	}
	if got[0].RunID != "run-3" || got[1].RunID != "run-2" {
		t.Fatalf("List() limit order = [%s %s], want [run-3 run-2]", got[0].RunID, got[1].RunID)
	}
}

func TestListSkipsRunsMissingMetadataWithWarning(t *testing.T) {
	workspaceRoot := t.TempDir()
	buf := captureWarnLogs(t)

	if err := os.MkdirAll(filepath.Join(workspaceRoot, ".compozy", "runs", "missing-run"), 0o755); err != nil {
		t.Fatalf("mkdir missing run dir: %v", err)
	}
	writeRunFixture(t, workspaceRoot, "valid-run", runFixture{
		runJSON: map[string]any{
			"run_id":         "valid-run",
			"mode":           "exec",
			"status":         "running",
			"workspace_root": workspaceRoot,
			"created_at":     time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC),
		},
	})

	got, err := List(workspaceRoot, ListOptions{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 1 || got[0].RunID != "valid-run" {
		t.Fatalf("List() = %#v, want only valid-run", got)
	}
	if logOutput := buf.String(); !strings.Contains(logOutput, "skipping run without run.json") ||
		!strings.Contains(logOutput, "missing-run") {
		t.Fatalf("warning log = %q, want missing-run warning", logOutput)
	}
}
