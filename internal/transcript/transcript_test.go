package transcript

import (
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
)

func TestAssembleLegacyACPEvents(t *testing.T) {
	t.Parallel()

	events := []store.SessionEvent{
		{
			ID:        "ev-1",
			Sequence:  1,
			TurnID:    "turn-legacy",
			Type:      acp.EventTypeThought,
			Content:   `{"sessionUpdate":"agent_thought_chunk","content":{"type":"text","text":"Thinking "}}`,
			Timestamp: time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
		},
		{
			ID:        "ev-2",
			Sequence:  2,
			TurnID:    "turn-legacy",
			Type:      acp.EventTypeThought,
			Content:   `{"sessionUpdate":"agent_thought_chunk","content":{"type":"text","text":"hard"}}`,
			Timestamp: time.Date(2026, 4, 3, 12, 0, 1, 0, time.UTC),
		},
		{
			ID:        "ev-3",
			Sequence:  3,
			TurnID:    "turn-legacy",
			Type:      acp.EventTypeAgentMessage,
			Content:   `{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"Let me read "}}`,
			Timestamp: time.Date(2026, 4, 3, 12, 0, 2, 0, time.UTC),
		},
		{
			ID:        "ev-4",
			Sequence:  4,
			TurnID:    "turn-legacy",
			Type:      acp.EventTypeAgentMessage,
			Content:   `{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"the file"}}`,
			Timestamp: time.Date(2026, 4, 3, 12, 0, 3, 0, time.UTC),
		},
		{
			ID:        "ev-5",
			Sequence:  5,
			TurnID:    "turn-legacy",
			Type:      acp.EventTypeToolCall,
			Content:   `{"_meta":{"claudeCode":{"toolName":"Read"}},"toolCallId":"call-1","sessionUpdate":"tool_call","rawInput":{},"status":"pending","title":"Read File","kind":"read","content":[]}`,
			Timestamp: time.Date(2026, 4, 3, 12, 0, 4, 0, time.UTC),
		},
		{
			ID:        "ev-6",
			Sequence:  6,
			TurnID:    "turn-legacy",
			Type:      acp.EventTypeToolCall,
			Content:   `{"_meta":{"claudeCode":{"toolName":"Read"}},"toolCallId":"call-1","sessionUpdate":"tool_call_update","rawInput":{"file_path":"/tmp/demo.txt"},"status":"in_progress","title":"Read /tmp/demo.txt","kind":"read","content":[]}`,
			Timestamp: time.Date(2026, 4, 3, 12, 0, 5, 0, time.UTC),
		},
		{
			ID:        "ev-7",
			Sequence:  7,
			TurnID:    "turn-legacy",
			Type:      acp.EventTypeToolResult,
			Content:   `{"_meta":{"claudeCode":{"toolName":"Read"}},"toolCallId":"call-1","sessionUpdate":"tool_call_update","status":"completed","rawOutput":"line1\nline2","content":[{"type":"content","content":{"type":"text","text":"line1\nline2"}}]}`,
			Timestamp: time.Date(2026, 4, 3, 12, 0, 6, 0, time.UTC),
		},
		{
			ID:        "ev-8",
			Sequence:  8,
			TurnID:    "turn-legacy",
			Type:      acp.EventTypeAgentMessage,
			Content:   `{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"Done."}}`,
			Timestamp: time.Date(2026, 4, 3, 12, 0, 7, 0, time.UTC),
		},
	}

	messages, err := Assemble(events)
	if err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}
	if len(messages) != 4 {
		t.Fatalf("Assemble() len = %d, want 4", len(messages))
	}

	if got := messages[0].Role; got != RoleAssistant {
		t.Fatalf("messages[0].Role = %q, want %q", got, RoleAssistant)
	}
	if got := messages[0].Thinking; got != "Thinking hard" {
		t.Fatalf("messages[0].Thinking = %q, want %q", got, "Thinking hard")
	}
	if got := messages[0].Content; got != "Let me read the file" {
		t.Fatalf("messages[0].Content = %q, want %q", got, "Let me read the file")
	}
	if !messages[0].ThinkingComplete {
		t.Fatal("messages[0].ThinkingComplete = false, want true")
	}

	if got := messages[1].Role; got != RoleToolCall {
		t.Fatalf("messages[1].Role = %q, want %q", got, RoleToolCall)
	}
	if got := messages[1].ToolName; got != "Read" {
		t.Fatalf("messages[1].ToolName = %q, want %q", got, "Read")
	}
	if got := string(messages[1].ToolInput); got != `{"file_path":"/tmp/demo.txt"}` {
		t.Fatalf("messages[1].ToolInput = %s", got)
	}

	if got := messages[2].Role; got != RoleToolResult {
		t.Fatalf("messages[2].Role = %q, want %q", got, RoleToolResult)
	}
	if messages[2].ToolResult == nil || messages[2].ToolResult.Content != "line1\nline2" {
		t.Fatalf("messages[2].ToolResult = %#v, want content", messages[2].ToolResult)
	}
	if messages[2].ToolError {
		t.Fatal("messages[2].ToolError = true, want false")
	}

	if got := messages[3].Role; got != RoleAssistant {
		t.Fatalf("messages[3].Role = %q, want %q", got, RoleAssistant)
	}
	if got := messages[3].Content; got != "Done." {
		t.Fatalf("messages[3].Content = %q, want %q", got, "Done.")
	}
}

func TestAssembleReadsCanonicalEnvelopeAndStableOrdering(t *testing.T) {
	t.Run(
		"Should assemble canonical runtime payloads in stable event order",
		testAssembleReadsCanonicalEnvelopeAndStableOrdering,
	)
}

func TestAssembleRendersSyntheticReentryAsSystemMessage(t *testing.T) {
	t.Run(
		"Should render a synthetic reentry runtime payload as a system message",
		testAssembleRendersSyntheticReentryAsSystemMessage,
	)
}

func TestAssemblePreservesMixedTurnOrderingAndToolPairingAcrossTurns(t *testing.T) {
	t.Run(
		"Should preserve mixed-turn ordering and runtime tool pairing",
		testAssemblePreservesMixedTurnOrderingAndToolPairingAcrossTurns,
	)
}

func TestAssembleSkipsIgnorableEvents(t *testing.T) {
	t.Parallel()

	events := []store.SessionEvent{
		{
			ID:        "ev-empty-1",
			Sequence:  1,
			Type:      acp.EventTypeAgentMessage,
			Content:   `{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"   "}}`,
			Timestamp: time.Date(2026, 4, 3, 14, 0, 0, 0, time.UTC),
		},
		{
			ID:        "ev-empty-2",
			Sequence:  2,
			Type:      acp.EventTypeThought,
			Content:   `{"sessionUpdate":"agent_thought_chunk","content":{"type":"text","text":" "}}`,
			Timestamp: time.Date(2026, 4, 3, 14, 0, 1, 0, time.UTC),
		},
		{
			ID:        "ev-empty-3",
			Sequence:  3,
			Type:      acp.EventTypeUserMessage,
			Content:   "",
			Timestamp: time.Date(2026, 4, 3, 14, 0, 2, 0, time.UTC),
		},
	}

	messages, err := Assemble(events)
	if err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}
	if len(messages) != 0 {
		t.Fatalf("Assemble() len = %d, want 0", len(messages))
	}
}

func TestAssemblePairsToolLifecycleWhenResultOmitsTurnID(t *testing.T) {
	t.Run(
		"Should pair a runtime tool result without a turn ID to its call",
		testAssemblePairsToolLifecycleWhenResultOmitsTurnID,
	)
}
