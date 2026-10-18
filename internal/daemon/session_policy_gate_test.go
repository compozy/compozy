package daemon

import (
	"reflect"
	"strings"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/session"
	taskpkg "github.com/compozy/compozy/internal/task"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func TestSessionPolicyGateAppliesEvidencePermissionPolicy(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve configured permission mode while appending evidence guidance", func(t *testing.T) {
		t.Parallel()

		opts := session.CreateOpts{Permissions: compozyconfig.PermissionModeDenyAll}
		applySessionPermissionPolicy(&opts, SessionPolicy{
			Runtime: SessionRuntimePolicy{Mode: SessionRuntimeModeEvidence},
		})

		if got, want := opts.Permissions, compozyconfig.PermissionModeDenyAll; got != want {
			t.Fatalf("Permissions = %q, want configured %q", got, want)
		}
		if !strings.Contains(opts.PromptOverlay, "Runtime evidence mode is enabled") {
			t.Fatalf("PromptOverlay = %q, want evidence guidance", opts.PromptOverlay)
		}
	})
}

func TestSessionPolicyGateAppliesAllowedToolsNarrowing(t *testing.T) {
	t.Parallel()

	t.Run("Should normalize concrete allowed tools", func(t *testing.T) {
		t.Parallel()

		opts := session.CreateOpts{}
		err := applyAllowedToolsNarrowing(&opts, []string{
			" " + toolspkg.ToolIDTaskUpdate.String() + " ",
			toolspkg.ToolIDTaskRead.String(),
			toolspkg.ToolIDTaskRead.String(),
		})
		if err != nil {
			t.Fatalf("applyAllowedToolsNarrowing() error = %v", err)
		}
		want := []string{toolspkg.ToolIDTaskRead.String(), toolspkg.ToolIDTaskUpdate.String()}
		if !reflect.DeepEqual(opts.AllowedToolsOverride, want) {
			t.Fatalf("AllowedToolsOverride = %#v, want %#v", opts.AllowedToolsOverride, want)
		}
	})

	t.Run("Should reject non canonical requested tools", func(t *testing.T) {
		t.Parallel()

		err := applyAllowedToolsNarrowing(&session.CreateOpts{}, []string{"Read"})
		if err == nil || !strings.Contains(err.Error(), "allowed_tools[0]") {
			t.Fatalf("applyAllowedToolsNarrowing() error = %v, want indexed validation error", err)
		}
	})
}

func TestSessionPolicyGateBuildsConcreteTaskRoleCreateOpts(t *testing.T) {
	t.Parallel()

	t.Run("Should map a workspace evidence profile to the expected session options", func(t *testing.T) {
		t.Parallel()

		activation := taskRoleActivation{
			TaskID:       "task-parity",
			RunID:        "run-parity",
			Scope:        taskpkg.ScopeWorkspace,
			WorkspaceID:  "ws-parity",
			AgentName:    "frontend-engineer",
			Provider:     "claude",
			Model:        "sonnet",
			Title:        "Parity task",
			Capabilities: []string{"frontend"},
			Worktree:     taskpkg.WorktreePolicy{Mode: taskpkg.WorktreeModeNone},
			Profile: &taskpkg.ExecutionProfile{
				TaskID: "task-parity",
				Worker: taskpkg.WorkerProfile{
					Mode:      taskpkg.WorkerModeSelect,
					AgentName: "frontend-engineer",
					Provider:  "claude",
					Model:     "sonnet",
				},
				Runtime: taskpkg.RuntimePolicy{Mode: taskpkg.RuntimeModeEvidence},
			},
			WorkspacePath: "/unused",
		}

		got, err := taskRoleCreateOpts(activation)
		if err != nil {
			t.Fatalf("taskRoleCreateOpts() error = %v", err)
		}
		want := session.CreateOpts{
			AgentName:     "frontend-engineer",
			Provider:      "claude",
			Model:         "sonnet",
			Name:          "task-role:frontend-engineer:run-parity:1a4f6338942e8365",
			Workspace:     "ws-parity",
			WorkspacePath: "",
			PromptOverlay: "A queued Compozy task run is assigned to this agent.\n\nTask: Parity task\nRun: run-parity\n\nCall the hosted native tool `compozy__task_run_claim_next` with `run_id` set to \"run-parity\" and `required_capabilities` set to [\"frontend\"] before doing any work. Do not use the CLI for session-bound lease operations. Maintain and settle the lease from this same session with `compozy__task_run_heartbeat`, `compozy__task_run_complete`, `compozy__task_run_fail`, or `compozy__task_run_release`.\n\nRuntime evidence mode is enabled for this task. You may boot local app runtimes, run browser or simulator validation, and capture runtime evidence artifacts required by the task.",
			Type:          session.SessionTypeSystem,
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("taskRoleCreateOpts() = %#v, want concrete options %#v", got, want)
		}
	})
}
