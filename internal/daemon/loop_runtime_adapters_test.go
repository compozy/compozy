package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/gate"
	goalpkg "github.com/compozy/compozy/internal/loop/goal"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	worktreepkg "github.com/compozy/compozy/internal/worktree"
)

func TestLoopCreationProfile(t *testing.T) {
	t.Parallel()

	t.Run("Should retain profile ownership in the immutable creation identity", func(t *testing.T) {
		t.Parallel()

		profile := profileFromPolicyResolution(
			session.CreateOpts{ProfileID: "profile-marketing", CWD: "/workspace"},
			&loopSessionPolicyResolution{
				workspace: workspacepkg.ResolvedWorkspace{Workspace: workspacepkg.Workspace{ID: "workspace-1"}},
				agent: compozyconfig.ResolvedAgent{
					Name: "reviewer", Provider: "codex", Model: "gpt-5", Permissions: "workspace-write",
				},
			},
		)
		if profile.ProfileID != "profile-marketing" {
			t.Fatalf("profileFromPolicyResolution().ProfileID = %q, want profile-marketing", profile.ProfileID)
		}
		if _, err := profile.Ref(); err != nil {
			t.Fatalf("profileFromPolicyResolution().Ref() error = %v", err)
		}
	})
}

func TestLoopCancellationSessionControllerShouldTreatMissingSessionsAsStopped(t *testing.T) {
	t.Parallel()

	operationErr := errors.New("session operation failed")
	tests := []struct {
		name    string
		err     error
		wantErr error
	}{
		{
			name: "Should accept a missing session as already stopped",
			err:  fmt.Errorf("missing prompt session: %w", session.ErrSessionNotFound),
		},
		{
			name: "Should accept an inactive session as already stopped",
			err:  fmt.Errorf("inactive prompt session: %w", session.ErrSessionNotActive),
		},
		{
			name:    "Should propagate process stop failures",
			err:     operationErr,
			wantErr: operationErr,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			sessions := &loopActionBinderSessionManager{stopErr: test.err}
			controller := loopCancellationSessionController{sessions: sessions}
			err := controller.StopLoopSession(t.Context(), "sess-cancel", "operator request")
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("session cancellation error = %v, want %v", err, test.wantErr)
			}

			sessions.mu.Lock()
			defer sessions.mu.Unlock()
			if !slices.Equal(sessions.stopIDs, []string{"sess-cancel"}) {
				t.Fatalf("StopWithCause session ids = %#v, want one exact session", sessions.stopIDs)
			}
			if !slices.Equal(sessions.stopCauses, []session.StopCause{session.CauseUserRequested}) {
				t.Fatalf("StopWithCause causes = %#v, want user requested", sessions.stopCauses)
			}
			if !slices.Equal(sessions.stopDetails, []string{"operator request"}) {
				t.Fatalf("StopWithCause details = %#v, want exact operator detail", sessions.stopDetails)
			}
		})
	}
}

func TestLoopActionToolWorkspaceRootResolverShouldRequireReadyWorktrees(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve a ready worktree in the requested workspace", func(t *testing.T) {
		t.Parallel()

		worktrees := &recordingTaskBridgeWorktrees{getItem: &worktreepkg.Worktree{
			ID: "wt-ready", WorkspaceID: "ws-loop", Path: "/worktrees/ready", State: worktreepkg.StateReady,
		}}
		resolver := &loopActionToolWorkspaceRootResolver{worktrees: worktrees}

		root, err := resolver.ResolveActionToolWorkspaceRoot(t.Context(), looppkg.ActionToolWorkspaceRootRequest{
			WorkspaceID: "ws-loop",
			Environment: dsl.EnvironmentSpec{Mode: dsl.EnvironmentWorktree, WorktreeRef: "ready-ref"},
		})
		if err != nil {
			t.Fatalf("ResolveActionToolWorkspaceRoot() error = %v", err)
		}
		if got, want := root, "/worktrees/ready"; got != want {
			t.Fatalf("ResolveActionToolWorkspaceRoot() = %q, want %q", got, want)
		}
		if len(worktrees.getCalls) != 1 || worktrees.getCalls[0] != (taskBridgeWorktreeGetCall{
			workspaceID: "ws-loop", ref: "ready-ref",
		}) {
			t.Fatalf("Get() calls = %#v, want workspace-scoped ready-ref", worktrees.getCalls)
		}
	})

	t.Run("Should preserve typed missing and non-ready worktree errors", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name      string
			worktrees *recordingTaskBridgeWorktrees
			wantErr   error
		}{
			{
				name:      "Should preserve a missing worktree lookup error",
				worktrees: &recordingTaskBridgeWorktrees{getErr: worktreepkg.ErrNotFound},
				wantErr:   worktreepkg.ErrNotFound,
			},
			{
				name: "Should reject a worktree that is not ready",
				worktrees: &recordingTaskBridgeWorktrees{getItem: &worktreepkg.Worktree{
					ID: "wt-pending", WorkspaceID: "ws-loop", State: worktreepkg.StatePending,
				}},
				wantErr: worktreepkg.ErrNotReady,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				resolver := &loopActionToolWorkspaceRootResolver{worktrees: tt.worktrees}
				_, err := resolver.ResolveActionToolWorkspaceRoot(
					t.Context(),
					looppkg.ActionToolWorkspaceRootRequest{
						WorkspaceID: "ws-loop",
						Environment: dsl.EnvironmentSpec{
							Mode: dsl.EnvironmentWorktree, WorktreeRef: "pending-ref",
						},
					},
				)
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("ResolveActionToolWorkspaceRoot() error = %v, want %v", err, tt.wantErr)
				}
			})
		}
	})
}

