// Suite: task-run interactive wizard
// Invariant: visible wizard state and keyboard actions preserve task-run selection semantics.
// Boundary IN: task files, Task Group plans, daemon run summaries, and Bubble Tea updates/views.
// Boundary OUT: daemon transport and executor behavior, covered by daemon and task-run integration suites.
package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
	apicore "github.com/compozy/compozy/internal/api/core"
	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/taskgroups"
)

func newTaskRunWizardModel(state *commandState, inputs taskRunFormInputs) *taskRunWizardModel {
	return newTaskRunWizardModelWithContext(context.Background(), state, inputs)
}

func TestTaskRunWizardWorkflowStatuses(t *testing.T) {
	t.Parallel()

	t.Run("Should show full lifecycle and progress for Task Groups", func(t *testing.T) {
		t.Parallel()

		state := newTaskRunWizardTestState(t, "auth")
		taskGroups := []taskgroups.TaskGroup{
			{ID: "TG-001", Title: "Data model", Directory: "_task_groups/001-data-model", Completed: true},
			{ID: "TG-002", Title: "API", Directory: "_task_groups/002-api"},
			{ID: "TG-003", Title: "UI", Directory: "_task_groups/003-ui"},
			{ID: "TG-004", Title: "Session hardening", Directory: "_task_groups/004-session-hardening"},
			{ID: "TG-005", Title: "Documentation", Directory: "_task_groups/005-documentation"},
		}
		writeTaskRunWizardPlanWithEdges(t, state, "auth", taskGroups, []taskgroups.Dependency{
			{From: "TG-002", To: "TG-003", Rationale: "UI consumes the API"},
		})
		writeTaskRunWizardTasks(t, state, "auth", taskGroups[0].Directory, "completed", "completed")
		writeTaskRunWizardTasks(t, state, "auth", taskGroups[1].Directory, "completed", "pending")
		writeTaskRunWizardTasks(t, state, "auth", taskGroups[2].Directory, "pending")
		writeTaskRunWizardTasks(t, state, "auth", taskGroups[3].Directory, "pending")
		writeTaskRunWizardTasks(t, state, "auth", taskGroups[4].Directory, "pending")

		wizard := newTaskRunWizardModelWithRunStatuses(
			context.Background(),
			state,
			taskRunFormInputs{},
			map[string]string{
				"auth/TG-002": "failed",
				"auth/TG-003": "failed",
				"auth/TG-004": "running",
			},
		)
		view := xansi.Strip(strings.Join(
			wizard.workflowListLines(wizard.filteredWorkflowOptions(), 180, 20),
			"\n",
		))
		for _, want := range []string{
			"auth — Running — 1/5 Task Groups completed — 3/7 tasks completed",
			"[✓] TG-001 — Data model — Completed — 2/2 tasks completed",
			"[ ] TG-002 — API — Ready to retry — 1/2 tasks completed",
			"[⊘] TG-003 — UI — Blocked — 0/1 tasks completed — waits for TG-002",
			"[!] TG-004 — Session hardening — Running — 0/1 tasks completed",
			"[!] TG-005 — Documentation — Ready — 0/1 tasks completed",
		} {
			if !strings.Contains(view, want) {
				t.Fatalf("workflow status view missing %q:\n%s", want, view)
			}
		}
	})

	t.Run("Should lock completed targets until include completed is enabled", func(t *testing.T) {
		t.Parallel()

		state := newTaskRunWizardTestState(t, "done", "ready")
		writeFormTaskFile(t, filepath.Join(state.workspaceRoot, ".compozy", "tasks", "done"), "task_01.md", "completed")
		writeFormTaskFile(t, filepath.Join(state.workspaceRoot, ".compozy", "tasks", "ready"), "task_01.md", "pending")
		wizard := newTaskRunWizardModel(state, taskRunFormInputs{selectedWorkflows: []string{"done"}})
		view := xansi.Strip(strings.Join(
			wizard.workflowListLines(wizard.filteredWorkflowOptions(), 100, 10),
			"\n",
		))
		if !strings.Contains(view, "[✓] done — Completed — 1/1 tasks completed") {
			t.Fatalf("completed workflow row missing full status:\n%s", view)
		}

		if len(wizard.inputs.selectedWorkflows) != 0 {
			t.Fatalf("initial selection = %#v, want completed target removed", wizard.inputs.selectedWorkflows)
		}
		wizard = updateTaskRunWizardTestModel(t, wizard, "space")
		if len(wizard.inputs.selectedWorkflows) != 0 {
			t.Fatalf("selection = %#v, want completed target locked", wizard.inputs.selectedWorkflows)
		}
		if !strings.Contains(wizard.message, "completed target is locked") {
			t.Fatalf("message = %q, want completed-target explanation", wizard.message)
		}

		wizard = updateTaskRunWizardTestModel(t, wizard, "i")
		if !wizard.inputs.includeCompleted {
			t.Fatal("expected include completed to be enabled")
		}
		wizard = updateTaskRunWizardTestModel(t, wizard, "space")
		if !slices.Equal(wizard.inputs.selectedWorkflows, []string{"done"}) {
			t.Fatalf("selection = %#v, want completed target after opt-in", wizard.inputs.selectedWorkflows)
		}

		wizard = updateTaskRunWizardTestModel(t, wizard, "i")
		if wizard.inputs.includeCompleted {
			t.Fatal("expected completed targets to be locked again")
		}
		if len(wizard.inputs.selectedWorkflows) != 0 {
			t.Fatalf("selection = %#v, want completed target removed after locking", wizard.inputs.selectedWorkflows)
		}
	})

	t.Run("Should derive Task Group completion from implementation tasks", func(t *testing.T) {
		t.Parallel()

		state := newTaskRunWizardTestState(t, "auth")
		taskGroups := []taskgroups.TaskGroup{
			{ID: "TG-001", Title: "Implemented", Directory: "_task_groups/001-implemented"},
			{ID: "TG-002", Title: "In progress", Directory: "_task_groups/002-in-progress"},
		}
		writeTaskRunWizardPlan(t, state, "auth", taskGroups...)
		writeTaskRunWizardTasks(t, state, "auth", taskGroups[0].Directory, "completed", "completed")
		writeTaskRunWizardTasks(t, state, "auth", taskGroups[1].Directory, "completed", "pending")

		wizard := newTaskRunWizardModel(state, taskRunFormInputs{})
		rawView := strings.Join(
			wizard.workflowListLines(wizard.filteredWorkflowOptions(), 180, 20),
			"\n",
		)
		view := xansi.Strip(rawView)
		for _, want := range []string{
			"auth — Ready — 1/2 Task Groups completed — 3/4 tasks completed",
			"[✓] TG-001 — Implemented — Completed — 2/2 tasks completed",
			"[ ] TG-002 — In progress — Ready — 1/2 tasks completed",
		} {
			if !strings.Contains(view, want) {
				t.Fatalf("workflow completion view missing %q:\n%s", want, view)
			}
		}
		if !strings.Contains(rawView, xansi.SGR(xansi.AttrStrikethrough)) {
			t.Fatalf("completed Task Group row is not struck through:\n%q", rawView)
		}

		wizard = updateTaskRunWizardTestModel(t, wizard, "down")
		wizard = updateTaskRunWizardTestModel(t, wizard, "space")
		if len(wizard.inputs.selectedWorkflows) != 0 {
			t.Fatalf("selection = %#v, want completed Task Group locked", wizard.inputs.selectedWorkflows)
		}
		if !strings.Contains(wizard.message, "completed target is locked") {
			t.Fatalf("message = %q, want completed-target explanation", wizard.message)
		}
	})

	t.Run("Should exclude completed Task Groups from group selection until opt-in", func(t *testing.T) {
		t.Parallel()

		state := newTaskRunWizardTestState(t, "auth")
		writeTaskRunWizardPlan(t, state, "auth",
			taskgroups.TaskGroup{ID: "TG-001", Title: "Done", Completed: true},
			taskgroups.TaskGroup{ID: "TG-002", Title: "Ready"},
		)
		wizard := newTaskRunWizardModel(state, taskRunFormInputs{})

		wizard = updateTaskRunWizardTestModel(t, wizard, "space")
		if !slices.Equal(wizard.inputs.selectedWorkflows, []string{"auth/TG-002"}) {
			t.Fatalf("group selection = %#v, want only unfinished Task Group", wizard.inputs.selectedWorkflows)
		}

		wizard = updateTaskRunWizardTestModel(t, wizard, "i")
		wizard = updateTaskRunWizardTestModel(t, wizard, "space")
		if !slices.Equal(wizard.inputs.selectedWorkflows, []string{"auth/TG-002", "auth/TG-001"}) {
			t.Fatalf("group selection = %#v, want completed Task Group after opt-in", wizard.inputs.selectedWorkflows)
		}
	})

	t.Run(
		"Should auto-select only eligible Task Groups and explicitly authorize a blocked selection",
		func(t *testing.T) {
			t.Parallel()

			state := newTaskRunWizardTestState(t, "auth")
			taskGroups := []taskgroups.TaskGroup{
				{ID: "TG-001", Title: "Foundation"},
				{ID: "TG-002", Title: "API"},
				{ID: "TG-003", Title: "UI"},
				{ID: "TG-004", Title: "Rollout"},
			}
			writeTaskRunWizardPlanWithEdges(t, state, "auth", taskGroups, []taskgroups.Dependency{
				{From: "TG-001", To: "TG-003", Rationale: "UI needs the foundation"},
				{From: "TG-002", To: "TG-003", Rationale: "UI needs the API"},
				{From: "TG-003", To: "TG-004", Rationale: "Rollout needs the UI"},
			})
			wizard := newTaskRunWizardModel(state, taskRunFormInputs{})

			view := xansi.Strip(strings.Join(
				wizard.workflowListLines(wizard.filteredWorkflowOptions(), 180, 20),
				"\n",
			))
			for _, want := range []string{
				"[⊘] TG-003 — UI — Blocked",
				"[⊘] TG-004 — Rollout — Blocked",
			} {
				if !strings.Contains(view, want) {
					t.Fatalf("blocked workflow view missing %q:\n%s", want, view)
				}
			}

			wizard = updateTaskRunWizardTestModel(t, wizard, "space")
			if !slices.Equal(wizard.inputs.selectedWorkflows, []string{"auth/TG-001", "auth/TG-002"}) {
				t.Fatalf(
					"parent selection = %#v, want only dependency-ready Task Groups",
					wizard.inputs.selectedWorkflows,
				)
			}
			if view := xansi.Strip(wizard.View().Content); !strings.Contains(view, "[-] auth") {
				t.Fatalf("eligible-only parent selection must remain partial:\n%s", view)
			}

			for range 3 {
				wizard = updateTaskRunWizardTestModel(t, wizard, "down")
			}
			wizard = updateTaskRunWizardTestModel(t, wizard, "space")
			if !slices.Equal(wizard.inputs.selectedWorkflows, []string{
				"auth/TG-001",
				"auth/TG-002",
				"auth/TG-003",
			}) {
				t.Fatalf("manual blocked selection = %#v, want TG-003 appended", wizard.inputs.selectedWorkflows)
			}
			if review := xansi.Strip(wizard.renderReviewStep(100)); !strings.Contains(review, "out-of-order") {
				t.Fatalf("review must disclose the dependency override:\n%s", review)
			}

			cmd := newTasksRunCommandWithDefaults(nil, defaultCommandStateDefaults())
			appliedState := newCommandState(commandKindTasksRun, core.ModePRDTasks)
			if err := wizard.inputs.apply(cmd, appliedState); err != nil {
				t.Fatalf("apply blocked workflow selection: %v", err)
			}
			if !appliedState.allowOutOfOrder || !cmd.Flags().Changed("allow-out-of-order") {
				t.Fatalf(
					"manual blocked selection must authorize this run: allow=%t changed=%t",
					appliedState.allowOutOfOrder,
					cmd.Flags().Changed("allow-out-of-order"),
				)
			}
		},
	)

	t.Run("Should preserve canonical Task Group ID order across readiness states", func(t *testing.T) {
		t.Parallel()

		state := newTaskRunWizardTestState(t, "auth")
		taskGroups := []taskgroups.TaskGroup{
			{ID: "TG-001", Title: "Foundation"},
			{ID: "TG-002", Title: "Blocked delivery"},
			{ID: "TG-003", Title: "Independent docs"},
			{ID: "TG-004", Title: "Blocked rollout"},
			{ID: "TG-005", Title: "Independent tooling"},
		}
		writeTaskRunWizardPlanWithEdges(t, state, "auth", taskGroups, []taskgroups.Dependency{
			{From: "TG-001", To: "TG-002", Rationale: "Delivery needs the foundation"},
			{From: "TG-002", To: "TG-004", Rationale: "Rollout needs delivery"},
		})

		options := buildTaskRunWizardWorkflowOptions(
			filepath.Join(state.workspaceRoot, ".compozy", "tasks"),
			nil,
		)
		got := make([]string, 0, len(options)-1)
		for _, option := range options {
			if !option.Group {
				got = append(got, option.Value)
			}
		}
		want := []string{
			"auth/TG-001",
			"auth/TG-002",
			"auth/TG-003",
			"auth/TG-004",
			"auth/TG-005",
		}
		if !slices.Equal(got, want) {
			t.Fatalf("Task Group order = %#v, want canonical ID order %#v", got, want)
		}
		if options[2].Status != taskRunWizardWorkflowBlocked ||
			options[4].Status != taskRunWizardWorkflowBlocked {
			t.Fatalf("readiness changed while ordering Task Groups: %#v", options)
		}
	})

	t.Run("Should load the latest task run status for each target", func(t *testing.T) {
		t.Parallel()

		client := &stubDaemonCommandClient{runs: []apicore.Run{
			{RunID: "alpha-latest", WorkflowSlug: "alpha", Mode: "task", Status: "failed"},
			{RunID: "alpha-older", WorkflowSlug: "alpha", Mode: "task", Status: "completed"},
			{RunID: "beta-latest", WorkflowSlug: "beta/TG-001", Mode: "task", Status: "running"},
		}}
		got, err := loadTaskRunWizardLatestRunStatuses(context.Background(), client, "/workspace")
		if err != nil {
			t.Fatalf("load latest run statuses: %v", err)
		}
		want := map[string]string{"alpha": "failed", "beta/TG-001": "running"}
		if len(got) != len(want) {
			t.Fatalf("statuses = %#v, want %#v", got, want)
		}
		for target, status := range want {
			if got[target] != status {
				t.Fatalf("status[%q] = %q, want %q", target, got[target], status)
			}
		}
		if len(client.runListRequests) != 1 {
			t.Fatalf("run list requests = %d, want 1", len(client.runListRequests))
		}
		request := client.runListRequests[0]
		if request.Workspace != "/workspace" || request.Mode != "task" || request.Limit != apicore.MaxPageLimit {
			t.Fatalf("run list request = %#v, want workspace-scoped task history", request)
		}
	})

	t.Run("Should report run history loading failures", func(t *testing.T) {
		t.Parallel()

		client := &stubDaemonCommandClient{runsErr: os.ErrPermission}
		_, err := loadTaskRunWizardLatestRunStatuses(context.Background(), client, "/workspace")
		if err == nil || !strings.Contains(err.Error(), "list task runs for picker") {
			t.Fatalf("error = %v, want wrapped run-history failure", err)
		}
	})
}

