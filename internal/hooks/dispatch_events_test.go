package hooks

import (
	"context"
	"testing"
	"time"
)

type recordingDispatchEventEmitter struct {
	calls int
}

func (e *recordingDispatchEventEmitter) EmitHookDispatchEvent(
	context.Context,
	any,
	RegisteredHook,
	DispatchPhase,
	HookRunOutcome,
	error,
	int,
	time.Time,
) {
	e.calls++
}

func TestDispatchEventEmitterContextRoundTrip(t *testing.T) {
	t.Parallel()

	emitter := &recordingDispatchEventEmitter{}

	ctx := WithDispatchEventEmitter(context.Background(), emitter)
	got := DispatchEventEmitterFromContext(ctx)
	if got != emitter {
		t.Fatalf("DispatchEventEmitterFromContext() = %#v, want %#v", got, emitter)
	}
	got.EmitHookDispatchEvent(
		ctx,
		PromptPayload{},
		RegisteredHook{Name: "hook"},
		DispatchPhaseStart,
		HookRunOutcomeApplied,
		nil,
		1,
		time.Now(),
	)
	if emitter.calls != 1 {
		t.Fatalf("emitter calls = %d, want 1", emitter.calls)
	}

	baseCtx := context.Background()
	if got := WithDispatchEventEmitter(baseCtx, nil); got != baseCtx {
		t.Fatalf("WithDispatchEventEmitter(ctx, nil) returned different context")
	}
	if got := DispatchEventEmitterFromContext(
		context.WithValue(context.Background(), dispatchEventEmitterContextKey{}, "bad"),
	); got != nil {
		t.Fatalf("DispatchEventEmitterFromContext(non-emitter) = %#v, want nil", got)
	}
}

func TestTurnIDFromPayloadTrimsSupportedPayloads(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		payload any
		want    string
	}{
		{
			name:    "Should return turn ID for input payload",
			payload: InputPreSubmitPayload{TurnContext: TurnContext{TurnID: " turn-input "}},
			want:    "turn-input",
		},
		{
			name:    "Should return turn ID for prompt payload",
			payload: PromptPayload{TurnContext: TurnContext{TurnID: " turn-prompt "}},
			want:    "turn-prompt",
		},
		{
			name:    "Should return turn ID for event payload",
			payload: EventRecordPayload{TurnContext: TurnContext{TurnID: " turn-event "}},
			want:    "turn-event",
		},
		{
			name:    "Should return turn ID for turn payload",
			payload: TurnPayload{TurnContext: TurnContext{TurnID: " turn "}},
			want:    "turn",
		},
		{
			name:    "Should return turn ID for message payload",
			payload: MessagePayload{TurnContext: TurnContext{TurnID: " turn-message "}},
			want:    "turn-message",
		},
		{
			name:    "Should return turn ID for tool pre-call payload",
			payload: ToolPreCallPayload{TurnContext: TurnContext{TurnID: " turn-tool-pre "}},
			want:    "turn-tool-pre",
		},
		{
			name:    "Should return turn ID for tool post-call payload",
			payload: ToolPostCallPayload{TurnContext: TurnContext{TurnID: " turn-tool-post "}},
			want:    "turn-tool-post",
		},
		{
			name:    "Should return turn ID for tool error payload",
			payload: ToolPostErrorPayload{TurnContext: TurnContext{TurnID: " turn-tool-error "}},
			want:    "turn-tool-error",
		},
		{
			name:    "Should return turn ID for permission request payload",
			payload: PermissionRequestPayload{TurnContext: TurnContext{TurnID: " turn-permission-request "}},
			want:    "turn-permission-request",
		},
		{
			name:    "Should return turn ID for permission resolution payload",
			payload: PermissionResolutionPayload{TurnContext: TurnContext{TurnID: " turn-permission-resolution "}},
			want:    "turn-permission-resolution",
		},
		{
			name:    "Should return turn ID for context compact payload",
			payload: ContextCompactPayload{TurnContext: TurnContext{TurnID: " turn-compact "}},
			want:    "turn-compact",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := TurnIDFromPayload(tc.payload)
			if got != tc.want {
				t.Fatalf("TurnIDFromPayload() = %q, want %q", got, tc.want)
			}
		})
	}

	if got := TurnIDFromPayload(SessionPreCreatePayload{}); got != "" {
		t.Fatalf("TurnIDFromPayload(unsupported) = %q, want empty", got)
	}
}

