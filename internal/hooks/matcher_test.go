package hooks

import (
	"testing"
)

func TestHookMatcherMatchesSession(t *testing.T) {
	t.Parallel()

	matcher := HookMatcher{
		WorkspaceID: "ws-1",
		AgentName:   "claude",
	}
	payload := SessionContext{
		WorkspaceID: "ws-1",
		AgentName:   "claude",
	}
	if !matcher.MatchesSession(payload) {
		t.Fatal("MatchesSession() = false, want true")
	}

	payload.AgentName = "codex"
	if matcher.MatchesSession(payload) {
		t.Fatal("MatchesSession() = true, want false for non-matching agent")
	}
}

func TestHookMatcherMatchesSessionBackedWorktree(t *testing.T) {
	t.Parallel()

	matcher := HookMatcher{WorktreeID: "wt-1"}
	payload := SessionContext{SessionRuntimeContext: NewSessionRuntimeContext("wt-1", "")}
	if !matcher.MatchesSession(payload) || !MatcherFieldAllowedForEvent(HookToolPreCall, matcherWorktreeIDKey) {
		t.Fatal("worktree matcher should be valid and match session-backed hook families")
	}
	payload.SessionRuntimeContext = NewSessionRuntimeContext("wt-2", "")
	if matcher.MatchesSession(payload) {
		t.Fatal("MatchesSession() = true for a different worktree")
	}
}

func TestHookMatcherMatchesToolWithWildcard(t *testing.T) {
	t.Parallel()

	readOnly := true
	matcher := HookMatcher{
		ToolID:       "compozy__read_*",
		ToolReadOnly: &readOnly,
	}

	payload := ToolPreCallPayload{
		ToolCallRef: ToolCallRef{
			ToolID:   "compozy__read_text_file",
			ReadOnly: true,
		},
	}
	if !matcher.MatchesToolPreCall(payload) {
		t.Fatal("MatchesToolPreCall() = false, want true")
	}

	payload.ToolID = "compozy__write_text_file"
	if matcher.MatchesToolPreCall(payload) {
		t.Fatal("MatchesToolPreCall() = true, want false for tool ID mismatch")
	}
}

func TestHookMatcherMatchesPermission(t *testing.T) {
	t.Parallel()

	matcher := HookMatcher{
		ToolName:      "fs/*",
		DecisionClass: "filesystem",
	}
	payload := PermissionRequestPayload{
		DecisionClass: "filesystem",
		ToolCall: PermissionToolCall{
			Kind: "fs/read_text_file",
		},
	}
	if !matcher.MatchesPermissionRequest(payload) {
		t.Fatal("MatchesPermissionRequest() = false, want true")
	}

	payload.DecisionClass = "terminal"
	if matcher.MatchesPermissionRequest(payload) {
		t.Fatal("MatchesPermissionRequest() = true, want false for decision class mismatch")
	}
}

func TestHookMatcherMatchesMessageAndContext(t *testing.T) {
	t.Parallel()

	messageMatcher := HookMatcher{
		MessageRole:      "assistant",
		MessageDeltaType: "text",
	}
	if !messageMatcher.MatchesMessage(MessagePayload{Role: "assistant", DeltaType: "text"}) {
		t.Fatal("MatchesMessage() = false, want true")
	}
	if messageMatcher.MatchesMessage(MessagePayload{Role: "user", DeltaType: "text"}) {
		t.Fatal("MatchesMessage() = true, want false for role mismatch")
	}

	contextMatcher := HookMatcher{
		CompactionMatcher: &CompactionMatcher{
			Reason:   "token_limit",
			Strategy: "summary",
		},
	}
	if !contextMatcher.MatchesContextCompact(ContextCompactPayload{
		Reason:   "token_limit",
		Strategy: "summary",
	}) {
		t.Fatal("MatchesContextCompact() = false, want true")
	}
	if contextMatcher.MatchesContextCompact(ContextCompactPayload{
		Reason:   "manual",
		Strategy: "summary",
	}) {
		t.Fatal("MatchesContextCompact() = true, want false for reason mismatch")
	}
}

func TestHookMatcherMatchesInput(t *testing.T) {
	t.Parallel()

	scopeMatcher := HookMatcher{
		AgentName:     "claude",
		WorkspaceID:   "ws-1",
		WorkspaceRoot: "/workspace/demo",
		InputClass:    "chat",
	}
	if !scopeMatcher.MatchesInput(InputPreSubmitPayload{
		SessionContext: SessionContext{
			AgentName:   "claude",
			WorkspaceID: "ws-1",
			Workspace:   "/workspace/demo",
		},
		InputClass: "chat",
	}) {
		t.Fatal("MatchesInput() = false, want true")
	}
}

