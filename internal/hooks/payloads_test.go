package hooks

import (
	"encoding/json"
	"reflect"

	"testing"
	"time"
)

func TestPayloadsAndPatchesJSONRoundTrip(t *testing.T) {
	t.Parallel()

	sampleSession := SessionContext{
		SessionID:             "sess-1",
		SessionName:           "demo",
		SessionType:           "user",
		AgentName:             "codex",
		WorkspaceID:           "ws-1",
		Workspace:             "/tmp/demo",
		SessionRuntimeContext: NewSessionRuntimeContext("", "acp-1"),
		State:                 "active",
		SessionSoulContext: optionalSessionSoulContext(
			"ss-1",
			"soul-digest",
		),
	}
	sampleTurn := TurnContext{TurnID: "turn-1"}
	samplePayloadBase := func(event HookEvent) PayloadBase {
		return PayloadBase{
			Event:     event,
			Timestamp: time.Date(2026, time.April, 9, 12, 0, 0, 0, time.UTC),
		}
	}
	sampleContextBlocks := []ContextBlock{
		{
			Kind: "policy",
			Text: "ctx",
			Metadata: map[string]string{
				"source": "test",
			},
		},
	}
	sampleRaw := json.RawMessage(`{"key":"value"}`)
	allowOnce := "allow-once"
	reason := "blocked"
	toolID := "compozy__grep"
	toolName := "grep"
	text := "patched"
	role := "assistant"
	deltaType := "text"
	sessionName := "patched-session"
	sessionType := "system"
	agentName := "native"
	workspaceID := "ws-2"
	workspace := "/tmp/other"
	title := "result"
	readOnly := true

	assertJSONRoundTrip(t, "SessionPreCreatePayload", SessionPreCreatePayload{
		PayloadBase:    samplePayloadBase(HookSessionPreCreate),
		SessionContext: sampleSession,
	})
	assertJSONRoundTrip(t, "SessionPostCreatePayload", SessionPostCreatePayload{
		PayloadBase:    samplePayloadBase(HookSessionPostCreate),
		SessionContext: sampleSession,
	})
	assertJSONRoundTrip(t, "SessionPreResumePayload", SessionPreResumePayload{
		PayloadBase:    samplePayloadBase(HookSessionPreResume),
		SessionContext: sampleSession,
	})
	assertJSONRoundTrip(t, "SessionPostResumePayload", SessionPostResumePayload{
		PayloadBase:    samplePayloadBase(HookSessionPostResume),
		SessionContext: sampleSession,
	})
	assertJSONRoundTrip(t, "SessionPreStopPayload", SessionPreStopPayload{
		PayloadBase:    samplePayloadBase(HookSessionPreStop),
		SessionContext: sampleSession,
	})
	assertJSONRoundTrip(t, "SessionPostStopPayload", SessionPostStopPayload{
		PayloadBase:    samplePayloadBase(HookSessionPostStop),
		SessionContext: sampleSession,
	})
	assertJSONRoundTrip(t, "SessionCreatePatch", SessionCreatePatch{
		Deny: true, DenyReason: "policy",
		SessionName: &sessionName,
		SessionType: &sessionType,
		AgentName:   &agentName,
		WorkspaceID: &workspaceID,
		Workspace:   &workspace,
	})
	assertJSONRoundTrip(t, "SessionPostCreatePatch", SessionPostCreatePatch{
		DenyReason:  "observe",
		SessionName: &sessionName,
	})
	assertJSONRoundTrip(t, "SessionPreResumePatch", SessionPreResumePatch{
		SessionType: &sessionType,
	})
	assertJSONRoundTrip(t, "SessionPostResumePatch", SessionPostResumePatch{
		AgentName: &agentName,
	})
	assertJSONRoundTrip(t, "SessionPreStopPatch", SessionPreStopPatch{
		Deny: true, DenyReason: "stop",
	})
	assertJSONRoundTrip(t, "SessionPostStopPatch", SessionPostStopPatch{
		Workspace: &workspace,
	})

	assertJSONRoundTrip(t, "InputPreSubmitPayload", InputPreSubmitPayload{
		PayloadBase:    samplePayloadBase(HookInputPreSubmit),
		SessionContext: sampleSession,
		TurnContext:    sampleTurn,
		InputClass:     "user_message",
		Message:        "hello",
		ContextBlocks:  sampleContextBlocks,
		Attachments: []InputAttachmentMetadata{{
			ID: "att_1", Name: "diagram.png", MIME: "image/png", Bytes: 42, Kind: "image",
		}},
	})
	assertJSONRoundTrip(t, "InputPreSubmitPatch", InputPreSubmitPatch{
		Deny: true, DenyReason: "input",
		Message:       &text,
		ContextBlocks: sampleContextBlocks,
	})

	assertJSONRoundTrip(t, "PromptPayload", PromptPayload{
		PayloadBase:    samplePayloadBase(HookPromptPostAssemble),
		SessionContext: sampleSession,
		TurnContext:    sampleTurn,
		InputClass:     "user_message",
		Prompt:         "assembled",
		ContextBlocks:  sampleContextBlocks,
	})
	assertJSONRoundTrip(t, "PromptPatch", PromptPatch{
		DenyReason:    "prompt",
		Prompt:        &text,
		ContextBlocks: sampleContextBlocks,
	})

	assertJSONRoundTrip(t, "EventPreRecordPayload", EventPreRecordPayload{
		PayloadBase:    samplePayloadBase(HookEventPreRecord),
		SessionContext: sampleSession,
		TurnContext:    sampleTurn,
		RecordType:     "tool_call",
		Sequence:       1,
		Content:        sampleRaw,
	})
	assertJSONRoundTrip(t, "EventPostRecordPayload", EventPostRecordPayload{
		PayloadBase:    samplePayloadBase(HookEventPostRecord),
		SessionContext: sampleSession,
		TurnContext:    sampleTurn,
		RecordType:     "tool_result",
		Sequence:       2,
		Content:        sampleRaw,
	})
	assertJSONRoundTrip(t, "SessionMessagePersistedPayload", SessionMessagePersistedPayload{
		PayloadBase:     samplePayloadBase(HookSessionMessagePersisted),
		SessionContext:  sampleSession,
		TurnContext:     sampleTurn,
		MessageID:       "msg-1",
		MessageSeq:      3,
		Role:            role,
		Text:            "assistant reply",
		Raw:             sampleRaw,
		Persisted:       sampleRaw,
		RootSessionID:   "root-session",
		ParentSessionID: "parent-session",
		ActorKind:       "agent_subagent",
		ActorID:         "child-session",
	})
	assertJSONRoundTrip(t, "EventPreRecordPatch", EventPreRecordPatch{
		Labels: map[string]string{"stage": "pre"},
	})
	assertJSONRoundTrip(t, "EventPostRecordPatch", EventPostRecordPatch{
		Labels: map[string]string{"stage": "post"},
	})
	assertJSONRoundTrip(t, "AutomationSchedulePayload", AutomationSchedulePayload{
		Mode:     "every",
		Interval: "5m",
	})
	assertJSONRoundTrip(t, "AutomationJobPreFirePayload", AutomationJobPreFirePayload{
		JobID:       "job-1",
		JobName:     "nightly-report",
		AgentName:   "researcher",
		WorkspaceID: "ws-1",
		Prompt:      "Generate the nightly report",
		Schedule: &AutomationSchedulePayload{
			Mode: "cron",
			Expr: "0 9 * * *",
		},
		Attempt: 1,
	})
	assertJSONRoundTrip(t, "AutomationJobPostFirePayload", AutomationJobPostFirePayload{
		JobID:       "job-1",
		JobName:     "nightly-report",
		AgentName:   "researcher",
		WorkspaceID: "ws-1",
		RunID:       "run-1",
		SessionID:   "sess-1",
	})
	assertJSONRoundTrip(t, "AutomationTriggerPreFirePayload", AutomationTriggerPreFirePayload{
		TriggerID:   "trigger-1",
		TriggerName: "push-review",
		Event:       "ext.github.push",
		AgentName:   "reviewer",
		WorkspaceID: "ws-1",
		Prompt:      "Review push",
		Payload:     map[string]any{"repo": "acme/api"},
		Attempt:     2,
	})
	assertJSONRoundTrip(t, "AutomationTriggerPostFirePayload", AutomationTriggerPostFirePayload{
		TriggerID:   "trigger-1",
		TriggerName: "push-review",
		Event:       "ext.github.push",
		AgentName:   "reviewer",
		WorkspaceID: "ws-1",
		RunID:       "run-2",
		SessionID:   "sess-2",
	})
	assertJSONRoundTrip(t, "AutomationRunCompletedPayload", AutomationRunCompletedPayload{
		RunID:       "run-3",
		JobID:       "job-1",
		AgentName:   "researcher",
		WorkspaceID: "ws-1",
		SessionID:   "sess-3",
		Attempt:     1,
		DurationMS:  4500,
	})
	assertJSONRoundTrip(t, "AutomationRunFailedPayload", AutomationRunFailedPayload{
		RunID:       "run-4",
		TriggerID:   "trigger-1",
		AgentName:   "reviewer",
		WorkspaceID: "ws-1",
		SessionID:   "sess-4",
		Error:       "boom",
		Attempt:     2,
		WillRetry:   true,
	})
	assertJSONRoundTrip(t, "AutomationFirePatch", AutomationFirePatch{
		Prompt: &text,
		Cancel: true,
	})
	assertJSONRoundTrip(t, "AutomationObservationPatch", AutomationObservationPatch{})

	assertJSONRoundTrip(t, "AgentPreStartPayload", AgentPreStartPayload{
		PayloadBase:    samplePayloadBase(HookAgentPreStart),
		SessionContext: sampleSession,
		Command:        "codex",
		Args:           []string{"serve"},
		Cwd:            "/tmp/demo",
		Provider:       "openai",
		Model:          "gpt-5.4",
	})
	assertJSONRoundTrip(t, "AgentSpawnedPayload", AgentSpawnedPayload{
		PayloadBase:    samplePayloadBase(HookAgentSpawned),
		SessionContext: sampleSession,
		Command:        "codex",
		Args:           []string{"serve"},
		Cwd:            "/tmp/demo",
		PID:            123,
		Provider:       "openai",
		Model:          "gpt-5.4",
	})
	assertJSONRoundTrip(t, "AgentCrashedPayload", AgentCrashedPayload{
		PayloadBase:    samplePayloadBase(HookAgentCrashed),
		SessionContext: sampleSession,
		Command:        "codex",
		Args:           []string{"serve"},
		Cwd:            "/tmp/demo",
		PID:            123,
		Provider:       "openai",
		Model:          "gpt-5.4",
		Error:          "boom",
	})
	assertJSONRoundTrip(t, "AgentStoppedPayload", AgentStoppedPayload{
		PayloadBase:    samplePayloadBase(HookAgentStopped),
		SessionContext: sampleSession,
		Command:        "codex",
		Args:           []string{"serve"},
		Cwd:            "/tmp/demo",
		PID:            123,
		Provider:       "openai",
		Model:          "gpt-5.4",
	})
	assertJSONRoundTrip(t, "AgentStartPatch", AgentStartPatch{
		DenyReason: "agent",
		Command:    &toolName,
		Args:       []string{"--safe"},
	})
	assertJSONRoundTrip(t, "AgentSpawnedPatch", AgentSpawnedPatch{
		Labels: map[string]string{"state": "spawned"},
	})
	assertJSONRoundTrip(t, "AgentCrashedPatch", AgentCrashedPatch{
		Labels: map[string]string{"state": "crashed"},
	})
	assertJSONRoundTrip(t, "AgentStoppedPatch", AgentStoppedPatch{
		Labels: map[string]string{"state": "stopped"},
	})

	authoredProvenance := AuthoredContextProvenance{
		WorkspaceID:      "ws-1",
		AgentName:        "coder",
		SourcePath:       ".compozy/agents/coder/SOUL.md",
		SnapshotID:       "ss-1",
		Digest:           "soul-digest",
		ConfigDigest:     "cfg-digest",
		ValidationStatus: "valid",
		Valid:            true,
		Active:           true,
		Reason:           "resolved",
	}
	authoredMutation := AuthoredMutationProvenance{
		ActorKind:  "extension",
		ActorID:    "ext-1",
		OriginKind: "host_api",
		OriginRef:  "agents/soul/put",
	}
	assertJSONRoundTrip(t, "AgentSoulSnapshotResolvedPayload", AgentSoulSnapshotResolvedPayload{
		PayloadBase:               samplePayloadBase(HookAgentSoulSnapshotResolved),
		AuthoredContextProvenance: authoredProvenance,
	})
	assertJSONRoundTrip(t, "AgentSoulMutationAfterPayload", AgentSoulMutationAfterPayload{
		PayloadBase:                samplePayloadBase(HookAgentSoulMutationAfter),
		AuthoredContextProvenance:  authoredProvenance,
		AuthoredMutationProvenance: authoredMutation,
		RevisionID:                 "sr-1",
		Action:                     "put",
		PreviousDigest:             "old-digest",
		NewDigest:                  "soul-digest",
	})
	assertJSONRoundTrip(t, "AgentHeartbeatPolicyResolvedPayload", AgentHeartbeatPolicyResolvedPayload{
		PayloadBase:      samplePayloadBase(HookAgentHeartbeatPolicyResolved),
		WorkspaceID:      "ws-1",
		AgentName:        "coder",
		SourcePath:       ".compozy/agents/coder/HEARTBEAT.md",
		SnapshotID:       "hbs-1",
		Digest:           "hb-digest",
		ConfigDigest:     "cfg-digest",
		ValidationStatus: "valid",
		Valid:            true,
		Active:           true,
		Summary:          "check in",
	})
	assertJSONRoundTrip(t, "AgentHeartbeatWakeBeforePayload", AgentHeartbeatWakeBeforePayload{
		PayloadBase:      samplePayloadBase(HookAgentHeartbeatWakeBefore),
		SessionContext:   sampleSession,
		PolicySnapshotID: "hbs-1",
		PolicyDigest:     "hb-digest",
		ConfigDigest:     "cfg-digest",
		Source:           "manual",
		DryRun:           true,
	})
	assertJSONRoundTrip(t, "AgentHeartbeatWakeAfterPayload", AgentHeartbeatWakeAfterPayload{
		PayloadBase:       samplePayloadBase(HookAgentHeartbeatWakeAfter),
		SessionContext:    sampleSession,
		WakeEventID:       "hwe-1",
		Result:            "skipped",
		Reason:            "quiet_window",
		PolicySnapshotID:  "hbs-1",
		PolicyDigest:      "hb-digest",
		ConfigDigest:      "cfg-digest",
		SyntheticPromptID: "turn-1",
		Source:            "manual",
	})
	assertJSONRoundTrip(t, "SessionHealthUpdateAfterPayload", SessionHealthUpdateAfterPayload{
		PayloadBase:         samplePayloadBase(HookSessionHealthUpdateAfter),
		SessionContext:      sampleSession,
		Health:              "healthy",
		Attachable:          true,
		EligibleForWake:     true,
		LastActivityAt:      time.Date(2026, time.April, 9, 11, 59, 0, 0, time.UTC),
		LastPresenceAt:      time.Date(2026, time.April, 9, 12, 0, 0, 0, time.UTC),
		IneligibilityReason: "",
	})
	assertJSONRoundTrip(t, "AuthoredContextObservationPatch", AuthoredContextObservationPatch{})

	assertJSONRoundTrip(t, "TurnStartPayload", TurnStartPayload{
		PayloadBase:    samplePayloadBase(HookTurnStart),
		SessionContext: sampleSession,
		TurnContext:    sampleTurn,
		InputClass:     "user_message",
		UserMessage:    "hello",
	})
	assertJSONRoundTrip(t, "TurnEndPayload", TurnEndPayload{
		PayloadBase:    samplePayloadBase(HookTurnEnd),
		SessionContext: sampleSession,
		TurnContext:    sampleTurn,
		InputClass:     "user_message",
		UserMessage:    "bye",
	})
	assertJSONRoundTrip(t, "TurnStartPatch", TurnStartPatch{
		DenyReason: "turn",
		Labels:     map[string]string{"phase": "start"},
	})
	assertJSONRoundTrip(t, "TurnEndPatch", TurnEndPatch{
		Labels: map[string]string{"phase": "end"},
	})

	assertJSONRoundTrip(t, "MessageStartPayload", MessageStartPayload{
		PayloadBase:    samplePayloadBase(HookMessageStart),
		SessionContext: sampleSession,
		TurnContext:    sampleTurn,
		MessageID:      "msg-1",
		Role:           "assistant",
		DeltaType:      "full",
		Text:           "hello",
		Raw:            sampleRaw,
	})
	assertJSONRoundTrip(t, "MessageDeltaPayload", MessageDeltaPayload{
		PayloadBase:    samplePayloadBase(HookMessageDelta),
		SessionContext: sampleSession,
		TurnContext:    sampleTurn,
		MessageID:      "msg-1",
		Role:           "assistant",
		DeltaType:      "text",
		Text:           "hel",
		Raw:            sampleRaw,
	})
	assertJSONRoundTrip(t, "MessageEndPayload", MessageEndPayload{
		PayloadBase:    samplePayloadBase(HookMessageEnd),
		SessionContext: sampleSession,
		TurnContext:    sampleTurn,
		MessageID:      "msg-1",
		Role:           "assistant",
		DeltaType:      "full",
		Text:           "hello",
		Raw:            sampleRaw,
	})
	assertJSONRoundTrip(t, "MessageStartPatch", MessageStartPatch{
		DenyReason: "message",
		Role:       &role,
		DeltaType:  &deltaType,
		Text:       &text,
	})
	assertJSONRoundTrip(t, "MessageDeltaPatch", MessageDeltaPatch{
		DeltaType: &deltaType,
	})
	assertJSONRoundTrip(t, "MessageEndPatch", MessageEndPatch{
		Text: &text,
	})

	assertJSONRoundTrip(t, "ToolPreCallPayload", ToolPreCallPayload{
		PayloadBase:    samplePayloadBase(HookToolPreCall),
		SessionContext: sampleSession,
		TurnContext:    sampleTurn,
		ToolCallID:     "tool-1",
		ToolID:         "compozy__grep",
		ReadOnly:       true,
		ToolInput:      sampleRaw,
	})
	assertJSONRoundTrip(t, "ToolPostCallPayload", ToolPostCallPayload{
		PayloadBase:    samplePayloadBase(HookToolPostCall),
		SessionContext: sampleSession,
		TurnContext:    sampleTurn,
		ToolCallID:     "tool-1",
		ToolID:         "compozy__grep",
		ReadOnly:       true,
		Title:          "grep result",
		ToolInput:      sampleRaw,
		ToolResult:     sampleRaw,
	})
	assertJSONRoundTrip(t, "ToolPostErrorPayload", ToolPostErrorPayload{
		PayloadBase:    samplePayloadBase(HookToolPostError),
		SessionContext: sampleSession,
		TurnContext:    sampleTurn,
		ToolCallID:     "tool-1",
		ToolID:         "compozy__grep",
		ReadOnly:       true,
		Title:          "grep error",
		ToolInput:      sampleRaw,
		Error:          "failed",
	})
	assertJSONRoundTrip(t, "ToolCallPatch", ToolCallPatch{
		DenyReason: "tool",
		ToolID:     &toolID,
		ReadOnly:   &readOnly,
		ToolInput:  sampleRaw,
	})
	assertJSONRoundTrip(t, "ToolResultPatch", ToolResultPatch{
		DenyReason: "result",
		Title:      &title,
		ToolResult: sampleRaw,
	})
	assertJSONRoundTrip(t, "ToolPostErrorPatch", ToolPostErrorPatch{
		Error: &reason,
	})

	assertJSONRoundTrip(t, "PermissionRequestPayload", PermissionRequestPayload{
		PayloadBase:    samplePayloadBase(HookPermissionRequest),
		SessionContext: sampleSession,
		TurnContext:    sampleTurn,
		RequestID:      "req-1",
		Action:         "session/request_permission",
		Resource:       "/tmp/demo.txt",
		Decision:       "pending",
		DecisionClass:  "interactive",
		ToolInput:      sampleRaw,
		ToolCall: PermissionToolCall{
			ID:     "tool-1",
			Kind:   "read",
			Title:  "Read file",
			Status: "pending",
			Locations: []ToolLocation{
				{Path: "/tmp/demo.txt", StartLine: 1, EndLine: 1},
			},
		},
		Options: []PermissionOption{
			{Decision: "allow-once", OptionID: "allow-once", Kind: "allow"},
		},
	})
	assertJSONRoundTrip(t, "PermissionResolvedPayload", PermissionResolvedPayload{
		PayloadBase:    samplePayloadBase(HookPermissionResolved),
		SessionContext: sampleSession,
		TurnContext:    sampleTurn,
		RequestID:      "req-1",
		Action:         "session/request_permission",
		Resource:       "/tmp/demo.txt",
		Decision:       "allow-once",
		DecisionClass:  "interactive",
		ToolInput:      sampleRaw,
		ToolCall:       PermissionToolCall{ID: "tool-1", Kind: "read", Title: "Read file", Status: "done"},
	})
	assertJSONRoundTrip(t, "PermissionDeniedPayload", PermissionDeniedPayload{
		PayloadBase:    samplePayloadBase(HookPermissionDenied),
		SessionContext: sampleSession,
		TurnContext:    sampleTurn,
		RequestID:      "req-2",
		Action:         "session/request_permission",
		Resource:       "/tmp/secret.txt",
		Decision:       "reject-once",
		DecisionClass:  "interactive",
		ToolCall:       PermissionToolCall{ID: "tool-2", Kind: "read", Title: "Read secret", Status: "done"},
	})
	assertJSONRoundTrip(t, "PermissionRequestPatch", PermissionRequestPatch{
		Deny: true, DenyReason: "permission",
		Decision:      &allowOnce,
		DecisionClass: &role,
		Reason:        &reason,
	})
	assertJSONRoundTrip(t, "PermissionResolvedPatch", PermissionResolvedPatch{})
	assertJSONRoundTrip(t, "PermissionDeniedPatch", PermissionDeniedPatch{})

	sampleTaskContext := TaskContext{
		TaskID:         "task-1",
		ParentTaskID:   "task-parent",
		WorkspaceID:    "ws-1",
		WorkflowID:     "wf-1",
		AgentName:      "codex",
		ActorKind:      "agent_session",
		ActorID:        "sess-1",
		OriginKind:     "agent_session",
		OriginRef:      "sess-1",
		TaskStatus:     "blocked",
		RunID:          "run-1",
		ReleaseReason:  "blocked",
		ClaimTokenHash: "sha256:abc123",
	}
	assertJSONRoundTrip(t, "TaskBlockedPayload", TaskBlockedPayload{
		PayloadBase: samplePayloadBase(HookTaskBlocked),
		TaskContext: sampleTaskContext,
		BlockID:     "block-1",
		Kind:        "needs_input",
		Reason:      "creator clarification required",
		Details:     sampleRaw,
	})
	assertJSONRoundTrip(t, "TaskUnblockedPayload", TaskUnblockedPayload{
		PayloadBase: samplePayloadBase(HookTaskUnblocked),
		TaskContext: sampleTaskContext,
		BlockID:     "block-1",
		Kind:        "needs_input",
		Reason:      "creator clarification required",
		ClearedAt:   samplePayloadBase(HookTaskUnblocked).Timestamp,
		ClearNote:   "creator answered",
	})
	assertJSONRoundTrip(t, "TaskNeedsAttentionPayload", TaskNeedsAttentionPayload{
		PayloadBase: samplePayloadBase(HookTaskNeedsAttention),
		TaskContext: sampleTaskContext,
		Reason:      "breaker limit reached",
		At:          samplePayloadBase(HookTaskNeedsAttention).Timestamp,
	})
	assertJSONRoundTrip(t, "TaskRecoveredPayload", TaskRecoveredPayload{
		PayloadBase: samplePayloadBase(HookTaskRecovered),
		TaskContext: sampleTaskContext,
		Note:        "operator recovered",
		At:          samplePayloadBase(HookTaskRecovered).Timestamp,
	})
	assertJSONRoundTrip(t, "TaskStatusChangedPayload", TaskStatusChangedPayload{
		PayloadBase: samplePayloadBase(HookTaskStatusChanged),
		TaskContext: sampleTaskContext,
		FromStatus:  "pending",
		ToStatus:    "blocked",
	})
	assertJSONRoundTrip(t, "TaskObservationPatch", TaskObservationPatch{
		Labels: map[string]string{"source": "test"},
	})

	assertJSONRoundTrip(
		t,
		"ContextPreCompactPayload",
		ContextPreCompactPayload{
			PayloadBase:    samplePayloadBase(HookContextPreCompact),
			SessionContext: sampleSession,
			TurnContext:    sampleTurn,
			CompactionID:   "c1",
			Trigger:        "agent",
		},
	)
	assertJSONRoundTrip(
		t,
		"ContextPostCompactPayload",
		ContextPostCompactPayload{
			PayloadBase:    samplePayloadBase(HookContextPostCompact),
			SessionContext: sampleSession,
			TurnContext:    sampleTurn,
			CompactionID:   "c1",
			Trigger:        "agent",
			Status:         "completed",
			Summary:        "after",
		},
	)
	assertJSONRoundTrip(t, "ContextPreCompactPatch", ContextPreCompactPatch{Labels: map[string]string{"k": "v"}})
	assertJSONRoundTrip(t, "ContextPostCompactPatch", ContextPostCompactPatch{Labels: map[string]string{"k": "v"}})
	assertJSONRoundTrip(
		t,
		"WindowManagerPayload",
		windowManagerDispatchTestPayload(HookWindowManagerLayoutApplied),
	)
	assertJSONRoundTrip(t, "WindowManagerObservationPatch", WindowManagerObservationPatch{})
}

