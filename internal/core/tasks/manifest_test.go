package tasks

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadValidatedTaskGraphManifest(t *testing.T) {
	t.Parallel()

	t.Run("Should load canonical nodes and graph edges from _tasks.md", func(t *testing.T) {
		t.Parallel()

		tasksDir := t.TempDir()
		writeTaskManifestTestFile(t, tasksDir, "_tasks.md", taskGraphManifestMarkdown("demo", []string{
			"    - from: task_01",
			"      to: task_03",
			"    - from: task_02",
			"      to: task_03",
		}))
		writeTaskManifestTestFile(t, tasksDir, "task_01.md", taskMarkdown(
			[]string{"status: pending", "title: Task 1", "type: backend", "complexity: low"},
			"# Task 1",
		))
		writeTaskManifestTestFile(t, tasksDir, "task_02.md", taskMarkdown(
			[]string{"status: pending", "title: Task 2", "type: backend", "complexity: low"},
			"# Task 2",
		))
		writeTaskManifestTestFile(t, tasksDir, "task_03.md", taskMarkdown(
			[]string{"status: pending", "title: Task 3", "type: backend", "complexity: low"},
			"# Task 3",
		))

		manifest, taskFiles, err := LoadValidatedTaskGraphManifest(context.Background(), tasksDir, "demo")
		if err != nil {
			t.Fatalf("LoadValidatedTaskGraphManifest() error = %v", err)
		}
		if manifest.SchemaVersion != TaskGraphManifestVersion || manifest.Workflow != "demo" {
			t.Fatalf("manifest = %#v, want v2 demo manifest", manifest)
		}
		if len(taskFiles) != 3 {
			t.Fatalf("task file count = %d, want 3", len(taskFiles))
		}
		if taskFiles[2].ID != "task_03" || taskFiles[2].Number != 3 || taskFiles[2].Entry.Title != "Task 3" {
			t.Fatalf("task_03 metadata = %#v", taskFiles[2])
		}
	})

	t.Run("Should reject duplicated dependencies in task frontmatter", func(t *testing.T) {
		t.Parallel()

		tasksDir := t.TempDir()
		writeTaskManifestTestFile(t, tasksDir, "_tasks.md", taskGraphManifestMarkdown("demo", nil))
		writeTaskManifestTestFile(t, tasksDir, "task_01.md", taskMarkdown(
			[]string{
				"status: pending",
				"title: Task 1",
				"type: backend",
				"complexity: low",
				"dependencies: []",
			},
			"# Task 1",
		))
		writeTaskManifestTestFile(t, tasksDir, "task_02.md", taskMarkdown(
			[]string{"status: pending", "title: Task 2", "type: backend", "complexity: low"},
			"# Task 2",
		))
		writeTaskManifestTestFile(t, tasksDir, "task_03.md", taskMarkdown(
			[]string{"status: pending", "title: Task 3", "type: backend", "complexity: low"},
			"# Task 3",
		))

		_, _, err := LoadValidatedTaskGraphManifest(context.Background(), tasksDir, "demo")
		var validationErr *TaskGraphManifestValidationError
		if !errors.As(err, &validationErr) {
			t.Fatalf("LoadValidatedTaskGraphManifest() error = %v, want manifest validation error", err)
		}
		found := false
		for _, issue := range validationErr.Issues {
			if issue.Field == "dependencies" {
				found = true
			}
		}
		if !found {
			t.Fatalf("validation issues = %#v, want dependencies issue", validationErr.Issues)
		}
	})

	t.Run("Should reject cyclic graph edges", func(t *testing.T) {
		t.Parallel()

		tasksDir := t.TempDir()
		writeTaskManifestTestFile(t, tasksDir, "_tasks.md", taskGraphManifestMarkdown("demo", []string{
			"    - from: task_01",
			"      to: task_02",
			"    - from: task_02",
			"      to: task_01",
		}))
		writeTaskManifestTestFile(t, tasksDir, "task_01.md", taskMarkdown(
			[]string{"status: pending", "title: Task 1", "type: backend", "complexity: low"},
			"# Task 1",
		))
		writeTaskManifestTestFile(t, tasksDir, "task_02.md", taskMarkdown(
			[]string{"status: pending", "title: Task 2", "type: backend", "complexity: low"},
			"# Task 2",
		))
		writeTaskManifestTestFile(t, tasksDir, "task_03.md", taskMarkdown(
			[]string{"status: pending", "title: Task 3", "type: backend", "complexity: low"},
			"# Task 3",
		))

		_, _, err := LoadValidatedTaskGraphManifest(context.Background(), tasksDir, "demo")
		var validationErr *TaskGraphManifestValidationError
		if !errors.As(err, &validationErr) {
			t.Fatalf("LoadValidatedTaskGraphManifest() error = %v, want manifest validation error", err)
		}
		found := false
		for _, issue := range validationErr.Issues {
			if issue.Field == "graph.edges" && strings.Contains(issue.Message, "cycle") {
				found = true
			}
		}
		if !found {
			t.Fatalf("validation issues = %#v, want graph.edges cycle issue", validationErr.Issues)
		}
	})

	t.Run("Should treat legacy markdown task lists as absent manifests", func(t *testing.T) {
		t.Parallel()

		tasksDir := t.TempDir()
		writeTaskManifestTestFile(t, tasksDir, "_tasks.md", "# Legacy Task List\n\n| # | Title |\n")

		_, err := ReadTaskGraphManifest(tasksDir)
		if !errors.Is(err, ErrTaskGraphManifestMissing) {
			t.Fatalf("ReadTaskGraphManifest() error = %v, want missing manifest sentinel", err)
		}
	})
}

