package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	xansi "github.com/charmbracelet/x/ansi"
	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/provider"
	"github.com/compozy/compozy/internal/core/taskgroups"
	"github.com/compozy/compozy/internal/core/tasks"
	"github.com/spf13/cobra"
)

func TestTasksRunFormHidesSequentialOnlyFields(t *testing.T) {
	t.Parallel()

	t.Run("Should hide sequential-only fields", func(t *testing.T) {
		t.Parallel()

		keys := formFieldKeys(
			newTasksRunCommandWithDefaults(nil, defaultCommandStateDefaults()),
			newCommandState(commandKindTasksRun, core.ModePRDTasks),
		)

		assertFieldKeysPresent(
			t,
			keys,
			"name",
			"ide",
			"model",
			"add-dir",
			"reasoning-effort",
			"define-task-runtime",
			"auto-commit",
		)
		assertFieldKeysAbsent(
			t,
			keys,
			"tasks-dir",
			"concurrent",
			"dry-run",
			"include-completed",
			"tail-lines",
			"access-mode",
			"timeout",
		)
	})
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

func TestFixReviewsFormStartsWithExactReviewTargetSelection(t *testing.T) {
	cmd := newReviewsFixCommandWithDefaults(defaultCommandStateDefaults())
	state := newCommandState(commandKindFixReviews, core.ModePRReview)
	builder := newFormBuilder(cmd, state)
	builder.reviewFixTargetOptions = []taskGroupPickerOption{
		{
			Value: "auth",
			Label: "[ ] auth",
		},
		{
			Value:                  "auth/TG-001",
			Label:                  "[✓] TG-001 — Data model — Review round 3 — No issues pending",
			Depth:                  1,
			Completed:              true,
			SelectionBlocked:       true,
			SelectionBlockedReason: reviewNoPendingIssuesReason,
		},
		{
			Value: "auth/TG-002",
			Label: "[ ] TG-002 — API — Review round 2 — 1 issue pending",
			Depth: 1,
		},
		{
			Value: "auth/TG-003",
			Label: "[ ] TG-003 — Web — Review round 1 — 3 issues pending",
			Depth: 1,
		},
		{
			Value:                  "auth/TG-004",
			Label:                  "[⊘] TG-004 — Rollout — No review round — No issues pending",
			Depth:                  1,
			SelectionBlocked:       true,
			SelectionBlockedReason: reviewImplementationBlockedReason,
		},
	}
	inputs := newFormInputs()
	inputs.register(builder)
	builder.build()

	if len(builder.fields) == 0 {
		t.Fatal("review form has no fields")
	}
	field, ok := builder.fields[0].(*huh.Select[string])
	if !ok {
		t.Fatalf("first review field = %T, want Task Group select", builder.fields[0])
	}
	field = field.WithWidth(160).WithHeight(10).(*huh.Select[string])
	_ = field.Focus()
	if field.GetFiltering() {
		t.Fatal("review target picker starts in filter mode; want list navigation until / is pressed")
	}
	updated, _ := field.Update(tea.KeyPressMsg(tea.Key{Code: '/'}))
	field, ok = updated.(*huh.Select[string])
	if !ok {
		t.Fatalf("filtering review field = %T, want Task Group select", updated)
	}
	if !field.GetFiltering() {
		t.Fatal("review target picker did not enter filter mode after /")
	}
	updated, _ = field.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	field, ok = updated.(*huh.Select[string])
	if !ok {
		t.Fatalf("cleared review field = %T, want Task Group select", updated)
	}
	updated, _ = field.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	field, ok = updated.(*huh.Select[string])
	if !ok {
		t.Fatalf("updated review field = %T, want Task Group select", updated)
	}
	updated, _ = field.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	field, ok = updated.(*huh.Select[string])
	if !ok {
		t.Fatalf("updated review field = %T, want Task Group select", updated)
	}
	rendered := xansi.Strip(field.View())
	for _, want := range []string{
		"Rows without pending issues stay visible but stay locked;",
		"[⊘] means no implementation tasks are complete",
		"[ ] auth",
		"  [✓] TG-001",
		"  [x] TG-002",
		"  [ ] TG-003",
		"  [⊘] TG-004",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("review target view missing %q:\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "[x] TG-001") || strings.Contains(rendered, "[x] TG-003") {
		t.Fatalf("review target view marks a non-active target as selected:\n%s", rendered)
	}
	_ = field.Blur()
	if blurred := xansi.Strip(field.View()); !strings.Contains(blurred, "[x] TG-002") {
		t.Fatalf("blurred review target view loses selected marker:\n%s", blurred)
	}
	var output bytes.Buffer
	input := iotest.OneByteReader(strings.NewReader("2\n5\n3\n"))
	if err := field.RunAccessible(&output, input); err != nil {
		t.Fatalf("run accessible review target field: %v", err)
	}
	if inputs.name != "auth/TG-002" {
		t.Fatalf("selected review target = %q, want exact Task Group reference", inputs.name)
	}
	accessibleOutput := xansi.Strip(output.String())
	if !strings.Contains(accessibleOutput, reviewImplementationBlockedReason) {
		t.Fatalf("review target output missing blocked-selection guidance:\n%s", output.String())
	}
	if !strings.Contains(accessibleOutput, reviewNoPendingIssuesReason) {
		t.Fatalf("review target output missing no-pending guidance:\n%s", output.String())
	}
	for _, want := range []string{"auth", "TG-001", "No issues pending", "TG-002", "1 issue pending", "TG-004"} {
		if !strings.Contains(accessibleOutput, want) {
			t.Fatalf("review target output missing %q:\n%s", want, output.String())
		}
	}
	for _, hidden := range []string{"Ready", "tasks completed", "issues total"} {
		if strings.Contains(accessibleOutput, hidden) {
			t.Fatalf("review target output includes hidden detail %q:\n%s", hidden, output.String())
		}
	}
	if strings.Contains(accessibleOutput, "(!)") {
		t.Fatalf("review target output includes the removed zero-pending marker:\n%s", output.String())
	}
}

func TestWatchReviewsFormCollectsReviewWatchInputs(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	baseDir := filepath.Join(tmp, ".compozy", "tasks")
	if err := os.MkdirAll(filepath.Join(baseDir, "demo"), 0o755); err != nil {
		t.Fatalf("create workflow dir: %v", err)
	}

	cmd := newReviewsWatchCommandWithDefaults(defaultCommandStateDefaults())
	state := newCommandState(commandKindWatchReviews, core.ModePRReview)
	builder := newFormBuilder(cmd, state)
	builder.tasksBaseDir = baseDir

	inputs := newFormInputs()
	inputs.register(builder)

	if !builder.nameFromDirList {
		t.Fatal("reviews watch should use directory select when workflows exist")
	}

	keys := make(map[string]struct{}, len(builder.fields))
	for _, field := range builder.fields {
		key := field.GetKey()
		if key != "" {
			keys[key] = struct{}{}
		}
	}

	assertFieldKeysPresent(t, keys, "name", "provider", "pr")
}

func TestTasksRunFormUsesSelectWhenTaskDirsExist(t *testing.T) {
	t.Parallel()

	t.Run("Should use select when task dirs exist", func(t *testing.T) {
		t.Parallel()

		tmp := t.TempDir()
		baseDir := filepath.Join(tmp, ".compozy", "tasks")
		for _, name := range []string{"alpha", "beta"} {
			if err := os.MkdirAll(filepath.Join(baseDir, name), 0o755); err != nil {
				t.Fatalf("create test dir: %v", err)
			}
		}

		keys := formFieldKeysWithBaseDir(
			newTasksRunCommandWithDefaults(nil, defaultCommandStateDefaults()),
			newCommandState(commandKindTasksRun, core.ModePRDTasks),
			baseDir,
		)

		assertFieldKeysPresent(t, keys, "name")
		assertFieldKeysAbsent(t, keys, "tasks-dir")
	})
}

func TestTasksRunFormFallsBackToInputWhenNoDirs(t *testing.T) {
	t.Parallel()

	t.Run("Should fall back to input when no dirs exist", func(t *testing.T) {
		t.Parallel()

		tmp := t.TempDir()
		baseDir := filepath.Join(tmp, ".compozy", "tasks")

		keys := formFieldKeysWithBaseDir(
			newTasksRunCommandWithDefaults(nil, defaultCommandStateDefaults()),
			newCommandState(commandKindTasksRun, core.ModePRDTasks),
			baseDir,
		)

		assertFieldKeysPresent(t, keys, "name")
		assertFieldKeysAbsent(t, keys, "tasks-dir")
	})
}

func TestTasksRunFormFallsBackToInputWhenAllTaskDirsAreCompleted(t *testing.T) {
	t.Parallel()

	t.Run("Should fall back to input when all task dirs are completed", func(t *testing.T) {
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
			newTasksRunCommandWithDefaults(nil, defaultCommandStateDefaults()),
			newCommandState(commandKindTasksRun, core.ModePRDTasks),
			baseDir,
		)

		assertFieldKeysPresent(t, keys, "name")
		assertFieldKeysAbsent(t, keys, "tasks-dir")
	})
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

	dirs := listTaskRunSubdirs(baseDir)
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

func TestTaskRunRuntimeFormPreseedsConfiguredTypeRules(t *testing.T) {
	t.Parallel()

	t.Run("Should preseed configured type rules", func(t *testing.T) {
		t.Parallel()

		workspaceRoot := t.TempDir()
		tasksDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "demo")
		if err := os.MkdirAll(tasksDir, 0o755); err != nil {
			t.Fatalf("mkdir tasks dir: %v", err)
		}
		writeFormTaskFile(t, tasksDir, "task_01.md", "pending")

		state := newCommandState(commandKindTasksRun, core.ModePRDTasks)
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

		form, err := newTaskRunRuntimeFormForSlugs(t.Context(), state, []string{"demo"})
		if err != nil {
			t.Fatalf("newTaskRunRuntimeFormForSlugs() error = %v", err)
		}
		if form == nil {
			t.Fatal("expected task runtime form")
			return
		}
		if !slices.Contains(form.selectedTypes, "demo::backend") {
			t.Fatalf("expected backend type to be preselected, got %#v", form.selectedTypes)
		}
		editor := form.typeEditors["demo::backend"]
		if editor == nil {
			t.Fatal("expected backend editor to be created")
			return
		}
		if editor.IDE != "claude" || editor.Model != "sonnet" || editor.ReasoningEffort != "high" {
			t.Fatalf("unexpected preseeded editor: %#v", editor)
		}
	})
}

func TestTaskRunRuntimeFormResolvesManifestDeclaredTaskGroupDirectory(t *testing.T) {
	// INVARIANT: a public initiative/TG-NNN reference resolves through the
	// manifest and never becomes a literal tasks-directory suffix.
	// OWNING_LAYER: unit. EXISTING_SUITE: internal/cli/form_test.go.
	t.Parallel()

	workspaceRoot := t.TempDir()
	initiative := "food-registration"
	initiativeDir := filepath.Join(workspaceRoot, ".compozy", "tasks", initiative)
	taskGroupDir := filepath.Join(initiativeDir, "_task_groups", "001-shared-foundation")
	if err := os.MkdirAll(taskGroupDir, 0o755); err != nil {
		t.Fatalf("mkdir task group directory: %v", err)
	}
	plan, err := taskgroups.RenderPlan(taskgroups.Plan{
		SchemaVersion: taskgroups.SchemaVersion,
		Initiative:    initiative,
		TaskGroups: []taskgroups.TaskGroup{{
			ID:         "TG-001",
			Title:      "Shared foundation",
			Outcome:    "Provide shared navigation primitives",
			Directory:  "_task_groups/001-shared-foundation",
			OwnedScope: []string{"navigation"},
		}},
	})
	if err != nil {
		t.Fatalf("render task group plan: %v", err)
	}
	if err := os.WriteFile(filepath.Join(initiativeDir, taskgroups.ManifestFileName), plan, 0o600); err != nil {
		t.Fatalf("write task group plan: %v", err)
	}
	writeFormTaskFile(t, taskGroupDir, "task_01.md", "pending")

	state := newCommandState(commandKindTasksRun, core.ModePRDTasks)
	state.workspaceRoot = workspaceRoot
	form, err := newTaskRunRuntimeFormForSlugs(t.Context(), state, []string{initiative + "/TG-001"})
	if err != nil {
		t.Fatalf("newTaskRunRuntimeFormForSlugs() error = %v", err)
	}
	if form == nil || len(form.taskOptions) != 1 {
		t.Fatalf("task options = %#v, want one manifest-resolved task", form)
	}
	if option := form.taskOptions[0]; option.Workflow != initiative+"/TG-001" || option.ID != "task_01" {
		t.Fatalf("task option = %#v, want task-group-scoped task_01", option)
	}

	legacyDir := filepath.Join(initiativeDir, "TG-999")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatalf("mkdir legacy directory: %v", err)
	}
	writeFormTaskFile(t, legacyDir, "task_99.md", "pending")
	_, err = newTaskRunRuntimeFormForSlugs(t.Context(), state, []string{initiative + "/TG-999"})
	if !errors.Is(err, taskgroups.ErrTaskGroupNotFound) {
		t.Fatalf("unknown task group error = %v, want ErrTaskGroupNotFound", err)
	}
}

