package cli

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"
	core "github.com/compozy/compozy/internal/core"
)

func TestTaskRunWizardModelSelectsMultipleWorkflowsAndSubmits(t *testing.T) {
	t.Parallel()

	state := newTaskRunWizardTestState(t, "alpha", "beta")
	wizard := newTaskRunWizardModel(state, taskRunFormInputs{
		ide:             "codex",
		reasoningEffort: "medium",
		accessMode:      core.AccessModeFull,
	})

	wizard = updateTaskRunWizardTestModel(t, wizard, "space")
	wizard = updateTaskRunWizardTestModel(t, wizard, "down")
	wizard = updateTaskRunWizardTestModel(t, wizard, "space")
	wizard = updateTaskRunWizardTestModel(t, wizard, "enter")

	if wizard.step != taskRunWizardStepRuntime {
		t.Fatalf("step = %v, want runtime", wizard.step)
	}
	if !slices.Equal(wizard.inputs.selectedWorkflows, []string{"alpha", "beta"}) {
		t.Fatalf("selected workflows = %#v, want [alpha beta]", wizard.inputs.selectedWorkflows)
	}

	wizard.runtimeCursor = taskRunWizardFieldReasoning
	wizard = updateTaskRunWizardTestModel(t, wizard, "right")
	if wizard.step != taskRunWizardStepRuntime {
		t.Fatalf("right should cycle runtime choice without changing step, got %v", wizard.step)
	}
	if wizard.inputs.reasoningEffort != "high" {
		t.Fatalf("reasoning effort = %q, want high", wizard.inputs.reasoningEffort)
	}
	wizard = updateTaskRunWizardTestModel(t, wizard, "left")
	if wizard.inputs.reasoningEffort != "medium" {
		t.Fatalf("reasoning effort after left = %q, want medium", wizard.inputs.reasoningEffort)
	}

	wizard.runtimeCursor = taskRunWizardFieldAccessMode
	wizard = updateTaskRunWizardTestModel(t, wizard, "enter")
	if wizard.step != taskRunWizardStepExecution {
		t.Fatalf("step = %v, want execution", wizard.step)
	}
	wizard.execCursor = taskRunWizardFieldDefineRuntime
	wizard = updateTaskRunWizardTestModel(t, wizard, "enter")
	if wizard.step != taskRunWizardStepReview {
		t.Fatalf("step = %v, want review", wizard.step)
	}
	wizard = updateTaskRunWizardTestModel(t, wizard, "enter")
	if !wizard.submitted {
		t.Fatal("expected wizard to submit from review step")
	}
}

func TestTaskRunWizardModelPreservesAndReordersWorkflowSelection(t *testing.T) {
	t.Parallel()

	state := newTaskRunWizardTestState(t, "alpha", "beta", "gamma")
	wizard := newTaskRunWizardModel(state, taskRunFormInputs{})

	wizard = updateTaskRunWizardTestModel(t, wizard, "down")
	wizard = updateTaskRunWizardTestModel(t, wizard, "space")
	wizard = updateTaskRunWizardTestModel(t, wizard, "up")
	wizard = updateTaskRunWizardTestModel(t, wizard, "space")

	if !slices.Equal(wizard.inputs.selectedWorkflows, []string{"beta", "alpha"}) {
		t.Fatalf("selected workflows = %#v, want [beta alpha]", wizard.inputs.selectedWorkflows)
	}

	wizard = updateTaskRunWizardTestModel(t, wizard, "right")
	wizard = updateTaskRunWizardTestModel(t, wizard, "d")
	if !slices.Equal(wizard.inputs.selectedWorkflows, []string{"alpha", "beta"}) {
		t.Fatalf("reordered workflows = %#v, want [alpha beta]", wizard.inputs.selectedWorkflows)
	}

	wizard = updateTaskRunWizardTestModel(t, wizard, "u")
	if !slices.Equal(wizard.inputs.selectedWorkflows, []string{"beta", "alpha"}) {
		t.Fatalf("reordered workflows = %#v, want [beta alpha]", wizard.inputs.selectedWorkflows)
	}
}

