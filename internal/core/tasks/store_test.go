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

func TestRefreshTaskMetaRejectsV1TaskArtifacts(t *testing.T) {
	t.Parallel()

	tasksDir := t.TempDir()
	v1Task := strings.Join([]string{
		"---",
		"status: pending",
		"domain: backend",
		"type: backend",
		"scope: small",
		"complexity: low",
		"---",
		"",
		"# Task 1: Example",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(tasksDir, "task_01.md"), []byte(v1Task), 0o600); err != nil {
		t.Fatalf("write v1 task: %v", err)
	}

	_, err := RefreshTaskMeta(tasksDir)
	if err == nil {
		t.Fatal("expected refresh to fail for v1 task metadata")
	}
	if !strings.Contains(err.Error(), "run `compozy migrate`") {
		t.Fatalf("expected migrate guidance, got %v", err)
	}
}

func TestSnapshotTaskMetaDoesNotCreateMetadataFile(t *testing.T) {
	t.Parallel()

	tasksDir := t.TempDir()
	writeTaskFile(t, tasksDir, "task_01.md", "pending")
	writeTaskFile(t, tasksDir, "task_02.md", "completed")

	meta, err := SnapshotTaskMeta(tasksDir)
	if err != nil {
		t.Fatalf("snapshot task meta: %v", err)
	}
	if meta.Total != 2 || meta.Completed != 1 || meta.Pending != 1 {
		t.Fatalf("unexpected snapshot counts: %#v", meta)
	}
	if _, err := os.Stat(MetaPath(tasksDir)); !os.IsNotExist(err) {
		t.Fatalf("expected snapshot to avoid writing _meta.md, got %v", err)
	}
}

func TestMarkTaskCompletedRewritesStatusAndPreservesBody(t *testing.T) {
	t.Parallel()

	tasksDir := t.TempDir()
	taskPath := filepath.Join(tasksDir, "task_01.md")
	content := strings.Join([]string{
		"---",
		"status: pending",
		"title: Task 01",
		"type: backend",
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

func TestCompleteNonTerminalTasksRewritesWorkflowAndRefreshesMeta(t *testing.T) {
	t.Parallel()

	tasksDir := t.TempDir()
	writeTaskFile(t, tasksDir, "task_01.md", "pending")
	writeTaskFile(t, tasksDir, "task_02.md", "in_progress")
	writeTaskFile(t, tasksDir, "task_03.md", "blocked")
	writeTaskFile(t, tasksDir, "task_04.md", "completed")
	writeTaskFile(t, tasksDir, "task_05.md", "done")
	writeTaskFile(t, tasksDir, "task_06.md", "finished")

	completed, err := CompleteNonTerminalTasks(tasksDir)
	if err != nil {
		t.Fatalf("complete non-terminal tasks: %v", err)
	}
	if completed != 3 {
		t.Fatalf("completed = %d, want 3", completed)
	}

	for _, name := range []string{
		"task_01.md",
		"task_02.md",
		"task_03.md",
		"task_04.md",
	} {
		body, err := os.ReadFile(filepath.Join(tasksDir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if !strings.Contains(string(body), "status: completed") {
			t.Fatalf("expected %s to be completed, got:\n%s", name, string(body))
		}
	}
	for name, status := range map[string]string{
		"task_05.md": "status: done",
		"task_06.md": "status: finished",
	} {
		body, err := os.ReadFile(filepath.Join(tasksDir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if !strings.Contains(string(body), status) {
			t.Fatalf("expected %s to remain terminal with %q, got:\n%s", name, status, string(body))
		}
	}

	meta, err := ReadTaskMeta(tasksDir)
	if err != nil {
		t.Fatalf("read task meta: %v", err)
	}
	if meta.Total != 6 || meta.Completed != 6 || meta.Pending != 0 {
		t.Fatalf("unexpected task meta after workflow completion: %#v", meta)
	}
}

func TestCountTasksRecursesAndCountsNested(t *testing.T) {
	t.Parallel()

	tasksDir := t.TempDir()
	files := map[string]string{
		"task_01.md":                  pendingTaskBody("Root 01"),
		"features/auth/task_01.md":    pendingTaskBody("Auth 01"),
		"features/auth/task_02.md":    completedTaskBody("Auth 02"),
		"features/payment/task_01.md": pendingTaskBody("Payment 01"),
		".cache/task_01.md":           pendingTaskBody("Hidden"),
		"reviews-001/task_01.md":      pendingTaskBody("Reviews"),
	}
	writeNestedFiles(t, tasksDir, files)

	total, completed, err := countTasks(tasksDir)
	if err != nil {
		t.Fatalf("countTasks: %v", err)
	}
	if total != 4 {
		t.Fatalf("total = %d, want 4", total)
	}
	if completed != 1 {
		t.Fatalf("completed = %d, want 1", completed)
	}
}

func TestMarkTaskCompletedAcceptsRelativeSubpath(t *testing.T) {
	t.Parallel()

	tasksDir := t.TempDir()
	rel := "features/auth/task_01.md"
	writeNestedFiles(t, tasksDir, map[string]string{
		rel: pendingTaskBody("Auth 01"),
	})

	if err := MarkTaskCompleted(tasksDir, rel); err != nil {
		t.Fatalf("mark task completed: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(tasksDir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	if !strings.Contains(string(body), "status: completed") {
		t.Fatalf("expected nested task to be completed, got:\n%s", string(body))
	}
}

func TestMarkTaskCompletedNormalizesBackslashes(t *testing.T) {
	t.Parallel()

	tasksDir := t.TempDir()
	writeNestedFiles(t, tasksDir, map[string]string{
		"features/auth/task_01.md": pendingTaskBody("Auth 01"),
	})

	if err := MarkTaskCompleted(tasksDir, `features\auth\task_01.md`); err != nil {
		t.Fatalf("mark task completed with backslash input: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(tasksDir, "features", "auth", "task_01.md"))
	if err != nil {
		t.Fatalf("read nested task: %v", err)
	}
	if !strings.Contains(string(body), "status: completed") {
		t.Fatalf("expected backslash input to resolve to nested task, got:\n%s", string(body))
	}
}

func TestResolveTaskNameRejectsEmpty(t *testing.T) {
	t.Parallel()

	cases := []string{"", "   ", "\t\n"}
	for _, input := range cases {
		input := input
		t.Run(strings.TrimSpace(input)+"|raw="+input, func(t *testing.T) {
			t.Parallel()
			_, err := resolveTaskName(input)
			if err == nil {
				t.Fatalf("expected error for empty input %q", input)
			}
			if !strings.Contains(err.Error(), "empty input") {
				t.Fatalf("error should mention empty input, got: %v", err)
			}
		})
	}
}

func TestResolveTaskNameRejectsLeadingSlash(t *testing.T) {
	t.Parallel()

	cases := []string{"/task_01.md", "/features/auth/task_01.md", "//features/task_01.md"}
	for _, input := range cases {
		input := input
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			_, err := resolveTaskName(input)
			if err == nil {
				t.Fatalf("expected error for leading-slash input %q", input)
			}
			if !strings.Contains(err.Error(), input) {
				t.Fatalf("error should reference offending input %q, got: %v", input, err)
			}
		})
	}
}

func TestResolveTaskNameRejectsParentSegment(t *testing.T) {
	t.Parallel()

	cases := []string{
		"../task_01.md",
		"features/../task_01.md",
		"features/auth/../task_01.md",
		"..",
	}
	for _, input := range cases {
		input := input
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			_, err := resolveTaskName(input)
			if err == nil {
				t.Fatalf("expected error for parent-segment input %q", input)
			}
			if !strings.Contains(err.Error(), "..") {
				t.Fatalf("error should reference \"..\" segment, got: %v", err)
			}
			if !strings.Contains(err.Error(), input) {
				t.Fatalf("error should reference offending input %q, got: %v", input, err)
			}
		})
	}
}

func TestResolveTaskNameRejectsNonTaskBasename(t *testing.T) {
	t.Parallel()

	cases := []string{
		"features/auth/notes.md",
		"task_bad.md",
		"task_01.txt",
		"features/auth/task_01",
	}
	for _, input := range cases {
		input := input
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			_, err := resolveTaskName(input)
			if err == nil {
				t.Fatalf("expected error for non-task basename %q", input)
			}
			if !strings.Contains(err.Error(), filepath.Base(input)) {
				t.Fatalf("error should reference invalid basename %q, got: %v", filepath.Base(input), err)
			}
		})
	}
}

func TestResolveTaskNameAcceptsValidRelpath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  string
	}{
		{"task_01.md", "task_01.md"},
		{"features/auth/task_01.md", "features/auth/task_01.md"},
		{"  features/auth/task_02.md  ", "features/auth/task_02.md"},
		{`features\auth\task_03.md`, "features/auth/task_03.md"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			got, err := resolveTaskName(tc.input)
			if err != nil {
				t.Fatalf("resolveTaskName(%q): %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("resolveTaskName(%q) = %q; want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestCompleteNonTerminalTasksHandlesNestedFixture(t *testing.T) {
	t.Parallel()

	tasksDir := t.TempDir()
	pending := []string{
		"task_01.md",
		"features/auth/task_01.md",
		"features/payment/task_01.md",
	}
	files := map[string]string{
		"features/auth/task_02.md": completedTaskBody("Auth 02"),
	}
	for _, rel := range pending {
		files[rel] = pendingTaskBody(rel)
	}
	writeNestedFiles(t, tasksDir, files)

	completed, err := CompleteNonTerminalTasks(tasksDir)
	if err != nil {
		t.Fatalf("CompleteNonTerminalTasks: %v", err)
	}
	if completed != len(pending) {
		t.Fatalf("completed = %d, want %d", completed, len(pending))
	}

	for rel := range files {
		body, err := os.ReadFile(filepath.Join(tasksDir, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if !strings.Contains(string(body), "status: completed") {
			t.Fatalf("expected %s to be completed, got:\n%s", rel, string(body))
		}
	}

	meta, err := ReadTaskMeta(tasksDir)
	if err != nil {
		t.Fatalf("read task meta: %v", err)
	}
	if meta.Total != len(files) || meta.Completed != len(files) || meta.Pending != 0 {
		t.Fatalf("unexpected task meta after nested completion: %#v", meta)
	}
}

func writeTaskFile(t *testing.T, tasksDir, name, status string) {
	t.Helper()

	content := strings.Join([]string{
		"---",
		"status: " + status,
		"title: " + name,
		"type: backend",
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