func TestHookMatcherMatchesPrompt(t *testing.T) {
	t.Parallel()

	scopeMatcher := HookMatcher{
		AgentName:     "claude",
		WorkspaceID:   "ws-1",
		WorkspaceRoot: "/workspace/demo",
		InputClass:    "chat",
	}
	if !scopeMatcher.MatchesPrompt(PromptPayload{
		SessionContext: SessionContext{
			AgentName:   "claude",
			WorkspaceID: "ws-1",
			Workspace:   "/workspace/demo",
		},
		InputClass: "chat",
	}) {
		t.Fatal("MatchesPrompt() = false, want true")
	}
}

func TestHookMatcherMatchesAgentPreStart(t *testing.T) {
	t.Parallel()

	scopeMatcher := HookMatcher{
		AgentName:     "claude",
		WorkspaceID:   "ws-1",
		WorkspaceRoot: "/workspace/demo",
	}
	if !scopeMatcher.MatchesAgentPreStart(AgentPreStartPayload{
		SessionContext: SessionContext{
			AgentName:   "claude",
			WorkspaceID: "ws-1",
			Workspace:   "/workspace/demo",
		},
	}) {
		t.Fatal("MatchesAgentPreStart() = false, want true")
	}
}

func TestHookMatcherMatchesAgentLifecycle(t *testing.T) {
	t.Parallel()

	scopeMatcher := HookMatcher{
		AgentName:     "claude",
		WorkspaceID:   "ws-1",
		WorkspaceRoot: "/workspace/demo",
	}
	if !scopeMatcher.MatchesAgentLifecycle(AgentLifecyclePayload{
		SessionContext: SessionContext{
			AgentName:   "claude",
			WorkspaceID: "ws-1",
			Workspace:   "/workspace/demo",
		},
	}) {
		t.Fatal("MatchesAgentLifecycle() = false, want true")
	}
}

func TestHookMatcherMatchesTurn(t *testing.T) {
	t.Parallel()

	scopeMatcher := HookMatcher{
		AgentName:     "claude",
		WorkspaceID:   "ws-1",
		WorkspaceRoot: "/workspace/demo",
		InputClass:    "chat",
	}
	if !scopeMatcher.MatchesTurn(TurnPayload{
		SessionContext: SessionContext{
			AgentName:   "claude",
			WorkspaceID: "ws-1",
			Workspace:   "/workspace/demo",
		},
		InputClass: "chat",
	}) {
		t.Fatal("MatchesTurn() = false, want true")
	}
}

func TestHookMatcherMatchesEvent(t *testing.T) {
	t.Parallel()

	eventMatcher := HookMatcher{
		AgentName:    "claude",
		ACPEventType: "permission",
		TurnID:       "turn-1",
	}
	if !eventMatcher.MatchesEvent(EventRecordPayload{
		SessionContext: SessionContext{AgentName: "claude"},
		TurnContext:    TurnContext{TurnID: "turn-1"},
		RecordType:     "permission",
	}) {
		t.Fatal("MatchesEvent() = false, want true")
	}
}

func TestHookMatcherMatchesAutomation(t *testing.T) {
	t.Parallel()

	matcher := HookMatcher{
		AgentName:   "reviewer",
		WorkspaceID: "ws-1",
	}
	if !matcher.MatchesAutomation("reviewer", "ws-1") {
		t.Fatal("MatchesAutomation() = false, want true")
	}
	if matcher.MatchesAutomation("coder", "ws-1") {
		t.Fatal("MatchesAutomation() = true, want false for agent mismatch")
	}
}

func TestHookMatcherMatchesToolResponses(t *testing.T) {
	t.Parallel()

	toolMatcher := HookMatcher{
		ToolID: "compozy__terminal_run",
	}
	if !toolMatcher.MatchesToolPostCall(ToolPostCallPayload{
		ToolCallRef: ToolCallRef{ToolID: "compozy__terminal_run"},
	}) {
		t.Fatal("MatchesToolPostCall() = false, want true")
	}
	if !toolMatcher.MatchesToolPostError(ToolPostErrorPayload{
		ToolCallRef: ToolCallRef{ToolID: "compozy__terminal_run"},
	}) {
		t.Fatal("MatchesToolPostError() = false, want true")
	}
}

func TestHookMatcherMatchesPermissionResolution(t *testing.T) {
	t.Parallel()

	permissionMatcher := HookMatcher{
		ToolName:      "terminal/run",
		DecisionClass: "command",
	}
	if !permissionMatcher.MatchesPermissionResolution(PermissionResolutionPayload{
		DecisionClass: "command",
		ToolCall: PermissionToolCall{
			Kind: "terminal/run",
		},
	}) {
		t.Fatal("MatchesPermissionResolution() = false, want true")
	}
}

