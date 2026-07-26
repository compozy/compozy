package coordinator

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	toolspkg "github.com/compozy/agh/internal/tools"
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
	baseRun.SetNetworkState(participation.Spec{
		Version:         participation.SpecVersion,
		Mode:            participation.ModeLive,
		ChannelStrategy: participation.StrategyNamed,
		ChannelID:       "ch-run-1",
		Source:          participation.SourceExplicitRequest,
	}, "", "", "")
	enabled := aghconfig.DefaultResolvedCoordinatorRole()
	enabled.Enabled = true

	tests := []struct {
		name   string
		task   taskpkg.Task
		run    taskpkg.Run
		cfg    aghconfig.ResolvedCoordinatorRole
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
			cfg:  aghconfig.DefaultResolvedCoordinatorRole(),
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
			name: "Should bootstrap local run without a channel",
			task: baseTask,
			run: func() taskpkg.Run {
				run := baseRun
				run.SetNetworkState(participation.LocalSpec(), "", "", "")
				return run
			}(),
			cfg:  enabled,
			want: DecisionBootstrap, should: true,
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

	t.Run("Should restrict local coordinator permissions to task tools", func(t *testing.T) {
		t.Parallel()

		policy := PermissionPolicy(participation.LocalSpec())
		if !slices.Contains(policy.Tools, toolspkg.ToolIDTaskRunClaimNext.String()) {
			t.Fatalf("policy tools = %#v, want %q", policy.Tools, toolspkg.ToolIDTaskRunClaimNext)
		}
		for _, networkTool := range []string{
			toolspkg.ToolIDNetworkChannels.String(),
			toolspkg.ToolIDNetworkInbox.String(),
			toolspkg.ToolIDNetworkSend.String(),
		} {
			if slices.Contains(policy.Tools, networkTool) {
				t.Fatalf("policy tools = %#v, want no local network tool %q", policy.Tools, networkTool)
			}
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
			if ToolAllowed(participation.LocalSpec(), denied) {
				t.Fatalf("ToolAllowed(%q) = true, want false", denied)
			}
		}
		for _, allowed := range []string{
			toolspkg.ToolIDSessionDescribe.String(),
			toolspkg.ToolIDTaskRunComplete.String(),
			toolspkg.ToolIDTaskCreate.String(),
		} {
			if !ToolAllowed(participation.LocalSpec(), allowed) {
				t.Fatalf("ToolAllowed(%q) = false, want true", allowed)
			}
		}
		if SpawnRoleAllowed("coordinator") {
			t.Fatal("SpawnRoleAllowed(coordinator) = true, want false")
		}
		if !SpawnRoleAllowed("worker") {
			t.Fatal("SpawnRoleAllowed(worker) = false, want true")
		}
		if got, want := policy.NetworkChannels, []string(nil); !slices.Equal(got, want) {
			t.Fatalf("NetworkChannels = %#v, want %#v", got, want)
		}
	})

	t.Run("Should add the network trio only for live participation", func(t *testing.T) {
		t.Parallel()

		live := participation.Spec{
			Version:         participation.SpecVersion,
			Mode:            participation.ModeLive,
			WorkspaceID:     "ws-1",
			ChannelStrategy: participation.StrategyNamed,
			ChannelID:       "ch-1",
			Source:          participation.SourceExplicitRequest,
		}
		policy := PermissionPolicy(live)
		for _, networkTool := range []string{
			toolspkg.ToolIDNetworkChannels.String(),
			toolspkg.ToolIDNetworkInbox.String(),
			toolspkg.ToolIDNetworkSend.String(),
		} {
			if !slices.Contains(policy.Tools, networkTool) {
				t.Fatalf("policy tools = %#v, want live network tool %q", policy.Tools, networkTool)
			}
		}
		if got, want := policy.NetworkChannels, []string{"ch-1"}; !slices.Equal(got, want) {
			t.Fatalf("NetworkChannels = %#v, want %#v", got, want)
		}
	})
}