func TestSessionContextFromPayloadCoversHookFamilies(t *testing.T) {
	t.Parallel()

	session := SessionContext{
		SessionID:   "session-1",
		AgentName:   "codex",
		WorkspaceID: "workspace-1",
		Workspace:   "/workspace",
	}
	cases := []struct {
		name     string
		payload  any
		expected SessionContext
	}{
		{
			name:     "Should return session context for session pre-create payload",
			payload:  SessionPreCreatePayload{SessionContext: session},
			expected: session,
		},
		{
			name:     "Should return session context for session lifecycle payload",
			payload:  SessionLifecyclePayload{SessionContext: session},
			expected: session,
		},
		{
			name:     "Should return session context for sandbox prepare payload",
			payload:  SandboxPreparePayload{SessionContext: session},
			expected: session,
		},
		{
			name:     "Should return session context for sandbox ready payload",
			payload:  SandboxReadyPayload{SessionContext: session},
			expected: session,
		},
		{
			name:     "Should return session context for sandbox sync-before payload",
			payload:  SandboxSyncBeforePayload{SessionContext: session},
			expected: session,
		},
		{
			name:     "Should return session context for sandbox sync-after payload",
			payload:  SandboxSyncAfterPayload{SessionContext: session},
			expected: session,
		},
		{
			name:     "Should return session context for sandbox stop payload",
			payload:  SandboxStopPayload{SessionContext: session},
			expected: session,
		},
		{
			name:     "Should return session context for input payload",
			payload:  InputPreSubmitPayload{SessionContext: session},
			expected: session,
		},
		{
			name:     "Should return session context for prompt payload",
			payload:  PromptPayload{SessionContext: session},
			expected: session,
		},
		{
			name:     "Should return session context for event payload",
			payload:  EventRecordPayload{SessionContext: session},
			expected: session,
		},
		{
			name:     "Should return session context for agent pre-start payload",
			payload:  AgentPreStartPayload{SessionContext: session},
			expected: session,
		},
		{
			name:     "Should return session context for agent lifecycle payload",
			payload:  AgentLifecyclePayload{SessionContext: session},
			expected: session,
		},
		{
			name:     "Should return session context for heartbeat wake-before payload",
			payload:  AgentHeartbeatWakeBeforePayload{SessionContext: session},
			expected: session,
		},
		{
			name:     "Should return session context for heartbeat wake-after payload",
			payload:  AgentHeartbeatWakeAfterPayload{SessionContext: session},
			expected: session,
		},
		{
			name:     "Should return session context for session health payload",
			payload:  SessionHealthUpdateAfterPayload{SessionContext: session},
			expected: session,
		},
		{
			name:     "Should return session context for turn payload",
			payload:  TurnPayload{SessionContext: session},
			expected: session,
		},
		{
			name:     "Should return session context for message payload",
			payload:  MessagePayload{SessionContext: session},
			expected: session,
		},
		{
			name:     "Should return session context for tool pre-call payload",
			payload:  ToolPreCallPayload{SessionContext: session},
			expected: session,
		},
		{
			name:     "Should return session context for tool post-call payload",
			payload:  ToolPostCallPayload{SessionContext: session},
			expected: session,
		},
		{
			name:     "Should return session context for tool error payload",
			payload:  ToolPostErrorPayload{SessionContext: session},
			expected: session,
		},
		{
			name:     "Should return session context for permission request payload",
			payload:  PermissionRequestPayload{SessionContext: session},
			expected: session,
		},
		{
			name:     "Should return session context for permission resolution payload",
			payload:  PermissionResolutionPayload{SessionContext: session},
			expected: session,
		},
		{
			name:     "Should return session context for context compact payload",
			payload:  ContextCompactPayload{SessionContext: session},
			expected: session,
		},
		{
			name: "Should map coordinator pre-spawn payload into session context",
			payload: CoordinatorPreSpawnPayload{CoordinatorContext: CoordinatorContext{
				CoordinatorSessionID: "coord-session",
				AgentName:            "coord-agent",
				WorkspaceID:          "coord-workspace",
				Workspace:            "/coord",
			}},
			expected: SessionContext{
				SessionID:   "coord-session",
				AgentName:   "coord-agent",
				WorkspaceID: "coord-workspace",
				Workspace:   "/coord",
			},
		},
		{
			name: "Should map coordinator lifecycle payload into session context",
			payload: CoordinatorLifecyclePayload{CoordinatorContext: CoordinatorContext{
				CoordinatorSessionID: "coord-life-session",
				AgentName:            "coord-life-agent",
				WorkspaceID:          "coord-life-workspace",
				Workspace:            "/coord-life",
			}},
			expected: SessionContext{
				SessionID:   "coord-life-session",
				AgentName:   "coord-life-agent",
				WorkspaceID: "coord-life-workspace",
				Workspace:   "/coord-life",
			},
		},
		{
			name: "Should return session context for task-enqueued payload",
			payload: TaskRunEnqueuedPayload{TaskRunContext: TaskRunContext{
				SessionID:      "task-session",
				AgentName:      "task-agent",
				WorkspaceID:    "task-workspace",
				SoulSnapshotID: "snap-task",
				SoulDigest:     "digest-task",
			}},
			expected: SessionContext{
				SessionID:          "task-session",
				AgentName:          "task-agent",
				WorkspaceID:        "task-workspace",
				SessionSoulContext: &SessionSoulContext{SoulSnapshotID: "snap-task", SoulDigest: "digest-task"},
			},
		},
		{
			name: "Should return session context for task block payload",
			payload: TaskBlockedPayload{TaskContext: TaskContext{
				AgentName:   "task-agent",
				WorkspaceID: "task-workspace",
			}},
			expected: SessionContext{
				AgentName:   "task-agent",
				WorkspaceID: "task-workspace",
			},
		},
		{
			name: "Should return session context for task attention payload",
			payload: TaskNeedsAttentionPayload{TaskContext: TaskContext{
				AgentName:   "attention-agent",
				WorkspaceID: "attention-workspace",
			}},
			expected: SessionContext{
				AgentName:   "attention-agent",
				WorkspaceID: "attention-workspace",
			},
		},
		{
			name: "Should return session context for task status-changed payload",
			payload: TaskStatusChangedPayload{TaskContext: TaskContext{
				AgentName:   "status-agent",
				WorkspaceID: "status-workspace",
			}},
			expected: SessionContext{
				AgentName:   "status-agent",
				WorkspaceID: "status-workspace",
			},
		},
		{
			name:     "Should return session context for task pre-claim payload",
			payload:  TaskRunPreClaimPayload{TaskRunContext: &TaskRunContext{SessionID: "task-pre"}},
			expected: SessionContext{SessionID: "task-pre"},
		},
		{
			name:     "Should return session context for task post-claim payload",
			payload:  TaskRunPostClaimPayload{TaskRunContext: TaskRunContext{SessionID: "task-post"}},
			expected: SessionContext{SessionID: "task-post"},
		},
		{
			name:     "Should return session context for task lease payload",
			payload:  TaskRunLeasePayload{TaskRunContext: TaskRunContext{SessionID: "task-lease"}},
			expected: SessionContext{SessionID: "task-lease"},
		},
		{
			name: "Should return session context for spawn pre-create payload",
			payload: SpawnPreCreatePayload{SpawnContext: SpawnContext{
				ChildSessionID: "child-session",
				AgentName:      "child-agent",
				WorkspaceID:    "spawn-workspace",
				Workspace:      "/spawn",
				SoulSnapshotID: "snap-spawn",
				SoulDigest:     "digest-spawn",
			}},
			expected: SessionContext{
				SessionID:          "child-session",
				AgentName:          "child-agent",
				WorkspaceID:        "spawn-workspace",
				Workspace:          "/spawn",
				SessionSoulContext: &SessionSoulContext{SoulSnapshotID: "snap-spawn", SoulDigest: "digest-spawn"},
			},
		},
		{
			name:     "Should return session context for spawn lifecycle payload",
			payload:  SpawnLifecyclePayload{SpawnContext: SpawnContext{ChildSessionID: "child-life"}},
			expected: SessionContext{SessionID: "child-life"},
		},
		{
			name:     "Should return empty session context for unsupported payloads",
			payload:  NetworkPayload{SessionID: "network-session"},
			expected: SessionContext{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := SessionContextFromPayload(tc.payload)
			assertSessionContext(t, got, tc.expected)
		})
	}
}

