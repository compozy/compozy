package devcycle

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	looppkg "github.com/compozy/agh/internal/loop"
)

func TestImportMarkdownTasksShouldLoadCompozyTaskManifest(t *testing.T) {
	t.Parallel()

	t.Run("Should import pending tasks in topological numeric order with hydrated bodies", func(t *testing.T) {
		t.Parallel()

		tasksDir := t.TempDir()
		writeImportTasksManifest(t, tasksDir, compozyTaskManifestVersion, []string{
			"    - from: task_02",
			"      to: task_03",
			"    - from: task_01",
			"      to: task_03",
		})
		writeImportTaskFile(t, tasksDir, "task_01.md", "pending", "First task", "# First\n\nDo one.")
		writeImportTaskFile(t, tasksDir, "task_02.md", "completed", "Second task", "# Second\n\nAlready done.")
		writeImportTaskFile(t, tasksDir, "task_03.md", "pending", "Third task", "# Third\n\nDo three.")

		result, err := importMarkdownTasks(filepath.Join(tasksDir, "task_*.md"))
		if err != nil {
			t.Fatalf("importMarkdownTasks() error = %v", err)
		}
		if len(result.Tasks) != 2 {
			t.Fatalf("len(tasks) = %d, want 2 pending tasks", len(result.Tasks))
		}
		if got, want := result.Tasks[0].ID, "task_01"; got != want {
			t.Fatalf("tasks[0].id = %q, want %q", got, want)
		}
		if result.Tasks[0].Blocks == nil || len(result.Tasks[0].Blocks) != 0 {
			t.Fatalf("tasks[0].blocks = %#v, want empty JSON array", result.Tasks[0].Blocks)
		}
		third := result.Tasks[1]
		if third.ID != "task_03" || third.Number != 3 || third.Title != "Third task" {
			t.Fatalf("third task = %#v, want task_03 metadata", third)
		}
		if !strings.Contains(third.Body, "Do three.") {
			t.Fatalf("third body = %q, want hydrated markdown body", third.Body)
		}
		if got, want := third.BodyRef, looppkg.OutputRefForPayload([]byte(third.Body)); got != want {
			t.Fatalf("third body_ref = %q, want %q", got, want)
		}
		if got, want := strings.Join(third.Blocks, ","), "task_01,task_02"; got != want {
			t.Fatalf("third blocks = %q, want %q", got, want)
		}
	})

	t.Run("Should return no work for an empty pending set", func(t *testing.T) {
		t.Parallel()

		tasksDir := t.TempDir()
		writeImportTasksManifest(t, tasksDir, compozyTaskManifestVersion, nil)
		writeImportTaskFile(t, tasksDir, "task_01.md", "done", "First task", "# First\n")
		writeImportTaskFile(t, tasksDir, "task_02.md", "finished", "Second task", "# Second\n")
		writeImportTaskFile(t, tasksDir, "task_03.md", "completed", "Third task", "# Third\n")

		result, err := importMarkdownTasks(filepath.Join(tasksDir, "task_*.md"))
		if err != nil {
			t.Fatalf("importMarkdownTasks() error = %v", err)
		}
		if len(result.Tasks) != 0 {
			t.Fatalf("len(tasks) = %d, want 0 pending tasks", len(result.Tasks))
		}
	})

	t.Run("Should reject unsupported manifest schema versions", func(t *testing.T) {
		t.Parallel()

		tasksDir := t.TempDir()
		writeImportTasksManifest(t, tasksDir, "compozy.tasks/v1", nil)
		writeImportTaskFile(t, tasksDir, "task_01.md", "pending", "First task", "# First\n")
		writeImportTaskFile(t, tasksDir, "task_02.md", "pending", "Second task", "# Second\n")
		writeImportTaskFile(t, tasksDir, "task_03.md", "pending", "Third task", "# Third\n")

		_, err := importMarkdownTasks(filepath.Join(tasksDir, "task_*.md"))
		if !errors.Is(err, looppkg.ErrValidation) ||
			!strings.Contains(err.Error(), `schema_version must be "compozy.tasks/v2"`) {
			t.Fatalf("importMarkdownTasks() error = %v, want schema_version validation", err)
		}
	})

	t.Run("Should reject invalid glob patterns", func(t *testing.T) {
		t.Parallel()

		_, err := importMarkdownTasks(filepath.Join(t.TempDir(), "["))
		if !errors.Is(err, looppkg.ErrValidation) || !strings.Contains(err.Error(), "invalid md_tasks glob") {
			t.Fatalf("importMarkdownTasks() error = %v, want invalid glob validation", err)
		}
	})

	t.Run("Should reject manifest files when the wildcard matches no task files", func(t *testing.T) {
		t.Parallel()

		tasksDir := t.TempDir()
		writeImportTasksManifest(t, tasksDir, compozyTaskManifestVersion, nil)
		writeImportTaskFile(t, tasksDir, "task_01.md", "pending", "First task", "# First\n")
		writeImportTaskFile(t, tasksDir, "task_02.md", "finished", "Second task", "# Second\n")
		writeImportTaskFile(t, tasksDir, "task_03.md", "completed", "Third task", "# Third\n")

		_, err := importMarkdownTasks(filepath.Join(tasksDir, "missing_*.md"))
		if !errors.Is(err, looppkg.ErrValidation) ||
			!strings.Contains(err.Error(), `md_tasks manifest file "task_01.md" is outside glob`) {
			t.Fatalf("importMarkdownTasks() error = %v, want empty-glob membership validation", err)
		}
	})

	t.Run("Should reject manifest node paths that traverse outside the tasks directory", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		tasksDir := filepath.Join(root, "tasks")
		outsideDir := filepath.Join(root, "outside")
		writeImportTasksFile(t, tasksDir, compozyTaskManifestFileName, strings.Join([]string{
			"---",
			`schema_version: "compozy.tasks/v2"`,
			"workflow: demo",
			"graph:",
			"  nodes:",
			"    - id: task_01",
			"      file: ../outside/task_01.md",
			"  edges: []",
			"---",
			"",
		}, "\n"))
		writeImportTaskFile(t, outsideDir, "task_01.md", "pending", "Outside task", "# Outside\n")

		_, err := importMarkdownTasks(filepath.Join(tasksDir, "missing_*.md"))
		if !errors.Is(err, looppkg.ErrValidation) ||
			!strings.Contains(err.Error(), `node file "../outside/task_01.md" must be a base name`) {
			t.Fatalf("importMarkdownTasks() error = %v, want traversal validation", err)
		}
	})

	t.Run("Should reject manifest task symlinks that resolve outside the tasks directory", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		tasksDir := filepath.Join(root, "tasks")
		outsideDir := filepath.Join(root, "outside")
		writeImportTasksFile(t, tasksDir, compozyTaskManifestFileName, strings.Join([]string{
			"---",
			`schema_version: "compozy.tasks/v2"`,
			"workflow: demo",
			"graph:",
			"  nodes:",
			"    - id: task_01",
			"      file: task_01.md",
			"  edges: []",
			"---",
			"",
		}, "\n"))
		outsidePath := filepath.Join(outsideDir, "task_01.md")
		writeImportTaskFile(t, outsideDir, "task_01.md", "pending", "Outside task", "# Outside\n")
		if err := os.Symlink(outsidePath, filepath.Join(tasksDir, "task_01.md")); err != nil {
			t.Fatalf("Symlink(%s) error = %v", outsidePath, err)
		}

		_, err := importMarkdownTasks(filepath.Join(tasksDir, "task_*.md"))
		if !errors.Is(err, looppkg.ErrValidation) ||
			!strings.Contains(err.Error(), `node file "task_01.md" resolves outside the tasks directory`) {
			t.Fatalf("importMarkdownTasks() error = %v, want symlink containment validation", err)
		}
	})

	t.Run("Should reject manifest symlinks that resolve outside the tasks directory", func(t *testing.T) {
		t.Parallel()

		tasksDir := t.TempDir()
		outsideDir := t.TempDir()
		writeImportTasksManifest(t, outsideDir, compozyTaskManifestVersion, nil)
		writeImportTaskFile(t, tasksDir, "task_01.md", "pending", "First task", "# First\n")
		writeImportTaskFile(t, tasksDir, "task_02.md", "pending", "Second task", "# Second\n")
		writeImportTaskFile(t, tasksDir, "task_03.md", "pending", "Third task", "# Third\n")
		outsideManifest := filepath.Join(outsideDir, compozyTaskManifestFileName)
		if err := os.Symlink(outsideManifest, filepath.Join(tasksDir, compozyTaskManifestFileName)); err != nil {
			t.Skipf("create manifest symlink: %v", err)
		}

		_, err := importMarkdownTasks(filepath.Join(tasksDir, "task_*.md"))
		if !errors.Is(err, looppkg.ErrValidation) ||
			!strings.Contains(err.Error(), "manifest") ||
			!strings.Contains(err.Error(), "resolves outside the tasks directory") {
			t.Fatalf("importMarkdownTasks() error = %v, want manifest symlink containment validation", err)
		}
	})

	t.Run("Should reject manifest files outside a non-empty glob", func(t *testing.T) {
		t.Parallel()

		tasksDir := t.TempDir()
		writeImportTasksManifest(t, tasksDir, compozyTaskManifestVersion, nil)
		writeImportTaskFile(t, tasksDir, "task_01.md", "pending", "First task", "# First\n")
		writeImportTaskFile(t, tasksDir, "task_02.md", "pending", "Second task", "# Second\n")
		writeImportTaskFile(t, tasksDir, "task_03.md", "pending", "Third task", "# Third\n")

		_, err := importMarkdownTasks(filepath.Join(tasksDir, "task_01.md"))
		if !errors.Is(err, looppkg.ErrValidation) ||
			!strings.Contains(err.Error(), `md_tasks manifest file "task_02.md" is outside glob`) {
			t.Fatalf("importMarkdownTasks() error = %v, want outside glob validation", err)
		}
	})

	t.Run("Should reject malformed manifest frontmatter", func(t *testing.T) {
		t.Parallel()

		tasksDir := t.TempDir()
		writeImportTasksFile(t, tasksDir, compozyTaskManifestFileName, strings.Join([]string{
			"---",
			"schema_version: [",
			"---",
			"",
		}, "\n"))

		_, err := importMarkdownTasks(filepath.Join(tasksDir, "task_*.md"))
		if !errors.Is(err, looppkg.ErrValidation) || !strings.Contains(err.Error(), "decode md_tasks manifest") {
			t.Fatalf("importMarkdownTasks() error = %v, want manifest decode validation", err)
		}
	})

	t.Run("Should reject dependencies drift in task frontmatter", func(t *testing.T) {
		t.Parallel()

		tasksDir := t.TempDir()
		writeImportTasksManifest(t, tasksDir, compozyTaskManifestVersion, nil)
		writeImportTaskFileWithFrontmatter(t, tasksDir, "task_01.md", []string{
			"status: pending",
			"title: First task",
			"dependencies: [task_02]",
		}, "# First\n")
		writeImportTaskFile(t, tasksDir, "task_02.md", "pending", "Second task", "# Second\n")
		writeImportTaskFile(t, tasksDir, "task_03.md", "pending", "Third task", "# Third\n")

		_, err := importMarkdownTasks(filepath.Join(tasksDir, "task_*.md"))
		if !errors.Is(err, looppkg.ErrValidation) ||
			!strings.Contains(err.Error(), "dependencies must live in _tasks.md") {
			t.Fatalf("importMarkdownTasks() error = %v, want dependencies validation", err)
		}
	})

	t.Run("Should reject task files missing status", func(t *testing.T) {
		t.Parallel()

		tasksDir := t.TempDir()
		writeImportTasksManifest(t, tasksDir, compozyTaskManifestVersion, nil)
		writeImportTaskFileWithFrontmatter(t, tasksDir, "task_01.md", []string{"title: First task"}, "# First\n")
		writeImportTaskFile(t, tasksDir, "task_02.md", "pending", "Second task", "# Second\n")
		writeImportTaskFile(t, tasksDir, "task_03.md", "pending", "Third task", "# Third\n")

		_, err := importMarkdownTasks(filepath.Join(tasksDir, "task_*.md"))
		if !errors.Is(err, looppkg.ErrValidation) || !strings.Contains(err.Error(), "missing status") {
			t.Fatalf("importMarkdownTasks() error = %v, want missing status validation", err)
		}
	})

	t.Run("Should reject malformed task frontmatter", func(t *testing.T) {
		t.Parallel()

		tasksDir := t.TempDir()
		writeImportTasksManifest(t, tasksDir, compozyTaskManifestVersion, nil)
		writeImportTasksFile(t, tasksDir, "task_01.md", strings.Join([]string{
			"---",
			"status: [",
			"---",
			"",
			"# First",
			"",
		}, "\n"))
		writeImportTaskFile(t, tasksDir, "task_02.md", "pending", "Second task", "# Second\n")
		writeImportTaskFile(t, tasksDir, "task_03.md", "pending", "Third task", "# Third\n")

		_, err := importMarkdownTasks(filepath.Join(tasksDir, "task_*.md"))
		if !errors.Is(err, looppkg.ErrValidation) || !strings.Contains(err.Error(), "parse md_tasks task file") {
			t.Fatalf("importMarkdownTasks() error = %v, want task parse validation", err)
		}
	})

	t.Run("Should reject manifest node file and id drift", func(t *testing.T) {
		t.Parallel()

		tasksDir := t.TempDir()
		writeImportTasksFile(t, tasksDir, compozyTaskManifestFileName, strings.Join([]string{
			"---",
			`schema_version: "compozy.tasks/v2"`,
			"workflow: demo",
			"graph:",
			"  nodes:",
			"    - id: task_01",
			"      file: task_02.md",
			"  edges: []",
			"---",
			"",
		}, "\n"))
		writeImportTaskFile(t, tasksDir, "task_02.md", "pending", "Second task", "# Second\n")

		_, err := importMarkdownTasks(filepath.Join(tasksDir, "task_*.md"))
		if !errors.Is(err, looppkg.ErrValidation) ||
			!strings.Contains(err.Error(), `node file "task_02.md" must match id "task_01"`) {
			t.Fatalf("importMarkdownTasks() error = %v, want id/file validation", err)
		}
	})

	t.Run("Should reject invalid graph edges", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name    string
			edges   []string
			wantErr string
		}{
			{
				name: "self-edge",
				edges: []string{
					"    - from: task_03",
					"      to: task_03",
				},
				wantErr: "self-edge",
			},
			{
				name: "unknown source",
				edges: []string{
					"    - from: task_99",
					"      to: task_03",
				},
				wantErr: `edge source "task_99" is not a graph node`,
			},
			{
				name: "unknown target",
				edges: []string{
					"    - from: task_01",
					"      to: task_99",
				},
				wantErr: `edge target "task_99" is not a graph node`,
			},
			{
				name: "duplicate edge",
				edges: []string{
					"    - from: task_01",
					"      to: task_03",
					"    - from: task_01",
					"      to: task_03",
				},
				wantErr: `duplicate md_tasks edge "task_01" -> "task_03"`,
			},
		}

		for _, tc := range cases {
			t.Run("Should reject "+tc.name, func(t *testing.T) {
				t.Parallel()

				tasksDir := t.TempDir()
				writeImportTasksManifest(t, tasksDir, compozyTaskManifestVersion, tc.edges)
				writeImportTaskFile(t, tasksDir, "task_01.md", "pending", "First task", "# First\n")
				writeImportTaskFile(t, tasksDir, "task_02.md", "pending", "Second task", "# Second\n")
				writeImportTaskFile(t, tasksDir, "task_03.md", "pending", "Third task", "# Third\n")

				_, err := importMarkdownTasks(filepath.Join(tasksDir, "task_*.md"))
				if !errors.Is(err, looppkg.ErrValidation) || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("importMarkdownTasks() error = %v, want %q validation", err, tc.wantErr)
				}
			})
		}
	})

	t.Run("Should reject cyclic task graphs with deterministic remaining nodes", func(t *testing.T) {
		t.Parallel()

		tasksDir := t.TempDir()
		writeImportTasksManifest(t, tasksDir, compozyTaskManifestVersion, []string{
			"    - from: task_01",
			"      to: task_02",
			"    - from: task_02",
			"      to: task_01",
		})
		writeImportTaskFile(t, tasksDir, "task_01.md", "pending", "First task", "# First\n")
		writeImportTaskFile(t, tasksDir, "task_02.md", "pending", "Second task", "# Second\n")
		writeImportTaskFile(t, tasksDir, "task_03.md", "pending", "Third task", "# Third\n")

		_, err := importMarkdownTasks(filepath.Join(tasksDir, "task_*.md"))
		if !errors.Is(err, looppkg.ErrValidation) ||
			!strings.Contains(err.Error(), "md_tasks graph contains a cycle involving: task_01, task_02") {
			t.Fatalf("importMarkdownTasks() error = %v, want cycle validation", err)
		}
	})
}