func TestTaskRuntimeFormUsesRecursiveWalkerWhenEnabled(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	tasksDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "demo")
	nestedDir := filepath.Join(tasksDir, "features", "auth")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("mkdir nested dir: %v", err)
	}
	writeFormTaskFile(t, tasksDir, "task_01.md", "pending")
	writeFormTaskFile(t, nestedDir, "task_01.md", "pending")

	entries, err := readTaskRuntimeFormEntries(tasksDir, false, true)
	if err != nil {
		t.Fatalf("readTaskRuntimeFormEntries(recursive=true): %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("recursive walker should discover root + nested entries, got %d: %#v", len(entries), entries)
	}

	state := newCommandState(commandKindTasksRun, core.ModePRDTasks)
	state.workspaceRoot = workspaceRoot
	state.name = "demo"
	state.recursive = true

	form, err := newTaskRunRuntimeFormForSlugs(t.Context(), state, []string{"demo"})
	if err != nil {
		t.Fatalf("newTaskRunRuntimeFormForSlugs() error = %v", err)
	}
	if form == nil {
		t.Fatal("expected task runtime form")
		return
	}
	if len(form.taskOptions) != 2 {
		t.Fatalf("expected recursive form to discover 2 tasks, got %d: %#v", len(form.taskOptions), form.taskOptions)
	}
}

