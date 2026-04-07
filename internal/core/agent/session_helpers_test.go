package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	acp "github.com/coder/acp-go-sdk"

	"github.com/compozy/compozy/internal/core/model"
)

func TestConvertACPUpdateVariants(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name              string
		update            acp.SessionUpdate
		wantKind          model.SessionUpdateKind
		wantTypes         []model.ContentBlockType
		wantThoughtTypes  []model.ContentBlockType
		wantPlanEntries   []model.SessionPlanEntry
		wantCommands      []model.SessionAvailableCommand
		wantCurrentModeID string
	}{
		{
			name:     "user message",
			update:   acp.UpdateUserMessageText("hello"),
			wantKind: model.UpdateKindUserMessageChunk,
		},
		{
			name:             "agent thought",
			update:           acp.UpdateAgentThoughtText("thinking"),
			wantKind:         model.UpdateKindAgentThoughtChunk,
			wantThoughtTypes: []model.ContentBlockType{model.BlockText},
		},
		{
			name:      "agent message",
			update:    acp.UpdateAgentMessageText("hello"),
			wantKind:  model.UpdateKindAgentMessageChunk,
			wantTypes: []model.ContentBlockType{model.BlockText},
		},
		{
			name:     "plan",
			wantKind: model.UpdateKindPlanUpdated,
			update: acp.UpdatePlan(acp.PlanEntry{
				Content:  "step",
				Status:   acp.PlanEntryStatusInProgress,
				Priority: acp.PlanEntryPriorityHigh,
			}),
			wantPlanEntries: []model.SessionPlanEntry{{
				Content:  "step",
				Status:   string(acp.PlanEntryStatusInProgress),
				Priority: string(acp.PlanEntryPriorityHigh),
			}},
		},
		{
			name:     "available commands",
			wantKind: model.UpdateKindAvailableCommandsUpdated,
			update: acp.SessionUpdate{
				AvailableCommandsUpdate: &acp.SessionAvailableCommandsUpdate{
					AvailableCommands: []acp.AvailableCommand{{
						Name:        "run",
						Description: "Run the task",
						Input: &acp.AvailableCommandInput{
							UnstructuredCommandInput: &acp.AvailableCommandUnstructuredCommandInput{
								Hint: "--fast",
							},
						},
					}},
				},
			},
			wantCommands: []model.SessionAvailableCommand{{
				Name:         "run",
				Description:  "Run the task",
				ArgumentHint: "--fast",
			}},
		},
		{
			name:     "current mode",
			wantKind: model.UpdateKindCurrentModeUpdated,
			update: acp.SessionUpdate{
				CurrentModeUpdate: &acp.SessionCurrentModeUpdate{
					CurrentModeId: "review",
				},
			},
			wantCurrentModeID: "review",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			converted, err := convertACPUpdate(model.IDEClaude, tc.update)
			if err != nil {
				t.Fatalf("convert acp update: %v", err)
			}
			if converted.Status != model.StatusRunning {
				t.Fatalf("unexpected status: %q", converted.Status)
			}
			if converted.Kind != tc.wantKind {
				t.Fatalf("unexpected kind: got %q want %q", converted.Kind, tc.wantKind)
			}
			assertBlockTypes(t, converted.Blocks, tc.wantTypes...)
			assertBlockTypes(t, converted.ThoughtBlocks, tc.wantThoughtTypes...)
			if diff := comparePlanEntries(converted.PlanEntries, tc.wantPlanEntries); diff != "" {
				t.Fatalf("unexpected plan entries: %s", diff)
			}
			if diff := compareAvailableCommands(converted.AvailableCommands, tc.wantCommands); diff != "" {
				t.Fatalf("unexpected commands: %s", diff)
			}
			if converted.CurrentModeID != tc.wantCurrentModeID {
				t.Fatalf("unexpected current mode id: got %q want %q", converted.CurrentModeID, tc.wantCurrentModeID)
			}
		})
	}
}