func TestTaskRunWizardModelBuildsWorkflowScopedOverrides(t *testing.T) {
	t.Parallel()

	state := newTaskRunWizardTestState(t, "alpha", "beta")
	for _, slug := range []string{"alpha", "beta"} {
		writeFormTaskFile(t, filepath.Join(state.workspaceRoot, ".compozy", "tasks", slug), "task_01.md", "pending")
	}
	wizard := newTaskRunWizardModel(state, taskRunFormInputs{
		ide:             "codex",
		reasoningEffort: "medium",
		accessMode:      core.AccessModeFull,
	})

	wizard = updateTaskRunWizardTestModel(t, wizard, "space")
	wizard = updateTaskRunWizardTestModel(t, wizard, "down")
	wizard = updateTaskRunWizardTestModel(t, wizard, "space")
	wizard = updateTaskRunWizardTestModel(t, wizard, "enter")
	wizard.runtimeCursor = taskRunWizardFieldAccessMode
	wizard = updateTaskRunWizardTestModel(t, wizard, "enter")
	wizard.execCursor = taskRunWizardFieldDefineRuntime
	wizard = updateTaskRunWizardTestModel(t, wizard, "space")
	wizard = updateTaskRunWizardTestModelWithCmd(t, wizard, "enter")

	if wizard.step != taskRunWizardStepOverrides {
		t.Fatalf("step = %v, want overrides", wizard.step)
	}
	if wizard.runtimeForm == nil {
		t.Fatal("expected runtime form state to load")
	}
	target, ok := wizard.currentOverrideTarget()
	if !ok {
		t.Fatal("expected current override target")
	}
	if target.Key != "alpha::backend" {
		t.Fatalf("target key = %q, want alpha::backend", target.Key)
	}
	wizard.toggleOverrideTarget(target)
	editor := wizard.currentOverrideEditor()
	if editor == nil {
		t.Fatal("expected current override editor after selecting target")
	}
	editor.IDE = "claude"
	editor.ReasoningEffort = "high"

	wizard = updateTaskRunWizardTestModel(t, wizard, "enter")
	if wizard.step != taskRunWizardStepReview {
		t.Fatalf("step = %v, want review", wizard.step)
	}
	if len(wizard.inputs.taskRuntimeRules) != 1 {
		t.Fatalf("task runtime rules = %#v, want one rule", wizard.inputs.taskRuntimeRules)
	}
	rule := wizard.inputs.taskRuntimeRules[0]
	if rule.Workflow == nil || *rule.Workflow != "alpha" ||
		rule.Type == nil || *rule.Type != "backend" ||
		rule.IDE == nil || *rule.IDE != "claude" ||
		rule.ReasoningEffort == nil || *rule.ReasoningEffort != "high" {
		t.Fatalf("unexpected rule: %#v", rule)
	}
}

func TestTaskRunWizardModelPreservesOverridesAcrossBackNavigation(t *testing.T) {
	t.Parallel()

	state := newTaskRunWizardTestState(t, "alpha", "beta")
	for _, slug := range []string{"alpha", "beta"} {
		writeFormTaskFile(t, filepath.Join(state.workspaceRoot, ".compozy", "tasks", slug), "task_01.md", "pending")
	}
	wizard := newTaskRunWizardModel(state, taskRunFormInputs{
		ide:             "codex",
		reasoningEffort: "medium",
		accessMode:      core.AccessModeFull,
	})
	wizard = updateTaskRunWizardTestModel(t, wizard, "space")
	wizard = updateTaskRunWizardTestModel(t, wizard, "down")
	wizard = updateTaskRunWizardTestModel(t, wizard, "space")
	wizard = updateTaskRunWizardTestModel(t, wizard, "enter")
	wizard.runtimeCursor = taskRunWizardFieldAccessMode
	wizard = updateTaskRunWizardTestModel(t, wizard, "enter")
	wizard.execCursor = taskRunWizardFieldDefineRuntime
	wizard = updateTaskRunWizardTestModel(t, wizard, "space")
	wizard = updateTaskRunWizardTestModelWithCmd(t, wizard, "enter")

	target, ok := wizard.currentOverrideTarget()
	if !ok {
		t.Fatal("expected current override target")
	}
	wizard.toggleOverrideTarget(target)
	editor := wizard.currentOverrideEditor()
	if editor == nil {
		t.Fatal("expected current override editor after selecting target")
	}
	editor.IDE = "claude"

	wizard = updateTaskRunWizardTestModel(t, wizard, "esc")
	if wizard.step != taskRunWizardStepExecution {
		t.Fatalf("step = %v, want execution after back", wizard.step)
	}
	wizard = updateTaskRunWizardTestModelWithCmd(t, wizard, "enter")
	if wizard.step != taskRunWizardStepOverrides {
		t.Fatalf("step = %v, want overrides after forward", wizard.step)
	}
	target, ok = wizard.currentOverrideTarget()
	if !ok || !wizard.overrideTargetSelected(target) {
		t.Fatalf("expected override target to remain selected, target=%#v ok=%v", target, ok)
	}
	editor = wizard.currentOverrideEditor()
	if editor == nil || editor.IDE != "claude" {
		t.Fatalf("override editor = %#v, want preserved IDE claude", editor)
	}
}

