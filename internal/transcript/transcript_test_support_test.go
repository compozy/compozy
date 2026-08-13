package transcript

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
)

func testToolResultMessage(t *testing.T, id, content string) Message {
	t.Helper()
	return Message{
		ID:       id,
		Role:     RoleToolResult,
		ToolName: "Read",
		ToolResult: &ToolResult{
			Content: content,
		},
	}
}

func testAssembleReadsCanonicalEnvelopeAndStableOrdering(t *testing.T) {
	t.Parallel()

	events := []store.SessionEvent{
		{
			ID:       "b",
			Sequence: 3,
			TurnID:   "turn-canonical",
			Type:     acp.EventTypeToolCall,
			Content: mustMarshalRuntimeEvent(
				t,
				acp.EventTypeToolCall,
				"turn-canonical",
				time.Date(2026, 4, 3, 13, 0, 2, 0, time.UTC),
				"",
				"Bash",
				"call-2",
				json.RawMessage(`{"command":"ls -la"}`),
				nil,
				false,
			),
			Timestamp: time.Date(2026, 4, 3, 13, 0, 2, 0, time.UTC),
		},
		{
			ID:       "a",
			Sequence: 1,
			TurnID:   "turn-canonical",
			Type:     acp.EventTypeUserMessage,
			Content: mustMarshalRuntimeEvent(
				t,
				acp.EventTypeUserMessage,
				"turn-canonical",
				time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC),
				"list files",
				"",
				"",
				nil,
				nil,
				false,
			),
			Timestamp: time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC),
		},
		{
			ID:       "c",
			Sequence: 4,
			TurnID:   "turn-canonical",
			Type:     acp.EventTypeToolResult,
			Content: mustMarshalRuntimeEvent(
				t,
				acp.EventTypeToolResult,
				"turn-canonical",
				time.Date(2026, 4, 3, 13, 0, 3, 0, time.UTC),
				"",
				"Bash",
				"call-2",
				nil,
				&ToolResult{Stdout: "ok"},
				false,
			),
			Timestamp: time.Date(2026, 4, 3, 13, 0, 3, 0, time.UTC),
		},
		{
			ID:       "d",
			Sequence: 2,
			TurnID:   "turn-canonical",
			Type:     acp.EventTypeAgentMessage,
			Content: mustMarshalRuntimeEvent(
				t,
				acp.EventTypeAgentMessage,
				"turn-canonical",
				time.Date(2026, 4, 3, 13, 0, 1, 0, time.UTC),
				"Listing files",
				"",
				"",
				nil,
				nil,
				false,
			),
			Timestamp: time.Date(2026, 4, 3, 13, 0, 1, 0, time.UTC),
		},
	}

	messages, err := Assemble(events)
	if err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}
	if len(messages) != 4 {
		t.Fatalf("Assemble() len = %d, want 4", len(messages))
	}

	if got := messages[0].Role; got != RoleUser {
		t.Fatalf("messages[0].Role = %q, want %q", got, RoleUser)
	}
	if got := messages[0].Content; got != "list files" {
		t.Fatalf("messages[0].Content = %q, want %q", got, "list files")
	}
	if got := messages[2].ToolName; got != "Bash" {
		t.Fatalf("messages[2].ToolName = %q, want %q", got, "Bash")
	}
	if got := string(messages[2].ToolInput); got != `{"command":"ls -la"}` {
		t.Fatalf("messages[2].ToolInput = %s", got)
	}
	if messages[3].ToolResult == nil || messages[3].ToolResult.Stdout != "ok" {
		t.Fatalf("messages[3].ToolResult = %#v, want stdout ok", messages[3].ToolResult)
	}
}

