package transcript

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
)

func TestParseLooseEventBuildsToolResultFromLoosePayload(t *testing.T) {
	t.Parallel()

	parsed := parseLooseEvent(event{Type: acp.EventTypeToolResult}, map[string]any{
		"type":         acp.EventTypeToolResult,
		"tool_call_id": "call-loose",
		"title":        "Bash",
		"rawInput": map[string]any{
			"command": "pwd",
		},
		"rawOutput": map[string]any{
			"stdout": "workspace\n",
		},
	})

	if got := parsed.ToolCallID; got != "call-loose" {
		t.Fatalf("ToolCallID = %q, want %q", got, "call-loose")
	}
	if got := parsed.ToolName; got != "Bash" {
		t.Fatalf("ToolName = %q, want %q", got, "Bash")
	}
	if got := string(parsed.ToolInput); got != `{"command":"pwd"}` {
		t.Fatalf("ToolInput = %s, want JSON command payload", got)
	}
	if parsed.ToolResult == nil {
		t.Fatal("ToolResult = nil, want populated result")
	}
	if got := parsed.ToolResult.Stdout; got != "workspace\n" {
		t.Fatalf("ToolResult.Stdout = %q, want %q", got, "workspace\n")
	}
	if parsed.ToolError {
		t.Fatal("ToolError = true, want false")
	}

	if got := string(firstNonEmptyRaw(nil, json.RawMessage(`{"ok":true}`))); got != `{"ok":true}` {
		t.Fatalf("firstNonEmptyRaw() = %s, want non-empty raw payload", got)
	}
	if got := firstNonNil(nil, "", "value"); got != "" {
		t.Fatalf("firstNonNil(nil, \"\", \"value\") = %#v, want empty string first", got)
	}
}

func TestMarshalAgentEventBuildsCanonicalPayload(t *testing.T) {
	t.Parallel()

	totalTokens := int64(4)
	payload, err := MarshalAgentEvent(acp.AgentEvent{
		Type:      acp.EventTypeDone,
		SessionID: "acp-1",
		TurnID:    "turn-1",
		Timestamp: time.Date(2026, 4, 3, 15, 0, 0, 0, time.UTC),
		Text:      "done",
		Error:     "none",
		Usage: &acp.TokenUsage{
			TurnID:      "turn-1",
			TotalTokens: &totalTokens,
			Timestamp:   time.Date(2026, 4, 3, 15, 0, 1, 0, time.UTC),
		},
	})
	if err != nil {
		t.Fatalf("MarshalAgentEvent(structured) error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatalf("json.Unmarshal(payload) error = %v", err)
	}
	if decoded["schema"] != CanonicalSchema {
		t.Fatalf("decoded[schema] = %v, want %q", decoded["schema"], CanonicalSchema)
	}
	if decoded["type"] != acp.EventTypeDone {
		t.Fatalf("decoded[type] = %v, want %q", decoded["type"], acp.EventTypeDone)
	}
	if decoded["text"] != "done" {
		t.Fatalf("decoded[text] = %v, want %q", decoded["text"], "done")
	}
}

func TestMarshalAgentEventExtractsToolResultShapeWithoutPersistingRaw(t *testing.T) {
	t.Run("Should extract structured tool result fields without persisting raw payloads", func(t *testing.T) {
		t.Parallel()

		payload, err := MarshalAgentEvent(acp.AgentEvent{
			Type: acp.EventTypeToolResult,
			Raw: json.RawMessage(`{
				"sessionUpdate":"tool_call_update",
				"status":"failed",
				"rawOutput":{"stderr":"boom"},
				"content":[{"type":"content","content":{"type":"text","text":"boom"}}],
				"_meta":{"claudeCode":{"toolName":"Bash"}},
				"rawInput":{"command":"pwd"}
			}`),
			Title: "tool result",
		})
		if err != nil {
			t.Fatalf("MarshalAgentEvent(raw) error = %v", err)
		}

		var decoded struct {
			Schema     string          `json:"schema"`
			ToolName   string          `json:"tool_name"`
			ToolInput  json.RawMessage `json:"tool_input"`
			ToolError  bool            `json:"tool_error"`
			ToolResult ToolResult      `json:"tool_result"`
			Raw        json.RawMessage `json:"raw"`
		}
		if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(raw payload) error = %v", err)
		}
		if decoded.Schema != CanonicalSchema {
			t.Fatalf("Schema = %q, want %q", decoded.Schema, CanonicalSchema)
		}
		if decoded.ToolName != "Bash" {
			t.Fatalf("ToolName = %q, want %q", decoded.ToolName, "Bash")
		}
		if got := string(decoded.ToolInput); got != `{"command":"pwd"}` {
			t.Fatalf("ToolInput = %s, want command payload", got)
		}
		if !decoded.ToolError {
			t.Fatal("ToolError = false, want true")
		}
		if decoded.ToolResult.Stderr != "boom" || decoded.ToolResult.Error != "boom" {
			t.Fatalf("ToolResult = %#v, want stderr/error boom", decoded.ToolResult)
		}
		if len(decoded.Raw) != 0 {
			t.Fatalf("Raw = %s, want empty persisted raw payload", string(decoded.Raw))
		}
	})
}

