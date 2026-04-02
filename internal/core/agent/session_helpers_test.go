package agent

import (
	"context"
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
		name      string
		update    acp.SessionUpdate
		wantTypes []model.ContentBlockType
	}{
		{
			name:      "user message",
			update:    acp.UpdateUserMessageText("hello"),
			wantTypes: []model.ContentBlockType{model.BlockText},
		},
		{
			name:      "agent thought",
			update:    acp.UpdateAgentThoughtText("thinking"),
			wantTypes: []model.ContentBlockType{model.BlockText},
		},
		{
			name: "plan",
			update: acp.UpdatePlan(acp.PlanEntry{
				Content:  "step",
				Status:   acp.PlanEntryStatusInProgress,
				Priority: acp.PlanEntryPriorityHigh,
			}),
			wantTypes: []model.ContentBlockType{model.BlockText},
		},
		{
			name: "available commands",
			update: acp.SessionUpdate{
				AvailableCommandsUpdate: &acp.SessionAvailableCommandsUpdate{
					AvailableCommands: []acp.AvailableCommand{{Name: "run"}},
				},
			},
			wantTypes: []model.ContentBlockType{model.BlockText},
		},
		{
			name: "current mode",
			update: acp.SessionUpdate{
				CurrentModeUpdate: &acp.SessionCurrentModeUpdate{
					CurrentModeId: "review",
				},
			},
			wantTypes: []model.ContentBlockType{model.BlockText},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			converted, err := convertACPUpdate(tc.update)
			if err != nil {
				t.Fatalf("convert acp update: %v", err)
			}
			if converted.Status != model.StatusRunning {
				t.Fatalf("unexpected status: %q", converted.Status)
			}
			assertBlockTypes(t, converted.Blocks, tc.wantTypes...)
		})
	}
}

func TestConvertACPUpdateToolCallVariants(t *testing.T) {
	t.Parallel()

	startUpdate, err := convertACPUpdate(acp.StartToolCall(
		acp.ToolCallId("tool-1"),
		"Read README",
		acp.WithStartRawInput(map[string]any{"path": "README.md"}),
	))
	if err != nil {
		t.Fatalf("convert start tool call: %v", err)
	}
	assertBlockTypes(t, startUpdate.Blocks, model.BlockToolUse)

	update, err := convertACPUpdate(acp.UpdateToolCall(
		acp.ToolCallId("tool-1"),
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
	assertBlockTypes(t, update.Blocks, model.BlockToolResult, model.BlockDiff, model.BlockTerminalOutput)
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

	raw := marshalRawJSON(map[string]string{"path": "main.go"})
	if string(raw) != `{"path":"main.go"}` {
		t.Fatalf("unexpected raw json: %s", string(raw))
	}
	if got := stringifyValue(map[string]string{"status": "ok"}); !strings.Contains(got, `"status":"ok"`) {
		t.Fatalf("unexpected stringified value: %s", got)
	}
	if got := stringifyValue("plain"); got != "plain" {
		t.Fatalf("unexpected plain string: %q", got)
	}
	if got := renderDiffText("main.go", "new", nil); !strings.Contains(got, "+++ main.go") {
		t.Fatalf("unexpected rendered diff: %q", got)
	}
}

func TestSessionPublishAndRemoveHelpers(t *testing.T) {
	t.Parallel()

	session := newSession("session-1")
	session.publish(model.SessionUpdate{})
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
	if err := assertBinaryExists("definitely-not-installed-binary"); err == nil {
		t.Fatal("expected missing binary error")
	}

	tempDir := t.TempDir()
	scriptPath := filepath.Join(tempDir, "fake-acp-help")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake helper script: %v", err)
	}
	if err := assertBinarySupported(scriptPath); err != nil {
		t.Fatalf("assert binary supported: %v", err)
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