func TestLoadedSessionTracksSuppressedReplayUpdatesWithoutPublishingThem(t *testing.T) {
	t.Parallel()

	session := newLoadedSession("sess-1", t.TempDir(), nil)
	update := model.SessionUpdate{
		Kind:          model.UpdateKindToolCallUpdated,
		ToolCallState: model.ToolCallStateFailed,
	}

	session.publish(context.Background(), update)

	session.mu.RLock()
	updatesSeen := session.updatesSeen
	session.mu.RUnlock()
	if updatesSeen != 1 {
		t.Fatalf("expected suppressed replay update to be tracked, got %d", updatesSeen)
	}
	if !session.lastUpdateFailedToolCall() {
		t.Fatal("expected suppressed replay update to retain failed tool-call state")
	}

	select {
	case got := <-session.Updates():
		t.Fatalf("expected suppressed replay update to stay hidden, got %#v", got)
	default:
	}
}

func TestConvertACPUpdateToolCallVariants(t *testing.T) {
	t.Parallel()

	startUpdate, err := convertACPUpdate(model.IDEClaude, acp.StartToolCall(
		acp.ToolCallId("tool-1"),
		"Read",
		acp.WithStartKind(acp.ToolKindRead),
		acp.WithStartRawInput(map[string]any{"path": "README.md"}),
	))
	if err != nil {
		t.Fatalf("convert start tool call: %v", err)
	}
	if startUpdate.Kind != model.UpdateKindToolCallStarted {
		t.Fatalf("unexpected start update kind: %q", startUpdate.Kind)
	}
	if startUpdate.ToolCallID != "tool-1" {
		t.Fatalf("unexpected start tool call id: %q", startUpdate.ToolCallID)
	}
	if startUpdate.ToolCallState != model.ToolCallStatePending {
		t.Fatalf("unexpected start tool call state: %q", startUpdate.ToolCallState)
	}
	assertBlockTypes(t, startUpdate.Blocks, model.BlockToolUse)
	toolUse, err := startUpdate.Blocks[0].AsToolUse()
	if err != nil {
		t.Fatalf("decode start tool use block: %v", err)
	}
	if toolUse.Name != "Read" {
		t.Fatalf("unexpected normalized tool name: %q", toolUse.Name)
	}
	if got := string(toolUse.Input); got != `{"file_path":"README.md"}` {
		t.Fatalf("unexpected normalized tool input: %s", got)
	}

	update, err := convertACPUpdate(model.IDEClaude, acp.UpdateToolCall(
		acp.ToolCallId("tool-1"),
		acp.WithUpdateTitle("Read"),
		acp.WithUpdateKind(acp.ToolKindRead),
		acp.WithUpdateRawInput(
			map[string]any{"path": "README.md", "startLineNumberBaseOne": 5, "endLineNumberBaseOne": 12},
		),
		acp.WithUpdateStatus(acp.ToolCallStatusFailed),
		acp.WithUpdateContent([]acp.ToolCallContent{
			acp.ToolContent(acp.TextBlock("failed")),
			acp.ToolDiffContent("README.md", "new", "old"),
			acp.ToolTerminalRef("term-1"),
		}),
		acp.WithUpdateRawOutput(map[string]any{"stderr": "boom"}),
	))
	if err != nil {
		t.Fatalf("convert tool call update: %v", err)
	}
	if update.Kind != model.UpdateKindToolCallUpdated {
		t.Fatalf("unexpected update kind: %q", update.Kind)
	}
	if update.ToolCallID != "tool-1" {
		t.Fatalf("unexpected update tool call id: %q", update.ToolCallID)
	}
	if update.ToolCallState != model.ToolCallStateFailed {
		t.Fatalf("unexpected update tool call state: %q", update.ToolCallState)
	}
	assertBlockTypes(
		t,
		update.Blocks,
		model.BlockToolUse,
		model.BlockToolResult,
		model.BlockDiff,
		model.BlockTerminalOutput,
	)
	updatedToolUse, err := update.Blocks[0].AsToolUse()
	if err != nil {
		t.Fatalf("decode updated tool use block: %v", err)
	}
	if got := string(updatedToolUse.Input); got != `{"end_line":12,"file_path":"README.md","start_line":5}` {
		t.Fatalf("unexpected updated tool input: %s", got)
	}
}