func TestImportTasksToolShouldReturnStructuredPayload(t *testing.T) {
	t.Parallel()

	t.Run("Should return tasks and count with md_tasks item shape", func(t *testing.T) {
		t.Parallel()

		tasksDir := t.TempDir()
		writeImportTasksManifest(t, tasksDir, compozyTaskManifestVersion, nil)
		writeImportTaskFile(t, tasksDir, "task_01.md", "pending", "First task", "# First\n")
		writeImportTaskFile(t, tasksDir, "task_02.md", "done", "Second task", "# Second\n")
		writeImportTaskFile(t, tasksDir, "task_03.md", "completed", "Third task", "# Third\n")

		output, err := importTasks(importTasksInput{Pattern: filepath.Join(tasksDir, "task_*.md")})
		if err != nil {
			t.Fatalf("importTasks() error = %v", err)
		}
		if output.Count != 1 || len(output.Tasks) != 1 {
			t.Fatalf("importTasks() output = %#v, want one pending task and count 1", output)
		}
		encoded, err := json.Marshal(output)
		if err != nil {
			t.Fatalf("Marshal(importTasks output) error = %v", err)
		}
		task := output.Tasks[0]
		want := fmt.Sprintf(
			`{"tasks":[{"id":"task_01","number":1,"title":"First task",`+
				`"path":%q,"body":%q,"body_ref":%q,"blocks":[]}],"count":1}`,
			task.Path,
			task.Body,
			task.BodyRef,
		)
		if got := string(encoded); got != want {
			t.Fatalf("importTasks() JSON = %s, want %s", got, want)
		}
	})

	t.Run("Should reject missing pattern", func(t *testing.T) {
		t.Parallel()

		_, err := importTasks(importTasksInput{})
		if !errors.Is(err, looppkg.ErrValidation) ||
			!strings.Contains(err.Error(), "import_tasks pattern is required") {
			t.Fatalf("importTasks() error = %v, want pattern validation", err)
		}
	})
}