func TestLoopActionSessionBinderShouldApplyPolicyGate(t *testing.T) {
	t.Parallel()

	t.Run("Should bind run-agent sessions with workspace policy and allowed tools", func(t *testing.T) {
		t.Parallel()

		allowedTools := []string{
			string(toolspkg.ToolIDTaskRead),
			string(toolspkg.ToolIDTaskUpdate),
		}
		resolved := loopActionBinderWorkspace(t, []compozyconfig.AgentDef{{
			Name:        "task-worker",
			Provider:    "mock",
			Prompt:      "Handle the loop node.",
			Tools:       append([]string(nil), allowedTools...),
			Permissions: string(compozyconfig.PermissionModeDenyAll),
		}})
		sessions := &loopActionBinderSessionManager{sessionID: "sess-loop-policy"}
		loopParticipation := daemonTestLiveParticipation("ws-loop", "loop-run-channel")
		binder := &loopActionSessionBinder{
			sessions: sessions,
			policyGate: &loopSessionPolicyGate{
				workspaceResolver: loopActionBinderWorkspaceResolver{
					byID: map[string]workspacepkg.ResolvedWorkspace{"ws-loop": resolved},
				},
			},
		}

		binding, err := binder.BindActionSession(context.Background(), looppkg.ActionSessionBindRequest{
			WorkspaceID:  looppkg.WorkspaceID("ws-loop"),
			LoopRunID:    looppkg.RunID("loop-run-policy"),
			Agent:        "task-worker",
			Handle:       "execute_task",
			AllowedTools: []string{allowedTools[1], allowedTools[0], allowedTools[0]},
			Runtime: &looppkg.RuntimeSpec{
				Provider: "codex", Model: "gpt-5.6-terra", Reasoning: "high",
				ACPOptions: []looppkg.ACPOptionSelection{{ID: "thinking", BoolValue: new(true)}},
			},
			ContractBlock:             "Follow the loop contract.",
			NetworkParticipation:      new(loopParticipation),
			ProvenanceParentSessionID: "sess-orchestrator",
		})
		if err != nil {
			t.Fatalf("BindActionSession() error = %v", err)
		}
		if got, want := binding.SessionID, "sess-loop-policy"; got != want {
			t.Fatalf("binding.SessionID = %q, want %q", got, want)
		}
		createCall := sessions.singleCreateCall(t)
		if got, want := createCall.SandboxRef, "evidence-lab"; got != want {
			t.Fatalf("CreateOpts.SandboxRef = %q, want %q", got, want)
		}
		if got, want := createCall.Permissions, compozyconfig.PermissionModeDenyAll; got != want {
			t.Fatalf("CreateOpts.Permissions = %q, want %q", got, want)
		}
		if got := createCall.RuntimeMode; got != "" {
			t.Fatalf("CreateOpts.RuntimeMode = %q, want normal action runtime", got)
		}
		if got, want := createCall.Workspace, "ws-loop"; got != want {
			t.Fatalf("CreateOpts.Workspace = %q, want %q", got, want)
		}
		if len(createCall.ACPOptions) != 1 || createCall.ACPOptions[0].ID != "thinking" ||
			createCall.ACPOptions[0].BoolValue == nil || !*createCall.ACPOptions[0].BoolValue {
			t.Fatalf("CreateOpts.ACPOptions = %#v, want thinking=true", createCall.ACPOptions)
		}
		if got := participationSnapshotValue(createCall.ResolvedNetworkParticipation); got != loopParticipation {
			t.Fatalf("CreateOpts participation = %#v, want owner-bound %#v", got, loopParticipation)
		}
		if got, want := createCall.NetworkOwnerKey, "loop_run:loop-run-policy"; got != want {
			t.Fatalf("CreateOpts.NetworkOwnerKey = %q, want %q", got, want)
		}
		if createCall.ProvenanceParentSessionID != "" {
			t.Fatalf(
				"ephemeral CreateOpts.ProvenanceParentSessionID = %q, want empty",
				createCall.ProvenanceParentSessionID,
			)
		}
		if !slices.Equal(createCall.AllowedToolsOverride, allowedTools) {
			t.Fatalf(
				"CreateOpts.AllowedToolsOverride = %#v, want %#v",
				createCall.AllowedToolsOverride,
				allowedTools,
			)
		}
		wantDenied := loopActionTerminalTools()
		if !slices.Equal(createCall.DeniedToolsOverride, wantDenied) {
			t.Fatalf(
				"CreateOpts.DeniedToolsOverride = %#v, want %#v",
				createCall.DeniedToolsOverride,
				wantDenied,
			)
		}
		if slices.Contains(createCall.DeniedToolsOverride, toolspkg.ToolIDTaskRunHeartbeat.String()) {
			t.Fatalf(
				"CreateOpts.DeniedToolsOverride = %#v, heartbeat must remain available",
				createCall.DeniedToolsOverride,
			)
		}
		if createCall.Provider != "codex" || createCall.Model != "gpt-5.6-terra" ||
			createCall.ReasoningEffort != "high" {
			t.Fatalf("CreateOpts runtime = %#v, want codex/gpt-5.6-terra@high", createCall)
		}
		if binding.AppliedRuntime.Provider != createCall.Provider ||
			binding.AppliedRuntime.Model != createCall.Model ||
			binding.AppliedRuntime.Reasoning != createCall.ReasoningEffort {
			t.Fatalf(
				"binding AppliedRuntime = %#v, want final CreateOpts runtime",
				binding.AppliedRuntime,
			)
		}
		if len(binding.AppliedRuntime.ACPOptions) != 1 ||
			binding.AppliedRuntime.ACPOptions[0].ID != "thinking" ||
			binding.AppliedRuntime.ACPOptions[0].BoolValue == nil ||
			!*binding.AppliedRuntime.ACPOptions[0].BoolValue {
			t.Fatalf(
				"binding AppliedRuntime ACPOptions = %#v, want thinking=true",
				binding.AppliedRuntime.ACPOptions,
			)
		}
	})

	t.Run("Should carry provenance in run-owned managed creation options", func(t *testing.T) {
		t.Parallel()

		resolved := loopActionBinderWorkspace(t, []compozyconfig.AgentDef{{
			Name: "task-worker", Provider: "mock", Prompt: "Handle the Goal node.",
		}})
		binder := &loopActionSessionBinder{policyGate: &loopSessionPolicyGate{
			workspaceResolver: loopActionBinderWorkspaceResolver{
				byID: map[string]workspacepkg.ResolvedWorkspace{"ws-loop": resolved},
			},
		}}
		_, opts, _, err := binder.resolveRunOwnedBindingProfile(
			context.Background(),
			looppkg.ActionSessionBindRequest{
				ProfileID: store.DefaultProfileID, WorkspaceID: "ws-loop",
				LoopRunID: "loop-run-goal", Agent: "task-worker",
				ProvenanceParentSessionID: "sess-orchestrator",
			},
			goalpkg.SessionBinding{},
			false,
		)
		if err != nil {
			t.Fatalf("resolveRunOwnedBindingProfile() error = %v", err)
		}
		if opts.ProvenanceParentSessionID != "sess-orchestrator" {
			t.Fatalf(
				"CreateOpts.ProvenanceParentSessionID = %q, want sess-orchestrator",
				opts.ProvenanceParentSessionID,
			)
		}
	})

	t.Run("Should report agent runtime when engine runtime is empty", func(t *testing.T) {
		t.Parallel()

		resolved := loopActionBinderWorkspace(t, []compozyconfig.AgentDef{{
			Name: "task-worker", Provider: "mock", Model: "agent-model", Prompt: "Handle the loop node.",
		}})
		sessions := &loopActionBinderSessionManager{sessionID: "sess-loop-agent-runtime"}
		binder := &loopActionSessionBinder{
			sessions: sessions,
			policyGate: &loopSessionPolicyGate{workspaceResolver: loopActionBinderWorkspaceResolver{
				byID: map[string]workspacepkg.ResolvedWorkspace{"ws-loop": resolved},
			}},
		}

		binding, err := binder.BindActionSession(context.Background(), looppkg.ActionSessionBindRequest{
			WorkspaceID: "ws-loop", Agent: "task-worker", Handle: "execute_task",
		})
		if err != nil {
			t.Fatalf("BindActionSession() error = %v", err)
		}
		if binding.AppliedRuntime.Provider != "mock" || binding.AppliedRuntime.Model != "agent-model" {
			t.Fatalf("AppliedRuntime = %#v, want agent runtime", binding.AppliedRuntime)
		}
	})

	t.Run("Should materialize one worktree for each per-run action instance", func(t *testing.T) {
		t.Parallel()

		resolved := loopActionBinderWorkspace(t, []compozyconfig.AgentDef{{
			Name: "task-worker", Provider: "mock", Prompt: "Handle the loop node.",
		}})
		sessions := &loopActionBinderSessionManager{sessionID: "sess-loop-worktree"}
		worktrees := &recordingTaskBridgeWorktrees{
			materializeFn: func(workspaceID string, request worktreepkg.RunWorktreeRequest) (*worktreepkg.Worktree, error) {
				item := int(request.RunID[len(request.RunID)-1] - '0')
				return &worktreepkg.Worktree{
					ID:          fmt.Sprintf("wt-loop-action-%d", item),
					WorkspaceID: workspaceID,
					Path:        fmt.Sprintf("/worktrees/loop-action-%d", item),
					Origin:      worktreepkg.OriginPerRun,
					RunID:       request.RunID,
					State:       worktreepkg.StateReady,
				}, nil
			},
		}
		binder := &loopActionSessionBinder{
			sessions:  sessions,
			worktrees: worktrees,
			policyGate: &loopSessionPolicyGate{workspaceResolver: loopActionBinderWorkspaceResolver{
				byID: map[string]workspacepkg.ResolvedWorkspace{"ws-loop": resolved},
			}},
		}

		for itemIndex := range 3 {
			_, err := binder.BindActionSession(context.Background(), looppkg.ActionSessionBindRequest{
				ProfileID:   "profile-marketing",
				WorkspaceID: "ws-loop",
				LoopRunID:   "loop-run-worktree",
				Generation:  2,
				NodeID:      "review",
				ItemIndex:   itemIndex,
				Agent:       "task-worker",
				Handle:      "review",
				Environment: &dsl.EnvironmentSpec{Mode: dsl.EnvironmentPerRun},
			})
			if err != nil {
				t.Fatalf("BindActionSession(item=%d) error = %v", itemIndex, err)
			}
		}
		if got, want := len(worktrees.materializeCalls), 3; got != want {
			t.Fatalf("MaterializeForRun() calls = %d, want %d", got, want)
		}
		for itemIndex, call := range worktrees.materializeCalls {
			wantRunID := fmt.Sprintf("loop:loop-run-worktree:2:review:%d", itemIndex)
			if call.workspaceID != "ws-loop" || call.request.RunID != wantRunID ||
				call.request.ProfileID != "profile-marketing" {
				t.Fatalf("MaterializeForRun() call[%d] = %#v, want %s", itemIndex, call, wantRunID)
			}
		}
		createCalls := sessions.allCreateCalls()
		if len(createCalls) != 3 {
			t.Fatalf("Create call count = %d, want 3", len(createCalls))
		}
		for itemIndex, createCall := range createCalls {
			wantWorktree := fmt.Sprintf("wt-loop-action-%d", itemIndex)
			wantPath := fmt.Sprintf("/worktrees/loop-action-%d", itemIndex)
			if createCall.Worktree != wantWorktree || createCall.CWD != wantPath {
				t.Fatalf(
					"CreateOpts[%d] environment = worktree %q cwd %q, want %q/%q",
					itemIndex,
					createCall.Worktree,
					createCall.CWD,
					wantWorktree,
					wantPath,
				)
			}
		}
	})

	t.Run("Should reject a worktree reference removed before session binding", func(t *testing.T) {
		t.Parallel()

		resolved := loopActionBinderWorkspace(t, []compozyconfig.AgentDef{{
			Name: "task-worker", Provider: "mock", Prompt: "Handle the loop node.",
		}})
		sessions := &loopActionBinderSessionManager{}
		worktrees := &recordingTaskBridgeWorktrees{getItem: &worktreepkg.Worktree{
			ID: "wt-removed", WorkspaceID: "ws-loop", State: worktreepkg.StateMissing,
		}}
		binder := &loopActionSessionBinder{
			sessions:  sessions,
			worktrees: worktrees,
			policyGate: &loopSessionPolicyGate{workspaceResolver: loopActionBinderWorkspaceResolver{
				byID: map[string]workspacepkg.ResolvedWorkspace{"ws-loop": resolved},
			}},
		}

		_, err := binder.BindActionSession(context.Background(), looppkg.ActionSessionBindRequest{
			WorkspaceID: "ws-loop",
			LoopRunID:   "loop-run-removed-ref",
			Generation:  1,
			NodeID:      "review",
			Agent:       "task-worker",
			Environment: &dsl.EnvironmentSpec{
				Mode: dsl.EnvironmentWorktree, WorktreeRef: "removed-loop-ref",
			},
		})
		if !errors.Is(err, worktreepkg.ErrNotReady) || !strings.Contains(err.Error(), "removed-loop-ref") {
			t.Fatalf("BindActionSession() error = %v, want named removed worktree", err)
		}
		if got := sessions.createCount(); got != 0 {
			t.Fatalf("Create() calls = %d, want none", got)
		}
		if len(worktrees.getCalls) != 1 || worktrees.getCalls[0].ref != "removed-loop-ref" {
			t.Fatalf("Get() calls = %#v, want late resolution of removed-loop-ref", worktrees.getCalls)
		}
	})

	t.Run("Should roll back a per-run action worktree when session creation fails", func(t *testing.T) {
		t.Parallel()

		resolved := loopActionBinderWorkspace(t, []compozyconfig.AgentDef{{
			Name: "task-worker", Provider: "mock", Prompt: "Handle the loop node.",
		}})
		wantErr := errors.New("loop session create failed")
		sessions := &loopActionBinderSessionManager{createErr: wantErr}
		worktrees := &recordingTaskBridgeWorktrees{materialized: &worktreepkg.Worktree{
			ID:          "wt-loop-failure",
			WorkspaceID: "ws-loop",
			Path:        "/worktrees/loop-failure",
			Origin:      worktreepkg.OriginPerRun,
			RunID:       "loop:loop-run-failure:1:review:0",
			State:       worktreepkg.StateReady,
		}}
		binder := &loopActionSessionBinder{
			sessions:  sessions,
			worktrees: worktrees,
			policyGate: &loopSessionPolicyGate{workspaceResolver: loopActionBinderWorkspaceResolver{
				byID: map[string]workspacepkg.ResolvedWorkspace{"ws-loop": resolved},
			}},
		}

		_, err := binder.BindActionSession(context.Background(), looppkg.ActionSessionBindRequest{
			WorkspaceID: "ws-loop",
			LoopRunID:   "loop-run-failure",
			Generation:  1,
			NodeID:      "review",
			Agent:       "task-worker",
			Handle:      "review",
			Environment: &dsl.EnvironmentSpec{Mode: dsl.EnvironmentPerRun},
		})
		if !errors.Is(err, wantErr) {
			t.Fatalf("BindActionSession() error = %v, want %v", err, wantErr)
		}
		if got, want := len(worktrees.rollbackCalls), 1; got != want {
			t.Fatalf("RollbackRunMaterialization() calls = %d, want %d", got, want)
		}
		if got := worktrees.rollbackCalls[0]; got.worktreeID != "wt-loop-failure" ||
			got.runID != "loop:loop-run-failure:1:review:0" {
			t.Fatalf("RollbackRunMaterialization() call = %#v, want exact execution instance", got)
		}
	})

	t.Run("Should propagate allowed-tools widening failures without a session binding", func(t *testing.T) {
		t.Parallel()

		readTool := string(toolspkg.ToolIDTaskRead)
		updateTool := string(toolspkg.ToolIDTaskUpdate)
		resolved := loopActionBinderWorkspace(t, []compozyconfig.AgentDef{{
			Name:     "task-worker",
			Provider: "mock",
			Prompt:   "Handle the loop node.",
			Tools:    []string{readTool},
		}})
		wideningErr := fmt.Errorf(
			"%w: allowed_tools includes %q which widens agent profile",
			session.ErrValidation,
			updateTool,
		)
		sessions := &loopActionBinderSessionManager{createErr: wideningErr}
		binder := &loopActionSessionBinder{
			sessions: sessions,
			policyGate: &loopSessionPolicyGate{
				workspaceResolver: loopActionBinderWorkspaceResolver{
					byID: map[string]workspacepkg.ResolvedWorkspace{"ws-loop": resolved},
				},
			},
		}

		_, err := binder.BindActionSession(context.Background(), looppkg.ActionSessionBindRequest{
			WorkspaceID:   looppkg.WorkspaceID("ws-loop"),
			Agent:         "task-worker",
			Handle:        "execute_task",
			AllowedTools:  []string{updateTool},
			ContractBlock: "Follow the loop contract.",
		})
		if !errors.Is(err, session.ErrValidation) {
			t.Fatalf("BindActionSession() error = %v, want session.ErrValidation", err)
		}
		if !strings.Contains(err.Error(), updateTool) {
			t.Fatalf("BindActionSession() error = %v, want rejected tool id", err)
		}
		if got, want := sessions.createCount(), 1; got != want {
			t.Fatalf("Create call count = %d, want %d validation attempt", got, want)
		}
	})

	t.Run("Should fail deterministically when the target agent cannot resolve", func(t *testing.T) {
		t.Parallel()

		resolved := loopActionBinderWorkspace(t, nil)
		sessions := &loopActionBinderSessionManager{sessionID: "unused"}
		binder := &loopActionSessionBinder{
			sessions: sessions,
			policyGate: &loopSessionPolicyGate{
				workspaceResolver: loopActionBinderWorkspaceResolver{
					byID: map[string]workspacepkg.ResolvedWorkspace{"ws-loop": resolved},
				},
			},
		}

		_, err := binder.BindActionSession(context.Background(), looppkg.ActionSessionBindRequest{
			WorkspaceID: looppkg.WorkspaceID("ws-loop"),
			Agent:       "missing-worker",
			Handle:      "execute_task",
		})
		if !errors.Is(err, workspacepkg.ErrAgentNotAvailable) {
			t.Fatalf("BindActionSession() error = %v, want ErrAgentNotAvailable", err)
		}
		if got := sessions.createCount(); got != 0 {
			t.Fatalf("Create call count = %d, want 0", got)
		}
	})

	t.Run("Should resolve a registered workspace path without registering it", func(t *testing.T) {
		t.Parallel()

		resolved := loopActionBinderWorkspace(t, []compozyconfig.AgentDef{{
			Name:        "task-worker",
			Provider:    "mock",
			Prompt:      "Handle the loop node.",
			Permissions: string(compozyconfig.PermissionModeDenyAll),
		}})
		workspacePath := resolved.RootDir
		resolveOrRegisterCalled := false
		sessions := &loopActionBinderSessionManager{sessionID: "sess-loop-path"}
		binder := &loopActionSessionBinder{
			sessions: sessions,
			policyGate: &loopSessionPolicyGate{
				workspaceResolver: loopActionBinderWorkspaceResolver{
					byPath:                  map[string]workspacepkg.ResolvedWorkspace{workspacePath: resolved},
					resolveOrRegisterCalled: &resolveOrRegisterCalled,
				},
			},
		}

		opts := session.CreateOpts{
			WorkspacePath: workspacePath,
		}
		_, err := binder.policyGate.applyResolved(context.Background(), &opts, "task-worker", nil)
		if err != nil {
			t.Fatalf("applyResolved() error = %v", err)
		}
		if resolveOrRegisterCalled {
			t.Fatal("ResolveOrRegister() was called for a registered loop workspace path")
		}
		if opts.SandboxRef != resolved.SandboxRef ||
			opts.Permissions != compozyconfig.PermissionModeDenyAll {
			t.Fatalf("resolved CreateOpts = %#v, want registered workspace sandbox and permissions", opts)
		}
	})
}