func TestConvertACPUpdateToolCallNormalizationScenarios(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		driverID  string
		update    func() acp.SessionUpdate
		wantTypes []model.ContentBlockType
		verify    func(t *testing.T, converted model.SessionUpdate)
	}{
		{
			name:     "normalize Codex web search tool call",
			driverID: model.IDECodex,
			update: func() acp.SessionUpdate {
				return acp.StartToolCall(
					acp.ToolCallId("tool-web"),
					"search_query",
					acp.WithStartKind(acp.ToolKindOther),
					acp.WithStartRawInput(map[string]any{
						"search_query": []map[string]any{
							{"q": "agent client protocol official docs"},
							{"q": "site:agentclientprotocol.com protocol docs"},
						},
						"response_length": "short",
					}),
				)
			},
			wantTypes: []model.ContentBlockType{model.BlockToolUse},
			verify: func(t *testing.T, converted model.SessionUpdate) {
				t.Helper()

				toolUse, err := converted.Blocks[0].AsToolUse()
				if err != nil {
					t.Fatalf("decode web search tool use block: %v", err)
				}
				if toolUse.Name != toolNameWebSearch {
					t.Fatalf("unexpected web search tool name: %q", toolUse.Name)
				}
				if got := string(
					toolUse.Input,
				); got != `{"queries":["agent client protocol official docs","site:agentclientprotocol.com protocol docs"],"query":"agent client protocol official docs","response_length":"short"}` {
					t.Fatalf("unexpected normalized web search input: %s", got)
				}
			},
		},
		{
			name:     "ignore generic tool call update header without metadata",
			driverID: model.IDEClaude,
			update: func() acp.SessionUpdate {
				return acp.UpdateToolCall(
					acp.ToolCallId("tool-1"),
					acp.WithUpdateTitle(toolNameToolCall),
					acp.WithUpdateRawInput(map[string]any{}),
					acp.WithUpdateContent([]acp.ToolCallContent{
						acp.ToolDiffContent("README.md", "new", "old"),
					}),
				)
			},
			wantTypes: []model.ContentBlockType{model.BlockDiff},
		},
		{
			name:     "preserve generic tool call update header when raw input is meaningful",
			driverID: model.IDEClaude,
			update: func() acp.SessionUpdate {
				return acp.UpdateToolCall(
					acp.ToolCallId("tool-raw"),
					acp.WithUpdateTitle(toolNameToolCall),
					acp.WithUpdateRawInput(map[string]any{"selection": "README.md"}),
				)
			},
			wantTypes: []model.ContentBlockType{model.BlockToolUse},
			verify: func(t *testing.T, converted model.SessionUpdate) {
				t.Helper()

				toolUse, err := converted.Blocks[0].AsToolUse()
				if err != nil {
					t.Fatalf("decode generic header-preserving tool use block: %v", err)
				}
				if toolUse.Name != toolNameToolCall {
					t.Fatalf("unexpected generic tool name: %q", toolUse.Name)
				}
				if got := string(toolUse.Input); got != `{"selection":"README.md"}` {
					t.Fatalf("unexpected preserved generic tool input: %s", got)
				}
			},
		},
		{
			name:     "omit null tool call input",
			driverID: model.IDEClaude,
			update: func() acp.SessionUpdate {
				return acp.StartToolCall(
					acp.ToolCallId("tool-null"),
					toolNameRead,
					acp.WithStartKind(acp.ToolKindRead),
					acp.WithStartRawInput(json.RawMessage(`null`)),
				)
			},
			wantTypes: []model.ContentBlockType{model.BlockToolUse},
			verify: func(t *testing.T, converted model.SessionUpdate) {
				t.Helper()

				toolUse, err := converted.Blocks[0].AsToolUse()
				if err != nil {
					t.Fatalf("decode null-input tool use block: %v", err)
				}
				if len(toolUse.Input) != 0 {
					t.Fatalf("expected null tool input to be omitted, got %s", string(toolUse.Input))
				}
			},
		},
		{
			name:     "preserve tool title name and raw input",
			driverID: model.IDEClaude,
			update: func() acp.SessionUpdate {
				update := acp.StartToolCall(
					acp.ToolCallId("tool-meta"),
					toolNameRead,
					acp.WithStartKind(acp.ToolKindRead),
					acp.WithStartRawInput(map[string]any{"path": "README.md"}),
				)
				update.ToolCall.Meta = map[string]any{"tool_name": "read_file"}
				return update
			},
			wantTypes: []model.ContentBlockType{model.BlockToolUse},
			verify: func(t *testing.T, converted model.SessionUpdate) {
				t.Helper()

				toolUse, err := converted.Blocks[0].AsToolUse()
				if err != nil {
					t.Fatalf("decode meta-aware tool use block: %v", err)
				}
				if toolUse.Name != toolNameRead {
					t.Fatalf("unexpected normalized display name: %q", toolUse.Name)
				}
				if toolUse.Title != toolNameRead {
					t.Fatalf("unexpected tool title: %q", toolUse.Title)
				}
				if toolUse.ToolName != "read_file" {
					t.Fatalf("unexpected programmatic tool name: %q", toolUse.ToolName)
				}
				if got := string(toolUse.Input); got != `{"file_path":"README.md"}` {
					t.Fatalf("unexpected normalized tool input: %s", got)
				}
				if got := string(toolUse.RawInput); got != `{"path":"README.md"}` {
					t.Fatalf("unexpected raw tool input: %s", got)
				}
			},
		},
		{
			name:     "prefer ACP meta tool name over descriptive title",
			driverID: model.IDEClaude,
			update: func() acp.SessionUpdate {
				update := acp.StartToolCall(
					acp.ToolCallId("tool-meta-hint"),
					"Searching docs",
					acp.WithStartKind(acp.ToolKindOther),
				)
				update.ToolCall.Meta = map[string]any{"tool_name": "search_query"}
				return update
			},
			wantTypes: []model.ContentBlockType{model.BlockToolUse},
			verify: func(t *testing.T, converted model.SessionUpdate) {
				t.Helper()

				toolUse, err := converted.Blocks[0].AsToolUse()
				if err != nil {
					t.Fatalf("decode meta-priority tool use block: %v", err)
				}
				if toolUse.Name != toolNameWebSearch {
					t.Fatalf("expected meta-aware tool normalization, got %q", toolUse.Name)
				}
				if toolUse.Title != "Searching docs" {
					t.Fatalf("expected descriptive title to remain visible, got %q", toolUse.Title)
				}
				if toolUse.ToolName != "search_query" {
					t.Fatalf("expected canonical tool name metadata to be preserved, got %q", toolUse.ToolName)
				}
			},
		},
		{
			name:     "route find search tools to find normalization",
			driverID: model.IDEClaude,
			update: func() acp.SessionUpdate {
				return acp.StartToolCall(
					acp.ToolCallId("tool-find"),
					"find",
					acp.WithStartKind(acp.ToolKindSearch),
					acp.WithStartRawInput(map[string]any{
						"pattern": "Annie Case",
						"ref_id":  "turn0search0",
					}),
				)
			},
			wantTypes: []model.ContentBlockType{model.BlockToolUse},
			verify: func(t *testing.T, converted model.SessionUpdate) {
				t.Helper()

				toolUse, err := converted.Blocks[0].AsToolUse()
				if err != nil {
					t.Fatalf("decode find tool use block: %v", err)
				}
				if toolUse.Name != toolNameFind {
					t.Fatalf("unexpected find tool normalization: %q", toolUse.Name)
				}
				if got := string(toolUse.Input); got != `{"pattern":"Annie Case","ref_id":"turn0search0"}` {
					t.Fatalf("unexpected normalized find input: %s", got)
				}
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run("Should "+tc.name, func(t *testing.T) {
			t.Parallel()

			converted, err := convertACPUpdate(tc.driverID, tc.update())
			if err != nil {
				t.Fatalf("convert acp tool call update: %v", err)
			}
			assertBlockTypes(t, converted.Blocks, tc.wantTypes...)
			if tc.verify != nil {
				tc.verify(t, converted)
			}
		})
	}
}

func TestConvertACPContentBlockFallbacks(t *testing.T) {
	t.Parallel()

	cases := []acp.ContentBlock{
		acp.AudioBlock("U29tZUF1ZGlv", "audio/mpeg"),
		acp.ResourceLinkBlock("docs", "https://example.com"),
	}

	for _, block := range cases {
		converted, err := convertACPContentBlock(block)
		if err != nil {
			t.Fatalf("convert content block fallback: %v", err)
		}
		assertBlockTypes(t, converted, model.BlockText)
	}
}

func TestSessionConversionHelpers(t *testing.T) {
	t.Parallel()

	raw, err := marshalRawJSON(map[string]string{"path": "main.go"})
	if err != nil {
		t.Fatalf("marshal raw json: %v", err)
	}
	if string(raw) != `{"path":"main.go"}` {
		t.Fatalf("unexpected raw json: %s", string(raw))
	}
	if got := stringifyValue(map[string]string{"status": "ok"}); !strings.Contains(got, `"status":"ok"`) {
		t.Fatalf("unexpected stringified value: %s", got)
	}
	if got := stringifyValue("plain"); got != "plain" {
		t.Fatalf("unexpected plain string: %q", got)
	}
	if got := renderDiffText("main.go", "new", nil); !strings.Contains(got, "+++ main.go\nnew\n") {
		t.Fatalf("unexpected rendered diff: %q", got)
	}
	oldText := "old"
	if got := renderDiffText(
		"dir\nmain\t.go",
		"new",
		&oldText,
	); got != "--- dir\\nmain\\t.go\nold\n+++ dir\\nmain\\t.go\nnew\n" {
		t.Fatalf("unexpected sanitized rendered diff: %q", got)
	}
}

func TestSessionPublishAndRemoveHelpers(t *testing.T) {
	t.Parallel()

	session := newSession("session-1")
	session.publish(context.Background(), model.SessionUpdate{})
	session.finish(model.StatusCompleted, nil)
	if err := session.Err(); err != nil {
		t.Fatalf("unexpected session error: %v", err)
	}

	client := &clientImpl{sessions: map[string]*sessionImpl{"session-1": session}}
	client.removeSession("session-1")
	if got := client.lookupSession("session-1"); got != nil {
		t.Fatal("expected session removal")
	}
}

func TestRegistryHelperFunctions(t *testing.T) {
	t.Parallel()

	assignments := sortedEnvAssignments(map[string]string{"B": "two", "A": "one space"})
	if len(assignments) != 2 || !strings.HasPrefix(assignments[0], "A=") {
		t.Fatalf("unexpected env assignments: %#v", assignments)
	}
	if quoted := quotedSupportedIDEs(); !strings.Contains(quoted, `"gemini"`) {
		t.Fatalf("expected gemini in supported ide list: %s", quoted)
	}
	if err := assertCommandExists(
		Spec{ID: "missing", DisplayName: "Missing", Command: "definitely-not-installed-binary"},
		[]string{"definitely-not-installed-binary", "--help"},
	); err == nil {
		t.Fatal("expected missing binary error")
	}

	tempDir := t.TempDir()
	scriptPath := filepath.Join(tempDir, "fake-acp-help")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake helper script: %v", err)
	}
	if _, err := resolveLaunchCommand(
		context.Background(),
		Spec{ID: "fake", DisplayName: "Fake", Command: scriptPath},
		"test-model",
		"medium",
		nil,
		model.AccessModeDefault,
		true,
	); err != nil {
		t.Fatalf("resolve launch command: %v", err)
	}
}

func TestClientCreateSessionRejectsEmptyWorkingDir(t *testing.T) {
	t.Parallel()

	client, err := NewClient(context.Background(), ClientConfig{IDE: model.IDECodex})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if _, err := client.CreateSession(context.Background(), SessionRequest{Prompt: []byte("hi")}); err == nil {
		t.Fatal("expected working directory error")
	}
}

func TestWrapACPErrorPassthrough(t *testing.T) {
	t.Parallel()

	wrapped := wrapACPError(context.Canceled)
	if wrapped != context.Canceled {
		t.Fatalf("expected passthrough error, got %v", wrapped)
	}
	if normalizeProcessWaitError(nil) != nil {
		t.Fatal("expected nil normalized wait error")
	}
}

func comparePlanEntries(got, want []model.SessionPlanEntry) string {
	if len(got) != len(want) {
		return "length mismatch"
	}
	for i := range got {
		if got[i] != want[i] {
			return "value mismatch"
		}
	}
	return ""
}

func compareAvailableCommands(got, want []model.SessionAvailableCommand) string {
	if len(got) != len(want) {
		return "length mismatch"
	}
	for i := range got {
		if got[i] != want[i] {
			return "value mismatch"
		}
	}
	return ""
}
