package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	core "github.com/compozy/compozy/internal/core"
)

func TestTaskRunWizardModel(t *testing.T) {
	t.Parallel()

	t.Run("Should select multiple workflows and submit", func(t *testing.T) {
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
	})

	t.Run("Should preserve and reorder workflow selection", func(t *testing.T) {
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
	})

	t.Run("Should build workflow scoped overrides", func(t *testing.T) {
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
			return
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
	})

	t.Run("Should preserve overrides across back navigation", func(t *testing.T) {
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
			return
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
	})

	t.Run("Should accept global shortcut characters in workflow input", func(t *testing.T) {
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
	})

	t.Run("Should accept navigation letters in runtime text input", func(t *testing.T) {
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
	})

	t.Run("Should accept navigation letters in override text input", func(t *testing.T) {
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
			return
		}
		if editor.Model != "haiku" {
			t.Fatalf("override model = %q, want haiku", editor.Model)
		}
		if wizard.overrideEditorCursor != taskRunWizardOverrideFieldModel {
			t.Fatalf("override editor cursor = %v, want model field", wizard.overrideEditorCursor)
		}
	})

	t.Run("Should filter workflow selection", func(t *testing.T) {
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
	})

	t.Run("Should accept manual workflow fallback", func(t *testing.T) {
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
	})

	t.Run("Should configure recovery controls", func(t *testing.T) {
		t.Parallel()

		state := newTaskRunWizardTestState(t, "alpha")
		wizard := newTaskRunWizardModel(state, taskRunFormInputs{
			selectedWorkflows: []string{"alpha"},
			ide:               "codex",
			reasoningEffort:   "medium",
			accessMode:        core.AccessModeFull,
		})
		wizard.ideOptions = []taskRunWizardChoice{
			{Label: "Codex", Value: "codex"},
			{Label: "Claude", Value: "claude"},
		}
		wizard.inputs.recoveryIDE = "codex"
		wizard.step = taskRunWizardStepExecution
		wizard.execCursor = taskRunWizardFieldRecoveryEnabled
		wizard.syncTextFocus()

		wizard = updateTaskRunWizardTestModel(t, wizard, "space")
		if !wizard.inputs.recoveryEnabled {
			t.Fatal("expected recovery toggle to enable recovery")
		}

		wizard.execCursor = taskRunWizardFieldRecoveryIDE
		wizard = updateTaskRunWizardTestModel(t, wizard, "right")
		if wizard.inputs.recoveryIDE != "claude" {
			t.Fatalf("recovery IDE = %q, want claude", wizard.inputs.recoveryIDE)
		}

		wizard.execCursor = taskRunWizardFieldRecoveryReasoning
		wizard.inputs.recoveryReasoning = "medium"
		wizard = updateTaskRunWizardTestModel(t, wizard, "right")
		if wizard.inputs.recoveryReasoning != "high" {
			t.Fatalf("recovery reasoning = %q, want high", wizard.inputs.recoveryReasoning)
		}

		wizard.execCursor = taskRunWizardFieldRecoveryModel
		wizard.inputs.recoveryModel = ""
		wizard.textInputs.recoveryModel.SetValue("")
		wizard.syncTextFocus()
		for _, key := range []string{"o", "3"} {
			wizard = updateTaskRunWizardTestModel(t, wizard, key)
		}
		if wizard.inputs.recoveryModel != "o3" {
			t.Fatalf("recovery model = %q, want o3", wizard.inputs.recoveryModel)
		}

		cmd := newTasksRunCommandWithDefaults(nil, defaultCommandStateDefaults())
		appliedState := newCommandState(commandKindTasksRun, core.ModePRDTasks)
		if err := wizard.inputs.apply(cmd, appliedState); err != nil {
			t.Fatalf("apply recovery wizard inputs: %v", err)
		}
		if !appliedState.recoveryEnabled ||
			appliedState.recoveryIDE != "claude" ||
			appliedState.recoveryModel != "o3" ||
			appliedState.recoveryReasoningEffort != "high" {
			t.Fatalf("unexpected applied recovery state: %#v", appliedState)
		}
		for _, flag := range []string{"recovery", "recovery-ide", "recovery-model", "recovery-reasoning"} {
			if !cmd.Flags().Changed(flag) {
				t.Fatalf("expected %s to be marked explicit", flag)
			}
		}
	})

	t.Run("Should configure parallel task controls", func(t *testing.T) {
		t.Parallel()

		state := newTaskRunWizardTestState(t, "alpha")
		wizard := newTaskRunWizardModel(state, taskRunFormInputs{
			selectedWorkflows: []string{"alpha"},
			ide:               "codex",
			reasoningEffort:   "medium",
			accessMode:        core.AccessModeFull,
		})
		wizard.ideOptions = []taskRunWizardChoice{
			{Label: "Codex", Value: "codex"},
			{Label: "Claude", Value: "claude"},
		}
		wizard.inputs.parallelResolverIDE = "codex"
		wizard.inputs.parallelResolverReasoning = "medium"
		wizard.step = taskRunWizardStepExecution
		wizard.execCursor = taskRunWizardFieldParallelTasks
		wizard.syncTextFocus()

		const renderHeight = 60
		if strings.Contains(wizard.renderExecutionStep(renderHeight), "Conflict resolver IDE") {
			t.Fatal("parallel resolver controls should be hidden while parallel tasks are disabled")
		}

		wizard = updateTaskRunWizardTestModel(t, wizard, "space")
		if !wizard.inputs.parallelTasks {
			t.Fatal("expected parallel task toggle to enable parallel tasks")
		}
		if !strings.Contains(wizard.renderExecutionStep(renderHeight), "Conflict resolver IDE") {
			t.Fatal("expected resolver controls when parallel tasks are enabled")
		}

		wizard.execCursor = taskRunWizardFieldParallelResolverIDE
		wizard = updateTaskRunWizardTestModel(t, wizard, "right")
		if wizard.inputs.parallelResolverIDE != "claude" {
			t.Fatalf("parallel resolver IDE = %q, want claude", wizard.inputs.parallelResolverIDE)
		}

		wizard.execCursor = taskRunWizardFieldParallelResolverReasoning
		wizard = updateTaskRunWizardTestModel(t, wizard, "right")
		if wizard.inputs.parallelResolverReasoning != "high" {
			t.Fatalf("parallel resolver reasoning = %q, want high", wizard.inputs.parallelResolverReasoning)
		}

		wizard.execCursor = taskRunWizardFieldParallelResolverModel
		wizard.inputs.parallelResolverModel = ""
		wizard.textInputs.parallelResolverModel.SetValue("")
		wizard.syncTextFocus()
		for _, key := range []string{"o", "3"} {
			wizard = updateTaskRunWizardTestModel(t, wizard, key)
		}
		if wizard.inputs.parallelResolverModel != "o3" {
			t.Fatalf("parallel resolver model = %q, want o3", wizard.inputs.parallelResolverModel)
		}

		cmd := newTasksRunCommandWithDefaults(nil, defaultCommandStateDefaults())
		appliedState := newCommandState(commandKindTasksRun, core.ModePRDTasks)
		if err := wizard.inputs.apply(cmd, appliedState); err != nil {
			t.Fatalf("apply parallel wizard inputs: %v", err)
		}
		if !appliedState.parallelTasks ||
			appliedState.parallelConflictResolverIDE != "claude" ||
			appliedState.parallelConflictResolverModel != "o3" ||
			appliedState.parallelConflictResolverReasoningEffort != "high" {
			t.Fatalf("unexpected applied parallel state: %#v", appliedState)
		}
		for _, flag := range []string{
			taskRunParallelTasksFlag,
			taskRunParallelConflictResolverIDEFlag,
			taskRunParallelConflictResolverModelFlag,
			taskRunParallelConflictResolverReasoningFlag,
		} {
			if !cmd.Flags().Changed(flag) {
				t.Fatalf("expected %s to be marked explicit", flag)
			}
		}
	})

	t.Run("Should hide parallel workflow controls for a single workflow", func(t *testing.T) {
		t.Parallel()

		state := newTaskRunWizardTestState(t, "alpha")
		wizard := newTaskRunWizardModel(state, taskRunFormInputs{
			selectedWorkflows: []string{"alpha"},
		})
		wizard.step = taskRunWizardStepExecution

		if slices.Contains(wizard.executionFields(), taskRunWizardFieldParallelWorkflows) {
			t.Fatal("parallel workflow control should be hidden for a single workflow")
		}
		if strings.Contains(wizard.renderExecutionStep(60), "Run Parallel Workflows") {
			t.Fatal("parallel workflow row should not render for a single workflow")
		}
	})

	t.Run("Should clear stale parallel workflow state for a single workflow", func(t *testing.T) {
		t.Parallel()

		cmd := newTasksRunCommandWithDefaults(nil, defaultCommandStateDefaults())
		appliedState := newCommandState(commandKindTasksRun, core.ModePRDTasks)
		appliedState.parallel = true
		appliedState.parallelLimit = 3

		inputs := taskRunFormInputs{selectedWorkflows: []string{"alpha"}}
		inputs.applyParallelControls(cmd, appliedState)
		if appliedState.parallel || appliedState.parallelLimit != 0 {
			t.Fatalf(
				"parallel state = parallel:%v limit:%d, want cleared",
				appliedState.parallel,
				appliedState.parallelLimit,
			)
		}
		if cmd.Flags().Changed("parallel") || cmd.Flags().Changed("parallel-limit") {
			t.Fatal("single workflow should not mark inter-workflow parallel flags as changed")
		}
	})

	t.Run("Should clear stale parallel limit when parallel workflows are disabled", func(t *testing.T) {
		t.Parallel()

		cmd := newTasksRunCommandWithDefaults(nil, defaultCommandStateDefaults())
		markInputFlagChanged(cmd, "parallel-limit")
		appliedState := newCommandState(commandKindTasksRun, core.ModePRDTasks)
		appliedState.parallel = true
		appliedState.parallelLimit = 3

		inputs := taskRunFormInputs{
			selectedWorkflows:     []string{"alpha", "beta"},
			parallelWorkflows:     false,
			parallelWorkflowLimit: "3",
		}
		inputs.applyParallelControls(cmd, appliedState)
		if appliedState.parallel || appliedState.parallelLimit != 0 {
			t.Fatalf(
				"parallel state = parallel:%v limit:%d, want serial mode with cleared limit",
				appliedState.parallel,
				appliedState.parallelLimit,
			)
		}
		if !cmd.Flags().Changed("parallel") {
			t.Fatal("serial multi-workflow mode should mark parallel as explicit")
		}
		if cmd.Flags().Changed("parallel-limit") {
			t.Fatal("serial multi-workflow mode should clear stale parallel-limit flag state")
		}
	})

	t.Run("Should configure parallel workflow controls", func(t *testing.T) {
		t.Parallel()

		state := newTaskRunWizardTestState(t, "alpha", "beta")
		wizard := newTaskRunWizardModel(state, taskRunFormInputs{
			selectedWorkflows: []string{"alpha", "beta"},
			ide:               "codex",
			reasoningEffort:   "medium",
			accessMode:        core.AccessModeFull,
		})
		wizard.step = taskRunWizardStepExecution
		wizard.syncTextFocus()

		if !slices.Contains(wizard.executionFields(), taskRunWizardFieldParallelWorkflows) {
			t.Fatal("expected parallel workflow control for multiple workflows")
		}
		if strings.Contains(wizard.renderExecutionStep(60), "Max concurrent") {
			t.Fatal("max concurrent should be hidden while parallel workflows are disabled")
		}

		wizard.execCursor = taskRunWizardFieldParallelWorkflows
		wizard = updateTaskRunWizardTestModel(t, wizard, "space")
		if !wizard.inputs.parallelWorkflows {
			t.Fatal("expected parallel workflow toggle to enable parallel workflows")
		}
		if !slices.Contains(wizard.executionFields(), taskRunWizardFieldParallelWorkflowLimit) {
			t.Fatal("expected max concurrent control once parallel workflows are enabled")
		}
		if !strings.Contains(wizard.renderExecutionStep(60), "Max concurrent") {
			t.Fatal("expected max concurrent row when parallel workflows are enabled")
		}

		wizard.execCursor = taskRunWizardFieldParallelWorkflowLimit
		wizard.inputs.parallelWorkflowLimit = ""
		wizard.textInputs.parallelWorkflowLimit.SetValue("")
		wizard.syncTextFocus()
		wizard = updateTaskRunWizardTestModel(t, wizard, "3")
		if wizard.inputs.parallelWorkflowLimit != "3" {
			t.Fatalf("max concurrent = %q, want 3", wizard.inputs.parallelWorkflowLimit)
		}

		cmd := newTasksRunCommandWithDefaults(nil, defaultCommandStateDefaults())
		appliedState := newCommandState(commandKindTasksRun, core.ModePRDTasks)
		if err := wizard.inputs.apply(cmd, appliedState); err != nil {
			t.Fatalf("apply parallel workflow wizard inputs: %v", err)
		}
		if !appliedState.parallel || appliedState.parallelLimit != 3 {
			t.Fatalf("unexpected applied parallel workflow state: parallel=%v limit=%d",
				appliedState.parallel, appliedState.parallelLimit)
		}
		for _, flag := range []string{"parallel", "parallel-limit"} {
			if !cmd.Flags().Changed(flag) {
				t.Fatalf("expected %s to be marked explicit", flag)
			}
		}
	})
}