func TestCorrelationFromPayloadCoversDispatchFamilies(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		payload  any
		expected DispatchCorrelation
	}{
		{
			name: "Should derive correlation from coordinator pre-spawn payload",
			payload: CoordinatorPreSpawnPayload{CoordinatorContext: CoordinatorContext{
				CoordinatorSessionID: " coord-session ",
				WorkflowID:           " workflow ",
			}},
			expected: DispatchCorrelation{
				CoordinatorSessionID: "coord-session",
				WorkflowID:           "workflow",
				ActorKind:            "agent_session",
				ActorID:              "coord-session",
			},
		},
		{
			name: "Should derive correlation from coordinator lifecycle payload",
			payload: CoordinatorLifecyclePayload{CoordinatorContext: CoordinatorContext{
				CoordinatorSessionID: " coord-life ",
				WorkflowID:           " workflow-life ",
			}},
			expected: DispatchCorrelation{
				CoordinatorSessionID: "coord-life",
				WorkflowID:           "workflow-life",
				ActorKind:            "agent_session",
				ActorID:              "coord-life",
			},
		},
		{
			name:    "Should derive correlation from task-enqueued payload",
			payload: TaskRunEnqueuedPayload{TaskRunContext: taskRunCorrelationContext()},
			expected: DispatchCorrelation{
				TaskID:               "task-1",
				RunID:                "run-1",
				LoopRunID:            "loop-run-1",
				WorkflowID:           "workflow-1",
				ActorKind:            "agent_session",
				ActorID:              "session-actor",
				ReleaseReason:        "completed",
				CoordinatorSessionID: "coordinator-session",
			},
		},
		{
			name:     "Should derive correlation from task block payload",
			payload:  TaskBlockedPayload{TaskContext: taskCorrelationContext()},
			expected: taskDispatchCorrelation(),
		},
		{
			name:     "Should derive correlation from task attention payload",
			payload:  TaskNeedsAttentionPayload{TaskContext: taskCorrelationContext()},
			expected: taskDispatchCorrelation(),
		},
		{
			name:     "Should derive correlation from task status-changed payload",
			payload:  TaskStatusChangedPayload{TaskContext: taskCorrelationContext()},
			expected: taskDispatchCorrelation(),
		},
		{
			name:     "Should derive correlation from task pre-claim payload",
			payload:  TaskRunPreClaimPayload{TaskRunContext: taskRunCorrelationContextPtr()},
			expected: taskRunDispatchCorrelation(),
		},
		{
			name:     "Should derive correlation from task post-claim payload",
			payload:  TaskRunPostClaimPayload{TaskRunContext: taskRunCorrelationContext()},
			expected: taskRunDispatchCorrelation(),
		},
		{
			name:     "Should derive correlation from task lease payload",
			payload:  TaskRunLeasePayload{TaskRunContext: taskRunCorrelationContext()},
			expected: taskRunDispatchCorrelation(),
		},
		{
			name: "Should derive correlation from loop terminal payload",
			payload: LoopTerminalPayload{LoopContext: LoopContext{
				TaskID:     " task-loop ",
				RunID:      " run-loop ",
				LoopRunID:  " loop-run ",
				LoopName:   " daily-review ",
				NodeID:     " node-a ",
				WorkflowID: " workflow-loop ",
				ActorKind:  " daemon ",
				ActorID:    " loop-hook ",
				SessionID:  " coordinator-session ",
			}},
			expected: DispatchCorrelation{
				TaskID:               "task-loop",
				RunID:                "run-loop",
				LoopRunID:            "loop-run",
				LoopName:             "daily-review",
				NodeID:               "node-a",
				WorkflowID:           "workflow-loop",
				CoordinatorSessionID: "coordinator-session",
				ActorKind:            "daemon",
				ActorID:              "loop-hook",
			},
		},
		{
			name: "Should derive correlation from spawn pre-create payload",
			payload: SpawnPreCreatePayload{SpawnContext: SpawnContext{
				TaskID:          " task-spawn ",
				RunID:           " run-spawn ",
				WorkflowID:      " workflow-spawn ",
				ChildSessionID:  " child ",
				ParentSessionID: " parent ",
			}},
			expected: DispatchCorrelation{
				TaskID:     "task-spawn",
				RunID:      "run-spawn",
				WorkflowID: "workflow-spawn",
				ActorKind:  "agent_session",
				ActorID:    "child",
			},
		},
		{
			name: "Should fall back to the parent session for spawn lifecycle correlation",
			payload: SpawnLifecyclePayload{SpawnContext: SpawnContext{
				TaskID:          " task-parent ",
				RunID:           " run-parent ",
				WorkflowID:      " workflow-parent ",
				ParentSessionID: " parent-only ",
			}},
			expected: DispatchCorrelation{
				TaskID:     "task-parent",
				RunID:      "run-parent",
				WorkflowID: "workflow-parent",
				ActorKind:  "agent_session",
				ActorID:    "parent-only",
			},
		},
		{
			name:    "Should derive correlation from network peer payload",
			payload: NetworkPayload{PeerFrom: " peer-a ", SessionID: " session-a "},
			expected: DispatchCorrelation{
				ActorKind: "network_peer",
				ActorID:   "peer-a",
			},
		},
		{
			name:    "Should fall back to session ID for network correlation",
			payload: NetworkPayload{SessionID: " network-session "},
			expected: DispatchCorrelation{
				ActorKind: "network_peer",
				ActorID:   "network-session",
			},
		},
		{
			name:     "Should return empty correlation for empty network payload",
			payload:  NetworkPayload{},
			expected: DispatchCorrelation{},
		},
		{
			name:    "Should fall back to session context when no specialized actor exists",
			payload: PromptPayload{SessionContext: SessionContext{SessionID: " prompt-session "}},
			expected: DispatchCorrelation{
				ActorKind: "agent_session",
				ActorID:   "prompt-session",
			},
		},
		{
			name:     "Should return empty correlation for unsupported payloads",
			payload:  struct{}{},
			expected: DispatchCorrelation{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := CorrelationFromPayload(tc.payload)
			if got != tc.expected {
				t.Fatalf("CorrelationFromPayload() = %#v, want %#v", got, tc.expected)
			}
		})
	}
}

