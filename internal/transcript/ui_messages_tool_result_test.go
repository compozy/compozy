package transcript

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/store"
)

func TestToUIMessagesToolResultContract(t *testing.T) {
	t.Parallel()

	t.Run("Should mark failed tool result as error part", func(t *testing.T) {
		t.Parallel()

		timestamp := time.Date(2026, 5, 14, 16, 0, 0, 0, time.UTC)
		events := []store.SessionEvent{
			mustUIAgentSessionEvent(t, "ev-tool-call", 1, timestamp, acp.AgentEvent{
				Type:       acp.EventTypeToolCall,
				SessionID:  "sess-tool-error",
				TurnID:     "turn-tool-error",
				Timestamp:  timestamp,
				Title:      "Bash",
				ToolCallID: "tool-error",
				Raw:        json.RawMessage("{\"rawInput\":{\"command\":\"pwd\"}}"),
			}),
			mustUIAgentSessionEvent(t, "ev-tool-result", 2, timestamp.Add(time.Second), acp.AgentEvent{
				Type:       acp.EventTypeToolResult,
				SessionID:  "sess-tool-error",
				TurnID:     "turn-tool-error",
				Timestamp:  timestamp.Add(time.Second),
				Title:      "Bash",
				ToolCallID: "tool-error",
				Raw: json.RawMessage(
					"{\"sessionUpdate\":\"tool_call_update\",\"status\":\"failed\"," +
						"\"rawOutput\":{\"stderr\":\"boom\"}," +
						"\"content\":[{\"type\":\"content\",\"content\":{\"type\":\"text\",\"text\":\"boom\"}}]," +
						"\"_meta\":{\"claudeCode\":{\"toolName\":\"Bash\"}}," +
						"\"rawInput\":{\"command\":\"pwd\"}}",
				),
			}),
		}

		messages, err := ToUIMessages(events)
		if err != nil {
			t.Fatalf("ToUIMessages() error = %v", err)
		}
		if got, want := len(messages), 1; got != want {
			t.Fatalf("len(messages) = %d, want %d; messages=%#v", got, want, messages)
		}

		toolPart := findUIToolPart(messages[0].Parts, "tool-Bash", "tool-error")
		if toolPart == nil {
			t.Fatalf("tool-Bash part not found; parts=%#v", messages[0].Parts)
		}
		if got, want := toolPart.State, uiToolStateError; got != want {
			t.Fatalf("tool part state = %q, want %q; part=%#v", got, want, *toolPart)
		}
		if !strings.Contains(toolPart.ErrorText, "boom") {
			t.Fatalf("tool part ErrorText = %q, want boom", toolPart.ErrorText)
		}
		if len(toolPart.Output) == 0 {
			t.Fatal("tool part Output = empty, want structured failure payload")
		}
	})

	t.Run("Should preserve tool input when result crosses an assistant flush", func(t *testing.T) {
		t.Parallel()

		timestamp := time.Date(2026, 5, 17, 17, 0, 0, 0, time.UTC)
		events := []store.SessionEvent{
			mustUIAgentSessionEvent(t, "ev-tool-call-cross", 1, timestamp, acp.AgentEvent{
				Type:       acp.EventTypeToolCall,
				SessionID:  "sess-tool-cross",
				TurnID:     "turn-tool-cross",
				Timestamp:  timestamp,
				Title:      "Bash",
				ToolCallID: "tool-cross",
				Raw:        json.RawMessage("{\"rawInput\":{\"command\":\"pwd\"}}"),
			}),
			mustUIAgentSessionEvent(t, "ev-synthetic-cross", 2, timestamp.Add(time.Second), acp.AgentEvent{
				Type:      acp.EventTypeSyntheticReentry,
				SessionID: "sess-tool-cross",
				TurnID:    "turn-synthetic-cross",
				Timestamp: timestamp.Add(time.Second),
				Text:      "network prompt",
			}),
			mustUIAgentSessionEvent(t, "ev-tool-result-cross", 3, timestamp.Add(2*time.Second), acp.AgentEvent{
				Type:       acp.EventTypeToolResult,
				SessionID:  "sess-tool-cross",
				TurnID:     "turn-tool-cross",
				Timestamp:  timestamp.Add(2 * time.Second),
				Title:      "Bash",
				ToolCallID: "tool-cross",
				Raw: json.RawMessage(
					"{\"sessionUpdate\":\"tool_call_update\",\"status\":\"completed\"," +
						"\"rawOutput\":{\"stdout\":\"workspace\"}," +
						"\"_meta\":{\"claudeCode\":{\"toolName\":\"Bash\"}}}",
				),
			}),
		}

		messages, err := ToUIMessages(events)
		if err != nil {
			t.Fatalf("ToUIMessages() error = %v", err)
		}
		if got, want := len(messages), 2; got != want {
			t.Fatalf("len(messages) = %d, want %d; messages=%#v", got, want, messages)
		}
		resultPart := findUIToolPart(messages[0].Parts, "tool-Bash", "tool-cross")
		if resultPart == nil {
			t.Fatalf("result tool part not found in historical entry; parts=%#v", messages[0].Parts)
		}
		if got, want := resultPart.State, uiToolStateOutput; got != want {
			t.Fatalf("result tool part state = %q, want %q; part=%#v", got, want, *resultPart)
		}
		assertUIToolInput(t, resultPart, "pwd")
	})

	t.Run("Should preserve tool input when result omits turn id", func(t *testing.T) {
		t.Parallel()

		timestamp := time.Date(2026, 5, 17, 17, 30, 0, 0, time.UTC)
		events := []store.SessionEvent{
			mustUIAgentSessionEvent(t, "ev-tool-call-omitted-turn", 1, timestamp, acp.AgentEvent{
				Type:       acp.EventTypeToolCall,
				SessionID:  "sess-tool-omitted-turn",
				TurnID:     "turn-tool-omitted-turn",
				Timestamp:  timestamp,
				Title:      "Bash",
				ToolCallID: "tool-omitted-turn",
				Raw:        json.RawMessage("{\"rawInput\":{\"command\":\"pwd\"}}"),
			}),
			mustUIAgentSessionEvent(t, "ev-tool-result-omitted-turn", 2, timestamp.Add(time.Second), acp.AgentEvent{
				Type:       acp.EventTypeToolResult,
				SessionID:  "sess-tool-omitted-turn",
				Timestamp:  timestamp.Add(time.Second),
				Title:      "Bash",
				ToolCallID: "tool-omitted-turn",
				Raw: json.RawMessage(
					"{\"sessionUpdate\":\"tool_call_update\",\"status\":\"completed\"," +
						"\"rawOutput\":{\"stdout\":\"workspace\"}," +
						"\"_meta\":{\"claudeCode\":{\"toolName\":\"Bash\"}}}",
				),
			}),
		}

		messages, err := ToUIMessages(events)
		if err != nil {
			t.Fatalf("ToUIMessages() error = %v", err)
		}
		if got, want := len(messages), 1; got != want {
			t.Fatalf("len(messages) = %d, want %d; messages=%#v", got, want, messages)
		}
		resultPart := findUIToolPart(messages[0].Parts, "tool-Bash", "tool-omitted-turn")
		if resultPart == nil {
			t.Fatalf("result tool part not found; parts=%#v", messages[0].Parts)
		}
		if got, want := resultPart.State, uiToolStateOutput; got != want {
			t.Fatalf("result tool part state = %q, want %q; part=%#v", got, want, *resultPart)
		}
		assertUIToolInput(t, resultPart, "pwd")
	})

	t.Run("Should preserve canonical tool result output for UI replay", func(t *testing.T) {
		t.Parallel()

		timestamp := time.Date(2026, 7, 19, 5, 0, 0, 0, time.UTC)
		events := []store.SessionEvent{
			mustUIAgentSessionEvent(t, "ev-tool-result-artifact", 1, timestamp, acp.AgentEvent{
				Type:       acp.EventTypeToolResult,
				SessionID:  "sess-tool-artifact",
				TurnID:     "turn-tool-artifact",
				Timestamp:  timestamp,
				Title:      "agh__memory_recall",
				ToolCallID: "tool-artifact",
				Raw: json.RawMessage(
					`{"sessionUpdate":"tool_call_update","status":"completed",` +
						`"content":[{"type":"content","content":{"type":"text","text":"bounded preview"}}],` +
						`"rawOutput":{"preview":"bounded preview","truncated":true,` +
						`"artifacts":[{"uri":"agh://tool-artifacts/art_abc"}]}}`,
				),
			}),
		}

		messages, err := ToUIMessages(events)
		if err != nil {
			t.Fatalf("ToUIMessages() error = %v", err)
		}
		if got, want := len(messages), 1; got != want {
			t.Fatalf("len(messages) = %d, want %d; messages=%#v", got, want, messages)
		}
		part := findUIToolPart(messages[0].Parts, "tool-agh__memory_recall", "tool-artifact")
		if part == nil {
			t.Fatalf("tool artifact part not found; parts=%#v", messages[0].Parts)
		}

		var payload UIAgentEventPayload
		if err := json.Unmarshal(part.Output, &payload); err != nil {
			t.Fatalf("json.Unmarshal(part.Output) error = %v; output=%s", err, string(part.Output))
		}
		var result ToolResult
		if err := json.Unmarshal(payload.Raw, &result); err != nil {
			t.Fatalf("json.Unmarshal(payload.Raw) error = %v; raw=%s", err, string(payload.Raw))
		}
		if got, want := result.Content, "bounded preview"; got != want {
			t.Fatalf("result.Content = %q, want %q", got, want)
		}
		var rawOutput map[string]any
		if err := json.Unmarshal(result.RawOutput, &rawOutput); err != nil {
			t.Fatalf("json.Unmarshal(result.RawOutput) error = %v; raw_output=%s", err, string(result.RawOutput))
		}
		if got, want := rawOutput["truncated"], true; got != want {
			t.Fatalf("raw_output.truncated = %#v, want %#v", got, want)
		}
	})
}

func findUIToolPart(parts []UIMessagePart, partType string, toolCallID string) *UIMessagePart {
	for index := range parts {
		part := &parts[index]
		if part.Type == partType && part.ToolCallID == toolCallID {
			return part
		}
	}
	return nil
}

func assertUIToolInput(t *testing.T, part *UIMessagePart, wantCommand string) {
	t.Helper()

	if part == nil {
		t.Fatal("tool part = nil, want input payload")
	}
	var input map[string]string
	if err := json.Unmarshal(part.Input, &input); err != nil {
		t.Fatalf("json.Unmarshal(part.Input) error = %v; input=%s", err, string(part.Input))
	}
	if input["command"] != wantCommand {
		t.Fatalf("tool part input command = %q, want %q; input=%s", input["command"], wantCommand, string(part.Input))
	}
}