func TestLoopGateCommandRunnerShouldExecuteInResolvedWorkspace(t *testing.T) {
	t.Run("Should resolve the run workspace before starting the command", func(t *testing.T) {
		t.Parallel()

		rootDir := t.TempDir()
		runner := loopGateCommandRunner{workspaceResolver: loopActionBinderWorkspaceResolver{
			byID: map[string]workspacepkg.ResolvedWorkspace{
				"ws-command": {Workspace: workspacepkg.Workspace{ID: "ws-command", RootDir: rootDir}},
			},
		}}
		result, err := runner.RunCommand(t.Context(), gate.CommandRequest{
			WorkspaceID: "ws-command",
			Command:     "pwd",
		})
		if err != nil {
			t.Fatalf("RunCommand() error = %v", err)
		}
		if got, want := strings.TrimSpace(result.Stdout), rootDir; got != want {
			t.Fatalf("RunCommand() stdout = %q, want workspace root %q", got, want)
		}
	})

	t.Run("Should fail before execution when the workspace cannot be resolved", func(t *testing.T) {
		t.Parallel()

		runner := loopGateCommandRunner{workspaceResolver: loopActionBinderWorkspaceResolver{}}
		_, err := runner.RunCommand(t.Context(), gate.CommandRequest{
			WorkspaceID: "ws-missing",
			Command:     "exit 0",
		})
		if !errors.Is(err, workspacepkg.ErrWorkspaceNotFound) {
			t.Fatalf("RunCommand() error = %v, want ErrWorkspaceNotFound", err)
		}
	})
}

