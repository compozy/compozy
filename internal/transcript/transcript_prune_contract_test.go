package transcript

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
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
				Content: mustMarshalRuntimeEvent(
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