func TestHookTypeValidationBranches(t *testing.T) {
	t.Parallel()

	t.Run("Should trim config hook source text", func(t *testing.T) {
		t.Parallel()

		var source HookSource
		if err := source.UnmarshalText([]byte(" config ")); err != nil {
			t.Fatalf("UnmarshalText(config) error = %v, want nil", err)
		}
		if source != HookSourceConfig {
			t.Fatalf("source = %v, want HookSourceConfig", source)
		}
	})

	t.Run("Should reject invalid hook source marshal", func(t *testing.T) {
		t.Parallel()

		_, err := HookSource(200).MarshalText()
		if err == nil {
			t.Fatal("MarshalText(invalid) error = nil, want error")
		}
		if got, want := err.Error(), `hooks: invalid hook source 200`; got != want {
			t.Fatalf("MarshalText(invalid) error = %q, want %q", got, want)
		}
	})

	t.Run("Should reject unknown hook source text", func(t *testing.T) {
		t.Parallel()

		var source HookSource
		err := source.UnmarshalText([]byte("unknown"))
		if err == nil {
			t.Fatal("UnmarshalText(unknown) error = nil, want error")
		}
		if got, want := err.Error(), `hooks: invalid hook source "unknown"`; got != want {
			t.Fatalf("UnmarshalText(unknown) error = %q, want %q", got, want)
		}
	})

	t.Run("Should convert in-range priority", func(t *testing.T) {
		t.Parallel()

		if _, err := PriorityFromInt(42); err != nil {
			t.Fatalf("PriorityFromInt(42) error = %v, want nil", err)
		}
	})

	t.Run("Should validate registered hook", func(t *testing.T) {
		t.Parallel()

		hook := RegisteredHook{
			Name:    "network-observer",
			Event:   HookNetworkMessagePersisted,
			Source:  HookSourceConfig,
			Mode:    HookModeAsync,
			Timeout: time.Second,
		}
		if err := hook.Validate(); err != nil {
			t.Fatalf("RegisteredHook.Validate() error = %v, want nil", err)
		}
	})

	t.Run("Should reject required async hook", func(t *testing.T) {
		t.Parallel()

		hook := RegisteredHook{
			Name:     "network-observer",
			Event:    HookNetworkMessagePersisted,
			Source:   HookSourceConfig,
			Mode:     HookModeAsync,
			Timeout:  time.Second,
			Required: true,
		}
		err := hook.Validate()
		if err == nil {
			t.Fatal("RegisteredHook.Validate(required async) error = nil, want error")
		}
		if got, want := err.Error(), `hooks: required hook "network-observer" must use sync mode`; got != want {
			t.Fatalf("RegisteredHook.Validate(required async) error = %q, want %q", got, want)
		}
	})

	t.Run("Should validate resolved hook", func(t *testing.T) {
		t.Parallel()

		resolved := &ResolvedHook{RegisteredHook: RegisteredHook{
			Name:   "resolved",
			Event:  HookPromptPostAssemble,
			Source: HookSourceConfig,
			Mode:   HookModeAsync,
			Executor: NewTypedNativeExecutor(
				func(context.Context, RegisteredHook, PromptPayload) (PromptPatch, error) {
					return PromptPatch{}, nil
				},
			),
		}, Decl: HookDecl{
			Name:         "resolved",
			ExecutorKind: HookExecutorNative,
			SkillSource:  HookSkillSourceWorkspace,
		}}
		if err := resolved.Validate(); err != nil {
			t.Fatalf("ResolvedHook.Validate() error = %v, want nil", err)
		}
	})

	t.Run("Should reject name mismatch for resolved hook", func(t *testing.T) {
		t.Parallel()

		resolved := &ResolvedHook{RegisteredHook: RegisteredHook{
			Name:   "resolved",
			Event:  HookPromptPostAssemble,
			Source: HookSourceConfig,
			Mode:   HookModeAsync,
			Executor: NewTypedNativeExecutor(
				func(context.Context, RegisteredHook, PromptPayload) (PromptPatch, error) {
					return PromptPatch{}, nil
				},
			),
		}, Decl: HookDecl{
			Name:         "other",
			ExecutorKind: HookExecutorNative,
			SkillSource:  HookSkillSourceWorkspace,
		}}
		err := resolved.Validate()
		if err == nil {
			t.Fatal("ResolvedHook.Validate(name mismatch) error = nil, want error")
		}
		if got, want := err.Error(), `hooks: resolved hook "resolved" does not match declaration "other"`; got != want {
			t.Fatalf("ResolvedHook.Validate(name mismatch) error = %q, want %q", got, want)
		}
	})

	t.Run("Should reject nil resolved hook", func(t *testing.T) {
		t.Parallel()

		err := (*ResolvedHook)(nil).Validate()
		if err == nil {
			t.Fatal("ResolvedHook.Validate(nil) error = nil, want error")
		}
		if got, want := err.Error(), "hooks: resolved hook is required"; got != want {
			t.Fatalf("ResolvedHook.Validate(nil) error = %q, want %q", got, want)
		}
	})
}

