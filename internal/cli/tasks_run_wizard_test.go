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
	wizard = updateTaskRunWizardTestModel(t, wizard, "space")
	if !wizard.inputs.defineTaskRuntime {
		t.Fatal("expected runtime-per-task toggle to be enabled")
	}
	wizard = updateTaskRunWizardTestModel(t, wizard, "enter")
	if wizard.step != taskRunWizardStepReview {
		t.Fatalf("step = %v, want review", wizard.step)
	}
	wizard = updateTaskRunWizardTestModel(t, wizard, "enter")
	if !wizard.submitted {
		t.Fatal("expected wizard to submit from review step")
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
