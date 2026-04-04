package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/huh/v2"
	core "github.com/compozy/compozy/internal/core"
	"github.com/spf13/cobra"
)

func TestStartFormHidesSequentialOnlyFields(t *testing.T) {
	t.Parallel()

	keys := formFieldKeys(newStartCommand(), newCommandState(commandKindStart, core.ModePRDTasks))

	assertFieldKeysPresent(
		t,
		keys,
		"name",
		"tasks-dir",
		"ide",
		"model",
		"add-dir",
		"reasoning-effort",
		"auto-commit",
	)
	assertFieldKeysAbsent(t, keys, "concurrent", "dry-run", "include-completed", "tail-lines", "access-mode", "timeout")
}

func TestFixReviewsFormKeepsConcurrentButHidesUnneededFields(t *testing.T) {
	t.Parallel()

	keys := formFieldKeys(newFixReviewsCommand(), newCommandState(commandKindFixReviews, core.ModePRReview))

	assertFieldKeysPresent(
		t,
		keys,
		"name",
		"round",
		"reviews-dir",
		"concurrent",
		"batch-size",
		"auto-commit",
		"ide",
		"model",
		"add-dir",
		"reasoning-effort",
	)
	assertFieldKeysAbsent(t, keys, "dry-run", "include-resolved", "tail-lines", "access-mode", "timeout")
}

func TestStartFormUsesSelectWhenTaskDirsExist(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	baseDir := filepath.Join(tmp, ".compozy", "tasks")
	for _, name := range []string{"alpha", "beta"} {
		if err := os.MkdirAll(filepath.Join(baseDir, name), 0o755); err != nil {
			t.Fatalf("create test dir: %v", err)
		}
	}

	keys := formFieldKeysWithBaseDir(
		newStartCommand(),
		newCommandState(commandKindStart, core.ModePRDTasks),
		baseDir,
	)

	assertFieldKeysPresent(t, keys, "name")
	assertFieldKeysAbsent(t, keys, "tasks-dir")
}

func TestStartFormFallsBackToInputWhenNoDirs(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	baseDir := filepath.Join(tmp, ".compozy", "tasks")

	keys := formFieldKeysWithBaseDir(
		newStartCommand(),
		newCommandState(commandKindStart, core.ModePRDTasks),
		baseDir,
	)

	assertFieldKeysPresent(t, keys, "name", "tasks-dir")
}

func TestStartFormFallsBackToInputWhenAllTaskDirsAreCompleted(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	baseDir := filepath.Join(tmp, ".compozy", "tasks")
	for _, name := range []string{"alpha", "beta"} {
		workflowDir := filepath.Join(baseDir, name)
		if err := os.MkdirAll(workflowDir, 0o755); err != nil {
			t.Fatalf("create workflow dir: %v", err)
		}
		writeFormTaskFile(t, workflowDir, "task_01.md", "completed")
	}

	keys := formFieldKeysWithBaseDir(
		newStartCommand(),
		newCommandState(commandKindStart, core.ModePRDTasks),
		baseDir,
	)

	assertFieldKeysPresent(t, keys, "name", "tasks-dir")
}

func TestFetchReviewsAlwaysUsesTextInput(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	baseDir := filepath.Join(tmp, ".compozy", "tasks")
	if err := os.MkdirAll(filepath.Join(baseDir, "alpha"), 0o755); err != nil {
		t.Fatalf("create test dir: %v", err)
	}

	cmd := newFetchReviewsCommand()
	state := newCommandState(commandKindFetchReviews, core.ModePRReview)
	builder := newFormBuilder(cmd, state)
	builder.tasksBaseDir = baseDir

	inputs := newFormInputs()
	inputs.register(builder)

	if builder.nameFromDirList {
		t.Fatal("fetch-reviews should not use directory select")
	}
}

func TestListTaskSubdirs(t *testing.T) {
	t.Parallel()

	t.Run("returns sorted directories", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()
		for _, name := range []string{"charlie", "alpha", "beta"} {
			if err := os.MkdirAll(filepath.Join(tmp, name), 0o755); err != nil {
				t.Fatalf("create test dir: %v", err)
			}
		}

		dirs := listTaskSubdirs(tmp)
		want := []string{"alpha", "beta", "charlie"}
		if len(dirs) != len(want) {
			t.Fatalf("got %v, want %v", dirs, want)
		}
		for i, d := range dirs {
			if d != want[i] {
				t.Fatalf("dirs[%d] = %q, want %q", i, d, want[i])
			}
		}
	})

	t.Run("excludes hidden directories", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()
		for _, name := range []string{".hidden", "visible"} {
			if err := os.MkdirAll(filepath.Join(tmp, name), 0o755); err != nil {
				t.Fatalf("create test dir: %v", err)
			}
		}

		dirs := listTaskSubdirs(tmp)
		if len(dirs) != 1 || dirs[0] != "visible" {
			t.Fatalf("got %v, want [visible]", dirs)
		}
	})

	t.Run("excludes archived workflows", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()
		for _, name := range []string{"_archived", "visible"} {
			if err := os.MkdirAll(filepath.Join(tmp, name), 0o755); err != nil {
				t.Fatalf("create test dir: %v", err)
			}
		}

		dirs := listTaskSubdirs(tmp)
		if len(dirs) != 1 || dirs[0] != "visible" {
			t.Fatalf("got %v, want [visible]", dirs)
		}
	})

	t.Run("excludes files", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()
		if err := os.MkdirAll(filepath.Join(tmp, "mydir"), 0o755); err != nil {
			t.Fatalf("create test dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(tmp, "myfile.md"), []byte("hi"), 0o644); err != nil {
			t.Fatalf("create test file: %v", err)
		}

		dirs := listTaskSubdirs(tmp)
		if len(dirs) != 1 || dirs[0] != "mydir" {
			t.Fatalf("got %v, want [mydir]", dirs)
		}
	})

	t.Run("returns nil for missing directory", func(t *testing.T) {
		t.Parallel()
		dirs := listTaskSubdirs(filepath.Join(t.TempDir(), "nonexistent"))
		if dirs != nil {
			t.Fatalf("got %v, want nil", dirs)
		}
	})
}

