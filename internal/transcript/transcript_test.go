package transcript

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/diagnostics"
	"github.com/compozy/agh/internal/store"
)

func TestPruneTranscript(t *testing.T) {
	t.Parallel()

	t.Run("Should summarize old tool results while preserving the recent tail", func(t *testing.T) {
		t.Parallel()

		messages := make([]Message, 10)
		for i := range messages {
			messages[i] = testToolResultMessage(
				t,
				fmt.Sprintf("result-%d", i),
				strings.Repeat(fmt.Sprintf("line-%d\n", i), 40),
			)
		}

		pruned := Prune(messages, PruneOptions{MaxToolResultLines: 1, MaxArgBytes: 64})

		if got := pruned[0].ToolResult.Content; strings.Count(got, "\n") != 0 {
			t.Fatalf("Prune() old result lines = %d, want 1 line: %q", strings.Count(got, "\n")+1, got)
		}
		if !strings.Contains(pruned[0].ToolResult.Content, "40 lines") {
			t.Fatalf("Prune() old result = %q, want line-count summary", pruned[0].ToolResult.Content)
		}
		if !reflect.DeepEqual(pruned[2:], messages[2:]) {
			t.Fatal("Prune() changed the protected eight-message tail")
		}

		before, err := json.Marshal(messages)
		if err != nil {
			t.Fatalf("json.Marshal(messages) error = %v", err)
		}
		after, err := json.Marshal(pruned)
		if err != nil {
			t.Fatalf("json.Marshal(pruned) error = %v", err)
		}
		if len(after) >= len(before) {
			t.Fatalf("Prune() bytes = %d, want less than original %d", len(after), len(before))
		}
		t.Logf(
			"pruner fixture reduced projection from %d to %d bytes (~%d to ~%d tokens)",
			len(before), len(after), len(before)/4, len(after)/4,
		)
	})

	t.Run("Should keep one full copy when identical tool results repeat", func(t *testing.T) {
		t.Parallel()

		content := strings.Repeat("duplicate output\n", 20)
		messages := []Message{
			testToolResultMessage(t, "result-1", content),
			testToolResultMessage(t, "result-2", content),
			testToolResultMessage(t, "result-3", content),
		}

		pruned := Prune(messages, PruneOptions{MaxToolResultLines: 1, MaxArgBytes: 64, Dedup: true})

		fullCopies := 0
		markers := 0
		for _, message := range pruned {
			switch {
			case message.ToolResult != nil && message.ToolResult.Content == content:
				fullCopies++
			case message.ToolResult != nil && strings.Contains(message.ToolResult.Content, "Duplicate tool output"):
				markers++
			}
		}
		if fullCopies != 1 || markers != 2 {
			t.Fatalf("Prune() full copies = %d, markers = %d, want 1 and 2", fullCopies, markers)
		}
	})

	t.Run("Should not deduplicate structurally distinct results containing NUL bytes", func(t *testing.T) {
		t.Parallel()

		prefix := strings.Repeat("x", minPrunableResultBytes)
		messages := []Message{
			{
				ID:       "result-nul-1",
				Role:     RoleToolResult,
				ToolName: "Bash",
				ToolResult: &ToolResult{
					Stdout: prefix + "\x00stderr-prefix",
					Stderr: "tail",
				},
			},
			{
				ID:       "result-nul-2",
				Role:     RoleToolResult,
				ToolName: "Bash",
				ToolResult: &ToolResult{
					Stdout: prefix,
					Stderr: "stderr-prefix\x00tail",
				},
			},
		}

		pruned := Prune(messages, PruneOptions{Dedup: true})
		if !reflect.DeepEqual(pruned, messages) {
			t.Fatalf("Prune() = %#v, want distinct results preserved", pruned)
		}
	})

	t.Run("Should truncate oversized string arguments while preserving valid JSON", func(t *testing.T) {
		t.Parallel()

		input := json.RawMessage(`{"path":"/tmp/demo","content":"` + strings.Repeat("x", 200) + `"}`)
		messages := []Message{{ID: "call-1", Role: RoleToolCall, ToolName: "Write", ToolInput: input}}

		pruned := Prune(messages, PruneOptions{MaxToolResultLines: 1, MaxArgBytes: 32})

		if !json.Valid(pruned[0].ToolInput) {
			t.Fatalf("Prune() ToolInput = %q, want valid JSON", pruned[0].ToolInput)
		}
		if !strings.Contains(string(pruned[0].ToolInput), "...[truncated]") {
			t.Fatalf("Prune() ToolInput = %q, want truncation marker", pruned[0].ToolInput)
		}
		if !reflect.DeepEqual(messages[0].ToolInput, input) {
			t.Fatal("Prune() mutated the source ToolInput")
		}
	})

	t.Run("Should preserve invalid tool argument JSON instead of corrupting replay", func(t *testing.T) {
		t.Parallel()

		input := json.RawMessage(`{"content":` + strings.Repeat("x", 200))
		messages := []Message{{ID: "call-1", Role: RoleToolCall, ToolInput: input}}

		pruned := Prune(messages, PruneOptions{MaxToolResultLines: 1, MaxArgBytes: 32})

		if !reflect.DeepEqual(pruned[0].ToolInput, input) {
			t.Fatalf("Prune() ToolInput = %q, want original invalid payload %q", pruned[0].ToolInput, input)
		}
	})

	t.Run("Should preserve user and assistant ordering across noisy tool traffic", func(t *testing.T) {
		t.Parallel()

		messages := make([]Message, 0, 40)
		wantConversation := make([]string, 0, 20)
		for i := range 10 {
			userID := fmt.Sprintf("user-%d", i)
			assistantID := fmt.Sprintf("assistant-%d", i)
			messages = append(messages,
				Message{ID: userID, Role: RoleUser, Content: "request"},
				Message{ID: assistantID, Role: RoleAssistant, Content: "response"},
				testToolResultMessage(t, fmt.Sprintf("result-%d", i), strings.Repeat("noise\n", 50)),
			)
			wantConversation = append(wantConversation, userID, assistantID)
		}

		pruned := Prune(messages, PruneOptions{MaxToolResultLines: 1, MaxArgBytes: 64, Dedup: true})

		gotConversation := make([]string, 0, len(wantConversation))
		for _, message := range pruned {
			if message.Role == RoleUser || message.Role == RoleAssistant {
				gotConversation = append(gotConversation, message.ID)
			}
		}
		if !slices.Equal(gotConversation, wantConversation) {
			t.Fatalf("Prune() conversation order = %v, want %v", gotConversation, wantConversation)
		}
	})

	t.Run("Should apply defaults to nested UTF-8 arguments without changing typed values", func(t *testing.T) {
		t.Parallel()

		if got := Prune(nil, PruneOptions{}); got != nil {
			t.Fatalf("Prune(nil) = %#v, want nil", got)
		}
		input := json.RawMessage(`{"items":[{"value":"` + strings.Repeat("é", 300) + `"},42,true]}`)
		messages := []Message{{ID: "call-1", Role: RoleToolCall, ToolInput: input}}

		pruned := Prune(messages, PruneOptions{})

		if !json.Valid(pruned[0].ToolInput) || !strings.Contains(string(pruned[0].ToolInput), "...[truncated]") {
			t.Fatalf("Prune() ToolInput = %q, want valid truncated UTF-8 JSON", pruned[0].ToolInput)
		}
		var decoded map[string]any
		if err := json.Unmarshal(pruned[0].ToolInput, &decoded); err != nil {
			t.Fatalf("json.Unmarshal(ToolInput) error = %v", err)
		}
		items, ok := decoded["items"].([]any)
		if !ok || len(items) != 3 || items[1] != float64(42) || items[2] != true {
			t.Fatalf("Prune() typed items = %#v, want string, 42, true", decoded["items"])
		}
	})

	t.Run("Should preserve oversized argument objects when no string leaf can be shortened", func(t *testing.T) {
		t.Parallel()

		values := make([]int, 300)
		for i := range values {
			values[i] = i
		}
		input, err := json.Marshal(map[string]any{"values": values})
		if err != nil {
			t.Fatalf("json.Marshal(values) error = %v", err)
		}
		messages := []Message{{ID: "call-1", Role: RoleToolCall, ToolInput: input}}

		pruned := Prune(messages, PruneOptions{MaxArgBytes: 32})

		if !slices.Equal(pruned[0].ToolInput, input) {
			t.Fatalf("Prune() ToolInput = %q, want unchanged numeric object %q", pruned[0].ToolInput, input)
		}
	})

	t.Run("Should summarize fallback output fields with default options", func(t *testing.T) {
		t.Parallel()

		messages := []Message{{
			Role:       RoleToolResult,
			ToolResult: &ToolResult{Stderr: strings.Repeat("failure detail\n", 30)},
		}}
		for range 8 {
			messages = append(messages, Message{Role: RoleSystem, Content: "protected"})
		}

		pruned := Prune(messages, PruneOptions{})

		if !strings.HasPrefix(pruned[0].ToolResult.Content, "[tool] failure detail") {
			t.Fatalf("Prune() fallback summary = %q", pruned[0].ToolResult.Content)
		}
	})

	t.Run("Should be deterministic without mutating the source projection", func(t *testing.T) {
		t.Parallel()

		messages := []Message{
			{ID: "user-1", Role: RoleUser, Content: "inspect the workspace"},
			testToolResultMessage(t, "result-1", strings.Repeat("workspace line\n", 30)),
			testToolResultMessage(t, "result-2", strings.Repeat("workspace line\n", 30)),
			{ID: "assistant-1", Role: RoleAssistant, Content: "inspection complete"},
		}
		before, err := json.Marshal(messages)
		if err != nil {
			t.Fatalf("json.Marshal(messages) error = %v", err)
		}

		first := Prune(messages, PruneOptions{MaxToolResultLines: 1, MaxArgBytes: 32, Dedup: true})
		second := Prune(messages, PruneOptions{MaxToolResultLines: 1, MaxArgBytes: 32, Dedup: true})

		if !reflect.DeepEqual(first, second) {
			t.Fatalf("Prune() outputs differ:\nfirst: %#v\nsecond: %#v", first, second)
		}
		after, err := json.Marshal(messages)
		if err != nil {
			t.Fatalf("json.Marshal(messages) after prune error = %v", err)
		}
		if !slices.Equal(before, after) {
			t.Fatalf("Prune() mutated source bytes:\nbefore: %s\nafter:  %s", before, after)
		}

		events := make([]store.SessionEvent, 10)
		for i := range events {
			toolCallID := fmt.Sprintf("persisted-call-%d", i)
			timestamp := time.Date(2026, 7, 15, 12, 0, i, 0, time.UTC)
			events[i] = store.SessionEvent{
				ID:       fmt.Sprintf("persisted-event-%d", i),
				Sequence: int64(i + 1),
				TurnID:   "persisted-turn",
				Type:     acp.EventTypeToolResult,
				Content: mustMarshalCanonical(
					t,
					acp.EventTypeToolResult,
					"persisted-turn",
					timestamp,
					"",
					"Read",
					toolCallID,
					nil,
					&ToolResult{Content: strings.Repeat(fmt.Sprintf("persisted-%d\n", i), 30)},
					false,
				),
				Timestamp: timestamp,
			}
		}
		rawBefore, err := json.Marshal(events)
		if err != nil {
			t.Fatalf("json.Marshal(events) error = %v", err)
		}
		projection, err := Assemble(events)
		if err != nil {
			t.Fatalf("Assemble(events) error = %v", err)
		}
		prunedProjection := Prune(projection, PruneOptions{Dedup: true})
		if reflect.DeepEqual(prunedProjection, projection) {
			t.Fatal("Prune() left the persisted-event projection unchanged")
		}
		rawAfter, err := json.Marshal(events)
		if err != nil {
			t.Fatalf("json.Marshal(events) after prune error = %v", err)
		}
		if !slices.Equal(rawBefore, rawAfter) {
			t.Fatalf("Prune() mutated persisted event bytes:\nbefore: %s\nafter:  %s", rawBefore, rawAfter)
		}
	})

	t.Run("Should preserve nil results and back-reference anonymous duplicates", func(t *testing.T) {
		t.Parallel()

		content := strings.Repeat("anonymous duplicate\n", 20)
		messages := []Message{
			{Role: RoleToolResult},
			{Role: RoleToolResult, ToolName: "Read", ToolResult: &ToolResult{Content: content}},
			{Role: RoleToolResult, ToolName: "Read", ToolResult: &ToolResult{Content: content}},
		}

		pruned := Prune(messages, PruneOptions{Dedup: true})

		if pruned[0].ToolResult != nil {
			t.Fatalf("Prune() nil result = %#v, want nil", pruned[0].ToolResult)
		}
		if got := pruned[1].ToolResult.Content; !strings.Contains(got, "newest matching result") {
			t.Fatalf("Prune() anonymous duplicate marker = %q, want stable newest-result reference", got)
		}
		if got := pruned[2].ToolResult.Content; got != content {
			t.Fatalf("Prune() newest anonymous result = %q, want full content", got)
		}
	})
}

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
	t.Parallel()

	events := []store.SessionEvent{
		{
			ID:       "b",
			Sequence: 3,
			TurnID:   "turn-canonical",
			Type:     acp.EventTypeToolCall,
			Content: mustMarshalCanonical(
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
			Content: mustMarshalCanonical(
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
			Content: mustMarshalCanonical(
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
			Content: mustMarshalCanonical(
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

func TestAssembleRendersSyntheticReentryAsSystemMessage(t *testing.T) {
	t.Parallel()

	events := []store.SessionEvent{
		{
			ID:       "user-1",
			Sequence: 1,
			TurnID:   "turn-user",
			Type:     acp.EventTypeUserMessage,
			Content: mustMarshalCanonical(
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
			Content: mustMarshalCanonical(
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

func TestAssemblePreservesMixedTurnOrderingAndToolPairingAcrossTurns(t *testing.T) {
	t.Parallel()

	events := []store.SessionEvent{
		{
			ID:       "user-1",
			Sequence: 1,
			TurnID:   "turn-user",
			Type:     acp.EventTypeUserMessage,
			Content: mustMarshalCanonical(
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
			Content: mustMarshalCanonical(
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
			Content: mustMarshalCanonical(
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
			Content: mustMarshalCanonical(
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
			Content: mustMarshalCanonical(
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
			Content: mustMarshalCanonical(
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
			Content: mustMarshalCanonical(
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
	t.Parallel()

	timestamp := time.Date(2026, 4, 18, 16, 0, 0, 0, time.UTC)
	events := []store.SessionEvent{
		{
			ID:       "call-1",
			Sequence: 1,
			TurnID:   "turn-tool",
			Type:     acp.EventTypeToolCall,
			Content: mustMarshalCanonical(
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
			Content: mustMarshalCanonical(
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

func TestClarificationEventProjectionPreservesTypedEvidence(t *testing.T) {
	t.Run("Should round-trip clarification evidence into the UI data payload", func(t *testing.T) {
		t.Parallel()

		raw := json.RawMessage(`{
			"status":"resolved",
			"request":{"request_id":"req-clarify","question":"Which channel?"},
			"answer":{"choice":1,"text":"","fallback":false},
			"at":"2026-07-15T12:01:00Z"
		}`)
		content, err := MarshalAgentEvent(acp.AgentEvent{
			Type:      acp.EventTypeClarify,
			SessionID: "sess-clarify",
			TurnID:    "clarify:req-clarify",
			RequestID: "req-clarify",
			Timestamp: time.Date(2026, 7, 15, 12, 1, 0, 0, time.UTC),
			Raw:       raw,
		})
		if err != nil {
			t.Fatalf("MarshalAgentEvent() error = %v", err)
		}
		decoded, err := UnmarshalAgentEvent(content)
		if err != nil {
			t.Fatalf("UnmarshalAgentEvent() error = %v", err)
		}
		payload := UIAgentEventPayloadFromEvent(decoded)
		if got, want := payload.Type, acp.EventTypeClarify; got != want {
			t.Fatalf("payload type = %q, want %q", got, want)
		}
		if got, want := payload.RequestID, "req-clarify"; got != want {
			t.Fatalf("payload request id = %q, want %q", got, want)
		}
		var gotRaw, wantRaw any
		if err := json.Unmarshal(payload.Raw, &gotRaw); err != nil {
			t.Fatalf("json.Unmarshal(payload raw) error = %v", err)
		}
		if err := json.Unmarshal(raw, &wantRaw); err != nil {
			t.Fatalf("json.Unmarshal(expected raw) error = %v", err)
		}
		if !reflect.DeepEqual(gotRaw, wantRaw) {
			t.Fatalf("payload raw = %#v, want %#v", gotRaw, wantRaw)
		}
	})
}

func TestToUIMessagesPreservesUserClientIdentity(t *testing.T) {
	t.Run("Should project the durable client message identity into user metadata", func(t *testing.T) {
		t.Parallel()

		timestamp := time.Date(2026, 7, 13, 23, 55, 0, 0, time.UTC)
		clientMessageID := "client-user-identity"
		event := acp.AgentEvent{
			Type:      acp.EventTypeUserMessage,
			TurnID:    "turn-user-identity",
			Timestamp: timestamp,
			Text:      "Transformed provider input.",
		}.WithClientMessageID(clientMessageID)
		authoredText := "Keep this message after reload."
		payload, err := MarshalPromptInputEvent(event, authoredText)
		if err != nil {
			t.Fatalf("MarshalAgentEvent() error = %v", err)
		}
		decoded, err := UnmarshalAgentEvent(payload)
		if err != nil {
			t.Fatalf("UnmarshalAgentEvent() error = %v", err)
		}
		if got, want := decoded.ClientMessageIDValue(), clientMessageID; got != want {
			t.Fatalf("ClientMessageID = %q, want %q", got, want)
		}
		if got, want := decoded.Text, event.Text; got != want {
			t.Fatalf("effective Text = %q, want %q", got, want)
		}

		messages, err := ToUIMessages([]store.SessionEvent{{
			ID:        "ev-user-identity",
			SessionID: "sess-user-identity",
			TurnID:    event.TurnID,
			Sequence:  1,
			Type:      event.Type,
			Content:   payload,
			Timestamp: timestamp,
		}})
		if err != nil {
			t.Fatalf("ToUIMessages() error = %v", err)
		}
		if got, want := len(messages), 1; got != want {
			t.Fatalf("len(messages) = %d, want %d", got, want)
		}
		var metadata struct {
			TurnID          string `json:"turn_id"`
			ClientMessageID string `json:"client_message_id"`
		}
		if err := json.Unmarshal(messages[0].Metadata, &metadata); err != nil {
			t.Fatalf("json.Unmarshal(metadata) error = %v", err)
		}
		if metadata.TurnID != event.TurnID || metadata.ClientMessageID != clientMessageID {
			t.Fatalf("metadata = %#v, want turn/client identity", metadata)
		}
		if got, want := messages[0].Parts[0].Text, authoredText; got != want {
			t.Fatalf("projected user text = %q, want exact authored text %q", got, want)
		}
	})
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

func TestTranscriptRedactsSecretsAcrossDisplaySurfaces(t *testing.T) {
	t.Parallel()

	runtimeSecret := "sk-transcript-display-secret-123456"
	unregisteredProviderSecret := "sk-ant-api03-abcdefghijklmnopqrstuv"
	cleanup := diagnostics.RegisterDynamicSecret(runtimeSecret)
	t.Cleanup(cleanup)

	t.Run("Should redact live event payloads before UI projection", func(t *testing.T) {
		t.Parallel()

		leaks := []string{
			runtimeSecret,
			unregisteredProviderSecret,
			"text-secret",
			"agh_claim_live_123",
			"bearer-secret",
			"failure-secret",
			"runtime-secret",
			"raw-secret",
			"raw-private-camel",
			"tool-name-secret",
			"tool-input-secret",
			"tool-credential-secret",
		}
		event := (acp.AgentEvent{
			Type:      acp.EventTypeAgentMessage,
			SessionID: "sess-redact",
			TurnID:    "turn-redact",
			Timestamp: time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC),
			Text:      "assistant stdout " + runtimeSecret + " token=text-secret agh_claim_live_123",
			Error:     "Bearer bearer-secret",
			Failure: &store.SessionFailure{
				Kind:    store.FailurePrompt,
				Summary: "secret_binding=failure-secret",
			},
			Runtime: &acp.RuntimeActivity{
				LastActivityDetail: "token=runtime-secret",
			},
			Raw: json.RawMessage(
				`{"access_token":"raw-secret","privateKey":"raw-private-camel","note":"` + runtimeSecret + `","opaque":"` + unregisteredProviderSecret + `"}`,
			),
		}).WithTool(
			"token=tool-name-secret",
			json.RawMessage(
				`{"apiKey":"tool-input-secret","credential":"tool-credential-secret","command":"echo ok"}`,
			),
			false,
		)

		assertNoDisplayLeaks(t, RedactAgentEvent(event), leaks)
		assertNoDisplayLeaks(t, UIAgentEventPayloadFromEvent(event), leaks)
	})

	t.Run("Should redact stored tool output before transcript and chat replay", func(t *testing.T) {
		t.Parallel()

		leaks := []string{
			runtimeSecret,
			"stdout-secret",
			"stderr-secret",
			"content-secret",
			"raw-binding",
			"raw-secret",
			"input-secret",
			"agh_claim_tool_123",
		}
		payload, err := MarshalAgentEvent(acp.AgentEvent{
			Type:      acp.EventTypeToolResult,
			SessionID: "sess-redact",
			TurnID:    "turn-redact",
			Timestamp: time.Date(2026, 5, 19, 10, 0, 1, 0, time.UTC),
			Title:     "Bash",
			Raw: json.RawMessage(`{
				"sessionUpdate":"tool_call_update",
				"status":"completed",
				"rawOutput":{
					"stdout":"runtime ` + runtimeSecret + ` token=stdout-secret agh_claim_tool_123",
					"stderr":"Bearer stderr-secret",
					"content":"secret_binding=raw-binding",
					"api_key":"raw-secret"
				},
				"content":[{"type":"content","content":{"type":"text","text":"token=content-secret ` + runtimeSecret + `"}}],
				"_meta":{"claudeCode":{"toolName":"Bash"}},
				"rawInput":{"api_key":"input-secret","command":"echo ok"}
			}`),
		})
		if err != nil {
			t.Fatalf("MarshalAgentEvent() error = %v", err)
		}
		assertNoDisplayLeaks(t, payload, leaks)

		events := []store.SessionEvent{{
			ID:        "ev-redact-tool",
			SessionID: "sess-redact",
			TurnID:    "turn-redact",
			Sequence:  1,
			Type:      acp.EventTypeToolResult,
			Content:   payload,
			Timestamp: time.Date(2026, 5, 19, 10, 0, 1, 0, time.UTC),
		}}
		transcriptMessages, err := Assemble(events)
		if err != nil {
			t.Fatalf("Assemble() error = %v", err)
		}
		assertNoDisplayLeaks(t, transcriptMessages, leaks)

		uiMessages, err := ToUIMessages(events)
		if err != nil {
			t.Fatalf("ToUIMessages() error = %v", err)
		}
		assertNoDisplayLeaks(t, uiMessages, leaks)
	})
}

func TestTranscriptRuntimeMarkers(t *testing.T) {
	t.Parallel()

	timestamp := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		turnID  string
		kind    string
		summary string
		want    string
	}{
		{
			name:    "Should render timeout marker from canonical marker event",
			turnID:  "turn-timeout",
			kind:    MarkerPromptTimeout,
			summary: "Runtime activity timed out (30 seconds idle).",
			want:    "Runtime activity timed out (30 seconds idle).",
		},
		{
			name:    "Should render unhealthy marker from canonical marker event",
			turnID:  "turn-unhealthy",
			kind:    MarkerSessionUnhealthy,
			summary: "Runtime health check failed; prompt may be stalled.",
			want:    "Runtime health check failed; prompt may be stalled.",
		},
		{
			name:    "Should render interrupted marker from canonical marker event",
			turnID:  "turn-interrupt",
			kind:    MarkerPromptInterrupted,
			summary: "operator interrupted the turn",
			want:    "operator interrupted the turn",
		},
	}

	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			marker, err := NewMarker(
				test.kind,
				test.summary,
				timestamp.Add(time.Duration(index)*time.Second),
				map[string]any{"failure_count": 2},
			)
			if err != nil {
				t.Fatalf("NewMarker() error = %v", err)
			}
			event, err := marker.AgentEvent("sess-marker", test.turnID)
			if err != nil {
				t.Fatalf("AgentEvent() error = %v", err)
			}
			events := []store.SessionEvent{
				mustUIAgentSessionEvent(
					t,
					"ev-marker-"+test.turnID,
					int64(index+1),
					event.Timestamp,
					event,
				),
			}
			transcriptMessages, err := Assemble(events)
			if err != nil {
				t.Fatalf("Assemble() error = %v", err)
			}
			if got, want := len(transcriptMessages), 1; got != want {
				t.Fatalf("len(transcriptMessages) = %d, want %d; messages=%#v", got, want, transcriptMessages)
			}
			if got, want := transcriptMessages[0].Role, RoleSystem; got != want {
				t.Fatalf("transcript role = %q, want %q", got, want)
			}
			if got := transcriptMessages[0].Content; got != test.want {
				t.Fatalf("transcript content = %q, want %q", got, test.want)
			}

			uiMessages, err := ToUIMessages(events)
			if err != nil {
				t.Fatalf("ToUIMessages() error = %v", err)
			}
			if got, want := len(uiMessages), 1; got != want {
				t.Fatalf("len(uiMessages) = %d, want %d; messages=%#v", got, want, uiMessages)
			}
			if got, want := uiMessages[0].Role, UIRoleAssistant; got != want {
				t.Fatalf("UI role = %q, want %q", got, want)
			}
			if got, want := len(uiMessages[0].Parts), 1; got != want {
				t.Fatalf("UI marker parts = %d, want %d; parts=%#v", got, want, uiMessages[0].Parts)
			}
			markerPart := uiMessages[0].Parts[0]
			if got, want := markerPart.Type, uiPartDataEvent; got != want {
				t.Fatalf("UI marker part type = %q, want %q", got, want)
			}
			var payload struct {
				Type string `json:"type"`
				Raw  struct {
					Kind       string         `json:"kind"`
					OccurredAt string         `json:"occurred_at"`
					Summary    string         `json:"summary"`
					Evidence   map[string]any `json:"evidence"`
				} `json:"raw"`
			}
			if err := json.Unmarshal(markerPart.Data, &payload); err != nil {
				t.Fatalf("UI marker data payload = %q, unmarshal error = %v", markerPart.Data, err)
			}
			if got, want := payload.Type, event.Type; got != want {
				t.Fatalf("UI marker event type = %q, want %q", got, want)
			}
			if got, want := payload.Raw.Kind, test.kind; got != want {
				t.Fatalf("UI marker raw kind = %q, want %q", got, want)
			}
			if got, want := payload.Raw.Summary, test.want; got != want {
				t.Fatalf("UI marker raw summary = %q, want %q", got, want)
			}
			gotOccurred, err := time.Parse(time.RFC3339Nano, payload.Raw.OccurredAt)
			if err != nil {
				t.Fatalf("UI marker raw occurred_at = %q, parse error = %v", payload.Raw.OccurredAt, err)
			}
			if !gotOccurred.Equal(marker.OccurredAt) {
				t.Fatalf("UI marker raw occurred_at = %s, want %s", gotOccurred, marker.OccurredAt)
			}
			if got, want := payload.Raw.Evidence["failure_count"], float64(2); got != want {
				t.Fatalf(
					"UI marker raw evidence[failure_count] = %v, want %v; evidence=%#v",
					got,
					want,
					payload.Raw.Evidence,
				)
			}
		})
	}
}

func TestToUIMessagesPermissionDataParts(t *testing.T) {
	t.Run("ShouldReplacePendingPermissionWithFinalDecision", func(t *testing.T) {
		t.Parallel()

		timestamp := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
		events := []store.SessionEvent{
			mustPermissionSessionEvent(t, "ev-pending", 1, timestamp, ""),
			mustPermissionSessionEvent(t, "ev-final", 2, timestamp.Add(time.Second), "allow-once"),
		}

		messages, err := ToUIMessages(events)
		if err != nil {
			t.Fatalf("ToUIMessages() error = %v", err)
		}
		if got, want := len(messages), 1; got != want {
			t.Fatalf("len(messages) = %d, want %d", got, want)
		}
		if got, want := len(messages[0].Parts), 1; got != want {
			t.Fatalf("len(parts) = %d, want %d; parts=%#v", got, want, messages[0].Parts)
		}

		part := messages[0].Parts[0]
		if got, want := part.Type, uiPartDataPermission; got != want {
			t.Fatalf("part.Type = %q, want %q", got, want)
		}
		if got, want := part.ID, "req-permission"; got != want {
			t.Fatalf("part.ID = %q, want %q", got, want)
		}

		var payload UIAgentEventPayload
		if err := json.Unmarshal(part.Data, &payload); err != nil {
			t.Fatalf("json.Unmarshal(part.Data) error = %v", err)
		}
		if got, want := payload.Decision, "allow-once"; got != want {
			t.Fatalf("payload.Decision = %q, want %q", got, want)
		}
	})

	t.Run("ShouldPreservePendingPermissionWithoutDecision", func(t *testing.T) {
		t.Parallel()

		timestamp := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
		events := []store.SessionEvent{
			mustPermissionSessionEvent(t, "ev-pending", 1, timestamp, ""),
		}

		messages, err := ToUIMessages(events)
		if err != nil {
			t.Fatalf("ToUIMessages() error = %v", err)
		}
		if got, want := len(messages), 1; got != want {
			t.Fatalf("len(messages) = %d, want %d", got, want)
		}
		if got, want := len(messages[0].Parts), 1; got != want {
			t.Fatalf("len(parts) = %d, want %d; parts=%#v", got, want, messages[0].Parts)
		}

		part := messages[0].Parts[0]
		if got, want := part.ID, "req-permission"; got != want {
			t.Fatalf("part.ID = %q, want %q", got, want)
		}

		var payload UIAgentEventPayload
		if err := json.Unmarshal(part.Data, &payload); err != nil {
			t.Fatalf("json.Unmarshal(part.Data) error = %v", err)
		}
		if payload.Decision != "" {
			t.Fatalf("payload.Decision = %q, want empty", payload.Decision)
		}
	})

	t.Run("ShouldPreservePermissionOptionsForReplay", func(t *testing.T) {
		t.Parallel()

		timestamp := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
		content, err := MarshalAgentEvent(acp.AgentEvent{
			Type:      acp.EventTypePermission,
			SessionID: "sess-permission",
			TurnID:    "turn-permission",
			RequestID: "req-permission-options",
			Timestamp: timestamp,
			Title:     "Bash",
			Action:    "session/request_permission",
			Resource:  "Bash",
			Raw: json.RawMessage(`{
				"request_id":"req-permission-options",
				"options":[
					{"decision":"allow-once","option_id":"allow-once","kind":"allow_once"},
					{"decision":"reject-once","option_id":"reject-once","kind":"reject_once"}
				],
				"tool_input":{"command":"touch blocked.txt"}
			}`),
		})
		if err != nil {
			t.Fatalf("MarshalAgentEvent() error = %v", err)
		}

		event := store.SessionEvent{
			ID:        "ev-permission-options",
			SessionID: "sess-permission",
			TurnID:    "turn-permission",
			Sequence:  1,
			Type:      acp.EventTypePermission,
			Content:   content,
			Timestamp: timestamp,
		}
		messages, err := ToUIMessages([]store.SessionEvent{event})
		if err != nil {
			t.Fatalf("ToUIMessages() error = %v", err)
		}
		if got, want := len(messages), 1; got != want {
			t.Fatalf("len(messages) = %d, want %d", got, want)
		}
		if got, want := len(messages[0].Parts), 1; got != want {
			t.Fatalf("len(parts) = %d, want %d; parts=%#v", got, want, messages[0].Parts)
		}

		var payload UIAgentEventPayload
		if err := json.Unmarshal(messages[0].Parts[0].Data, &payload); err != nil {
			t.Fatalf("json.Unmarshal(part.Data) error = %v", err)
		}
		var raw struct {
			Options []struct {
				Decision string `json:"decision"`
			} `json:"options"`
			ToolInput struct {
				Command string `json:"command"`
			} `json:"tool_input"`
		}
		if err := json.Unmarshal(payload.Raw, &raw); err != nil {
			t.Fatalf("json.Unmarshal(payload.Raw) error = %v", err)
		}
		if got, want := len(raw.Options), 2; got != want {
			t.Fatalf("len(raw.Options) = %d, want %d", got, want)
		}
		if got, want := raw.Options[0].Decision, "allow-once"; got != want {
			t.Fatalf("raw.Options[0].Decision = %q, want %q", got, want)
		}
		if got, want := raw.Options[1].Decision, "reject-once"; got != want {
			t.Fatalf("raw.Options[1].Decision = %q, want %q", got, want)
		}
		if got, want := raw.ToolInput.Command, "touch blocked.txt"; got != want {
			t.Fatalf("raw.ToolInput.Command = %q, want %q", got, want)
		}
	})
}

func TestToUIMessagesOrderedAssistantParts(t *testing.T) {
	t.Run("ShouldPreserveFatalPromptErrorAsDataPart", func(t *testing.T) {
		t.Parallel()

		timestamp := time.Date(2026, 5, 14, 15, 32, 0, 0, time.UTC)
		errorText := `{"code":-32603,"message":"Internal error","data":{"error":"peer disconnected before response"}}`
		events := []store.SessionEvent{
			mustUIAgentSessionEvent(t, "ev-text", 1, timestamp, acp.AgentEvent{
				Type:      acp.EventTypeAgentMessage,
				SessionID: "sess-failed",
				TurnID:    "turn-failed",
				Timestamp: timestamp,
				Text:      "partial response",
			}),
			mustUIAgentSessionEvent(t, "ev-error", 2, timestamp.Add(time.Second), acp.AgentEvent{
				Type:      acp.EventTypeError,
				SessionID: "sess-failed",
				TurnID:    "turn-failed",
				Timestamp: timestamp.Add(time.Second),
				Error:     errorText,
				Failure: &store.SessionFailure{
					Kind:    store.FailureProcess,
					Summary: "peer disconnected before response",
				},
			}),
		}

		messages, err := ToUIMessages(events)
		if err != nil {
			t.Fatalf("ToUIMessages() error = %v", err)
		}
		if got, want := len(messages), 1; got != want {
			t.Fatalf("len(messages) = %d, want %d; messages=%#v", got, want, messages)
		}
		if got, want := len(messages[0].Parts), 2; got != want {
			t.Fatalf("len(messages[0].Parts) = %d, want %d; parts=%#v", got, want, messages[0].Parts)
		}
		if got, want := messages[0].Parts[0].Type, uiPartText; got != want {
			t.Fatalf("parts[0].Type = %q, want %q", got, want)
		}
		if got, want := messages[0].Parts[0].State, uiPartStateDone; got != want {
			t.Fatalf("parts[0].State = %q, want %q", got, want)
		}

		errorPart := messages[0].Parts[1]
		if got, want := errorPart.Type, uiPartDataEvent; got != want {
			t.Fatalf("parts[1].Type = %q, want %q", got, want)
		}

		var payload UIAgentEventPayload
		if err := json.Unmarshal(errorPart.Data, &payload); err != nil {
			t.Fatalf("json.Unmarshal(errorPart.Data) error = %v", err)
		}
		if got, want := payload.Type, acp.EventTypeError; got != want {
			t.Fatalf("payload.Type = %q, want %q", got, want)
		}
		if got, want := payload.Error, errorText; got != want {
			t.Fatalf("payload.Error = %q, want %q", got, want)
		}
		if payload.Failure == nil {
			t.Fatal("payload.Failure = nil, want process failure")
		}
		if got, want := payload.Failure.Kind, store.FailureProcess; got != want {
			t.Fatalf("payload.Failure.Kind = %q, want %q", got, want)
		}
	})

	t.Run("ShouldPreserveTextToolTextOrderInsideOneAssistantMessage", func(t *testing.T) {
		t.Parallel()

		timestamp := time.Date(2026, 5, 13, 10, 0, 0, 0, time.UTC)
		events := []store.SessionEvent{
			mustUIAgentSessionEvent(t, "ev-text-1", 1, timestamp, acp.AgentEvent{
				Type:      acp.EventTypeAgentMessage,
				SessionID: "sess-mixed",
				TurnID:    "turn-mixed",
				Timestamp: timestamp,
				Text:      "text 1",
			}),
			mustUIAgentSessionEvent(t, "ev-tool-2", 2, timestamp.Add(time.Second), acp.AgentEvent{
				Type:       acp.EventTypeToolCall,
				SessionID:  "sess-mixed",
				TurnID:     "turn-mixed",
				Timestamp:  timestamp.Add(time.Second),
				Title:      "Bash",
				ToolCallID: "tool-2",
				Raw:        json.RawMessage("{\"rawInput\":{\"command\":\"pwd\"}}"),
			}),
			mustUIAgentSessionEvent(t, "ev-tool-3", 3, timestamp.Add(2*time.Second), acp.AgentEvent{
				Type:       acp.EventTypeToolCall,
				SessionID:  "sess-mixed",
				TurnID:     "turn-mixed",
				Timestamp:  timestamp.Add(2 * time.Second),
				Title:      "Read",
				ToolCallID: "tool-3",
				Raw:        json.RawMessage("{\"rawInput\":{\"file_path\":\"README.md\"}}"),
			}),
			mustUIAgentSessionEvent(t, "ev-text-4", 4, timestamp.Add(3*time.Second), acp.AgentEvent{
				Type:      acp.EventTypeAgentMessage,
				SessionID: "sess-mixed",
				TurnID:    "turn-mixed",
				Timestamp: timestamp.Add(3 * time.Second),
				Text:      "text 4",
			}),
			mustUIAgentSessionEvent(t, "ev-text-5", 5, timestamp.Add(4*time.Second), acp.AgentEvent{
				Type:      acp.EventTypeAgentMessage,
				SessionID: "sess-mixed",
				TurnID:    "turn-mixed",
				Timestamp: timestamp.Add(4 * time.Second),
				Text:      "text 5",
			}),
			mustUIAgentSessionEvent(t, "ev-done", 6, timestamp.Add(5*time.Second), acp.AgentEvent{
				Type:             acp.EventTypeDone,
				SessionID:        "sess-mixed",
				TurnID:           "turn-mixed",
				Timestamp:        timestamp.Add(5 * time.Second),
				StopReason:       string(acp.PromptStopReasonEndTurn),
				PromptStopReason: acp.PromptStopReasonEndTurn,
			}),
		}

		messages, err := ToUIMessages(events)
		if err != nil {
			t.Fatalf("ToUIMessages() error = %v", err)
		}
		if got, want := len(messages), 1; got != want {
			t.Fatalf("len(messages) = %d, want %d; messages=%#v", got, want, messages)
		}

		got := uiVisiblePartSignatures(messages[0].Parts)
		want := []string{
			"text:turn-mixed-text-1:text 1:done",
			"tool-Bash:tool-2:input-available",
			"tool-Read:tool-3:input-available",
			"text:turn-mixed-text-2:text 4text 5:done",
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("visible part signatures = %#v, want %#v; parts=%#v", got, want, messages[0].Parts)
		}
	})

	t.Run("ShouldPreserveReasoningAsASeparateOrderedPart", func(t *testing.T) {
		t.Parallel()

		timestamp := time.Date(2026, 5, 13, 11, 0, 0, 0, time.UTC)
		events := []store.SessionEvent{
			mustUIAgentSessionEvent(t, "ev-text-1", 1, timestamp, acp.AgentEvent{
				Type:      acp.EventTypeAgentMessage,
				SessionID: "sess-reasoning",
				TurnID:    "turn-reasoning",
				Timestamp: timestamp,
				Text:      "visible",
			}),
			mustUIAgentSessionEvent(t, "ev-thought-1", 2, timestamp.Add(time.Second), acp.AgentEvent{
				Type:      acp.EventTypeThought,
				SessionID: "sess-reasoning",
				TurnID:    "turn-reasoning",
				Timestamp: timestamp.Add(time.Second),
				Text:      "checking",
			}),
			mustUIAgentSessionEvent(t, "ev-text-2", 3, timestamp.Add(2*time.Second), acp.AgentEvent{
				Type:      acp.EventTypeAgentMessage,
				SessionID: "sess-reasoning",
				TurnID:    "turn-reasoning",
				Timestamp: timestamp.Add(2 * time.Second),
				Text:      "answer",
			}),
			mustUIAgentSessionEvent(t, "ev-done", 4, timestamp.Add(3*time.Second), acp.AgentEvent{
				Type:             acp.EventTypeDone,
				SessionID:        "sess-reasoning",
				TurnID:           "turn-reasoning",
				Timestamp:        timestamp.Add(3 * time.Second),
				StopReason:       string(acp.PromptStopReasonEndTurn),
				PromptStopReason: acp.PromptStopReasonEndTurn,
			}),
		}

		messages, err := ToUIMessages(events)
		if err != nil {
			t.Fatalf("ToUIMessages() error = %v", err)
		}
		if got, want := len(messages), 1; got != want {
			t.Fatalf("len(messages) = %d, want %d; messages=%#v", got, want, messages)
		}

		got := uiVisiblePartSignatures(messages[0].Parts)
		want := []string{
			"text:turn-reasoning-text-1:visible:done",
			"reasoning:turn-reasoning-reasoning-1:checking:done",
			"text:turn-reasoning-text-2:answer:done",
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("visible part signatures = %#v, want %#v; parts=%#v", got, want, messages[0].Parts)
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
		RequestID: "req-permission",
		Timestamp: timestamp,
		Title:     "Bash",
		Action:    "session/request_permission",
		Resource:  "Bash",
		Decision:  decision,
		Raw:       json.RawMessage(`{"command":"pwd"}`),
	})
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

func TestUnmarshalAgentEventRoundTripPreservesStructuredFieldsWithoutRaw(t *testing.T) {
	t.Run("Should round-trip structured fields without restoring canonical raw payloads", func(t *testing.T) {
		t.Parallel()

		payload, err := MarshalAgentEvent(acp.AgentEvent{
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
		})
		if err != nil {
			t.Fatalf("MarshalAgentEvent() error = %v", err)
		}

		event, err := UnmarshalAgentEvent(payload)
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

func mustMarshalCanonical(
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

	payload, err := canonicalPayload(
		eventType,
		turnID,
		timestamp,
		text,
		toolName,
		toolCallID,
		toolInput,
		toolResult,
		toolError,
	)
	if err != nil {
		t.Fatalf("canonicalPayload() error = %v", err)
	}
	return string(payload)
}