func TestTaskRunWizardTextInputAcceptsGlobalShortcutCharacters(t *testing.T) {
	t.Parallel()

	state := newTaskRunWizardTestState(t)
	wizard := newTaskRunWizardModel(state, taskRunFormInputs{})

	wizard = updateTaskRunWizardTestModel(t, wizard, "q")
	wizard = updateTaskRunWizardTestModel(t, wizard, "?")

	if wizard.canceled {
		t.Fatal("text input q should not cancel the wizard")
	}
	if wizard.showHelp {
		t.Fatal("text input ? should not open help")
	}
	if got := wizard.textInputs.manualWorkflow.Value(); got != "q?" {
		t.Fatalf("manual workflow value = %q, want q?", got)
	}
}

func TestTaskRunWizardRuntimeTextInputAcceptsNavigationLetters(t *testing.T) {
	t.Parallel()

	state := newTaskRunWizardTestState(t, "alpha")
	wizard := newTaskRunWizardModel(state, taskRunFormInputs{})
	wizard.runtimeCursor = taskRunWizardFieldModel
	wizard.step = taskRunWizardStepRuntime
	wizard.syncTextFocus()

	for _, key := range []string{"h", "a", "i", "k", "u"} {
		wizard = updateTaskRunWizardTestModel(t, wizard, key)
	}

	if wizard.inputs.model != "haiku" {
		t.Fatalf("runtime model = %q, want haiku", wizard.inputs.model)
	}
	if wizard.runtimeCursor != taskRunWizardFieldModel {
		t.Fatalf("runtime cursor = %v, want model field", wizard.runtimeCursor)
	}
}

func TestTaskRunWizardOverrideTextInputAcceptsNavigationLetters(t *testing.T) {
	t.Parallel()

	state := newTaskRunWizardTestState(t, "alpha", "beta")
	for _, slug := range []string{"alpha", "beta"} {
		writeFormTaskFile(t, filepath.Join(state.workspaceRoot, ".compozy", "tasks", slug), "task_01.md", "pending")
	}
	wizard := newTaskRunWizardModel(state, taskRunFormInputs{
		ide:             "codex",
		reasoningEffort: "medium",
		accessMode:      core.AccessModeFull,
	})
	wizard = updateTaskRunWizardTestModel(t, wizard, "space")
	wizard = updateTaskRunWizardTestModel(t, wizard, "down")
	wizard = updateTaskRunWizardTestModel(t, wizard, "space")
	wizard = updateTaskRunWizardTestModel(t, wizard, "enter")
	wizard.runtimeCursor = taskRunWizardFieldAccessMode
	wizard = updateTaskRunWizardTestModel(t, wizard, "enter")
	wizard.execCursor = taskRunWizardFieldDefineRuntime
	wizard = updateTaskRunWizardTestModel(t, wizard, "space")
	wizard = updateTaskRunWizardTestModelWithCmd(t, wizard, "enter")

	target, ok := wizard.currentOverrideTarget()
	if !ok {
		t.Fatal("expected current override target")
	}
	wizard.toggleOverrideTarget(target)
	wizard.overrideFocus = taskRunWizardOverrideFocusEditor
	wizard.overrideEditorCursor = taskRunWizardOverrideFieldModel
	wizard.syncTextFocus()
	for _, key := range []string{"h", "a", "i", "k", "u"} {
		wizard = updateTaskRunWizardTestModel(t, wizard, key)
	}

	editor := wizard.currentOverrideEditor()
	if editor == nil {
		t.Fatal("expected current override editor")
	}
	if editor.Model != "haiku" {
		t.Fatalf("override model = %q, want haiku", editor.Model)
	}
	if wizard.overrideEditorCursor != taskRunWizardOverrideFieldModel {
		t.Fatalf("override editor cursor = %v, want model field", wizard.overrideEditorCursor)
	}
}