func TestListStartTaskSubdirsFiltersCompletedWorkflowsAndBootstrapsMeta(t *testing.T) {
	t.Parallel()

	baseDir := filepath.Join(t.TempDir(), ".compozy", "tasks")
	pendingDir := filepath.Join(baseDir, "alpha")
	completedDir := filepath.Join(baseDir, "beta")
	emptyDir := filepath.Join(baseDir, "gamma")
	for _, dir := range []string{pendingDir, completedDir, emptyDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	writeFormTaskFile(t, pendingDir, "task_01.md", "pending")
	writeFormTaskFile(t, completedDir, "task_01.md", "completed")

	dirs := listStartTaskSubdirs(baseDir)
	want := []string{"alpha", "gamma"}
	if len(dirs) != len(want) {
		t.Fatalf("got %v, want %v", dirs, want)
	}
	for i, dir := range dirs {
		if dir != want[i] {
			t.Fatalf("dirs[%d] = %q, want %q", i, dir, want[i])
		}
	}

	for _, dir := range []string{pendingDir, completedDir, emptyDir} {
		if _, err := os.Stat(filepath.Join(dir, "_meta.md")); err != nil {
			t.Fatalf("expected bootstrapped meta in %s: %v", dir, err)
		}
	}
}

func TestFormSelectOptionsOmitRecommendedSuffixes(t *testing.T) {
	t.Parallel()

	t.Run("ide field", func(t *testing.T) {
		t.Parallel()

		var selected string
		builder := newFormBuilder(newStartCommand(), newCommandState(commandKindStart, core.ModePRDTasks))
		builder.addIDEField(&selected)

		view := renderSingleFormFieldForTest(t, builder.fields, "ide")
		if !strings.Contains(view, "Codex") {
			t.Fatalf("expected IDE selector to contain Codex, got %q", view)
		}
		if strings.Contains(view, "Codex (recommended)") {
			t.Fatalf("expected IDE selector to omit recommended suffix, got %q", view)
		}
	})

	t.Run("reasoning effort field", func(t *testing.T) {
		t.Parallel()

		var selected string
		builder := newFormBuilder(newStartCommand(), newCommandState(commandKindStart, core.ModePRDTasks))
		builder.addReasoningEffortField(&selected)

		view := renderSingleFormFieldForTest(t, builder.fields, "reasoning-effort")
		if !strings.Contains(view, "Medium") {
			t.Fatalf("expected reasoning selector to contain Medium, got %q", view)
		}
		if strings.Contains(view, "Medium (recommended)") {
			t.Fatalf("expected reasoning selector to omit recommended suffix, got %q", view)
		}
	})
}

func formFieldKeys(cmd *cobra.Command, state *commandState) map[string]struct{} {
	return formFieldKeysWithBaseDir(cmd, state, filepath.Join(os.TempDir(), "nonexistent-looper-test-dir"))
}

func formFieldKeysWithBaseDir(cmd *cobra.Command, state *commandState, baseDir string) map[string]struct{} {
	inputs := newFormInputs()
	builder := newFormBuilder(cmd, state)
	builder.tasksBaseDir = baseDir
	inputs.register(builder)

	keys := make(map[string]struct{}, len(builder.fields))
	for _, field := range builder.fields {
		key := field.GetKey()
		if key == "" {
			continue
		}
		keys[key] = struct{}{}
	}

	return keys
}

func assertFieldKeysPresent(t *testing.T, keys map[string]struct{}, want ...string) {
	t.Helper()

	for _, key := range want {
		if _, ok := keys[key]; !ok {
			t.Fatalf("expected form fields to include %q, got %#v", key, keys)
		}
	}
}

func assertFieldKeysAbsent(t *testing.T, keys map[string]struct{}, forbidden ...string) {
	t.Helper()

	for _, key := range forbidden {
		if _, ok := keys[key]; ok {
			t.Fatalf("expected form fields to omit %q, got %#v", key, keys)
		}
	}
}

func writeFormTaskFile(t *testing.T, workflowDir, name, status string) {
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

	if err := os.WriteFile(filepath.Join(workflowDir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func renderSingleFormFieldForTest(t *testing.T, fields []huh.Field, key string) string {
	t.Helper()

	for _, field := range fields {
		if field.GetKey() != key {
			continue
		}
		field = field.WithTheme(darkHuhTheme()).WithWidth(80).WithHeight(8)
		_ = field.Focus()
		return field.View()
	}

	t.Fatalf("field %q not found", key)
	return ""
}