func TestCoordinatorListAccessorsReturnCopies(t *testing.T) {
	t.Parallel()

	t.Run("Should protect coordinator allowlists from caller mutation", func(t *testing.T) {
		t.Parallel()

		tools := ToolAllowlist(participation.LocalSpec())
		if len(tools) == 0 {
			t.Fatal("ToolAllowlist() returned empty list, want coordinator tools")
		}
		tools[0] = toolspkg.ToolIDTaskCancel.String()
		if ToolAllowed(participation.LocalSpec(), toolspkg.ToolIDTaskCancel.String()) {
			t.Fatal("ToolAllowed(task.cancel) = true after caller mutation, want immutable allowlist")
		}
		policy := PermissionPolicy(participation.LocalSpec())
		if slices.Contains(policy.Tools, toolspkg.ToolIDTaskCancel.String()) {
			t.Fatalf("PermissionPolicy() Tools = %#v, want no caller-mutated task.cancel", policy.Tools)
		}

		kinds := OperationalMessageKinds()
		if len(kinds) == 0 {
			t.Fatal("OperationalMessageKinds() returned empty list, want message kinds")
		}
		kinds[0] = "mutated-kind"
		overlay := PromptOverlay(PromptInput{NetworkParticipation: participation.LocalSpec()})
		if strings.Contains(overlay, "mutated-kind") {
			t.Fatalf("PromptOverlay() used caller-mutated message kind:\n%s", overlay)
		}
	})
}

func TestLineageAndHealthySession(t *testing.T) {
	t.Parallel()

	t.Run("Should build coordinator lineage and reject unhealthy sessions", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
		cfg := aghconfig.DefaultResolvedCoordinatorRole()
		cfg.Enabled = true
		cfg.TTL = 2 * time.Hour
		cfg.MaxChildren = 3
		policy := PermissionPolicy(participation.Spec{
			Version: participation.SpecVersion, Mode: participation.ModeLive,
			WorkspaceID: "ws-1", ChannelStrategy: participation.StrategyNamed,
			ChannelID: "ch-1", Source: participation.SourceExplicitRequest,
		})

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
		if !slices.Equal(lineage.PermissionPolicy.NetworkChannels, []string{"ch-1"}) {
			t.Fatalf("PermissionPolicy.NetworkChannels = %#v, want ch-1", lineage.PermissionPolicy.NetworkChannels)
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

func TestPromptOverlayUsesParticipationSpecificPublicAPIs(t *testing.T) {
	t.Parallel()

	t.Run("Should omit channel guidance for local coordinators", func(t *testing.T) {
		t.Parallel()

		overlay := PromptOverlay(PromptInput{
			WorkspaceID:          "ws-1",
			TaskID:               "task-1",
			RunID:                "run-1",
			WorkflowID:           "wf-1",
			NetworkParticipation: participation.LocalSpec(),
		})
		for _, required := range []string{
			"agh me context",
			"agh task create",
			"agh task next|heartbeat|complete|fail|release",
			"agh spawn",
			"The current coordinator run is the active execution boundary",
			"Never spawn another coordinator",
		} {
			if !strings.Contains(overlay, required) {
				t.Fatalf("PromptOverlay missing %q:\n%s", required, overlay)
			}
		}
		for _, forbidden := range []string{"agh ch", "participation_channel", "coordination_channel_id", "Channel communication"} {
			if strings.Contains(overlay, forbidden) {
				t.Fatalf("PromptOverlay contains local-only forbidden guidance %q:\n%s", forbidden, overlay)
			}
		}
	})

	t.Run("Should include channel guidance for live coordinators", func(t *testing.T) {
		t.Parallel()

		overlay := PromptOverlay(PromptInput{
			WorkspaceID: "ws-1", TaskID: "task-1", RunID: "run-1", WorkflowID: "wf-1",
			NetworkParticipation: participation.Spec{
				Version: participation.SpecVersion, Mode: participation.ModeLive,
				WorkspaceID: "ws-1", ChannelStrategy: participation.StrategyNamed,
				ChannelID: "ch-run-1", Source: participation.SourceExplicitRequest,
			},
		})
		for _, required := range []string{"agh ch list|recv|send|reply", "participation_channel: ch-run-1", "Channel communication"} {
			if !strings.Contains(overlay, required) {
				t.Fatalf("PromptOverlay missing %q:\n%s", required, overlay)
			}
		}
	})
}
