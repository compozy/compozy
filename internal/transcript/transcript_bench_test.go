package transcript

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/store"
)

func BenchmarkAssembleMixedTranscript(b *testing.B) {
	b.ReportAllocs()

	events := benchmarkTranscriptEvents()

	for b.Loop() {
		messages, err := Assemble(events)
		if err != nil {
			b.Fatalf("Assemble() error = %v", err)
		}
		if len(messages) != 4 {
			b.Fatalf("Assemble() len = %d, want 4", len(messages))
		}
	}
}

func BenchmarkBuildToolResultObjectRawOutput(b *testing.B) {
	b.ReportAllocs()

	rawOutput := map[string]any{
		"stdout":    "workspace\n",
		"stderr":    "",
		"content":   "workspace\n",
		"file_path": "/tmp/workspace.txt",
		"structuredPatch": map[string]any{
			"ops": []map[string]any{
				{
					"op":   "replace",
					"path": "/tmp/workspace.txt",
				},
			},
		},
	}

	for b.Loop() {
		result := buildToolResult("Read", false, "", rawOutput)
		if result == nil {
			b.Fatal("buildToolResult() = nil")
			return
		}
		if result.Content != "workspace\n" {
			b.Fatalf("buildToolResult().Content = %q", result.Content)
		}
	}
}

func BenchmarkMarshalAgentEventToolResult(b *testing.B) {
	b.ReportAllocs()

	event := benchmarkToolResultAgentEvent()

	for b.Loop() {
		payload, err := MarshalAgentEvent(event)
		if err != nil {
			b.Fatalf("MarshalAgentEvent() error = %v", err)
		}
		if payload == "" {
			b.Fatal("MarshalAgentEvent() returned empty payload")
		}
	}
}

func BenchmarkUnmarshalAgentEventCanonical(b *testing.B) {
	b.ReportAllocs()

	payload, err := MarshalAgentEvent(benchmarkToolResultAgentEvent())
	if err != nil {
		b.Fatalf("MarshalAgentEvent() setup error = %v", err)
	}

	for b.Loop() {
		event, err := UnmarshalAgentEvent(payload)
		if err != nil {
			b.Fatalf("UnmarshalAgentEvent() error = %v", err)
		}
		if event.Type != acp.EventTypeToolResult {
			b.Fatalf("UnmarshalAgentEvent().Type = %q", event.Type)
		}
	}
}

func benchmarkTranscriptEvents() []store.SessionEvent {
	base := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
	return []store.SessionEvent{
		{
			ID:        "ev-1",
			Sequence:  1,
			TurnID:    "turn-bench",
			Type:      acp.EventTypeThought,
			Content:   `{"sessionUpdate":"agent_thought_chunk","content":{"type":"text","text":"Thinking "}}`,
			Timestamp: base,
		},
		{
			ID:        "ev-2",
			Sequence:  2,
			TurnID:    "turn-bench",
			Type:      acp.EventTypeThought,
			Content:   `{"sessionUpdate":"agent_thought_chunk","content":{"type":"text","text":"hard"}}`,
			Timestamp: base.Add(time.Second),
		},
		{
			ID:        "ev-3",
			Sequence:  3,
			TurnID:    "turn-bench",
			Type:      acp.EventTypeAgentMessage,
			Content:   `{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"Let me read "}}`,
			Timestamp: base.Add(2 * time.Second),
		},
		{
			ID:        "ev-4",
			Sequence:  4,
			TurnID:    "turn-bench",
			Type:      acp.EventTypeAgentMessage,
			Content:   `{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"the file"}}`,
			Timestamp: base.Add(3 * time.Second),
		},
		{
			ID:        "ev-5",
			Sequence:  5,
			TurnID:    "turn-bench",
			Type:      acp.EventTypeToolCall,
			Content:   `{"_meta":{"claudeCode":{"toolName":"Read"}},"toolCallId":"call-1","sessionUpdate":"tool_call","rawInput":{},"status":"pending","title":"Read File","kind":"read","content":[]}`,
			Timestamp: base.Add(4 * time.Second),
		},
		{
			ID:        "ev-6",
			Sequence:  6,
			TurnID:    "turn-bench",
			Type:      acp.EventTypeToolCall,
			Content:   `{"_meta":{"claudeCode":{"toolName":"Read"}},"toolCallId":"call-1","sessionUpdate":"tool_call_update","rawInput":{"file_path":"/tmp/demo.txt"},"status":"in_progress","title":"Read /tmp/demo.txt","kind":"read","content":[]}`,
			Timestamp: base.Add(5 * time.Second),
		},
		{
			ID:        "ev-7",
			Sequence:  7,
			TurnID:    "turn-bench",
			Type:      acp.EventTypeToolResult,
			Content:   `{"_meta":{"claudeCode":{"toolName":"Read"}},"toolCallId":"call-1","sessionUpdate":"tool_call_update","status":"completed","rawOutput":{"stdout":"line1\nline2","content":"line1\nline2","file_path":"/tmp/demo.txt"},"content":[{"type":"content","content":{"type":"text","text":"line1\nline2"}}]}`,
			Timestamp: base.Add(6 * time.Second),
		},
		{
			ID:        "ev-8",
			Sequence:  8,
			TurnID:    "turn-bench",
			Type:      acp.EventTypeAgentMessage,
			Content:   `{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"Done."}}`,
			Timestamp: base.Add(7 * time.Second),
		},
	}
}

func benchmarkToolResultAgentEvent() acp.AgentEvent {
	return acp.AgentEvent{
		Type:      acp.EventTypeToolResult,
		SessionID: "session-bench",
		TurnID:    "turn-bench",
		RequestID: "request-bench",
		Timestamp: time.Date(2026, 4, 3, 15, 0, 0, 0, time.UTC),
		Title:     "tool result",
		Raw: json.RawMessage(`{
			"sessionUpdate":"tool_call_update",
			"status":"failed",
			"rawOutput":{"stderr":"boom","content":"boom"},
			"content":[{"type":"content","content":{"type":"text","text":"boom"}}],
			"_meta":{"claudeCode":{"toolName":"Bash"}},
			"rawInput":{"command":"pwd"}
		}`),
	}
}
