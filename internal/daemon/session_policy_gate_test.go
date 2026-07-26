package daemon

import (
	"reflect"
	"strings"
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func TestSessionPolicyGateAppliesSandboxPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		policy  SessionPolicy
		wantRef string
		wantOff bool
	}{
		{
			name:    "Should disable sandbox for none mode",
			policy:  SessionPolicy{Sandbox: SessionSandboxPolicy{Mode: SessionSandboxModeNone, SandboxRef: "ignored"}},
			wantOff: true,
		},
		{
			name: "Should propagate sandbox ref for ref mode",
			policy: SessionPolicy{Sandbox: SessionSandboxPolicy{
				Mode:       SessionSandboxModeRef,
				SandboxRef: " evidence-lab ",
			}},
			wantRef: "evidence-lab",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts := session.CreateOpts{SandboxRef: "preexisting"}
			applySessionSandboxPolicy(&opts, tt.policy)

			if got := opts.DisableSandbox; got != tt.wantOff {
				t.Fatalf("DisableSandbox = %v, want %v", got, tt.wantOff)
			}
			if got := opts.SandboxRef; got != tt.wantRef {
				t.Fatalf("SandboxRef = %q, want %q", got, tt.wantRef)
			}
		})
	}
}

func TestSessionPolicyGateAppliesEvidencePermissionPolicy(t *testing.T) {
	t.Parallel()

	t.Run("Should auto approve evidence mode only with sandbox ref", func(t *testing.T) {
		t.Parallel()

		opts := session.CreateOpts{Permissions: aghconfig.PermissionModeDenyAll}
		applySessionPermissionPolicy(&opts, SessionPolicy{
			Sandbox: SessionSandboxPolicy{Mode: SessionSandboxModeRef, SandboxRef: "evidence-lab"},
			Runtime: SessionRuntimePolicy{Mode: SessionRuntimeModeEvidence},
		})

		if got, want := opts.Permissions, aghconfig.PermissionModeApproveAll; got != want {
			t.Fatalf("Permissions = %q, want %q", got, want)
		}
		if !strings.Contains(opts.PromptOverlay, "Runtime evidence mode is enabled") {
			t.Fatalf("PromptOverlay = %q, want evidence guidance", opts.PromptOverlay)
		}
	})

	t.Run("Should preserve configured permission mode without sandbox ref", func(t *testing.T) {
		t.Parallel()

		opts := session.CreateOpts{Permissions: aghconfig.PermissionModeDenyAll}
		applySessionPermissionPolicy(&opts, SessionPolicy{
			Runtime: SessionRuntimePolicy{Mode: SessionRuntimeModeEvidence},
		})

		if got, want := opts.Permissions, aghconfig.PermissionModeDenyAll; got != want {
			t.Fatalf("Permissions = %q, want configured %q", got, want)
		}
		if !strings.Contains(opts.PromptOverlay, "did not select a sandbox") {
			t.Fatalf("PromptOverlay = %q, want sandbox warning", opts.PromptOverlay)
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
			TaskID:               "task-parity",
			RunID:                "run-parity",
			Scope:                taskpkg.ScopeWorkspace,
			WorkspaceID:          "ws-parity",
			AgentName:            "frontend-engineer",
			Provider:             "claude",
			Model:                "sonnet",
			NetworkParticipation: daemonTestLiveParticipationPtr("ws-parity", "design-review"),
			Title:                "Parity task",
			Capabilities:         []string{"frontend"},
			Profile: &taskpkg.ExecutionProfile{
				TaskID: "task-parity",
				Worker: taskpkg.WorkerProfile{
					Mode:      taskpkg.WorkerModeSelect,
					AgentName: "frontend-engineer",
					Provider:  "claude",
					Model:     "sonnet",
				},
				Sandbox: taskpkg.SandboxPolicy{
					Mode:       taskpkg.SandboxModeRef,
					SandboxRef: "evidence-lab",
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
			AgentName:       "frontend-engineer",
			Provider:        "claude",
			Model:           "sonnet",
			SandboxRef:      "evidence-lab",
			DisableSandbox:  false,
			Permissions:     aghconfig.PermissionModeApproveAll,
			Name:            "task-role:frontend-engineer:run-parity:63e604975a4c215d",
			Workspace:       "ws-parity",
			WorkspacePath:   "",
			NetworkOwnerKey: "task_run:run-parity",
			ResolvedNetworkParticipation: participationSnapshotPointer(
				daemonTestLiveParticipation("ws-parity", "design-review"),
			),
			PromptOverlay: "A queued AGH task run is assigned to this agent.\n\nTask: Parity task\nRun: run-parity\nCoordination channel: design-review\n\nUse `agh task next --run-id 'run-parity' --wait -o json --capability 'frontend'` once to claim work for this session before changing files. Complete or fail the claimed run through the AGH task lease commands from this same session.\n\nRuntime evidence mode is enabled for this task. You may boot local app runtimes, run browser or simulator validation, and capture runtime evidence artifacts required by the task.",
			Type:          session.SessionTypeSystem,
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("taskRoleCreateOpts() = %#v, want concrete options %#v", got, want)
		}
	})
}