func testAssembleRendersSyntheticReentryAsSystemMessage(t *testing.T) {
	t.Parallel()

	events := []store.SessionEvent{
		{
			ID:       "user-1",
			Sequence: 1,
			TurnID:   "turn-user",
			Type:     acp.EventTypeUserMessage,
			Content: mustMarshalRuntimeEvent(
				t,
				acp.EventTypeUserMessage,
				"turn-user",
				time.Date(2026, 4, 18, 11, 0, 0, 0, time.UTC),
				"human prompt",
				"",
				"",
				nil,
				nil,
				false,
			),
			Timestamp: time.Date(2026, 4, 18, 11, 0, 0, 0, time.UTC),
		},
		{
			ID:       "synth-1",
			Sequence: 2,
			TurnID:   "turn-synth",
			Type:     acp.EventTypeSyntheticReentry,
			Content: mustMarshalRuntimeEvent(
				t,
				acp.EventTypeSyntheticReentry,
				"turn-synth",
				time.Date(2026, 4, 18, 11, 0, 1, 0, time.UTC),
				"daemon wake-up",
				"",
				"",
				nil,
				nil,
				false,
			),
			Timestamp: time.Date(2026, 4, 18, 11, 0, 1, 0, time.UTC),
		},
	}

	messages, err := Assemble(events)
	if err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("Assemble() len = %d, want 2", len(messages))
	}
	if got := messages[0].Role; got != RoleUser {
		t.Fatalf("messages[0].Role = %q, want %q", got, RoleUser)
	}
	if got := messages[1].Role; got != RoleSystem {
		t.Fatalf("messages[1].Role = %q, want %q", got, RoleSystem)
	}
	if got := messages[1].Content; got != "daemon wake-up" {
		t.Fatalf("messages[1].Content = %q, want %q", got, "daemon wake-up")
	}
}

func testAssemblePreservesMixedTurnOrderingAndToolPairingAcrossTurns(t *testing.T) {
	t.Parallel()

	events := []store.SessionEvent{
		{
			ID:       "user-1",
			Sequence: 1,
			TurnID:   "turn-user",
			Type:     acp.EventTypeUserMessage,
			Content: mustMarshalRuntimeEvent(
				t,
				acp.EventTypeUserMessage,
				"turn-user",
				time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC),
				"user prompt",
				"",
				"",
				nil,
				nil,
				false,
			),
			Timestamp: time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC),
		},
		{
			ID:       "call-user",
			Sequence: 2,
			TurnID:   "turn-user",
			Type:     acp.EventTypeToolCall,
			Content: mustMarshalRuntimeEvent(
				t,
				acp.EventTypeToolCall,
				"turn-user",
				time.Date(2026, 4, 18, 12, 0, 1, 0, time.UTC),
				"",
				"Bash",
				"call-user",
				json.RawMessage(`{"command":"echo user"}`),
				nil,
				false,
			),
			Timestamp: time.Date(2026, 4, 18, 12, 0, 1, 0, time.UTC),
		},
		{
			ID:       "synth-1",
			Sequence: 3,
			TurnID:   "turn-synth",
			Type:     acp.EventTypeSyntheticReentry,
			Content: mustMarshalRuntimeEvent(
				t,
				acp.EventTypeSyntheticReentry,
				"turn-synth",
				time.Date(2026, 4, 18, 12, 0, 2, 0, time.UTC),
				"daemon wake-up",
				"",
				"",
				nil,
				nil,
				false,
			),
			Timestamp: time.Date(2026, 4, 18, 12, 0, 2, 0, time.UTC),
		},
		{
			ID:       "result-user",
			Sequence: 4,
			TurnID:   "turn-user",
			Type:     acp.EventTypeToolResult,
			Content: mustMarshalRuntimeEvent(
				t,
				acp.EventTypeToolResult,
				"turn-user",
				time.Date(2026, 4, 18, 12, 0, 3, 0, time.UTC),
				"",
				"Bash",
				"call-user",
				nil,
				&ToolResult{Stdout: "user"},
				false,
			),
			Timestamp: time.Date(2026, 4, 18, 12, 0, 3, 0, time.UTC),
		},
		{
			ID:       "network-1",
			Sequence: 5,
			TurnID:   "turn-network",
			Type:     acp.EventTypeUserMessage,
			Content: mustMarshalRuntimeEvent(
				t,
				acp.EventTypeUserMessage,
				"turn-network",
				time.Date(2026, 4, 18, 12, 0, 4, 0, time.UTC),
				"network prompt",
				"",
				"",
				nil,
				nil,
				false,
			),
			Timestamp: time.Date(2026, 4, 18, 12, 0, 4, 0, time.UTC),
		},
		{
			ID:       "call-network",
			Sequence: 6,
			TurnID:   "turn-network",
			Type:     acp.EventTypeToolCall,
			Content: mustMarshalRuntimeEvent(
				t,
				acp.EventTypeToolCall,
				"turn-network",
				time.Date(2026, 4, 18, 12, 0, 5, 0, time.UTC),
				"",
				"Bash",
				"call-network",
				json.RawMessage(`{"command":"echo network"}`),
				nil,
				false,
			),
			Timestamp: time.Date(2026, 4, 18, 12, 0, 5, 0, time.UTC),
		},
		{
			ID:       "result-network",
			Sequence: 7,
			TurnID:   "turn-network",
			Type:     acp.EventTypeToolResult,
			Content: mustMarshalRuntimeEvent(
				t,
				acp.EventTypeToolResult,
				"turn-network",
				time.Date(2026, 4, 18, 12, 0, 6, 0, time.UTC),
				"",
				"Bash",
				"call-network",
				nil,
				&ToolResult{Stdout: "network"},
				false,
			),
			Timestamp: time.Date(2026, 4, 18, 12, 0, 6, 0, time.UTC),
		},
	}

	messages, err := Assemble(events)
	if err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}
	if len(messages) != 7 {
		t.Fatalf("Assemble() len = %d, want 7", len(messages))
	}

	if got := messages[0].Role; got != RoleUser {
		t.Fatalf("messages[0].Role = %q, want %q", got, RoleUser)
	}
	if got := messages[0].Content; got != "user prompt" {
		t.Fatalf("messages[0].Content = %q, want %q", got, "user prompt")
	}
	if got := messages[1].Role; got != RoleToolCall {
		t.Fatalf("messages[1].Role = %q, want %q", got, RoleToolCall)
	}
	if got := messages[2].Role; got != RoleSystem {
		t.Fatalf("messages[2].Role = %q, want %q", got, RoleSystem)
	}
	if got := messages[2].Content; got != "daemon wake-up" {
		t.Fatalf("messages[2].Content = %q, want %q", got, "daemon wake-up")
	}
	if got := messages[3].Role; got != RoleToolResult {
		t.Fatalf("messages[3].Role = %q, want %q", got, RoleToolResult)
	}
	if got := messages[4].Role; got != RoleUser {
		t.Fatalf("messages[4].Role = %q, want %q", got, RoleUser)
	}
	if got := messages[4].Content; got != "network prompt" {
		t.Fatalf("messages[4].Content = %q, want %q", got, "network prompt")
	}
	if got := messages[5].Role; got != RoleToolCall {
		t.Fatalf("messages[5].Role = %q, want %q", got, RoleToolCall)
	}
	if got := messages[6].Role; got != RoleToolResult {
		t.Fatalf("messages[6].Role = %q, want %q", got, RoleToolResult)
	}
	if got, want := messages[1].ID, "call-user"; got != want {
		t.Fatalf("messages[1].ID = %q, want %q", got, want)
	}
	if got, want := messages[3].ID, "call-user"; got != want {
		t.Fatalf("messages[3].ID = %q, want %q", got, want)
	}
	if got, want := messages[5].ID, "call-network"; got != want {
		t.Fatalf("messages[5].ID = %q, want %q", got, want)
	}
	if got, want := messages[6].ID, "call-network"; got != want {
		t.Fatalf("messages[6].ID = %q, want %q", got, want)
	}
	if messages[3].ToolResult == nil || messages[3].ToolResult.Stdout != "user" {
		t.Fatalf("messages[3].ToolResult = %#v, want stdout user", messages[3].ToolResult)
	}
	if messages[6].ToolResult == nil || messages[6].ToolResult.Stdout != "network" {
		t.Fatalf("messages[6].ToolResult = %#v, want stdout network", messages[6].ToolResult)
	}
}

