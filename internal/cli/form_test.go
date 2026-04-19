package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"charm.land/huh/v2"
	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/provider"
	"github.com/compozy/compozy/internal/core/tasks"
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
		"define-task-runtime",
		"auto-commit",
	)
	assertFieldKeysAbsent(t, keys, "concurrent", "dry-run", "include-completed", "tail-lines", "access-mode", "timeout")
}

func TestFixReviewsFormKeepsConcurrentButHidesUnneededFields(t *testing.T) {
	t.Parallel()

	keys := formFieldKeys(
		newReviewsFixCommandWithDefaults(defaultCommandStateDefaults()),
		newCommandState(commandKindFixReviews, core.ModePRReview),
	)

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
	now := time.Now().UTC()
	for _, name := range []string{"alpha", "beta"} {
		workflowDir := filepath.Join(baseDir, name)
		if err := os.MkdirAll(workflowDir, 0o755); err != nil {
			t.Fatalf("create workflow dir: %v", err)
		}
		writeFormTaskFile(t, workflowDir, "task_01.md", "completed")
		if err := tasks.WriteTaskMeta(workflowDir, model.TaskMeta{
			CreatedAt: now,
			UpdatedAt: now,
			Total:     1,
			Completed: 1,
			Pending:   0,
		}); err != nil {
			t.Fatalf("write meta for %s: %v", name, err)
		}
	}

	keys := formFieldKeysWithBaseDir(
		newStartCommand(),
		newCommandState(commandKindStart, core.ModePRDTasks),
		baseDir,
	)

	assertFieldKeysPresent(t, keys, "name", "tasks-dir")
}

func TestFetchReviewsUsesSelectWhenTaskDirsExist(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	baseDir := filepath.Join(tmp, ".compozy", "tasks")
	if err := os.MkdirAll(filepath.Join(baseDir, "alpha"), 0o755); err != nil {
		t.Fatalf("create test dir: %v", err)
	}

	cmd := newReviewsFetchCommandWithDefaults(defaultCommandStateDefaults())
	state := newCommandState(commandKindFetchReviews, core.ModePRReview)
	builder := newFormBuilder(cmd, state)
	builder.tasksBaseDir = baseDir

	inputs := newFormInputs()
	inputs.register(builder)

	if !builder.nameFromDirList {
		t.Fatal("reviews fetch should use directory select when workflows exist")
	}
}

func TestFetchReviewsFallsBackToInputWhenNoDirs(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	baseDir := filepath.Join(tmp, ".compozy", "tasks")

	keys := formFieldKeysWithBaseDir(
		newReviewsFetchCommandWithDefaults(defaultCommandStateDefaults()),
		newCommandState(commandKindFetchReviews, core.ModePRReview),
		baseDir,
	)

	assertFieldKeysPresent(t, keys, "name", "provider", "pr", "round")
}