func TestHookMatcherMatchesAutonomyPayloads(t *testing.T) {
	t.Parallel()

	coordinatorMatcher := HookMatcher{
		AgentName:   "coordinator",
		WorkspaceID: "ws-1",
		Autonomy: &AutonomyMatcher{
			TaskID:               "task-1",
			RunID:                "run-1",
			WorkflowID:           "wf-1",
			CoordinatorSessionID: "coord-sess-1",
		},
	}
	if !coordinatorMatcher.MatchesCoordinator(CoordinatorContext{
		AgentName:            "coordinator",
		WorkspaceID:          "ws-1",
		TaskID:               "task-1",
		RunID:                "run-1",
		WorkflowID:           "wf-1",
		CoordinatorSessionID: "coord-sess-1",
	}) {
		t.Fatal("MatchesCoordinator() = false, want true")
	}

	taskRunMatcher := HookMatcher{
		WorkspaceID: "ws-1",
		Autonomy: &AutonomyMatcher{
			TaskID:        "task-1",
			RunID:         "run-1",
			ReleaseReason: "timeout",
		},
	}
	if !taskRunMatcher.MatchesTaskRun(TaskRunContext{
		WorkspaceID:   "ws-1",
		TaskID:        "task-1",
		RunID:         "run-1",
		ReleaseReason: "timeout",
	}) {
		t.Fatal("MatchesTaskRun() = false, want true")
	}
	if taskRunMatcher.MatchesTaskRun(TaskRunContext{
		WorkspaceID:   "ws-1",
		TaskID:        "task-1",
		RunID:         "run-2",
		ReleaseReason: "timeout",
	}) {
		t.Fatal("MatchesTaskRun() = true, want false for run mismatch")
	}

	taskMatcher := HookMatcher{
		WorkspaceID: "ws-1",
		Autonomy: &AutonomyMatcher{
			TaskID:        "task-1",
			RunID:         "run-1",
			ReleaseReason: "blocked",
		},
	}
	t.Run("Should match task contexts with autonomy fields", func(t *testing.T) {
		t.Parallel()

		if !taskMatcher.MatchesTask(TaskContext{
			WorkspaceID:   "ws-1",
			TaskID:        "task-1",
			RunID:         "run-1",
			ReleaseReason: "blocked",
		}) {
			t.Fatal("MatchesTask() = false, want true")
		}
	})
	t.Run("Should reject task contexts with mismatched task id", func(t *testing.T) {
		t.Parallel()

		if taskMatcher.MatchesTask(TaskContext{
			WorkspaceID:   "ws-1",
			TaskID:        "task-2",
			RunID:         "run-1",
			ReleaseReason: "blocked",
		}) {
			t.Fatal("MatchesTask() = true, want false for task mismatch")
		}
	})

	spawnMatcher := HookMatcher{
		WorkspaceID: "ws-1",
		Autonomy: &AutonomyMatcher{
			ParentSessionID: "parent-1",
			RootSessionID:   "root-1",
			ChildSessionID:  "child-*",
			SpawnRole:       "reviewer",
		},
	}
	if !spawnMatcher.MatchesSpawn(SpawnContext{
		WorkspaceID:     "ws-1",
		ParentSessionID: "parent-1",
		RootSessionID:   "root-1",
		ChildSessionID:  "child-1",
		SpawnRole:       "reviewer",
	}) {
		t.Fatal("MatchesSpawn() = false, want true")
	}
	if spawnMatcher.MatchesSpawn(SpawnContext{
		WorkspaceID:     "ws-1",
		ParentSessionID: "parent-1",
		RootSessionID:   "root-1",
		ChildSessionID:  "child-1",
		SpawnRole:       "coder",
	}) {
		t.Fatal("MatchesSpawn() = true, want false for spawn role mismatch")
	}
}

func TestMatcherFieldAllowedForEvent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		event HookEvent
		field string
		want  bool
	}{
		{
			name:  "Should allow workspace root for session post-create hook",
			event: HookSessionPostCreate,
			field: "workspace_root",
			want:  true,
		},
		{
			name:  "Should allow workspace id for task-run enqueued hook",
			event: HookTaskRunEnqueued,
			field: "workspace_id",
			want:  true,
		},
		{
			name:  "Should allow task id for task blocked hook",
			event: HookTaskBlocked,
			field: "task_id",
			want:  true,
		},
		{
			name:  "Should deny workspace root for task blocked hook",
			event: HookTaskBlocked,
			field: "workspace_root",
			want:  false,
		},
		{
			name:  "Should deny workspace root for task-run enqueued hook",
			event: HookTaskRunEnqueued,
			field: "workspace_root",
			want:  false,
		},
		{
			name:  "Should deny workspace id for message delta hook",
			event: HookMessageDelta,
			field: "workspace_id",
			want:  false,
		},
		{name: "Should deny invalid event", event: HookEvent("bad.event"), field: "workspace_id", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := MatcherFieldAllowedForEvent(tt.event, tt.field); got != tt.want {
				t.Fatalf("MatcherFieldAllowedForEvent(%q, %q) = %v, want %v", tt.event, tt.field, got, tt.want)
			}
		})
	}
}