func writeImportTasksManifest(t *testing.T, tasksDir string, schemaVersion string, edges []string) {
	t.Helper()
	lines := []string{
		"---",
		fmt.Sprintf("schema_version: %q", schemaVersion),
		"workflow: demo",
		"graph:",
		"  nodes:",
		"    - id: task_01",
		"      file: task_01.md",
		"    - id: task_02",
		"      file: task_02.md",
		"    - id: task_03",
		"      file: task_03.md",
	}
	if len(edges) == 0 {
		lines = append(lines, "  edges: []")
	} else {
		lines = append(lines, "  edges:")
		lines = append(lines, edges...)
	}
	lines = append(lines, "---", "", "# Demo tasks", "")
	writeImportTasksFile(t, tasksDir, compozyTaskManifestFileName, strings.Join(lines, "\n"))
}

func writeImportTaskFile(t *testing.T, tasksDir string, name string, status string, title string, body string) {
	t.Helper()
	writeImportTaskFileWithFrontmatter(t, tasksDir, name, []string{
		"status: " + status,
		"title: " + title,
	}, body)
}

func writeImportTaskFileWithFrontmatter(
	t *testing.T,
	tasksDir string,
	name string,
	metadata []string,
	body string,
) {
	t.Helper()
	lines := append([]string{"---"}, metadata...)
	lines = append(lines, "---", "", body)
	writeImportTasksFile(t, tasksDir, name, strings.Join(lines, "\n"))
}

func writeImportTasksFile(t *testing.T, tasksDir string, name string, content string) {
	t.Helper()
	path := filepath.Join(tasksDir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}