func TestTaskRunWizardModel(t *testing.T) {
	t.Parallel()

	t.Run("Should expose Task Group children and select one exact task group", func(t *testing.T) {
		t.Parallel()

		state := newTaskRunWizardTestState(t, "general-task")
		writeTaskRunWizardPlan(t, state, "general-task",
			taskgroups.TaskGroup{ID: "TG-001", Title: "Shared foundation"},
			taskgroups.TaskGroup{ID: "TG-002", Title: "Feature delivery"},
		)
		wizard := newTaskRunWizardModel(state, taskRunFormInputs{})

		view := wizard.View().Content
		for _, want := range []string{"general-task", "TG-001", "Shared foundation", "TG-002", "Feature delivery"} {
			if !strings.Contains(view, want) {
				t.Fatalf("workflow view missing %q:\n%s", want, view)
			}
		}
		listLines := wizard.workflowListLines(wizard.filteredWorkflowOptions(), 64, 8)
		if got := xansi.Strip(listLines[2]); !strings.HasPrefix(got, "    [ ] TG-001 — Shared foundation") {
			t.Fatalf("first Task Group row = %q, want an indented child checkbox", got)
		}

		wizard = updateTaskRunWizardTestModel(t, wizard, "down")
		wizard = updateTaskRunWizardTestModel(t, wizard, "space")

		if !slices.Equal(wizard.inputs.selectedWorkflows, []string{"general-task/TG-001"}) {
			t.Fatalf(
				"selected workflows = %#v, want exact Task Group reference",
				wizard.inputs.selectedWorkflows,
			)
		}
		cmd := newTasksRunCommandWithDefaults(nil, defaultCommandStateDefaults())
		appliedState := newCommandState(commandKindTasksRun, core.ModePRDTasks)
		if err := wizard.inputs.apply(cmd, appliedState); err != nil {
			t.Fatalf("apply selected Task Group: %v", err)
		}
		if appliedState.name != "general-task/TG-001" || appliedState.multiple != "" {
			t.Fatalf(
				"applied selection name=%q multiple=%q, want one exact Task Group",
				appliedState.name,
				appliedState.multiple,
			)
		}
	})

	t.Run("Should reflect full and partial Task Group group selection", func(t *testing.T) {
		t.Parallel()

		state := newTaskRunWizardTestState(t, "general-task")
		writeTaskRunWizardPlan(t, state, "general-task",
			taskgroups.TaskGroup{ID: "TG-001", Title: "Shared foundation"},
			taskgroups.TaskGroup{ID: "TG-002", Title: "Feature delivery"},
		)
		wizard := newTaskRunWizardModel(state, taskRunFormInputs{})

		wizard = updateTaskRunWizardTestModel(t, wizard, "space")
		if !slices.Equal(wizard.inputs.selectedWorkflows, []string{
			"general-task/TG-001",
			"general-task/TG-002",
		}) {
			t.Fatalf("group selection = %#v, want both Task Groups", wizard.inputs.selectedWorkflows)
		}
		if view := xansi.Strip(wizard.View().Content); !strings.Contains(view, "[x] general-task") {
			t.Fatalf("selected group view missing checked parent:\n%s", view)
		}

		wizard = updateTaskRunWizardTestModel(t, wizard, "down")
		wizard = updateTaskRunWizardTestModel(t, wizard, "space")
		if !slices.Equal(wizard.inputs.selectedWorkflows, []string{"general-task/TG-002"}) {
			t.Fatalf("partial group selection = %#v, want only TG-002", wizard.inputs.selectedWorkflows)
		}
		if view := xansi.Strip(wizard.View().Content); !strings.Contains(view, "[-] general-task") {
			t.Fatalf("partial group view missing mixed parent:\n%s", view)
		}
	})

	t.Run("Should not fabricate Task Group children from an invalid plan", func(t *testing.T) {
		t.Parallel()

		state := newTaskRunWizardTestState(t, "general-task")
		planPath := filepath.Join(
			state.workspaceRoot,
			".compozy",
			"tasks",
			"general-task",
			taskgroups.ManifestFileName,
		)
		if err := os.WriteFile(planPath, []byte("---\nschema_version: invalid\n---\n"), 0o644); err != nil {
			t.Fatalf("write invalid Task Group plan: %v", err)
		}

		wizard := newTaskRunWizardModel(state, taskRunFormInputs{})
		view := wizard.View().Content
		if strings.Contains(view, "TG-001") {
			t.Fatalf("workflow view fabricated a task group child from an invalid plan:\n%s", view)
		}
	})

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

	t.Run("Should remove the focused run-order item with Space", func(t *testing.T) {
		t.Parallel()

		state := newTaskRunWizardTestState(t, "alpha", "beta", "gamma")
		wizard := newTaskRunWizardModel(state, taskRunFormInputs{
			selectedWorkflows: []string{"alpha", "beta"},
		})
		wizard.workflowFocus = taskRunWizardWorkflowFocusOrder
		wizard.orderCursor = 0

		wizard = updateTaskRunWizardTestModel(t, wizard, "space")
		if !slices.Equal(wizard.inputs.selectedWorkflows, []string{"beta"}) {
			t.Fatalf("selected workflows = %#v, want [beta]", wizard.inputs.selectedWorkflows)
		}
		if wizard.workflowFocus != taskRunWizardWorkflowFocusOrder {
			t.Fatalf("workflow focus = %v, want run order", wizard.workflowFocus)
		}

		wizard = updateTaskRunWizardTestModel(t, wizard, "space")
		if len(wizard.inputs.selectedWorkflows) != 0 {
			t.Fatalf("selected workflows = %#v, want empty", wizard.inputs.selectedWorkflows)
		}
		if wizard.workflowFocus != taskRunWizardWorkflowFocusList {
			t.Fatalf("workflow focus = %v, want list after removing last item", wizard.workflowFocus)
		}
	})

	t.Run("Should include an unselected highlight before Enter advances", func(t *testing.T) {
		t.Parallel()

		state := newTaskRunWizardTestState(t, "alpha", "beta")
		wizard := newTaskRunWizardModel(state, taskRunFormInputs{})
		wizard.workflowCursor = 1

		wizard = updateTaskRunWizardTestModel(t, wizard, "enter")
		if wizard.step != taskRunWizardStepRuntime {
			t.Fatalf("step = %v, want runtime", wizard.step)
		}
		if !slices.Equal(wizard.inputs.selectedWorkflows, []string{"beta"}) {
			t.Fatalf("selected workflows = %#v, want [beta]", wizard.inputs.selectedWorkflows)
		}
	})

	t.Run("Should not advance an empty filtered selection", func(t *testing.T) {
		t.Parallel()

		state := newTaskRunWizardTestState(t, "alpha", "beta")
		wizard := newTaskRunWizardModel(state, taskRunFormInputs{})
		wizard.searchQuery = "no-match"

		wizard = updateTaskRunWizardTestModel(t, wizard, "enter")
		if wizard.step != taskRunWizardStepWorkflows {
			t.Fatalf("step = %v, want workflows", wizard.step)
		}
		if !strings.Contains(wizard.message, "select at least one workflow") {
			t.Fatalf("message = %q, want selection explanation", wizard.message)
		}
	})

	t.Run("Should preserve selection order across filter changes", func(t *testing.T) {
		t.Parallel()

		state := newTaskRunWizardTestState(t, "alpha", "beta", "gamma")
		wizard := newTaskRunWizardModel(state, taskRunFormInputs{selectedWorkflows: []string{"gamma"}})
		wizard.searchQuery = "alpha"
		wizard.workflowCursor = 0
		wizard = updateTaskRunWizardTestModel(t, wizard, "space")
		wizard.searchQuery = "beta"
		wizard.workflowCursor = 0
		wizard = updateTaskRunWizardTestModel(t, wizard, "space")
		wizard.searchQuery = ""

		if !slices.Equal(wizard.inputs.selectedWorkflows, []string{"gamma", "alpha", "beta"}) {
			t.Fatalf(
				"selected workflows = %#v, want selection order to survive filters",
				wizard.inputs.selectedWorkflows,
			)
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
		if strings.Contains(wizard.renderExecutionStep(60), "Multi-workflow mode") {
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
		serialView := xansi.Strip(wizard.renderExecutionStep(80))
		if !strings.Contains(serialView, "Serial queue (no worktrees)") {
			t.Fatalf("serial mode is not explicit in execution view: %q", serialView)
		}
		if strings.Contains(wizard.renderExecutionStep(60), "Max concurrent") {
			t.Fatal("max concurrent should be hidden while parallel workflows are disabled")
		}

		wizard.execCursor = taskRunWizardFieldParallelWorkflows
		wizard = updateTaskRunWizardTestModel(t, wizard, "space")
		if !wizard.inputs.parallelWorkflows {
			t.Fatal("expected parallel workflow toggle to enable parallel workflows")
		}
		parallelView := xansi.Strip(wizard.renderExecutionStep(80))
		if !strings.Contains(parallelView, "Parallel workflows (git worktrees)") {
			t.Fatalf("parallel mode is not explicit in execution view: %q", parallelView)
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
		review := xansi.Strip(wizard.renderReviewStep(80))
		if !strings.Contains(review, "Multi-workflow mode") ||
			!strings.Contains(review, "Parallel workflows (git worktrees)") {
			t.Fatalf("review does not show resolved multi-workflow mode: %q", review)
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

func TestTaskRunWizardReasoningOptionsIncludeModernEfforts(t *testing.T) {
	t.Parallel()

	options := taskRunWizardReasoningOptions()
	for _, effort := range []string{"max", "ultra"} {
		if !taskRunWizardChoiceContains(options, effort) {
			t.Fatalf("reasoning options do not contain %q: %#v", effort, options)
		}
	}
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

func writeTaskRunWizardPlan(
	t *testing.T,
	state *commandState,
	initiative string,
	taskGroups ...taskgroups.TaskGroup,
) {
	t.Helper()
	writeTaskRunWizardPlanWithEdges(t, state, initiative, taskGroups, nil)
}

func writeTaskRunWizardPlanWithEdges(
	t *testing.T,
	state *commandState,
	initiative string,
	taskGroups []taskgroups.TaskGroup,
	edges []taskgroups.Dependency,
) {
	t.Helper()

	for i := range taskGroups {
		taskGroups[i].Outcome = "Deliver " + taskGroups[i].Title
		taskGroups[i].OwnedScope = []string{"scope/" + taskGroups[i].ID}
	}
	content, err := taskgroups.RenderPlan(taskgroups.Plan{
		SchemaVersion: taskgroups.SchemaVersion,
		Initiative:    initiative,
		TaskGroups:    taskGroups,
		Edges:         edges,
	})
	if err != nil {
		t.Fatalf("render Task Group plan: %v", err)
	}
	planPath := filepath.Join(
		state.workspaceRoot,
		".compozy",
		"tasks",
		initiative,
		taskgroups.ManifestFileName,
	)
	if err := os.WriteFile(planPath, content, 0o644); err != nil {
		t.Fatalf("write Task Group plan: %v", err)
	}
}

func writeTaskRunWizardTasks(
	t *testing.T,
	state *commandState,
	initiative string,
	taskGroupDirectory string,
	statuses ...string,
) {
	t.Helper()

	tasksDir := filepath.Join(
		state.workspaceRoot,
		".compozy",
		"tasks",
		initiative,
		filepath.FromSlash(taskGroupDirectory),
	)
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("mkdir Task Group tasks dir: %v", err)
	}
	for index, status := range statuses {
		writeFormTaskFile(t, tasksDir, fmt.Sprintf("task_%02d.md", index+1), status)
	}
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

func TestReviewFixImplementationBlocked(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		option taskRunWizardWorkflowOption
		want   bool
	}{
		{
			name:   "Should not block a target with at least one completed task",
			option: taskRunWizardWorkflowOption{TaskProgressKnown: true, TotalTasks: 3, CompletedTasks: 1},
			want:   false,
		},
		{
			name:   "Should block a known target with zero completed tasks",
			option: taskRunWizardWorkflowOption{TaskProgressKnown: true, TotalTasks: 3, CompletedTasks: 0},
			want:   true,
		},
		{
			name:   "Should block a target whose task progress is unknown",
			option: taskRunWizardWorkflowOption{TaskProgressKnown: false, TotalTasks: 3, CompletedTasks: 2},
			want:   true,
		},
		{
			name:   "Should block a target with no implementation tasks",
			option: taskRunWizardWorkflowOption{TaskProgressKnown: true, TotalTasks: 0, CompletedTasks: 0},
			want:   true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := reviewFixImplementationBlocked(tc.option); got != tc.want {
				t.Fatalf("reviewFixImplementationBlocked(%+v) = %v, want %v", tc.option, got, tc.want)
			}
		})
	}
}