func TestTaskRuntimeFormUsesFlatWalkerByDefault(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	tasksDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "demo")
	nestedDir := filepath.Join(tasksDir, "features", "auth")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("mkdir nested dir: %v", err)
	}
	writeFormTaskFile(t, tasksDir, "task_01.md", "pending")
	writeFormTaskFile(t, nestedDir, "task_01.md", "pending")

	entries, err := readTaskRuntimeFormEntries(tasksDir, false, false)
	if err != nil {
		t.Fatalf("readTaskRuntimeFormEntries(recursive=false): %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("flat walker should ignore nested entries, got %d: %#v", len(entries), entries)
	}

	state := newCommandState(commandKindTasksRun, core.ModePRDTasks)
	state.workspaceRoot = workspaceRoot
	state.name = "demo"
	state.recursive = false

	form, err := newTaskRunRuntimeFormForSlugs(t.Context(), state, []string{"demo"})
	if err != nil {
		t.Fatalf("newTaskRunRuntimeFormForSlugs() error = %v", err)
	}
	if form == nil {
		t.Fatal("expected task runtime form")
		return
	}
	if len(form.taskOptions) != 1 {
		t.Fatalf("expected flat form to discover 1 task, got %d: %#v", len(form.taskOptions), form.taskOptions)
	}
}