func TestValidatePinnedRuntimeShouldRejectRuntimeDivergence(t *testing.T) {
	t.Parallel()

	profile := store.SessionCreationProfile{
		ProfileID: store.DefaultProfileID,
		Provider:  "claude", Model: "opus", ReasoningEffort: "high",
		ACPOptions: []store.SessionACPOptionSelection{{ID: "thinking", BoolValue: new(true)}},
	}
	cases := []struct {
		name    string
		runtime looppkg.RuntimeSpec
	}{
		{name: "Should reject provider divergence", runtime: looppkg.RuntimeSpec{Provider: "codex"}},
		{name: "Should reject model divergence", runtime: looppkg.RuntimeSpec{Model: "sonnet"}},
		{name: "Should reject reasoning divergence", runtime: looppkg.RuntimeSpec{Reasoning: "max"}},
		{
			name: "Should reject ACP option divergence",
			runtime: looppkg.RuntimeSpec{
				ACPOptions: []looppkg.ACPOptionSelection{{ID: "thinking", BoolValue: new(false)}},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := validatePinnedRuntime(tc.runtime, profile)
			reason, reasonMatched := errors.AsType[*looppkg.ReasonError](err)
			if !reasonMatched || reason.Code != looppkg.ReasonCodeContinuousBindingMismatch ||
				!errors.Is(err, looppkg.ErrTransitionConflict) {
				t.Fatalf("validatePinnedRuntime() error = %v, want continuous binding mismatch", err)
			}
		})
	}
}