func assertJSONRoundTrip[T any](t *testing.T, name string, sample T) {
	t.Helper()

	t.Run(name, func(t *testing.T) {
		t.Parallel()

		data, err := json.Marshal(sample)
		if err != nil {
			t.Fatalf("json.Marshal(%s) error = %v", name, err)
		}

		var decoded T
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("json.Unmarshal(%s) error = %v", name, err)
		}

		if !reflect.DeepEqual(sample, decoded) {
			t.Fatalf("%s round-trip mismatch\ngot:  %#v\nwant: %#v", name, decoded, sample)
		}
	})
}

// UT-042: the hook boundary preserves delegation metadata on the wire.
func TestSubagentHookPayloads(t *testing.T) {
	t.Run("Should serialize delegation and settlement payloads", func(t *testing.T) {
		t.Parallel()
		pre := SpawnPreCreatePayload{
			SpawnRole: "subagent",
			Subagent:  &SubagentSpawnPayload{Title: "Review", Role: "review", TaskChars: 42},
		}
		raw, err := json.Marshal(pre)
		if err != nil {
			t.Fatal(err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatal(err)
		}
		sub, ok := decoded["subagent"].(map[string]any)
		if !ok || decoded["spawn_role"] != "subagent" || sub["title"] != "Review" || sub["role"] != "review" ||
			sub["task_chars"] != float64(42) {
			t.Fatalf("payload = %s", raw)
		}
		settled := SubagentSettledPayload{
			Event:           HookSubagentSettled,
			WorkspaceID:     "ws",
			SubagentID:      "sub",
			ParentSessionID: "parent",
			ChildSessionID:  new("child"),
			Origin:          "delegated",
			Status:          "completed",
			Runtime:         SubagentRuntimePayload{Provider: "codex", Model: "model"},
			DurationMS:      123,
		}
		raw, err = json.Marshal(settled)
		if err != nil {
			t.Fatal(err)
		}
		var got SubagentSettledPayload
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, settled) {
			t.Fatalf("round trip = %#v", got)
		}
		if HookSubagentSettled.SyncEligible() || HookSubagentSettled.Family() != HookEventFamilySubagent {
			t.Fatal("settlement must be observe-only in subagent family")
		}
	})
}