func testAssemblePairsToolLifecycleWhenResultOmitsTurnID(t *testing.T) {
	t.Parallel()

	timestamp := time.Date(2026, 4, 18, 16, 0, 0, 0, time.UTC)
	events := []store.SessionEvent{
		{
			ID:       "call-1",
			Sequence: 1,
			TurnID:   "turn-tool",
			Type:     acp.EventTypeToolCall,
			Content: mustMarshalRuntimeEvent(
				t,
				acp.EventTypeToolCall,
				"turn-tool",
				timestamp,
				"",
				"Bash",
				"shared-call",
				json.RawMessage(`{"command":"pwd"}`),
				nil,
				false,
			),
			Timestamp: timestamp,
		},
		{
			ID:       "result-1",
			Sequence: 2,
			Type:     acp.EventTypeToolResult,
			Content: mustMarshalRuntimeEvent(
				t,
				acp.EventTypeToolResult,
				"",
				timestamp.Add(time.Second),
				"",
				"Bash",
				"shared-call",
				nil,
				&ToolResult{Stdout: "workspace"},
				false,
			),
			Timestamp: timestamp.Add(time.Second),
		},
	}

	messages, err := Assemble(events)
	if err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}
	if got, want := len(messages), 2; got != want {
		t.Fatalf("Assemble() len = %d, want %d", got, want)
	}
	if got, want := messages[0].Role, RoleToolCall; got != want {
		t.Fatalf("messages[0].Role = %q, want %q", got, want)
	}
	if got, want := messages[1].Role, RoleToolResult; got != want {
		t.Fatalf("messages[1].Role = %q, want %q", got, want)
	}
	if got, want := messages[0].ID, "shared-call"; got != want {
		t.Fatalf("messages[0].ID = %q, want %q", got, want)
	}
	if got, want := messages[1].ID, "shared-call"; got != want {
		t.Fatalf("messages[1].ID = %q, want %q", got, want)
	}
	if messages[1].ToolResult == nil || messages[1].ToolResult.Stdout != "workspace" {
		t.Fatalf("messages[1].ToolResult = %#v, want stdout workspace", messages[1].ToolResult)
	}
}