// TestTaskRunWizardViewFitsTerminalBounds guards the layout invariant that the
// rendered wizard never emits a line wider than the terminal nor more lines than
// the terminal height. A regression here produces wrapped dividers and vertical
// scroll (the symptom that motivated the inline -> alt-screen + width fixes).
func TestTaskRunWizardView(t *testing.T) {
	t.Parallel()

	t.Run("Should fit terminal bounds across breakpoints", func(t *testing.T) {
		t.Parallel()

		state := newTaskRunWizardTestState(t, "alpha", "beta", "gamma")
		dims := []struct {
			name string
			w, h int
		}{
			{"Should fit minimum terminal bounds", 72, 22},
			{"Should fit standard terminal bounds", 80, 24},
			{"Should fit wide terminal bounds", 120, 40},
			{"Should fit ultrawide terminal bounds", 200, 50},
		}
		steps := []taskRunWizardStep{
			taskRunWizardStepWorkflows,
			taskRunWizardStepRuntime,
			taskRunWizardStepExecution,
			taskRunWizardStepOverrides,
			taskRunWizardStepReview,
		}
		for _, dim := range dims {
			dim := dim
			t.Run(dim.name, func(t *testing.T) {
				t.Parallel()
				wizard := newTaskRunWizardModel(state, taskRunFormInputs{})
				updated, _ := wizard.Update(tea.WindowSizeMsg{Width: dim.w, Height: dim.h})
				typed, ok := updated.(*taskRunWizardModel)
				if !ok {
					t.Fatalf("resize model type = %T, want *taskRunWizardModel", updated)
				}
				typed = updateTaskRunWizardTestModel(t, typed, "space")
				for _, step := range steps {
					typed.step = step
					typed.syncTextFocus()
					assertTaskRunWizardViewFits(t, typed, dim.w, dim.h)
				}
				typed.step = taskRunWizardStepWorkflows
				typed.showHelp = true
				assertTaskRunWizardViewFits(t, typed, dim.w, dim.h)
			})
		}
	})

	t.Run("Should fit bounds with every execution section expanded", func(t *testing.T) {
		t.Parallel()

		dims := []struct {
			name string
			w, h int
		}{
			{name: "Should fit expanded execution sections at minimum terminal bounds", w: 72, h: 22},
			{name: "Should fit expanded execution sections at standard terminal bounds", w: 80, h: 24},
			{name: "Should fit expanded execution sections at wide terminal bounds", w: 120, h: 40},
		}
		for _, dim := range dims {
			dim := dim
			t.Run(dim.name, func(t *testing.T) {
				t.Parallel()
				state := newTaskRunWizardTestState(t, "alpha", "beta")
				wizard := newTaskRunWizardModel(state, taskRunFormInputs{
					selectedWorkflows: []string{"alpha", "beta"},
					parallelTasks:     true,
					parallelWorkflows: true,
					recoveryEnabled:   true,
				})
				updated, _ := wizard.Update(tea.WindowSizeMsg{Width: dim.w, Height: dim.h})
				typed, ok := updated.(*taskRunWizardModel)
				if !ok {
					t.Fatalf("resize model type = %T, want *taskRunWizardModel", updated)
				}
				typed.step = taskRunWizardStepExecution
				// Focus the last field so the scroll window's lower bound is exercised.
				typed.execCursor = taskRunWizardFieldDefineRuntime
				typed.syncTextFocus()
				assertTaskRunWizardViewFits(t, typed, dim.w, dim.h)
			})
		}
	})

	t.Run("Should keep focus visible when focus span exceeds content height", func(t *testing.T) {
		t.Parallel()

		start, end, _, _ := wizardScrollWindow(20, 4, 8, 4)
		if start > 4 || end <= 4 {
			t.Fatalf("window [%d,%d) does not contain focus line 4", start, end)
		}
	})
}

func assertTaskRunWizardViewFits(t *testing.T, wizard *taskRunWizardModel, width, height int) {
	t.Helper()
	lines := strings.Split(wizard.View().Content, "\n")
	if len(lines) > height {
		t.Fatalf("view rendered %d lines, want <= %d (step=%d help=%v)",
			len(lines), height, wizard.step, wizard.showHelp)
	}
	for i, line := range lines {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("line %d width %d exceeds terminal width %d (step=%d help=%v): %q",
				i, got, width, wizard.step, wizard.showHelp, line)
		}
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
