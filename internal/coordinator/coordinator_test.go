package coordinator

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func TestDecideBootstrap(t *testing.T) {
	t.Parallel()

	baseTask := taskpkg.Task{
		ID:          "task-1",
		Scope:       taskpkg.ScopeWorkspace,
		WorkspaceID: "ws-1",
	}
	baseRun := taskpkg.Run{
		ID:       "run-1",
		TaskID:   "task-1",
		Status:   taskpkg.TaskRunStatusQueued,
		Metadata: json.RawMessage(`{"workflow_id":"wf-1"}`),
	}
	enabled := compozyconfig.DefaultResolvedCoordinatorRole()
	enabled.Enabled = true

	tests := []struct {
		name   string
		task   taskpkg.Task
		run    taskpkg.Run
		cfg    compozyconfig.ResolvedCoordinatorRole
		want   string
		should bool
	}{
		{
			name: "Should bootstrap enabled workspace executable run",
			task: baseTask,
			run:  baseRun,
			cfg:  enabled,
			want: DecisionBootstrap, should: true,
		},
		{
			name: "Should skip disabled config",
			task: baseTask,
			run:  baseRun,
			cfg:  compozyconfig.DefaultResolvedCoordinatorRole(),
			want: DecisionDisabled,
		},
		{
			name: "Should skip global scope",
			task: func() taskpkg.Task {
				task := baseTask
				task.Scope = taskpkg.ScopeGlobal
				task.WorkspaceID = ""
				return task
			}(),
			run:  baseRun,
			cfg:  enabled,
			want: DecisionGlobalScope,
		},
		{
			name: "Should skip completed run",
			task: baseTask,
			run: func() taskpkg.Run {
				run := baseRun
				run.Status = taskpkg.TaskRunStatusCompleted
				return run
			}(),
			cfg:  enabled,
			want: DecisionNonExecutableRun,
		},
		{
			name: "Should skip loop action worker run",
			task: baseTask,
			run: func() taskpkg.Run {
				run := baseRun
				run.RunKind = taskpkg.RunKindWorker
				run.LoopRunID = "loop-run-1"
				return run
			}(),
			cfg:  enabled,
			want: DecisionLoopWorker,
		},
		{
			name: "Should skip task run mismatch",
			task: baseTask,
			run: func() taskpkg.Run {
				run := baseRun
				run.TaskID = "other-task"
				return run
			}(),
			cfg:  enabled,
			want: DecisionTaskRunMismatch,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := DecideBootstrap(tc.task, tc.run, tc.cfg)
			if got.ShouldBootstrap != tc.should {
				t.Fatalf("ShouldBootstrap = %v, want %v", got.ShouldBootstrap, tc.should)
			}
			if got.Reason != tc.want {
				t.Fatalf("Reason = %q, want %q", got.Reason, tc.want)
			}
			if tc.should && got.WorkflowID != "wf-1" {
				t.Fatalf("WorkflowID = %q, want wf-1", got.WorkflowID)
			}
		})
	}
}

func TestPermissionPolicyRestrictsCoordinatorSurface(t *testing.T) {
	t.Parallel()

	t.Run("Should restrict coordinator permissions to task tools", func(t *testing.T) {
		t.Parallel()

		policy := PermissionPolicy()
		if !slices.Contains(policy.Tools, toolspkg.ToolIDTaskRunClaimNext.String()) {
			t.Fatalf("policy tools = %#v, want %q", policy.Tools, toolspkg.ToolIDTaskRunClaimNext)
		}
		if err := store.ValidateSessionLineage("coord-1", &store.SessionLineage{
			SpawnRole:        "coordinator",
			TTLExpiresAt:     new(time.Date(2026, 4, 26, 14, 0, 0, 0, time.UTC)),
			SpawnBudget:      store.SessionSpawnBudget{MaxChildren: 5, MaxDepth: session.DefaultSpawnMaxDepth},
			PermissionPolicy: policy,
		}); err != nil {
			t.Fatalf("ValidateSessionLineage(coordinator policy) error = %v", err)
		}
		for _, denied := range []string{
			toolspkg.ToolIDTaskCancel.String(),
			toolspkg.ToolIDToolInfo.String(),
			"agent.spawn.coordinator",
		} {
			if ToolAllowed(denied) {
				t.Fatalf("ToolAllowed(%q) = true, want false", denied)
			}
		}
		for _, allowed := range []string{
			toolspkg.ToolIDSessionDescribe.String(),
			toolspkg.ToolIDTaskRunComplete.String(),
			toolspkg.ToolIDTaskCreate.String(),
		} {
			if !ToolAllowed(allowed) {
				t.Fatalf("ToolAllowed(%q) = false, want true", allowed)
			}
		}
		if SpawnRoleAllowed("coordinator") {
			t.Fatal("SpawnRoleAllowed(coordinator) = true, want false")
		}
		if !SpawnRoleAllowed("worker") {
			t.Fatal("SpawnRoleAllowed(worker) = false, want true")
		}
	})
}