func TestTaskRunWizardModelFiltersWorkflowSelection(t *testing.T) {
	t.Parallel()

	state := newTaskRunWizardTestState(t, "alpha", "beta", "gamma")
	wizard := newTaskRunWizardModel(state, taskRunFormInputs{})

	wizard = updateTaskRunWizardTestModel(t, wizard, "/")
	wizard = updateTaskRunWizardTestModel(t, wizard, "t")
	wizard = updateTaskRunWizardTestModel(t, wizard, "a")
	wizard = updateTaskRunWizardTestModel(t, wizard, "enter")
	wizard = updateTaskRunWizardTestModel(t, wizard, "a")

	if !slices.Equal(wizard.inputs.selectedWorkflows, []string{"beta"}) {
		t.Fatalf("selected workflows = %#v, want [beta]", wizard.inputs.selectedWorkflows)
	}
}

func TestTaskRunWizardModelAcceptsManualWorkflowFallback(t *testing.T) {
	t.Parallel()

	state := newTaskRunWizardTestState(t)
	wizard := newTaskRunWizardModel(state, taskRunFormInputs{})
	wizard.textInputs.manualWorkflow.SetValue("manual")

	wizard = updateTaskRunWizardTestModel(t, wizard, "enter")

	if wizard.step != taskRunWizardStepRuntime {
		t.Fatalf("step = %v, want runtime", wizard.step)
	}
	if !slices.Equal(selectedTaskRunWizardWorkflows(wizard.inputs), []string{"manual"}) {
		t.Fatalf("selected workflows = %#v, want [manual]", selectedTaskRunWizardWorkflows(wizard.inputs))
	}
}

func newTaskRunWizardTestState(t *testing.T, slugs ...string) *commandState {
	t.Helper()

	workspaceRoot := t.TempDir()
	for _, slug := range slugs {
		dir := filepath.Join(workspaceRoot, ".compozy", "tasks", slug)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir workflow dir %q: %v", slug, err)
		}
	}
	state := newCommandState(commandKindTasksRun, core.ModePRDTasks)
	state.workspaceRoot = workspaceRoot
	return state
}

func updateTaskRunWizardTestModel(t *testing.T, wizard *taskRunWizardModel, key string) *taskRunWizardModel {
	t.Helper()

	updated, _ := wizard.Update(taskRunWizardTestKey(key))
	typed, ok := updated.(*taskRunWizardModel)
	if !ok {
		t.Fatalf("updated model type = %T, want *taskRunWizardModel", updated)
	}
	return typed
}

func updateTaskRunWizardTestModelWithCmd(t *testing.T, wizard *taskRunWizardModel, key string) *taskRunWizardModel {
	t.Helper()

	updated, cmd := wizard.Update(taskRunWizardTestKey(key))
	typed, ok := updated.(*taskRunWizardModel)
	if !ok {
		t.Fatalf("updated model type = %T, want *taskRunWizardModel", updated)
	}
	if cmd == nil {
		return typed
	}
	loaded, _ := typed.Update(cmd())
	typed, ok = loaded.(*taskRunWizardModel)
	if !ok {
		t.Fatalf("loaded model type = %T, want *taskRunWizardModel", loaded)
	}
	return typed
}

func taskRunWizardTestKey(key string) tea.KeyPressMsg {
	switch key {
	case "backspace":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyBackspace})
	case "down":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyDown})
	case "enter":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter})
	case "esc":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc})
	case "left":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft})
	case "right":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyRight})
	case "shift+tab":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyTab, Mod: tea.ModShift})
	case "space":
		return tea.KeyPressMsg(tea.Key{Text: " ", Code: tea.KeySpace})
	case "tab":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyTab})
	case "up":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyUp})
	default:
		runes := []rune(key)
		if len(runes) != 1 {
			panic("unsupported test key: " + key)
		}
		return tea.KeyPressMsg(tea.Key{Text: key, Code: runes[0]})
	}
}