func TestTaskRunFormInputsApplyMultipleWorkflowSelection(t *testing.T) {
	t.Parallel()

	t.Run("Should apply multiple workflow selection correctly", func(t *testing.T) {
		t.Parallel()

		state := newCommandState(commandKindTasksRun, core.ModePRDTasks)
		cmd := newTasksRunCommandWithDefaults(nil, defaultCommandStateDefaults())
		inputs := &taskRunFormInputs{
			selectedWorkflows:      []string{"alpha", "beta"},
			ide:                    "codex",
			model:                  "gpt-5.5",
			reasoningEffort:        "high",
			accessMode:             core.AccessModeFull,
			timeout:                "15m",
			tailLines:              "25",
			maxRetries:             "2",
			retryBackoffMultiplier: "2.25",
			dryRun:                 true,
			autoCommit:             true,
			includeCompleted:       true,
			recursive:              true,
		}

		if err := inputs.apply(cmd, state); err != nil {
			t.Fatalf("apply task run form inputs: %v", err)
		}

		if state.name != "" || state.multiple != "alpha,beta" {
			t.Fatalf("unexpected workflow selection state: name=%q multiple=%q", state.name, state.multiple)
		}
		for _, flag := range []string{
			"multiple",
			"ide",
			"model",
			"reasoning-effort",
			"access-mode",
			"timeout",
			"tail-lines",
			"max-retries",
			"retry-backoff-multiplier",
			"dry-run",
			"auto-commit",
			"include-completed",
			"recursive",
			"task-runtime",
		} {
			if !cmd.Flags().Changed(flag) {
				t.Fatalf("expected %s to be marked explicit", flag)
			}
		}
		if !state.dryRun || !state.autoCommit || !state.includeCompleted || !state.recursive {
			t.Fatalf("expected bool fields to apply, got %#v", state.runtimeConfig)
		}
		if state.tailLines != 25 || state.maxRetries != 2 || state.retryBackoffMultiplier != 2.25 {
			t.Fatalf("unexpected numeric fields: tail=%d retries=%d backoff=%f",
				state.tailLines,
				state.maxRetries,
				state.retryBackoffMultiplier,
			)
		}
		if !state.replaceConfiguredTaskRunRules {
			t.Fatal("expected task runtime rules to replace configured rules")
		}
	})
}