func taskCorrelationContext() TaskContext {
	return TaskContext{
		TaskID:        " task-1 ",
		RunID:         " run-1 ",
		WorkflowID:    " workflow-1 ",
		ActorKind:     " agent_session ",
		ActorID:       " session-actor ",
		ReleaseReason: " blocked ",
	}
}

func taskDispatchCorrelation() DispatchCorrelation {
	return DispatchCorrelation{
		TaskID:        "task-1",
		RunID:         "run-1",
		WorkflowID:    "workflow-1",
		ActorKind:     "agent_session",
		ActorID:       "session-actor",
		ReleaseReason: "blocked",
	}
}

func taskRunCorrelationContext() TaskRunContext {
	return TaskRunContext{
		TaskID:        " task-1 ",
		RunID:         " run-1 ",
		LoopRunID:     " loop-run-1 ",
		WorkflowID:    " workflow-1 ",
		ActorKind:     " agent_session ",
		ActorID:       " session-actor ",
		ReleaseReason: " completed ",
		SessionID:     " coordinator-session ",
	}
}

func taskRunCorrelationContextPtr() *TaskRunContext {
	contextSnapshot := taskRunCorrelationContext()
	return &contextSnapshot
}

func taskRunDispatchCorrelation() DispatchCorrelation {
	return DispatchCorrelation{
		TaskID:               "task-1",
		RunID:                "run-1",
		LoopRunID:            "loop-run-1",
		WorkflowID:           "workflow-1",
		ActorKind:            "agent_session",
		ActorID:              "session-actor",
		ReleaseReason:        "completed",
		CoordinatorSessionID: "coordinator-session",
	}
}

func assertSessionContext(t *testing.T, got SessionContext, want SessionContext) {
	t.Helper()

	if got.SessionID != want.SessionID ||
		got.AgentName != want.AgentName ||
		got.WorkspaceID != want.WorkspaceID ||
		got.Workspace != want.Workspace {
		t.Fatalf("SessionContextFromPayload() = %#v, want %#v", got, want)
	}
	if want.SessionSoulContext == nil {
		if got.SessionSoulContext != nil {
			t.Fatalf("SessionSoulContext = %#v, want nil", got.SessionSoulContext)
		}
		return
	}
	if got.SessionSoulContext == nil {
		t.Fatalf("SessionSoulContext = nil, want %#v", want.SessionSoulContext)
	}
	if got.SoulSnapshotID != want.SoulSnapshotID || got.SoulDigest != want.SoulDigest {
		t.Fatalf("SessionSoulContext = %#v, want %#v", got.SessionSoulContext, want.SessionSoulContext)
	}
}
