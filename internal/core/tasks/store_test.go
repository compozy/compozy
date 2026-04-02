package tasks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/model"
)

func TestRefreshTaskMetaCreatesAndReadsSummary(t *testing.T) {
	t.Parallel()

	tasksDir := t.TempDir()
	writeTaskFile(t, tasksDir, "task_01.md", "pending")
	writeTaskFile(t, tasksDir, "task_02.md", "completed")
	writeTaskFile(t, tasksDir, "task_03.md", "done")
	writeTaskFile(t, tasksDir, "task_04.md", "finished")

	meta, err := RefreshTaskMeta(tasksDir)
	if err != nil {
		t.Fatalf("refresh task meta: %v", err)
	}
	if meta.Total != 4 || meta.Completed != 3 || meta.Pending != 1 {
		t.Fatalf("unexpected task counts: %#v", meta)
	}
	if meta.CreatedAt.IsZero() || meta.UpdatedAt.IsZero() {
		t.Fatalf("expected timestamps to be populated: %#v", meta)
	}

	readMeta, err := ReadTaskMeta(tasksDir)
	if err != nil {
		t.Fatalf("read task meta: %v", err)
	}
	if readMeta.Total != 4 || readMeta.Completed != 3 || readMeta.Pending != 1 {
		t.Fatalf("unexpected persisted task counts: %#v", readMeta)
	}
}

func TestRefreshTaskMetaPreservesCreatedAtAndUpdatesUpdatedAt(t *testing.T) {
	t.Parallel()

	tasksDir := t.TempDir()
	writeTaskFile(t, tasksDir, "task_01.md", "pending")

	createdAt := time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 3, 30, 12, 5, 0, 0, time.UTC)
	if err := WriteTaskMeta(tasksDir, model.TaskMeta{
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		Total:     1,
		Completed: 1,
		Pending:   0,
	}); err != nil {
		t.Fatalf("write existing meta: %v", err)
	}

	meta, err := RefreshTaskMeta(tasksDir)
	if err != nil {
		t.Fatalf("refresh task meta: %v", err)
	}
	if !meta.CreatedAt.Equal(createdAt) {
		t.Fatalf(
			"created_at changed\nwant: %s\ngot:  %s",
			createdAt.Format(time.RFC3339),
			meta.CreatedAt.Format(time.RFC3339),
		)
	}
	if !meta.UpdatedAt.After(updatedAt) {
		t.Fatalf("expected updated_at to move forward: %#v", meta)
	}
	if meta.Total != 1 || meta.Completed != 0 || meta.Pending != 1 {
		t.Fatalf("unexpected refreshed counts: %#v", meta)
	}
}

func TestRefreshTaskMetaRejectsLegacyTaskArtifacts(t *testing.T) {
	t.Parallel()

	tasksDir := t.TempDir()
	legacyTask := strings.Join([]string{
		"## status: pending",
		"<task_context><domain>backend</domain><type>feature</type><scope>small</scope><complexity>low</complexity></task_context>",
		"# Task 1",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(tasksDir, "task_01.md"), []byte(legacyTask), 0o600); err != nil {
		t.Fatalf("write legacy task: %v", err)
	}

	_, err := RefreshTaskMeta(tasksDir)
	if err == nil {
		t.Fatal("expected refresh to fail for legacy task metadata")
	}
	if !strings.Contains(err.Error(), "run `compozy migrate`") {
		t.Fatalf("expected migrate guidance, got %v", err)
	}
}

func TestMarkTaskCompletedRewritesStatusAndPreservesBody(t *testing.T) {
	t.Parallel()

	tasksDir := t.TempDir()
	taskPath := filepath.Join(tasksDir, "task_01.md")
	content := strings.Join([]string{
		"---",
		"status: pending",
		"domain: backend",
		"type: feature",
		"scope: small",
		"complexity: low",
		"custom_field: keep-me",
		"---",
		"",
		"# Task 01",
		"",
		"- [ ] subtask",
		"",
	}, "\n")
	if err := os.WriteFile(taskPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write task: %v", err)
	}

	if err := MarkTaskCompleted(tasksDir, "task_01.md"); err != nil {
		t.Fatalf("mark task completed: %v", err)
	}

	rewritten, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("read rewritten task: %v", err)
	}
	got := string(rewritten)
	if !strings.Contains(got, "status: completed") {
		t.Fatalf("expected rewritten task to include completed status, got:\n%s", got)
	}
	if !strings.Contains(got, "custom_field: keep-me") {
		t.Fatalf("expected rewritten task to preserve custom metadata, got:\n%s", got)
	}
	if !strings.Contains(got, "# Task 01\n\n- [ ] subtask\n") {
		t.Fatalf("expected rewritten task to preserve task body, got:\n%s", got)
	}
}

func TestMarkTaskCompletedCanonicalizesTerminalStatuses(t *testing.T) {
	t.Parallel()

	tasksDir := t.TempDir()
	taskPath := filepath.Join(tasksDir, "task_01.md")
	writeTaskFile(t, tasksDir, "task_01.md", "done")

	if err := MarkTaskCompleted(tasksDir, "task_01.md"); err != nil {
		t.Fatalf("mark task completed: %v", err)
	}

	rewritten, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("read rewritten task: %v", err)
	}
	if !strings.Contains(string(rewritten), "status: completed") {
		t.Fatalf("expected terminal task status to be canonicalized, got:\n%s", string(rewritten))
	}
}

func writeTaskFile(t *testing.T, tasksDir, name, status string) {
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

	if err := os.WriteFile(filepath.Join(tasksDir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
