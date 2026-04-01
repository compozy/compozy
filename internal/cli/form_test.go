package cli

import (
	"os"
	"path/filepath"
	"testing"

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
		"timeout",
		"auto-commit",
	)
	assertFieldKeysAbsent(t, keys, "concurrent", "tail-lines", "dry-run", "include-completed")
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
		"grouped",
		"auto-commit",
		"ide",
		"model",
		"add-dir",
		"reasoning-effort",
		"timeout",
	)
	assertFieldKeysAbsent(t, keys, "tail-lines", "dry-run", "include-resolved")
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