func TestBuildToolResultDecodesRawJSONObjectPayload(t *testing.T) {
	t.Parallel()

	t.Run("ShouldDecodeRawJSONObjectPayload", func(t *testing.T) {
		result := buildToolResult("Read", false, "", json.RawMessage(`{
			"stdout":"workspace\n",
			"content":"workspace\n",
			"structuredPatch":{"ops":[{"op":"replace","path":"/tmp/demo.txt"}]}
		}`))
		if result == nil {
			t.Fatal("buildToolResult() = nil, want populated result")
			return
		}
		if got := result.Stdout; got != "workspace\n" {
			t.Fatalf("Stdout = %q, want %q", got, "workspace\n")
		}
		if got := result.Content; got != "workspace\n" {
			t.Fatalf("Content = %q, want %q", got, "workspace\n")
		}
		if len(result.StructuredPatch) == 0 {
			t.Fatal("StructuredPatch = empty, want preserved patch payload")
		}
	})
}

func TestUnmarshalAgentEventRoundTripPreservesStructuredFieldsWithoutRaw(t *testing.T) {
	t.Run("Should round-trip structured fields without restoring canonical raw payloads", func(t *testing.T) {
		t.Parallel()

		event := (acp.AgentEvent{
			Type:             acp.EventTypeAgentMessage,
			SessionID:        "acp-1",
			TurnID:           "turn-1",
			RequestID:        "req-1",
			Timestamp:        time.Date(2026, 4, 11, 2, 0, 0, 0, time.UTC),
			Text:             "hello",
			Title:            "assistant",
			Error:            "",
			PromptStopReason: acp.PromptStopReasonMaxTokens,
			AvailableCommands: acp.NewAvailableCommandSet([]store.SessionAdvertisedCommand{
				{Name: "compact", Description: "Compact context"},
				{Name: "review", Description: "Review changes"},
			}),
			Raw: json.RawMessage(`{"chunk":1}`),
		}).WithPromptRuntime(&acp.PromptRuntime{
			Provider:        "codex",
			Model:           "gpt-5.6",
			ReasoningEffort: "high",
			Speed:           speedpkg.SpeedFast,
		})
		payload, err := MarshalAgentEvent(event)
		if err != nil {
			t.Fatalf("MarshalAgentEvent() error = %v", err)
		}

		event, err = UnmarshalAgentEvent(payload)
		if err != nil {
			t.Fatalf("UnmarshalAgentEvent() error = %v", err)
		}
		if got, want := event.Type, acp.EventTypeAgentMessage; got != want {
			t.Fatalf("Type = %q, want %q", got, want)
		}
		if got, want := event.SessionID, "acp-1"; got != want {
			t.Fatalf("SessionID = %q, want %q", got, want)
		}
		if got, want := event.TurnID, "turn-1"; got != want {
			t.Fatalf("TurnID = %q, want %q", got, want)
		}
		if got, want := event.RequestID, "req-1"; got != want {
			t.Fatalf("RequestID = %q, want %q", got, want)
		}
		if got, want := event.Text, "hello"; got != want {
			t.Fatalf("Text = %q, want %q", got, want)
		}
		if got, want := event.PromptStopReason, acp.PromptStopReasonMaxTokens; got != want {
			t.Fatalf("PromptStopReason = %q, want %q", got, want)
		}
		if got, want := event.PromptRuntimeSnapshot(), (&acp.PromptRuntime{
			Provider:        "codex",
			Model:           "gpt-5.6",
			ReasoningEffort: "high",
			Speed:           speedpkg.SpeedFast,
		}); !reflect.DeepEqual(got, want) {
			t.Fatalf("PromptRuntime = %#v, want %#v", got, want)
		}
		if got, want := event.AvailableCommands.Values(), []store.SessionAdvertisedCommand{
			{Name: "compact", Description: "Compact context"},
			{Name: "review", Description: "Review changes"},
		}; !slices.EqualFunc(got, want, store.SessionAdvertisedCommandsEqual) {
			t.Fatalf("AvailableCommands = %#v, want %#v", got, want)
		}
		if len(event.Raw) != 0 {
			t.Fatalf("Raw = %s, want empty canonical raw payload", string(event.Raw))
		}
	})

	t.Run("Should round-trip typed tool metadata ahead of conflicting legacy raw metadata", func(t *testing.T) {
		t.Parallel()

		payload, err := MarshalAgentEvent((acp.AgentEvent{
			Type:       acp.EventTypeToolResult,
			Timestamp:  time.Date(2026, 6, 11, 2, 0, 0, 0, time.UTC),
			ToolCallID: "call-typed",
			Raw: json.RawMessage(`{
				"_meta":{"claudeCode":{"toolName":"legacy__shell"}},
				"rawInput":{"command":"legacy"},
				"status":"completed"
			}`),
		}).WithTool("typed__shell", json.RawMessage(`{"command":"typed"}`), true))
		if err != nil {
			t.Fatalf("MarshalAgentEvent() error = %v", err)
		}

		event, err := UnmarshalAgentEvent(payload)
		if err != nil {
			t.Fatalf("UnmarshalAgentEvent() error = %v", err)
		}
		if got, want := event.ToolName(), "typed__shell"; got != want {
			t.Fatalf("ToolName = %q, want %q", got, want)
		}
		if got, want := string(event.ToolInput()), `{"command":"typed"}`; got != want {
			t.Fatalf("ToolInput = %s, want %s", got, want)
		}
		if !event.ToolError() {
			t.Fatal("ToolError = false, want true")
		}
	})

	t.Run("Should round-trip a redacted tool failure detail for presentation consumers", func(t *testing.T) {
		t.Parallel()

		payload, err := MarshalAgentEvent(acp.AgentEvent{
			Type:       acp.EventTypeToolResult,
			Timestamp:  time.Date(2026, 6, 11, 2, 0, 0, 0, time.UTC),
			ToolCallID: "call-failure-detail",
			Raw: json.RawMessage(`{
				"_meta":{"claudeCode":{"toolName":"Bash"}},
				"status":"failed",
				"content":"command failed: password=hunter2"
			}`),
		})
		if err != nil {
			t.Fatalf("MarshalAgentEvent() error = %v", err)
		}

		event, err := UnmarshalAgentEvent(payload)
		if err != nil {
			t.Fatalf("UnmarshalAgentEvent() error = %v", err)
		}
		if !event.ToolError() {
			t.Fatal("ToolError = false, want true")
		}
		if got := event.ToolErrorDetail(); strings.Contains(got, "hunter2") || !strings.Contains(got, "[REDACTED]") {
			t.Fatalf("ToolErrorDetail = %q, want redacted failure detail", got)
		}
	})

	t.Run("Should preserve typed tool success ahead of stale legacy failure", func(t *testing.T) {
		t.Parallel()

		payload, err := MarshalAgentEvent((acp.AgentEvent{
			Type:       acp.EventTypeToolResult,
			Timestamp:  time.Date(2026, 6, 11, 2, 0, 0, 0, time.UTC),
			ToolCallID: "call-typed-success",
			Raw: json.RawMessage(`{
				"_meta":{"claudeCode":{"toolName":"legacy__shell"}},
				"rawInput":{"command":"legacy"},
				"status":"failed",
				"content":"stale legacy failure"
			}`),
		}).WithTool("typed__shell", json.RawMessage(`{"command":"typed"}`), false))
		if err != nil {
			t.Fatalf("MarshalAgentEvent() error = %v", err)
		}

		event, err := UnmarshalAgentEvent(payload)
		if err != nil {
			t.Fatalf("UnmarshalAgentEvent() error = %v", err)
		}
		if got, want := event.ToolName(), "typed__shell"; got != want {
			t.Fatalf("ToolName = %q, want %q", got, want)
		}
		if event.ToolError() {
			t.Fatal("ToolError = true, want typed success")
		}
	})

	t.Run("Should backfill absent typed tool metadata from legacy raw fields", func(t *testing.T) {
		t.Parallel()

		payload, err := MarshalAgentEvent(acp.AgentEvent{
			Type:       acp.EventTypeToolResult,
			Timestamp:  time.Date(2026, 6, 11, 2, 0, 1, 0, time.UTC),
			ToolCallID: "call-legacy",
			Raw: json.RawMessage(`{
				"_meta":{"claudeCode":{"toolName":"legacy__read"}},
				"rawInput":{"path":"README.md"},
				"status":"failed"
			}`),
		})
		if err != nil {
			t.Fatalf("MarshalAgentEvent() error = %v", err)
		}

		event, err := UnmarshalAgentEvent(payload)
		if err != nil {
			t.Fatalf("UnmarshalAgentEvent() error = %v", err)
		}
		if got, want := event.ToolName(), "legacy__read"; got != want {
			t.Fatalf("ToolName = %q, want %q", got, want)
		}
		if got, want := string(event.ToolInput()), `{"path":"README.md"}`; got != want {
			t.Fatalf("ToolInput = %s, want %s", got, want)
		}
		if !event.ToolError() {
			t.Fatal("ToolError = false, want true")
		}
	})

	t.Run("Should preserve typed tool metadata when legacy raw is malformed", func(t *testing.T) {
		t.Parallel()

		payload, err := MarshalAgentEvent((acp.AgentEvent{
			Type:       acp.EventTypeToolCall,
			Timestamp:  time.Date(2026, 6, 11, 2, 0, 2, 0, time.UTC),
			ToolCallID: "call-malformed",
			Raw:        json.RawMessage(`{"rawInput":`),
		}).WithTool("typed__search", json.RawMessage(`{"query":"typed metadata"}`), false))
		if err != nil {
			t.Fatalf("MarshalAgentEvent() error = %v", err)
		}

		event, err := UnmarshalAgentEvent(payload)
		if err != nil {
			t.Fatalf("UnmarshalAgentEvent() error = %v", err)
		}
		if got, want := event.ToolName(), "typed__search"; got != want {
			t.Fatalf("ToolName = %q, want %q", got, want)
		}
		if got, want := string(event.ToolInput()), `{"query":"typed metadata"}`; got != want {
			t.Fatalf("ToolInput = %s, want %s", got, want)
		}
		if event.ToolError() {
			t.Fatal("ToolError = true, want false")
		}
	})
}