func TestFetchReviewsFormOmitsNitpicksToggle(t *testing.T) {
	t.Parallel()

	t.Run("Should omit nitpicks toggle in the reviews fetch form", func(t *testing.T) {
		t.Parallel()

		keys := formFieldKeys(
			newReviewsFetchCommandWithDefaults(defaultCommandStateDefaults()),
			newCommandState(commandKindFetchReviews, core.ModePRReview),
		)

		assertFieldKeysPresent(t, keys, "name", "provider", "pr", "round")
		assertFieldKeysAbsent(t, keys, "nitpicks")
	})
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

func TestListStartTaskSubdirsFiltersCompletedWorkflows(t *testing.T) {
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

	// Pre-create a legacy _meta.md fixture so ReadTaskMeta can detect the
	// completed workflow. Daemon-backed sync no longer keeps this file current.
	now := time.Now().UTC()
	if err := tasks.WriteTaskMeta(completedDir, model.TaskMeta{
		CreatedAt: now,
		UpdatedAt: now,
		Total:     1,
		Completed: 1,
		Pending:   0,
	}); err != nil {
		t.Fatalf("write completed meta: %v", err)
	}

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

	// Listing must NOT create _meta.md as a side effect in workflows that
	// did not already have one.
	for _, dir := range []string{pendingDir, emptyDir} {
		if _, err := os.Stat(filepath.Join(dir, "_meta.md")); err == nil {
			t.Fatalf("listing should not bootstrap _meta.md in %s", dir)
		}
	}
}

func TestStartTaskRuntimeFormPreseedsConfiguredTypeRules(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	tasksDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "demo")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("mkdir tasks dir: %v", err)
	}
	writeFormTaskFile(t, tasksDir, "task_01.md", "pending")

	state := newCommandState(commandKindStart, core.ModePRDTasks)
	state.workspaceRoot = workspaceRoot
	state.name = "demo"
	state.ide = "codex"
	state.reasoningEffort = "medium"
	state.configuredTaskRuntimeRules = []model.TaskRuntimeRule{{
		Type:            stringPointer("backend"),
		IDE:             stringPointer("claude"),
		Model:           stringPointer("sonnet"),
		ReasoningEffort: stringPointer("high"),
	}}

	form, err := newStartTaskRuntimeForm(state)
	if err != nil {
		t.Fatalf("newStartTaskRuntimeForm() error = %v", err)
	}
	if form == nil {
		t.Fatal("expected task runtime form")
	}
	if !slices.Contains(form.selectedTypes, "backend") {
		t.Fatalf("expected backend type to be preselected, got %#v", form.selectedTypes)
	}
	editor := form.typeEditors["backend"]
	if editor == nil {
		t.Fatal("expected backend editor to be created")
	}
	if editor.IDE != "claude" || editor.Model != "sonnet" || editor.ReasoningEffort != "high" {
		t.Fatalf("unexpected preseeded editor: %#v", editor)
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

func TestFormSelectOptionsIncludeExtensionCatalogEntries(t *testing.T) {
	supportsAddDirs := true
	restoreIDE, err := agent.ActivateOverlay([]agent.OverlayEntry{{
		Name:            "ext-adapter",
		Command:         "mock-acp --serve",
		DisplayName:     "Mock ACP",
		DefaultModel:    "ext-model",
		SetupAgentName:  "codex",
		SupportsAddDirs: &supportsAddDirs,
	}})
	if err != nil {
		t.Fatalf("activate IDE overlay: %v", err)
	}
	defer restoreIDE()

	restoreProvider, err := provider.ActivateOverlay([]provider.OverlayEntry{{
		Name:        "ext-review",
		Command:     "coderabbit",
		DisplayName: "Extension Review",
	}})
	if err != nil {
		t.Fatalf("activate provider overlay: %v", err)
	}
	defer restoreProvider()

	t.Run("ShouldRenderOverlayIDEInTheSelectField", func(t *testing.T) {
		builder := newFormBuilder(newStartCommand(), newCommandState(commandKindStart, core.ModePRDTasks))
		selected := "ext-adapter"
		builder.addIDEField(&selected)
		if len(builder.fields) != 1 {
			t.Fatalf("expected IDE field to be registered, got %d fields", len(builder.fields))
		}
		field := builder.fields[0]
		if got := field.GetKey(); got != "ide" {
			t.Fatalf("field key = %q, want %q", got, "ide")
		}
		if got := field.GetValue(); got != selected {
			t.Fatalf("field value = %#v, want %q", got, selected)
		}
		assertFieldViewContains(t, field, "Mock ACP")
	})

	t.Run("ShouldRenderOverlayProviderInTheSelectField", func(t *testing.T) {
		builder := newFormBuilder(
			newReviewsFetchCommandWithDefaults(defaultCommandStateDefaults()),
			newCommandState(commandKindFetchReviews, core.ModePRReview),
		)
		selected := "ext-review"
		builder.addProviderField(&selected)
		if len(builder.fields) != 1 {
			t.Fatalf("expected provider field to be registered, got %d fields", len(builder.fields))
		}
		field := builder.fields[0]
		if got := field.GetKey(); got != "provider" {
			t.Fatalf("field key = %q, want %q", got, "provider")
		}
		if got := field.GetValue(); got != selected {
			t.Fatalf("field value = %#v, want %q", got, selected)
		}
		assertFieldViewContains(t, field, "Extension Review")
	})
}

func assertFieldViewContains(t *testing.T, field huh.Field, wants ...string) {
	t.Helper()

	field = field.WithWidth(120).WithHeight(24)
	_ = field.Focus()
	view := field.View()
	for _, want := range wants {
		if !strings.Contains(view, want) {
			t.Fatalf("expected field view to contain %q, got:\n%s", want, view)
		}
	}
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
		"title: " + name,
		"type: backend",
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
