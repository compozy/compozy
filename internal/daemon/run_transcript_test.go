package daemon

import (
	"encoding/json"
	"testing"

	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/core/model"
)

func TestRunUIMessagesFromSessionPreservesStructuredEntries(t *testing.T) {
	t.Run("Should preserve structured entries", func(t *testing.T) {
		t.Parallel()

		textBlock := mustContractBlock(t, model.TextBlock{Text: "hello **world**"})
		thoughtBlock := mustContractBlock(t, model.TextBlock{Text: "checking state"})
		toolUseBlock := mustContractBlock(t, model.ToolUseBlock{
			ID:       "tool-1",
			Name:     "Bash",
			Title:    "Run tests",
			ToolName: "Bash",
			Input:    json.RawMessage(`{"command":"go test ./..."}`),
		})
		terminalBlock := mustContractBlock(t, model.TerminalOutputBlock{
			Command:  "go test ./...",
			Output:   "ok",
			ExitCode: 0,
		})
		noticeBlock := mustContractBlock(t, model.TextBlock{Text: "session complete"})

		messages := runUIMessagesFromSession(apicore.SessionViewSnapshot{
			Entries: []apicore.SessionEntry{
				{
					ID:      "assistant-1",
					Kind:    apicore.SessionEntryKind("assistant_message"),
					Title:   "Assistant",
					Preview: "hello world",
					Blocks:  []apicore.ContentBlock{textBlock},
				},
				{
					ID:      "thinking-1",
					Kind:    apicore.SessionEntryKind("assistant_thinking"),
					Title:   "Thinking",
					Preview: "checking state",
					Blocks:  []apicore.ContentBlock{thoughtBlock},
				},
				{
					ID:            "tool-1",
					Kind:          apicore.SessionEntryKind("tool_call"),
					Title:         "Run tests",
					Preview:       "$ go test ./...",
					ToolCallID:    "tool-1",
					ToolCallState: apicore.ToolCallState("completed"),
					Blocks:        []apicore.ContentBlock{toolUseBlock, terminalBlock},
				},
				{
					ID:      "runtime-1",
					Kind:    apicore.SessionEntryKind("runtime_notice"),
					Title:   "Runtime",
					Preview: "session complete",
					Blocks:  []apicore.ContentBlock{noticeBlock},
				},
			},
		})

		if len(messages) != 4 {
			t.Fatalf("len(messages) = %d, want 4", len(messages))
		}
		if got := messages[0].Parts[0].Text; got != "hello **world**" {
			t.Fatalf("assistant text = %q, want markdown text", got)
		}
		if got := messages[1].Parts[0].Type; got != "reasoning" {
			t.Fatalf("thinking part type = %q, want reasoning", got)
		}
		tool := messages[2].Parts[0]
		if tool.Type != "dynamic-tool" || tool.ToolName != "Bash" || tool.State != "output-available" {
			t.Fatalf("tool part = %#v, want completed dynamic Bash tool", tool)
		}
		if len(tool.Output) == 0 {
			t.Fatal("tool output is empty, want structured block payload")
		}
		if got := messages[3].Parts[0].Type; got != "data-compozy-event" {
			t.Fatalf("runtime part type = %q, want data-compozy-event", got)
		}
	})
}

func TestRunUIMessagesFromSessionMarksFailedTools(t *testing.T) {
	t.Run("Should mark failed tools", func(t *testing.T) {
		t.Parallel()

		toolUseBlock := mustContractBlock(t, model.ToolUseBlock{
			ID:       "tool-2",
			Name:     "Read",
			ToolName: "Read",
			Input:    json.RawMessage(`{"file_path":"missing.go"}`),
		})
		resultBlock := mustContractBlock(t, model.ToolResultBlock{
			ToolUseID: "tool-2",
			Content:   "file not found",
			IsError:   true,
		})

		messages := runUIMessagesFromSession(apicore.SessionViewSnapshot{
			Entries: []apicore.SessionEntry{{
				ID:            "tool-2",
				Kind:          apicore.SessionEntryKind("tool_call"),
				Title:         "Read",
				Preview:       "file not found",
				ToolCallID:    "tool-2",
				ToolCallState: apicore.ToolCallState("failed"),
				Blocks:        []apicore.ContentBlock{toolUseBlock, resultBlock},
			}},
		})

		if len(messages) != 1 || len(messages[0].Parts) != 1 {
			t.Fatalf("messages = %#v, want one failed tool message", messages)
		}
		part := messages[0].Parts[0]
		if part.State != "output-error" || part.ErrorText != "file not found" {
			t.Fatalf("failed tool part = %#v, want output-error with error text", part)
		}
	})
}

func TestAggregateRunTranscriptSessionUsesNewestSessionMetadata(t *testing.T) {
	t.Run("Should use newest session metadata", func(t *testing.T) {
		t.Parallel()

		got := aggregateRunTranscriptSession([]apicore.RunJobState{
			{
				Index: 0,
				Summary: &apicore.RunJobSummary{Session: apicore.SessionViewSnapshot{
					Revision: 1,
					Session: apicore.SessionMetaState{
						Status:        "running",
						CurrentModeID: "plan",
					},
				}},
			},
			{
				Index: 1,
				Summary: &apicore.RunJobSummary{Session: apicore.SessionViewSnapshot{
					Revision: 2,
					Session: apicore.SessionMetaState{
						Status:        "completed",
						CurrentModeID: "code",
					},
				}},
			},
		})

		if got.Revision != 2 {
			t.Fatalf("Revision = %d, want 2", got.Revision)
		}
		if got.Session.Status != "completed" || got.Session.CurrentModeID != "code" {
			t.Fatalf("Session = %#v, want newest metadata", got.Session)
		}
	})
}

func mustContractBlock(t *testing.T, payload any) apicore.ContentBlock {
	t.Helper()
	block, err := model.NewContentBlock(payload)
	if err != nil {
		t.Fatalf("NewContentBlock() error = %v", err)
	}
	return contractContentBlocks([]model.ContentBlock{block})[0]
}