func TestLoopGateJudgeRunnerShouldApplyPolicyGate(t *testing.T) {
	t.Parallel()

	t.Run("Should force an isolated judge policy and stop the temporary session", func(t *testing.T) {
		t.Parallel()

		resolved := loopActionBinderWorkspace(t, []compozyconfig.AgentDef{{
			Name:        "loop-judge",
			Provider:    "mock",
			Prompt:      "Judge the loop gate.",
			Permissions: string(compozyconfig.PermissionModeApproveAll),
		}})
		sessions := &loopActionBinderSessionManager{
			sessionID: "sess-loop-judge",
			events: []acp.AgentEvent{{
				Type: acp.EventTypeAgentMessage,
				Text: `{"verdict":"pass","evidence":{"checked":true}}`,
			}},
		}
		loopParticipation := daemonTestLiveParticipation("ws-loop", "loop-run-channel")
		runner := &loopGateJudgeRunner{
			sessions: sessions,
			policyGate: &loopSessionPolicyGate{
				workspaceResolver: loopActionBinderWorkspaceResolver{
					byID: map[string]workspacepkg.ResolvedWorkspace{"ws-loop": resolved},
				},
			},
			executions: newLoopJudgeExecutionRegistry(),
		}

		_, err := runner.Judge(context.Background(), gate.JudgeRequest{
			GateID:        "quality-gate",
			CriterionID:   "review",
			LoopRunID:     "loop-run-judge",
			Attempt:       2,
			CorrelationID: "goal-judge:stable-attempt",
			WorkspaceID:   "ws-loop",
			Agent:         "loop-judge",
			Runtime: looppkg.RuntimeSpec{
				ACPOptions: []looppkg.ACPOptionSelection{{ID: "thinking", BoolValue: new(true)}},
			},
			Rubric:               "Review the evidence.",
			NetworkParticipation: new(loopParticipation),
		})
		if err != nil {
			t.Fatalf("Judge() error = %v", err)
		}
		createCall := sessions.singleCreateCall(t)
		if got, want := createCall.SandboxRef, "evidence-lab"; got != want {
			t.Fatalf("CreateOpts.SandboxRef = %q, want %q", got, want)
		}
		if got, want := createCall.Permissions, compozyconfig.PermissionModeDenyAll; got != want {
			t.Fatalf("CreateOpts.Permissions = %q, want %q", got, want)
		}
		if got := participationSnapshotValue(createCall.ResolvedNetworkParticipation); got != loopParticipation {
			t.Fatalf("CreateOpts participation = %#v, want owner-bound %#v", got, loopParticipation)
		}
		if got, want := createCall.NetworkOwnerKey, "loop_run:loop-run-judge"; got != want {
			t.Fatalf("CreateOpts.NetworkOwnerKey = %q, want %q", got, want)
		}
		if len(createCall.ACPOptions) != 1 || createCall.ACPOptions[0].ID != "thinking" ||
			createCall.ACPOptions[0].BoolValue == nil || !*createCall.ACPOptions[0].BoolValue {
			t.Fatalf("judge CreateOpts.ACPOptions = %#v, want thinking=true", createCall.ACPOptions)
		}
		if got, want := sessions.stopCount(), 1; got != want {
			t.Fatalf("Stop call count = %d, want %d", got, want)
		}
		if got, want := sessions.singleStopID(t), "sess-loop-judge"; got != want {
			t.Fatalf("Stop session ID = %q, want %q", got, want)
		}
		promptCall := sessions.singlePromptCall(t)
		if promptCall.PromptMeta.Judge == nil {
			t.Fatal("PromptOpts.PromptMeta.Judge = nil")
		}
		if got, want := *promptCall.PromptMeta.Judge, (acp.PromptJudgeMeta{
			Role: acp.PromptJudgeRoleAgent, Attempt: 2, CorrelationID: "goal-judge:stable-attempt",
			GateID: "quality-gate", CriterionID: "review",
		}); got != want {
			t.Fatalf("PromptOpts.PromptMeta.Judge = %#v, want %#v", got, want)
		}
	})

	t.Run("Should stop the temporary judge session after malformed output", func(t *testing.T) {
		t.Parallel()

		sessions := &loopActionBinderSessionManager{
			sessionID: "sess-loop-judge-malformed",
			events: []acp.AgentEvent{{
				Type: acp.EventTypeAgentMessage,
				Text: "not-json",
			}},
		}
		runner := loopJudgeRunnerForTest(t, sessions)
		response, err := runner.Judge(context.Background(), loopJudgeRequestForTest())
		if err != nil {
			t.Fatalf("Judge() error = %v", err)
		}
		if response.Raw != "not-json" {
			t.Fatalf("JudgeResponse.Raw = %q, want malformed output preserved", response.Raw)
		}
		if got := sessions.stopCount(); got != 1 {
			t.Fatalf("Stop call count = %d, want 1", got)
		}
	})

	t.Run("Should reject provider tool activity before accepting a parseable verdict", func(t *testing.T) {
		t.Parallel()

		sessions := &loopActionBinderSessionManager{
			sessionID: "sess-loop-judge-tool-leak",
			events: []acp.AgentEvent{
				{
					Type:       acp.EventTypeToolCall,
					ToolCallID: "cursor-read-1",
					Title:      "Read repository files",
				},
				{
					Type: acp.EventTypeAgentMessage,
					Text: `{"verdict":"pass","blocking_issues":[],"evidence":{"checked":true}}`,
				},
			},
		}
		_, err := loopJudgeRunnerForTest(t, sessions).Judge(context.Background(), loopJudgeRequestForTest())
		if err == nil || !strings.Contains(err.Error(), "verdict-only judge attempted tool activity") {
			t.Fatalf("Judge() error = %v, want verdict-only tool activity rejection", err)
		}
		if got, want := sessions.cancelCount(), 1; got != want {
			t.Fatalf("CancelPrompt call count = %d, want %d", got, want)
		}
		if got, want := sessions.stopCount(), 1; got != want {
			t.Fatalf("Stop call count = %d, want %d", got, want)
		}
	})

	t.Run("Should use only the final agent message as verdict authority", func(t *testing.T) {
		t.Parallel()

		const verdict = `{"verdict":"pass","blocking_issues":[],"evidence":{"candidate_text":"GREEN"}}`
		sessions := &loopActionBinderSessionManager{
			sessionID: "sess-loop-judge-reasoning",
			events: []acp.AgentEvent{
				{Type: acp.EventTypeThought, Text: "The candidate satisfies the contract."},
				{Type: acp.EventTypeAgentMessage, Text: verdict},
			},
		}
		response, err := loopJudgeRunnerForTest(t, sessions).Judge(
			context.Background(),
			loopJudgeRequestForTest(),
		)
		if err != nil {
			t.Fatalf("Judge() error = %v", err)
		}
		if got := response.Raw; got != verdict {
			t.Fatalf("JudgeResponse.Raw = %q, want final agent verdict %q", got, verdict)
		}
		result := gate.ParseJudgeVerdict("review", dsl.CriterionAgentJudge, response.Raw)
		if !result.Passed || result.Outcome != gate.VerdictOutcomeApproved {
			t.Fatalf("ParseJudgeVerdict() = %#v, want approved", result)
		}
	})

	t.Run("Should concatenate streamed verdict chunks without inventing separators", func(t *testing.T) {
		t.Parallel()

		const verdict = `{"verdict":"pass","blocking_issues":[],"evidence":{"candidate_text":"GREEN","exact_match":true}}`
		split := len(verdict) / 2
		sessions := &loopActionBinderSessionManager{
			sessionID: "sess-loop-judge-chunked-verdict",
			events: []acp.AgentEvent{
				{Type: acp.EventTypeThought, Text: "The candidate satisfies the contract."},
				{Type: acp.EventTypeAgentMessage, Text: verdict[:split]},
				{Type: acp.EventTypeAgentMessage, Text: verdict[split:]},
			},
		}
		response, err := loopJudgeRunnerForTest(t, sessions).Judge(
			context.Background(),
			loopJudgeRequestForTest(),
		)
		if err != nil {
			t.Fatalf("Judge() error = %v", err)
		}
		if got := response.Raw; got != verdict {
			t.Fatalf("JudgeResponse.Raw = %q, want streamed verdict %q", got, verdict)
		}
		result := gate.ParseJudgeVerdict("review", dsl.CriterionAgentJudge, response.Raw)
		if !result.Passed || result.Outcome != gate.VerdictOutcomeApproved {
			t.Fatalf("ParseJudgeVerdict() = %#v, want approved", result)
		}
	})

	t.Run("Should cancel and join the exact active judge execution", func(t *testing.T) {
		t.Parallel()

		started := make(chan struct{})
		released := make(chan struct{})
		sessions := &loopActionBinderSessionManager{
			sessionID:              "sess-loop-judge-clear",
			blockPromptUntilCancel: true,
			promptStarted:          started,
			promptReleased:         released,
		}
		runner := loopJudgeRunnerForTest(t, sessions)
		runner.executions = newLoopJudgeExecutionRegistry()
		req := loopJudgeRequestForTest()
		req.CorrelationID = "judge-attempt-clear"
		judgeDone := make(chan error, 1)
		go func() {
			_, err := runner.Judge(context.Background(), req)
			judgeDone <- err
		}()
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("judge prompt did not start")
		}

		revoker := loopGoalPromptLeaseRevoker{judges: runner}
		if err := revoker.RevokeGoalPromptLease(
			context.Background(),
			looppkg.GoalPromptLease{JudgeAttemptID: "judge-attempt-clear"},
			string(looppkg.TransitionCauseGoalClear),
		); err != nil {
			t.Fatalf("RevokeGoalPromptLease() error = %v", err)
		}
		select {
		case err := <-judgeDone:
			if err != nil {
				t.Fatalf("Judge() error = %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("judge execution did not join after revoke")
		}
		if got, want := sessions.cancelCount(), 1; got != want {
			t.Fatalf("CancelPrompt call count = %d, want %d", got, want)
		}
		if got, want := sessions.stopCount(), 1; got != want {
			t.Fatalf("Stop call count = %d, want %d", got, want)
		}
	})

	t.Run("Should defer revocation until a creating judge can bind", func(t *testing.T) {
		t.Parallel()

		createStarted := make(chan struct{})
		createReleased := make(chan struct{})
		createCanceled := make(chan struct{})
		sessions := &loopActionBinderSessionManager{
			sessionID:      "sess-loop-judge-creating",
			createStarted:  createStarted,
			createReleased: createReleased,
			createCanceled: createCanceled,
		}
		runner := loopJudgeRunnerForTest(t, sessions)
		runner.executions = newLoopJudgeExecutionRegistry()
		req := loopJudgeRequestForTest()
		req.CorrelationID = "judge-attempt-creating"

		judgeDone := make(chan error, 1)
		go func() {
			_, err := runner.Judge(context.Background(), req)
			judgeDone <- err
		}()
		select {
		case <-createStarted:
		case <-time.After(5 * time.Second):
			t.Fatal("judge creation did not start")
		}

		revokeDone := make(chan error, 1)
		go func() {
			revokeDone <- runner.revokeExecution(context.Background(), req.CorrelationID)
		}()
		waitForCondition(t, "judge revocation recorded before bind", func() bool {
			runner.executions.mu.Lock()
			defer runner.executions.mu.Unlock()
			execution := runner.executions.active[req.CorrelationID]
			return execution != nil && execution.revoked
		})
		select {
		case <-createCanceled:
			t.Fatal("judge creation context canceled before the session could bind")
		default:
		}

		close(createReleased)
		select {
		case err := <-revokeDone:
			if err != nil {
				t.Fatalf("revokeExecution() error = %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("judge revocation did not join after creation completed")
		}
		select {
		case err := <-judgeDone:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("Judge() error = %v, want context.Canceled", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("judge execution did not finish after deferred revocation")
		}
		if got := sessions.cancelCount(); got != 0 {
			t.Fatalf("CancelPrompt call count = %d, want 0 before prompt start", got)
		}
		if got := sessions.stopCount(); got != 1 {
			t.Fatalf("Stop call count = %d, want 1 durable cleanup", got)
		}
		if sessions.stopObservedCanceledContext() {
			t.Fatal("Stop context was canceled, want detached cleanup context")
		}
	})

	t.Run("Should surface judge cleanup failure to the revoker", func(t *testing.T) {
		t.Parallel()

		stopErr := errors.New("persist stopped judge session")
		started := make(chan struct{})
		released := make(chan struct{})
		sessions := &loopActionBinderSessionManager{
			sessionID:              "sess-loop-judge-clear-stop-failure",
			stopErr:                stopErr,
			blockPromptUntilCancel: true,
			promptStarted:          started,
			promptReleased:         released,
		}
		runner := loopJudgeRunnerForTest(t, sessions)
		runner.executions = newLoopJudgeExecutionRegistry()
		req := loopJudgeRequestForTest()
		req.CorrelationID = "judge-attempt-clear-stop-failure"
		judgeDone := make(chan error, 1)
		go func() {
			_, err := runner.Judge(context.Background(), req)
			judgeDone <- err
		}()
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("judge prompt did not start")
		}

		revoker := loopGoalPromptLeaseRevoker{judges: runner}
		err := revoker.RevokeGoalPromptLease(
			context.Background(),
			looppkg.GoalPromptLease{JudgeAttemptID: req.CorrelationID},
			string(looppkg.TransitionCauseGoalClear),
		)
		if !errors.Is(err, stopErr) {
			t.Fatalf("RevokeGoalPromptLease() error = %v, want judge cleanup failure", err)
		}
		select {
		case err := <-judgeDone:
			if !errors.Is(err, stopErr) {
				t.Fatalf("Judge() error = %v, want judge cleanup failure", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("judge execution did not join after cleanup failure")
		}
	})

	t.Run("Should fence a judge revoked before runtime registration", func(t *testing.T) {
		t.Parallel()

		sessions := &loopActionBinderSessionManager{sessionID: "sess-loop-judge-late"}
		runner := loopJudgeRunnerForTest(t, sessions)
		runner.executions = newLoopJudgeExecutionRegistry()
		release, err := runner.executions.reserve("judge-attempt-revoked-before-begin")
		if err != nil {
			t.Fatalf("reserve judge execution: %v", err)
		}
		defer release()
		revoker := loopGoalPromptLeaseRevoker{judges: runner}
		if err := revoker.RevokeGoalPromptLease(
			context.Background(),
			looppkg.GoalPromptLease{JudgeAttemptID: "judge-attempt-revoked-before-begin"},
			string(looppkg.TransitionCauseGoalClear),
		); err != nil {
			t.Fatalf("RevokeGoalPromptLease() error = %v", err)
		}
		req := loopJudgeRequestForTest()
		req.CorrelationID = "judge-attempt-revoked-before-begin"
		_, err = runner.Judge(context.Background(), req)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Judge() error = %v, want context.Canceled", err)
		}
		if got := sessions.createCount(); got != 0 {
			t.Fatalf("Create call count = %d, want 0", got)
		}
	})

	t.Run("Should release an abandoned revoked reservation", func(t *testing.T) {
		t.Parallel()

		registry := newLoopJudgeExecutionRegistry()
		release, err := registry.reserve("judge-attempt-abandoned")
		if err != nil {
			t.Fatalf("reserve judge execution: %v", err)
		}
		runner := loopJudgeRunnerForTest(t, &loopActionBinderSessionManager{})
		runner.executions = registry
		if err := runner.revokeExecution(context.Background(), "judge-attempt-abandoned"); err != nil {
			t.Fatalf("revokeExecution() error = %v", err)
		}
		release()

		registry.mu.Lock()
		defer registry.mu.Unlock()
		if len(registry.pending) != 0 || len(registry.revoked) != 0 {
			t.Fatalf(
				"registry pending = %d revoked = %d, want no abandoned state",
				len(registry.pending),
				len(registry.revoked),
			)
		}
	})

	t.Run("Should stop with a detached context after prompt cancellation", func(t *testing.T) {
		t.Parallel()

		sessions := &loopActionBinderSessionManager{
			sessionID: "sess-loop-judge-canceled",
			promptErr: context.Canceled,
		}
		runner := loopJudgeRunnerForTest(t, sessions)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := runner.Judge(ctx, loopJudgeRequestForTest())
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Judge() error = %v, want context.Canceled", err)
		}
		if got := sessions.stopCount(); got != 1 {
			t.Fatalf("Stop call count = %d, want 1", got)
		}
		if sessions.stopObservedCanceledContext() {
			t.Fatal("Stop context was canceled, want detached cleanup context")
		}
	})

	t.Run("Should surface a temporary-session cleanup failure", func(t *testing.T) {
		t.Parallel()

		stopErr := errors.New("stop judge session")
		sessions := &loopActionBinderSessionManager{
			sessionID: "sess-loop-judge-stop-failure",
			events: []acp.AgentEvent{{
				Type: acp.EventTypeAgentMessage,
				Text: `{"verdict":"pass","evidence":{"checked":true}}`,
			}},
			stopErr: stopErr,
		}
		_, err := loopJudgeRunnerForTest(t, sessions).Judge(context.Background(), loopJudgeRequestForTest())
		if !errors.Is(err, stopErr) {
			t.Fatalf("Judge() error = %v, want cleanup failure", err)
		}
		if got := sessions.stopCount(); got != 1 {
			t.Fatalf("Stop call count = %d, want 1", got)
		}
	})

	t.Run("Should stop a partially created session when creation reports failure", func(t *testing.T) {
		t.Parallel()

		createErr := errors.New("create judge session")
		sessions := &loopActionBinderSessionManager{
			sessionID:                "sess-loop-judge-partial-create",
			createErr:                createErr,
			returnSessionOnCreateErr: true,
		}
		_, err := loopJudgeRunnerForTest(t, sessions).Judge(context.Background(), loopJudgeRequestForTest())
		if !errors.Is(err, createErr) {
			t.Fatalf("Judge() error = %v, want create failure", err)
		}
		if got := sessions.stopCount(); got != 1 {
			t.Fatalf("Stop call count = %d, want 1", got)
		}
	})
}

func TestCollectLoopPromptResultShouldNotTreatProtocolRawAsStructuredOutput(t *testing.T) {
	t.Parallel()

	t.Run("Should ignore ACP protocol raw and preserve agent text", func(t *testing.T) {
		t.Parallel()

		manager := loopPromptResultSessionManager{
			events: []acp.AgentEvent{{
				Text: "```json\n{\"summary\":\"done\",\"message\":\"loop channel result\"}\n```",
				Raw:  json.RawMessage(`{"session_update":"agent_message_chunk"}`),
			}},
		}
		result, err := collectLoopPromptResult(
			context.Background(),
			manager,
			"sess-loop",
			looppkg.ActionPromptRequest{Message: "loop event probe"},
		)
		if err != nil {
			t.Fatalf("collectLoopPromptResult() error = %v", err)
		}
		if strings.TrimSpace(result.Text) == "" || !strings.Contains(result.Text, `"summary":"done"`) {
			t.Fatalf("collectLoopPromptResult() text = %q, want agent text", result.Text)
		}
		if len(result.Structured) != 0 {
			t.Fatalf("collectLoopPromptResult() structured = %s, want empty protocol raw ignored", result.Structured)
		}
	})

	t.Run("Should concatenate streamed deltas without corrupting JSON string literals", func(t *testing.T) {
		t.Parallel()

		manager := loopPromptResultSessionManager{events: []acp.AgentEvent{
			{Type: acp.EventTypeAgentMessage, Text: `{"status":"completed","summary":"root scripts (dev:`},
			{Type: acp.EventTypeAgentMessage, Text: `server, test) pass",`},
			{Type: acp.EventTypeAgentMessage, Text: `"files_changed":["server/index.ts"]}`},
		}}
		result, err := collectLoopPromptResult(
			context.Background(),
			manager,
			"sess-loop-deltas",
			looppkg.ActionPromptRequest{Message: "loop delta probe"},
		)
		if err != nil {
			t.Fatalf("collectLoopPromptResult() error = %v", err)
		}
		if !json.Valid([]byte(result.Text)) {
			t.Fatalf("collectLoopPromptResult() text = %q, want valid JSON across deltas", result.Text)
		}
		var decoded map[string]any
		if err := json.Unmarshal([]byte(result.Text), &decoded); err != nil {
			t.Fatalf("unmarshal joined deltas error = %v", err)
		}
		if decoded["summary"] != "root scripts (dev:server, test) pass" {
			t.Fatalf("joined summary = %v, want delta boundary inside the string preserved", decoded["summary"])
		}
	})

	t.Run("Should keep only agent-message chunks when the agent answered", func(t *testing.T) {
		t.Parallel()

		manager := loopPromptResultSessionManager{events: []acp.AgentEvent{
			{Type: acp.EventTypeThought, Text: "Let me inspect {\"name\":\"todo-app\"} first"},
			{Type: acp.EventTypeUserMessage, Text: "kickoff directive"},
			{Type: acp.EventTypeAgentMessage, Text: `{"status":"completed","summary":"shipped"}`},
		}}
		result, err := collectLoopPromptResult(
			context.Background(),
			manager,
			"sess-loop-filter",
			looppkg.ActionPromptRequest{Message: "loop filter probe"},
		)
		if err != nil {
			t.Fatalf("collectLoopPromptResult() error = %v", err)
		}
		if strings.Contains(result.Text, "todo-app") || strings.Contains(result.Text, "kickoff directive") {
			t.Fatalf("collectLoopPromptResult() text = %q, want thought and user chunks excluded", result.Text)
		}
		if !strings.Contains(result.Text, `"status":"completed"`) {
			t.Fatalf("collectLoopPromptResult() text = %q, want the agent answer preserved", result.Text)
		}
	})

	t.Run("Should fall back to every chunk when no agent message arrived", func(t *testing.T) {
		t.Parallel()

		manager := loopPromptResultSessionManager{events: []acp.AgentEvent{
			{Type: acp.EventTypeThought, Text: "only thoughts this turn"},
		}}
		result, err := collectLoopPromptResult(
			context.Background(),
			manager,
			"sess-loop-fallback",
			looppkg.ActionPromptRequest{Message: "loop fallback probe"},
		)
		if err != nil {
			t.Fatalf("collectLoopPromptResult() error = %v", err)
		}
		if !strings.Contains(result.Text, "only thoughts this turn") {
			t.Fatalf("collectLoopPromptResult() text = %q, want fallback to full transcript", result.Text)
		}
	})

	t.Run("Should preserve a reported zero token total", func(t *testing.T) {
		t.Parallel()

		zero := int64(0)
		manager := loopPromptResultSessionManager{events: []acp.AgentEvent{{
			Usage: &acp.TokenUsage{TotalTokens: &zero},
		}}}
		var reported []int64
		result, err := collectLoopPromptResult(
			context.Background(),
			manager,
			"sess-loop",
			looppkg.ActionPromptRequest{
				Message: "loop usage probe",
				UsageReporter: looppkg.ActionUsageReporterFunc(func(tokens int64) {
					reported = append(reported, tokens)
				}),
			},
		)
		if err != nil {
			t.Fatalf("collectLoopPromptResult() error = %v", err)
		}
		if result.TokensUsed != 0 || !result.TokensReported || !slices.Equal(reported, []int64{0}) {
			t.Fatalf("collected usage = %#v reports = %#v, want reported zero", result, reported)
		}
	})
}

type loopActionBinderSessionManager struct {
	mu                       sync.Mutex
	sessionID                string
	createErr                error
	returnSessionOnCreateErr bool
	createStarted            chan struct{}
	createReleased           chan struct{}
	createCanceled           chan struct{}
	promptErr                error
	cancelErr                error
	stopErr                  error
	events                   []acp.AgentEvent
	blockPromptUntilCancel   bool
	promptStarted            chan struct{}
	promptReleased           chan struct{}
	promptReleaseOnce        sync.Once
	createCalls              []session.CreateOpts
	promptCalls              []session.PromptOpts
	cancelIDs                []string
	stopIDs                  []string
	stopCauses               []session.StopCause
	stopDetails              []string
	stopSawCanceledContexts  []bool
}

func (m *loopActionBinderSessionManager) Status(context.Context, string) (*session.Info, error) {
	return nil, session.ErrSessionNotFound
}

func (m *loopActionBinderSessionManager) Create(
	ctx context.Context,
	opts session.CreateOpts,
) (*session.Session, error) {
	m.mu.Lock()
	m.createCalls = append(m.createCalls, opts)
	createErr := m.createErr
	returnSessionOnCreateErr := m.returnSessionOnCreateErr
	started := m.createStarted
	released := m.createReleased
	canceled := m.createCanceled
	m.mu.Unlock()
	if started != nil {
		close(started)
	}
	if released != nil {
		select {
		case <-released:
		case <-ctx.Done():
			if canceled != nil {
				close(canceled)
			}
			return nil, ctx.Err()
		}
	}
	if createErr != nil && !returnSessionOnCreateErr {
		return nil, createErr
	}
	sessionID := strings.TrimSpace(m.sessionID)
	if sessionID == "" {
		sessionID = "sess-loop-action"
	}
	created := &session.Session{
		ID:                   sessionID,
		Name:                 opts.Name,
		AgentName:            opts.AgentName,
		Model:                opts.Model,
		WorkspaceID:          opts.Workspace,
		Workspace:            firstNonEmpty(opts.Workspace, opts.WorkspacePath),
		NetworkParticipation: daemonTestParticipationFromCreateOpts(opts),
		Type:                 opts.Type,
		State:                session.StateActive,
		CreatedAt:            time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC),
	}
	return created, createErr
}

func (m *loopActionBinderSessionManager) Prompt(
	ctx context.Context,
	_ string,
	_ string,
) (<-chan acp.AgentEvent, error) {
	m.mu.Lock()
	promptErr := m.promptErr
	configured := append([]acp.AgentEvent(nil), m.events...)
	blockUntilCancel := m.blockPromptUntilCancel
	started := m.promptStarted
	released := m.promptReleased
	m.mu.Unlock()
	if promptErr != nil {
		return nil, promptErr
	}
	if blockUntilCancel {
		events := make(chan acp.AgentEvent)
		if started != nil {
			close(started)
		}
		go func() {
			select {
			case <-released:
			case <-ctx.Done():
			}
			close(events)
		}()
		return events, nil
	}
	events := make(chan acp.AgentEvent, len(configured))
	for _, event := range configured {
		select {
		case events <- event:
		case <-ctx.Done():
			close(events)
			return nil, ctx.Err()
		}
	}
	close(events)
	return events, nil
}

func (m *loopActionBinderSessionManager) PromptWithOpts(
	ctx context.Context,
	_ string,
	opts session.PromptOpts,
) (<-chan acp.AgentEvent, error) {
	m.mu.Lock()
	m.promptCalls = append(m.promptCalls, opts)
	m.mu.Unlock()
	return m.Prompt(ctx, "", opts.Message)
}

func (m *loopActionBinderSessionManager) CancelPrompt(
	_ context.Context,
	id string,
) (session.PromptCancelResult, error) {
	m.mu.Lock()
	m.cancelIDs = append(m.cancelIDs, id)
	released := m.promptReleased
	m.mu.Unlock()
	if released != nil {
		m.promptReleaseOnce.Do(func() { close(released) })
	}
	return session.PromptCancelResult{Outcome: session.PromptCancelOutcomeCanceled}, m.cancelErr
}

func (m *loopActionBinderSessionManager) StopWithCause(
	ctx context.Context,
	id string,
	cause session.StopCause,
	detail string,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopIDs = append(m.stopIDs, id)
	m.stopCauses = append(m.stopCauses, cause)
	m.stopDetails = append(m.stopDetails, detail)
	m.stopSawCanceledContexts = append(m.stopSawCanceledContexts, ctx.Err() != nil)
	return m.stopErr
}

func (m *loopActionBinderSessionManager) singleCreateCall(t *testing.T) session.CreateOpts {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.createCalls) != 1 {
		t.Fatalf("Create call count = %d, want 1", len(m.createCalls))
	}
	return m.createCalls[0]
}

func (m *loopActionBinderSessionManager) createCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.createCalls)
}