func mustUIAgentSessionEvent(
	t *testing.T,
	id string,
	sequence int64,
	timestamp time.Time,
	event acp.AgentEvent,
) store.SessionEvent {
	t.Helper()

	content, err := MarshalAgentEvent(event)
	if err != nil {
		t.Fatalf("MarshalAgentEvent(%s) error = %v", event.Type, err)
	}

	return store.SessionEvent{
		ID:        id,
		SessionID: event.SessionID,
		TurnID:    event.TurnID,
		Sequence:  sequence,
		Type:      event.Type,
		Content:   content,
		Timestamp: timestamp,
	}
}

func uiVisiblePartSignatures(parts []UIMessagePart) []string {
	signatures := make([]string, 0, len(parts))
	for _, part := range parts {
		switch {
		case part.Type == uiPartText:
			signatures = append(signatures, part.Type+":"+part.ID+":"+part.Text+":"+part.State)
		case part.Type == uiPartReasoning:
			signatures = append(signatures, part.Type+":"+part.ID+":"+part.Text+":"+part.State)
		case part.Type == uiPartDynamicTool || strings.HasPrefix(part.Type, "tool-"):
			signatures = append(signatures, part.Type+":"+part.ToolCallID+":"+part.State)
		}
	}
	return signatures
}

func mustPermissionSessionEvent(
	t *testing.T,
	id string,
	sequence int64,
	timestamp time.Time,
	decision string,
) store.SessionEvent {
	t.Helper()

	content, err := MarshalAgentEvent(acp.AgentEvent{
		Type:      acp.EventTypePermission,
		SessionID: "sess-permission",
		TurnID:    "turn-permission",
		Timestamp: timestamp,
		Title:     "Bash",
		Action:    "session/request_permission",
		Resource:  "Bash",
		Decision:  decision,
		Raw:       json.RawMessage(`{"command":"pwd"}`),
	}.WithRequestID("req-permission"))
	if err != nil {
		t.Fatalf("MarshalAgentEvent() error = %v", err)
	}

	return store.SessionEvent{
		ID:        id,
		SessionID: "sess-permission",
		TurnID:    "turn-permission",
		Sequence:  sequence,
		Type:      acp.EventTypePermission,
		Content:   content,
		Timestamp: timestamp,
	}
}

func assertNoDisplayLeaks(t *testing.T, value any, leaks []string) {
	t.Helper()

	var data []byte
	switch typed := value.(type) {
	case string:
		data = []byte(typed)
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			t.Fatalf("json.Marshal(%T) error = %v", value, err)
		}
		data = encoded
	}
	for _, leak := range leaks {
		if strings.Contains(string(data), leak) {
			t.Fatalf("display payload leaked %q: %s", leak, data)
		}
	}
}

func mustMarshalRuntimeEvent(
	t *testing.T,
	eventType string,
	turnID string,
	timestamp time.Time,
	text string,
	toolName string,
	toolCallID string,
	toolInput json.RawMessage,
	toolResult *ToolResult,
	toolError bool,
) string {
	t.Helper()

	event := acp.AgentEvent{
		Type:       eventType,
		TurnID:     turnID,
		Timestamp:  timestamp,
		Text:       text,
		ToolCallID: toolCallID,
	}.WithTool(toolName, toolInput, toolError)
	if toolResult != nil {
		rawOutput, err := json.Marshal(toolResult)
		if err != nil {
			t.Fatalf("json.Marshal(tool result) error = %v", err)
		}
		rawEvent, err := json.Marshal(struct {
			RawOutput json.RawMessage `json:"rawOutput"`
		}{RawOutput: rawOutput})
		if err != nil {
			t.Fatalf("json.Marshal(raw agent event) error = %v", err)
		}
		event.Raw = rawEvent
	}

	payload, err := MarshalAgentEvent(event)
	if err != nil {
		t.Fatalf("MarshalAgentEvent() error = %v", err)
	}
	return payload
}
