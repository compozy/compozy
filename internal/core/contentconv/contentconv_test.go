package contentconv

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

func TestPublicContentBlock(t *testing.T) {
	t.Parallel()

	uri := "file:///tmp/demo.png"
	oldText := "before"
	input := json.RawMessage(`{"command":"rg foo"}`)
	rawInput := json.RawMessage(`{"raw":"value"}`)

	tests := []struct {
		name  string
		block model.ContentBlock
		check func(*testing.T, kinds.ContentBlock)
	}{
		{
			name:  "text",
			block: mustModelContentBlock(t, model.TextBlock{Text: "hello"}),
			check: func(t *testing.T, block kinds.ContentBlock) {
				t.Helper()
				value, err := block.AsText()
				if err != nil {
					t.Fatalf("AsText: %v", err)
				}
				if value.Type != kinds.BlockText || value.Text != "hello" {
					t.Fatalf("unexpected text block: %#v", value)
				}
			},
		},
		{
			name: "tool use",
			block: mustModelContentBlock(t, model.ToolUseBlock{
				ID:       "call-1",
				Name:     "shell",
				Title:    "Shell",
				ToolName: "bash",
				Input:    input,
				RawInput: rawInput,
			}),
			check: func(t *testing.T, block kinds.ContentBlock) {
				t.Helper()
				value, err := block.AsToolUse()
				if err != nil {
					t.Fatalf("AsToolUse: %v", err)
				}
				if value.Type != kinds.BlockToolUse || value.ID != "call-1" || value.Name != "shell" ||
					value.Title != "Shell" ||
					value.ToolName != "bash" {
					t.Fatalf("unexpected tool use block: %#v", value)
				}
				if string(value.Input) != string(input) || string(value.RawInput) != string(rawInput) {
					t.Fatalf("unexpected tool payloads: %#v", value)
				}
			},
		},
		{
			name:  "tool result",
			block: mustModelContentBlock(t, model.ToolResultBlock{ToolUseID: "call-1", Content: "done", IsError: true}),
			check: func(t *testing.T, block kinds.ContentBlock) {
				t.Helper()
				value, err := block.AsToolResult()
				if err != nil {
					t.Fatalf("AsToolResult: %v", err)
				}
				if value.Type != kinds.BlockToolResult || value.ToolUseID != "call-1" || value.Content != "done" ||
					!value.IsError {
					t.Fatalf("unexpected tool result block: %#v", value)
				}
			},
		},
		{
			name: "diff",
			block: mustModelContentBlock(
				t,
				model.DiffBlock{FilePath: "main.go", Diff: "@@ -1 +1 @@", OldText: &oldText, NewText: "after"},
			),
			check: func(t *testing.T, block kinds.ContentBlock) {
				t.Helper()
				value, err := block.AsDiff()
				if err != nil {
					t.Fatalf("AsDiff: %v", err)
				}
				if value.Type != kinds.BlockDiff || value.FilePath != "main.go" || value.Diff != "@@ -1 +1 @@" ||
					value.NewText != "after" {
					t.Fatalf("unexpected diff block: %#v", value)
				}
				if value.OldText == nil || *value.OldText != oldText {
					t.Fatalf("unexpected old text: %#v", value.OldText)
				}
			},
		},
		{
			name: "terminal output",
			block: mustModelContentBlock(
				t,
				model.TerminalOutputBlock{Command: "go test ./...", Output: "ok", ExitCode: 0, TerminalID: "term-1"},
			),
			check: func(t *testing.T, block kinds.ContentBlock) {
				t.Helper()
				value, err := block.AsTerminalOutput()
				if err != nil {
					t.Fatalf("AsTerminalOutput: %v", err)
				}
				if value.Type != kinds.BlockTerminalOutput || value.Command != "go test ./..." ||
					value.Output != "ok" ||
					value.TerminalID != "term-1" {
					t.Fatalf("unexpected terminal output block: %#v", value)
				}
			},
		},
		{
			name:  "image",
			block: mustModelContentBlock(t, model.ImageBlock{Data: "base64", MimeType: "image/png", URI: &uri}),
			check: func(t *testing.T, block kinds.ContentBlock) {
				t.Helper()
				value, err := block.AsImage()
				if err != nil {
					t.Fatalf("AsImage: %v", err)
				}
				if value.Type != kinds.BlockImage || value.Data != "base64" || value.MimeType != "image/png" {
					t.Fatalf("unexpected image block: %#v", value)
				}
				if value.URI == nil || *value.URI != uri {
					t.Fatalf("unexpected image uri: %#v", value.URI)
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			converted, err := PublicContentBlock(tt.block)
			if err != nil {
				t.Fatalf("PublicContentBlock: %v", err)
			}
			tt.check(t, converted)
		})
	}
}

func TestInternalContentBlock(t *testing.T) {
	t.Parallel()

	uri := "file:///tmp/demo.png"
	oldText := "before"
	input := json.RawMessage(`{"command":"rg foo"}`)
	rawInput := json.RawMessage(`{"raw":"value"}`)

	tests := []struct {
		name  string
		block kinds.ContentBlock
		check func(*testing.T, model.ContentBlock)
	}{
		{
			name:  "text",
			block: mustKindsContentBlock(t, kinds.TextBlock{Text: "hello"}),
			check: func(t *testing.T, block model.ContentBlock) {
				t.Helper()
				value, err := block.AsText()
				if err != nil {
					t.Fatalf("AsText: %v", err)
				}
				if value.Type != model.BlockText || value.Text != "hello" {
					t.Fatalf("unexpected text block: %#v", value)
				}
			},
		},
		{
			name: "tool use",
			block: mustKindsContentBlock(t, kinds.ToolUseBlock{
				ID:       "call-1",
				Name:     "shell",
				Title:    "Shell",
				ToolName: "bash",
				Input:    input,
				RawInput: rawInput,
			}),
			check: func(t *testing.T, block model.ContentBlock) {
				t.Helper()
				value, err := block.AsToolUse()
				if err != nil {
					t.Fatalf("AsToolUse: %v", err)
				}
				if value.Type != model.BlockToolUse || value.ID != "call-1" || value.Name != "shell" ||
					value.Title != "Shell" ||
					value.ToolName != "bash" {
					t.Fatalf("unexpected tool use block: %#v", value)
				}
				if string(value.Input) != string(input) || string(value.RawInput) != string(rawInput) {
					t.Fatalf("unexpected tool payloads: %#v", value)
				}
			},
		},
		{
			name:  "tool result",
			block: mustKindsContentBlock(t, kinds.ToolResultBlock{ToolUseID: "call-1", Content: "done", IsError: true}),
			check: func(t *testing.T, block model.ContentBlock) {
				t.Helper()
				value, err := block.AsToolResult()
				if err != nil {
					t.Fatalf("AsToolResult: %v", err)
				}
				if value.Type != model.BlockToolResult || value.ToolUseID != "call-1" || value.Content != "done" ||
					!value.IsError {
					t.Fatalf("unexpected tool result block: %#v", value)
				}
			},
		},
		{
			name: "diff",
			block: mustKindsContentBlock(
				t,
				kinds.DiffBlock{FilePath: "main.go", Diff: "@@ -1 +1 @@", OldText: &oldText, NewText: "after"},
			),
			check: func(t *testing.T, block model.ContentBlock) {
				t.Helper()
				value, err := block.AsDiff()
				if err != nil {
					t.Fatalf("AsDiff: %v", err)
				}
				if value.Type != model.BlockDiff || value.FilePath != "main.go" || value.Diff != "@@ -1 +1 @@" ||
					value.NewText != "after" {
					t.Fatalf("unexpected diff block: %#v", value)
				}
				if value.OldText == nil || *value.OldText != oldText {
					t.Fatalf("unexpected old text: %#v", value.OldText)
				}
			},
		},
		{
			name: "terminal output",
			block: mustKindsContentBlock(
				t,
				kinds.TerminalOutputBlock{Command: "go test ./...", Output: "ok", ExitCode: 0, TerminalID: "term-1"},
			),
			check: func(t *testing.T, block model.ContentBlock) {
				t.Helper()
				value, err := block.AsTerminalOutput()
				if err != nil {
					t.Fatalf("AsTerminalOutput: %v", err)
				}
				if value.Type != model.BlockTerminalOutput || value.Command != "go test ./..." ||
					value.Output != "ok" ||
					value.TerminalID != "term-1" {
					t.Fatalf("unexpected terminal output block: %#v", value)
				}
			},
		},
		{
			name:  "image",
			block: mustKindsContentBlock(t, kinds.ImageBlock{Data: "base64", MimeType: "image/png", URI: &uri}),
			check: func(t *testing.T, block model.ContentBlock) {
				t.Helper()
				value, err := block.AsImage()
				if err != nil {
					t.Fatalf("AsImage: %v", err)
				}
				if value.Type != model.BlockImage || value.Data != "base64" || value.MimeType != "image/png" {
					t.Fatalf("unexpected image block: %#v", value)
				}
				if value.URI == nil || *value.URI != uri {
					t.Fatalf("unexpected image uri: %#v", value.URI)
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			converted, err := InternalContentBlock(tt.block)
			if err != nil {
				t.Fatalf("InternalContentBlock: %v", err)
			}
			tt.check(t, converted)
		})
	}
}

func TestSessionUpdateRoundTrip(t *testing.T) {
	t.Parallel()

	update := model.SessionUpdate{
		Kind:          model.UpdateKindToolCallUpdated,
		ToolCallID:    "call-1",
		ToolCallState: model.ToolCallStateCompleted,
		Blocks: []model.ContentBlock{
			mustModelContentBlock(t, model.ToolUseBlock{
				ID:       "call-1",
				Name:     "shell",
				Title:    "Shell",
				ToolName: "bash",
				Input:    json.RawMessage(`{"command":"go test ./..."}`),
			}),
			mustModelContentBlock(t, model.ToolResultBlock{
				ToolUseID: "call-1",
				Content:   "ok",
			}),
		},
		ThoughtBlocks: []model.ContentBlock{
			mustModelContentBlock(t, model.TextBlock{Text: "thinking"}),
		},
		PlanEntries: []model.SessionPlanEntry{{
			Content:  "Step 1",
			Priority: "high",
			Status:   "in_progress",
		}},
		AvailableCommands: []model.SessionAvailableCommand{{
			Name:         "/help",
			Description:  "Show help",
			ArgumentHint: "[topic]",
		}},
		CurrentModeID: "plan",
		Usage: model.Usage{
			InputTokens:  1,
			OutputTokens: 2,
			TotalTokens:  3,
			CacheReads:   4,
			CacheWrites:  5,
		},
		Status: model.StatusRunning,
	}

	publicUpdate, err := PublicSessionUpdate(update)
	if err != nil {
		t.Fatalf("PublicSessionUpdate: %v", err)
	}
	if publicUpdate.Kind != kinds.UpdateKindToolCallUpdated || publicUpdate.ToolCallID != "call-1" ||
		publicUpdate.CurrentModeID != "plan" {
		t.Fatalf("unexpected public update: %#v", publicUpdate)
	}

	restored, err := InternalSessionUpdate(publicUpdate)
	if err != nil {
		t.Fatalf("InternalSessionUpdate: %v", err)
	}

	if !reflect.DeepEqual(update, restored) {
		t.Fatalf("unexpected roundtrip update\nwant: %#v\ngot:  %#v", update, restored)
	}
}

func TestPublicContentBlockRejectsUnsupportedType(t *testing.T) {
	t.Parallel()

	_, err := PublicContentBlock(model.ContentBlock{
		Type: "unsupported",
		Data: json.RawMessage(`{"demo":true}`),
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported content block type") {
		t.Fatalf("expected unsupported content block error, got %v", err)
	}
}

func TestInternalContentBlockRejectsDecodeFailures(t *testing.T) {
	t.Parallel()

	_, err := InternalContentBlock(kinds.ContentBlock{
		Type: kinds.BlockText,
		Data: json.RawMessage(`{"type":"text","text":`),
	})
	if err == nil {
		t.Fatal("expected decode error")
	}
}

func mustModelContentBlock(t *testing.T, payload any) model.ContentBlock {
	t.Helper()

	block, err := model.NewContentBlock(payload)
	if err != nil {
		t.Fatalf("model.NewContentBlock(%T): %v", payload, err)
	}
	return block
}

func mustKindsContentBlock(t *testing.T, payload any) kinds.ContentBlock {
	t.Helper()

	block, err := kinds.NewContentBlock(payload)
	if err != nil {
		t.Fatalf("kinds.NewContentBlock(%T): %v", payload, err)
	}
	return block
}