func (m *loopActionBinderSessionManager) allCreateCalls() []session.CreateOpts {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]session.CreateOpts(nil), m.createCalls...)
}

func (m *loopActionBinderSessionManager) stopCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.stopIDs)
}

func (m *loopActionBinderSessionManager) cancelCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.cancelIDs)
}

func (m *loopActionBinderSessionManager) singleStopID(t *testing.T) string {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.stopIDs) != 1 {
		t.Fatalf("Stop call count = %d, want 1", len(m.stopIDs))
	}
	return m.stopIDs[0]
}

func (m *loopActionBinderSessionManager) stopObservedCanceledContext() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Contains(m.stopSawCanceledContexts, true)
}

type loopActionBinderWorkspaceResolver struct {
	byID                    map[string]workspacepkg.ResolvedWorkspace
	byPath                  map[string]workspacepkg.ResolvedWorkspace
	resolveOrRegisterCalled *bool
}

func (r loopActionBinderWorkspaceResolver) Resolve(
	_ context.Context,
	idOrPath string,
) (workspacepkg.ResolvedWorkspace, error) {
	resolved, ok := r.byID[strings.TrimSpace(idOrPath)]
	if !ok {
		resolved, ok = r.byPath[strings.TrimSpace(idOrPath)]
	}
	if !ok {
		return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
	}
	return resolved, nil
}