func TestTaskRunRuntimeFormScopesDuplicateTaskIDsByWorkflow(t *testing.T) {
	t.Parallel()

	t.Run("Should scope duplicate task IDs by workflow", func(t *testing.T) {
		t.Parallel()

		workspaceRoot := t.TempDir()
		for _, slug := range []string{"alpha", "beta"} {
			workflowDir := filepath.Join(workspaceRoot, ".compozy", "tasks", slug)
			if err := os.MkdirAll(workflowDir, 0o755); err != nil {
				t.Fatalf("mkdir %s: %v", slug, err)
			}
			writeFormTaskFile(t, workflowDir, "task_01.md", "pending")
		}

		state := newCommandState(commandKindTasksRun, core.ModePRDTasks)
		state.workspaceRoot = workspaceRoot
		state.ide = "codex"
		state.reasoningEffort = "medium"

		form, err := newTaskRunRuntimeFormForSlugs(t.Context(), state, []string{"alpha", "beta"})
		if err != nil {
			t.Fatalf("newTaskRunRuntimeFormForSlugs() error = %v", err)
		}
		if form == nil {
			t.Fatal("expected multi-workflow task runtime form")
			return
		}
		if len(form.taskOptions) != 2 {
			t.Fatalf("expected two task options, got %#v", form.taskOptions)
		}
		if form.taskOptions[0].Key != "alpha::task_01" || form.taskOptions[1].Key != "beta::task_01" {
			t.Fatalf("unexpected task option keys: %#v", form.taskOptions)
		}

		form.selectedTypes = []string{"beta::backend"}
		form.typeEditors["beta::backend"] = &taskRuntimeEditor{IDE: "claude", ReasoningEffort: "high"}
		form.selectedTasks = []string{"alpha::task_01"}
		form.taskEditors["alpha::task_01"] = &taskRuntimeEditor{Model: "alpha-model"}
		form.apply(state)

		if len(state.executionTaskRuntimeRules) != 2 {
			t.Fatalf("expected two workflow-scoped runtime rules, got %#v", state.executionTaskRuntimeRules)
		}
		typeRule := state.executionTaskRuntimeRules[0]
		if typeRule.Workflow == nil || *typeRule.Workflow != "beta" ||
			typeRule.Type == nil || *typeRule.Type != "backend" ||
			typeRule.IDE == nil || *typeRule.IDE != "claude" ||
			typeRule.ReasoningEffort == nil || *typeRule.ReasoningEffort != "high" {
			t.Fatalf("unexpected type rule: %#v", typeRule)
		}
		taskRule := state.executionTaskRuntimeRules[1]
		if taskRule.Workflow == nil || *taskRule.Workflow != "alpha" ||
			taskRule.ID == nil || *taskRule.ID != "task_01" ||
			taskRule.Model == nil || *taskRule.Model != "alpha-model" {
			t.Fatalf("unexpected task rule: %#v", taskRule)
		}
	})
}