func TestCoordinatorListAccessorsReturnCopies(t *testing.T) {
	t.Parallel()

	t.Run("Should protect coordinator allowlists from caller mutation", func(t *testing.T) {
		t.Parallel()

		tools := ToolAllowlist()
		if len(tools) == 0 {
			t.Fatal("ToolAllowlist() returned empty list, want coordinator tools")
		}
		tools[0] = toolspkg.ToolIDTaskCancel.String()
		if ToolAllowed(toolspkg.ToolIDTaskCancel.String()) {
			t.Fatal("ToolAllowed(task.cancel) = true after caller mutation, want immutable allowlist")
		}
		policy := PermissionPolicy()
		if slices.Contains(policy.Tools, toolspkg.ToolIDTaskCancel.String()) {
			t.Fatalf("PermissionPolicy() Tools = %#v, want no caller-mutated task.cancel", policy.Tools)
		}
	})
}

func TestLineageAndHealthySession(t *testing.T) {
	t.Parallel()

	t.Run("Should build coordinator lineage and reject unhealthy sessions", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
		cfg := compozyconfig.DefaultResolvedCoordinatorRole()
		cfg.Enabled = true
		cfg.TTL = 2 * time.Hour
		cfg.MaxChildren = 3
		policy := PermissionPolicy()

		lineage := Lineage(now, cfg, policy)
		if lineage.SpawnRole != string(session.SessionTypeCoordinator) {
			t.Fatalf("SpawnRole = %q, want coordinator", lineage.SpawnRole)
		}
		if lineage.TTLExpiresAt == nil || !lineage.TTLExpiresAt.Equal(now.Add(2*time.Hour)) {
			t.Fatalf("TTLExpiresAt = %#v, want %s", lineage.TTLExpiresAt, now.Add(2*time.Hour))
		}
		if lineage.SpawnBudget.MaxChildren != 3 || lineage.SpawnBudget.MaxDepth != session.DefaultSpawnMaxDepth {
			t.Fatalf("SpawnBudget = %#v, want max children 3 and default depth", lineage.SpawnBudget)
		}
		if !slices.Equal(lineage.PermissionPolicy.Tools, policy.Tools) {
			t.Fatalf("PermissionPolicy.Tools = %#v, want %#v", lineage.PermissionPolicy.Tools, policy.Tools)
		}

		info := &session.Info{
			ID:          "coord-1",
			Type:        session.SessionTypeCoordinator,
			WorkspaceID: "ws-1",
			State:       session.StateActive,
			Lineage:     lineage,
		}
		if !HealthySession(info, "ws-1", now) {
			t.Fatal("HealthySession(active coordinator) = false, want true")
		}
		if HealthySession(info, "ws-2", now) {
			t.Fatal("HealthySession(other workspace) = true, want false")
		}
		info.State = session.StateStopped
		if HealthySession(info, "ws-1", now) {
			t.Fatal("HealthySession(stopped) = true, want false")
		}
		info.State = session.StateActive
		expired := now.Add(-time.Minute)
		info.Lineage = &store.SessionLineage{TTLExpiresAt: &expired}
		if HealthySession(info, "ws-1", now) {
			t.Fatal("HealthySession(expired ttl) = true, want false")
		}
	})
}

func TestPromptOverlayUsesPublicAPIs(t *testing.T) {
	t.Parallel()

	t.Run("Should name active worker worktrees in stable run order", func(t *testing.T) {
		t.Parallel()

		overlay := PromptOverlay(PromptInput{
			WorkspaceID: "ws-1",
			TaskID:      "task-1",
			RunID:       "run-1",
			WorkerWorktrees: []WorkerWorktreeBinding{
				{RunID: " run-z ", WorktreeID: " wt-z "},
				{RunID: "run-a", WorktreeID: "wt-a"},
				{RunID: "", WorktreeID: "wt-invalid"},
			},
		})
		want := "Active worker worktrees:\n- run_id: run-a; worktree_id: wt-a\n" +
			"- run_id: run-z; worktree_id: wt-z"
		if !strings.Contains(overlay, want) {
			t.Fatalf("PromptOverlay worker worktrees = %q, want ordered bindings %q", overlay, want)
		}
		if strings.Contains(overlay, "wt-invalid") {
			t.Fatalf("PromptOverlay contains incomplete worker binding:\n%s", overlay)
		}
	})

	t.Run("Should describe the bounded coordinator API surface", func(t *testing.T) {
		t.Parallel()

		overlay := PromptOverlay(PromptInput{
			WorkspaceID: "ws-1",
			TaskID:      "task-1",
			RunID:       "run-1",
			WorkflowID:  "wf-1",
		})
		for _, required := range []string{
			toolspkg.ToolIDSessionDescribe.String(),
			toolspkg.ToolIDTaskCreate.String(),
			toolspkg.ToolIDTaskRunClaimNext.String(),
			toolspkg.ToolIDTaskRunHeartbeat.String(),
			toolspkg.ToolIDTaskRunComplete.String(),
			toolspkg.ToolIDTaskRunFail.String(),
			toolspkg.ToolIDTaskRunRelease.String(),
			"compozy spawn",
			"The current coordinator run is the active execution boundary",
			"Never spawn another coordinator",
		} {
			if !strings.Contains(overlay, required) {
				t.Fatalf("PromptOverlay missing %q:\n%s", required, overlay)
			}
		}
		for _, forbidden := range []string{
			"compozy task",
		} {
			if strings.Contains(overlay, forbidden) {
				t.Fatalf("PromptOverlay contains forbidden guidance %q:\n%s", forbidden, overlay)
			}
		}
	})
}