func (r loopActionBinderWorkspaceResolver) ResolveOrRegister(
	_ context.Context,
	path string,
) (workspacepkg.ResolvedWorkspace, error) {
	if r.resolveOrRegisterCalled != nil {
		*r.resolveOrRegisterCalled = true
	}
	resolved, ok := r.byPath[strings.TrimSpace(path)]
	if !ok {
		return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
	}
	return resolved, nil
}

func loopActionBinderWorkspace(t *testing.T, agents []compozyconfig.AgentDef) workspacepkg.ResolvedWorkspace {
	t.Helper()
	cfg := compozyconfig.DefaultWithHome(compozyconfig.HomePaths{HomeDir: t.TempDir()})
	cfg.Defaults.Provider = "mock"
	cfg.Defaults.Sandbox = "evidence-lab"
	cfg.Providers["mock"] = compozyconfig.ProviderConfig{Command: "mock-acp"}
	cfg.Sandboxes["evidence-lab"] = compozyconfig.SandboxProfile{Backend: "local"}
	return workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID:         "ws-loop",
			RootDir:    t.TempDir(),
			SandboxRef: "evidence-lab",
		},
		WorkspaceID: "ws-loop",
		Config:      cfg,
		Agents:      append([]compozyconfig.AgentDef(nil), agents...),
	}
}