func TestLoadValidatedTaskGraphManifestRejectsEscapedNodeFiles(t *testing.T) {
	// INVARIANT: every graph node resolves to a task file inside its manifest root.
	// OWNING_LAYER: service-integration. EXISTING_SUITE: internal/core/tasks/manifest_test.go.
	t.Parallel()

	testCases := []struct {
		name string
		file func(t *testing.T, tasksDir, siblingTask string) string
	}{
		{
			name: "parent directory reference",
			file: func(_ *testing.T, _, _ string) string {
				return "../TG-001/task_01.md"
			},
		},
		{
			name: "absolute reference",
			file: func(_ *testing.T, _, siblingTask string) string {
				return filepath.ToSlash(siblingTask)
			},
		},
		{
			name: "symlink reference",
			file: func(t *testing.T, tasksDir, siblingTask string) string {
				t.Helper()
				link := filepath.Join(tasksDir, "task_01.md")
				if err := os.Symlink(siblingTask, link); err != nil {
					t.Fatalf("create task symlink: %v", err)
				}
				return "task_01.md"
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			tasksDir := filepath.Join(root, "TG-002")
			siblingTask := filepath.Join(root, "TG-001", "task_01.md")
			if err := os.MkdirAll(tasksDir, 0o755); err != nil {
				t.Fatalf("create tasks directory: %v", err)
			}
			writeTaskManifestTestFile(t, filepath.Dir(siblingTask), filepath.Base(siblingTask), taskMarkdown(
				[]string{"status: pending", "title: Sibling task", "type: backend", "complexity: low"},
				"# Sibling task",
			))
			file := tt.file(t, tasksDir, siblingTask)
			writeTaskManifestTestFile(t, tasksDir, "_tasks.md", singleTaskGraphManifestMarkdown("demo/TG-002", file))

			_, _, err := LoadValidatedTaskGraphManifest(context.Background(), tasksDir, "demo/TG-002")
			var validationErr *TaskGraphManifestValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("LoadValidatedTaskGraphManifest() error = %v, want manifest validation error", err)
			}
			found := false
			for _, issue := range validationErr.Issues {
				if strings.Contains(issue.Message, "must resolve within task root") {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("validation issues = %#v, want containment failure", validationErr.Issues)
			}
		})
	}
}

func taskGraphManifestMarkdown(workflow string, edges []string) string {
	lines := []string{
		"---",
		"schema_version: \"compozy.tasks/v2\"",
		"workflow: " + workflow,
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
	lines = append(lines,
		"---",
		"",
		"# "+workflow+" Tasks",
		"",
	)
	return strings.Join(lines, "\n")
}

func singleTaskGraphManifestMarkdown(workflow string, file string) string {
	return strings.Join([]string{
		"---",
		"schema_version: \"compozy.tasks/v2\"",
		"workflow: " + workflow,
		"graph:",
		"  nodes:",
		"    - id: task_01",
		"      file: " + file,
		"  edges: []",
		"---",
		"",
		"# " + workflow + " Tasks",
		"",
	}, "\n")
}

func writeTaskManifestTestFile(t *testing.T, dir string, name string, content string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