func TestTaskRunRuntimeFormPreservesSingleWorkflowScopedRules(t *testing.T) {
	t.Parallel()

	t.Run("Should preselect and preserve workflow scoped rules for one workflow", func(t *testing.T) {
		t.Parallel()

		workspaceRoot := t.TempDir()
		workflowDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "alpha")
		if err := os.MkdirAll(workflowDir, 0o755); err != nil {
			t.Fatalf("mkdir alpha: %v", err)
		}
		writeFormTaskFile(t, workflowDir, "task_01.md", "pending")

		state := newCommandState(commandKindTasksRun, core.ModePRDTasks)
		state.workspaceRoot = workspaceRoot
		state.executionTaskRuntimeRules = []model.TaskRuntimeRule{
			{
				Workflow: stringPointer("alpha"),
				Type:     stringPointer("backend"),
				IDE:      stringPointer("claude"),
			},
			{
				Workflow: stringPointer("alpha"),
				ID:       stringPointer("task_01"),
				Model:    stringPointer("alpha-model"),
			},
		}

		form, err := newTaskRunRuntimeFormForSlugs(t.Context(), state, []string{"alpha"})
		if err != nil {
			t.Fatalf("newTaskRunRuntimeFormForSlugs() error = %v", err)
		}
		if form == nil {
			t.Fatal("expected single-workflow task runtime form")
			return
		}
		if !slices.Equal(form.selectedTypes, []string{"alpha::backend"}) {
			t.Fatalf("selected types = %#v, want alpha-scoped backend", form.selectedTypes)
		}
		if !slices.Equal(form.selectedTasks, []string{"alpha::task_01"}) {
			t.Fatalf("selected tasks = %#v, want alpha-scoped task", form.selectedTasks)
		}
		if form.typeOptions[0].Label != "backend" || strings.Contains(form.taskOptions[0].Label, "alpha /") {
			t.Fatalf("single-workflow labels should omit workflow prefix, got type=%q task=%q",
				form.typeOptions[0].Label,
				form.taskOptions[0].Label,
			)
		}

		form.apply(state)
		if len(state.executionTaskRuntimeRules) != 2 {
			t.Fatalf("expected two preserved workflow-scoped runtime rules, got %#v", state.executionTaskRuntimeRules)
		}
		for _, rule := range state.executionTaskRuntimeRules {
			if rule.Workflow == nil || *rule.Workflow != "alpha" {
				t.Fatalf("expected preserved alpha workflow on rule, got %#v", rule)
			}
		}
	})
}

func TestFormSelectOptionsOmitRecommendedSuffixes(t *testing.T) {
	t.Parallel()

	t.Run("ide field", func(t *testing.T) {
		t.Parallel()

		var selected string
		builder := newFormBuilder(
			newTasksRunCommandWithDefaults(nil, defaultCommandStateDefaults()),
			newCommandState(commandKindTasksRun, core.ModePRDTasks),
		)
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
		builder := newFormBuilder(
			newTasksRunCommandWithDefaults(nil, defaultCommandStateDefaults()),
			newCommandState(commandKindTasksRun, core.ModePRDTasks),
		)
		builder.addReasoningEffortField(&selected)

		view := renderSingleFormFieldForTest(t, builder.fields, "reasoning-effort")
		if !strings.Contains(view, "Medium") {
			t.Fatalf("expected reasoning selector to contain Medium, got %q", view)
		}
		if strings.Contains(view, "Medium (recommended)") {
			t.Fatalf("expected reasoning selector to omit recommended suffix, got %q", view)
		}
	})

	for _, tc := range []struct {
		value string
		label string
	}{
		{value: "max", label: "Maximum"},
		{value: "ultra", label: "Ultra"},
	} {
		tc := tc
		t.Run("Should include "+tc.label+" in reasoning effort field", func(t *testing.T) {
			t.Parallel()

			selected := tc.value
			builder := newFormBuilder(
				newTasksRunCommandWithDefaults(nil, defaultCommandStateDefaults()),
				newCommandState(commandKindTasksRun, core.ModePRDTasks),
			)
			builder.addReasoningEffortField(&selected)

			if len(builder.fields) != 1 {
				t.Fatalf("expected one reasoning field, got %d", len(builder.fields))
			}
			field := builder.fields[0]
			if got := field.GetValue(); got != tc.value {
				t.Fatalf("reasoning field value = %#v, want %q", got, tc.value)
			}
			assertFieldViewContains(t, field, tc.label)
		})
	}
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
		builder := newFormBuilder(
			newTasksRunCommandWithDefaults(nil, defaultCommandStateDefaults()),
			newCommandState(commandKindTasksRun, core.ModePRDTasks),
		)
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