type loopPromptResultSessionManager struct {
	events []acp.AgentEvent
}

func (m loopPromptResultSessionManager) Create(context.Context, session.CreateOpts) (*session.Session, error) {
	return nil, errors.New("unexpected Create call")
}

func (m loopPromptResultSessionManager) Status(context.Context, string) (*session.Info, error) {
	return nil, errors.New("unexpected Status call")
}

func (m loopPromptResultSessionManager) Prompt(
	ctx context.Context,
	_ string,
	_ string,
) (<-chan acp.AgentEvent, error) {
	events := make(chan acp.AgentEvent, len(m.events))
	for _, event := range m.events {
		select {
		case events <- event:
		case <-ctx.Done():
			close(events)
			return nil, ctx.Err()
		}
	}
	close(events)
	return events, nil
}

func (m loopPromptResultSessionManager) PromptWithOpts(
	ctx context.Context,
	_ string,
	_ session.PromptOpts,
) (<-chan acp.AgentEvent, error) {
	return m.Prompt(ctx, "", "")
}

func (m loopPromptResultSessionManager) CancelPrompt(
	context.Context,
	string,
) (session.PromptCancelResult, error) {
	return session.PromptCancelResult{}, errors.New("unexpected CancelPrompt call")
}

func (m loopPromptResultSessionManager) StopWithCause(
	context.Context,
	string,
	session.StopCause,
	string,
) error {
	return errors.New("unexpected StopWithCause call")
}

func loopJudgeRunnerForTest(
	t *testing.T,
	sessions *loopActionBinderSessionManager,
) *loopGateJudgeRunner {
	t.Helper()
	resolved := loopActionBinderWorkspace(t, []compozyconfig.AgentDef{{
		Name:        "loop-judge",
		Provider:    "mock",
		Prompt:      "Judge the loop gate.",
		Permissions: string(compozyconfig.PermissionModeApproveAll),
	}})
	return &loopGateJudgeRunner{
		sessions: sessions,
		policyGate: &loopSessionPolicyGate{
			workspaceResolver: loopActionBinderWorkspaceResolver{
				byID: map[string]workspacepkg.ResolvedWorkspace{"ws-loop": resolved},
			},
		},
		executions: newLoopJudgeExecutionRegistry(),
	}
}

func loopJudgeRequestForTest() gate.JudgeRequest {
	return gate.JudgeRequest{
		GateID:        "quality-gate",
		CriterionID:   "review",
		Attempt:       1,
		CorrelationID: "goal-judge:test",
		WorkspaceID:   "ws-loop",
		Agent:         "loop-judge",
		Rubric:        "Review the evidence.",
	}
}

func (m *loopActionBinderSessionManager) singlePromptCall(t *testing.T) session.PromptOpts {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.promptCalls) != 1 {
		t.Fatalf("PromptWithOpts call count = %d, want 1", len(m.promptCalls))
	}
	return m.promptCalls[0]
}
