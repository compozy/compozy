package kinds

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

func TestUsageAddAndTotal(t *testing.T) {
	t.Parallel()

	usage := Usage{InputTokens: 2, OutputTokens: 3}
	usage.Add(Usage{InputTokens: 4, OutputTokens: 5, CacheReads: 1, CacheWrites: 2})

	if usage.InputTokens != 6 {
		t.Fatalf("unexpected input tokens: %d", usage.InputTokens)
	}
	if usage.OutputTokens != 8 {
		t.Fatalf("unexpected output tokens: %d", usage.OutputTokens)
	}
	if usage.CacheReads != 1 {
		t.Fatalf("unexpected cache reads: %d", usage.CacheReads)
	}
	if usage.CacheWrites != 2 {
		t.Fatalf("unexpected cache writes: %d", usage.CacheWrites)
	}
	if got := usage.Total(); got != 14 {
		t.Fatalf("unexpected derived total: %d", got)
	}

	usage.TotalTokens = 99
	if got := usage.Total(); got != 99 {
		t.Fatalf("unexpected explicit total: %d", got)
	}
}

func TestContentBlocksRoundTripForAllTypes(t *testing.T) {
	t.Parallel()

	oldText := "old"
	uri := "https://example.com/image.png"
	cases := []struct {
		name       string
		block      any
		decodeType any
	}{
		{name: "text", block: TextBlock{Text: "hello"}, decodeType: TextBlock{}},
		{
			name: "tool use",
			block: ToolUseBlock{
				ID:       "tool-1",
				Name:     "shell",
				Title:    "Shell",
				ToolName: "exec",
				Input:    json.RawMessage(`{"cmd":"echo hi"}`),
				RawInput: json.RawMessage(`{"cmd":"echo hi","cwd":"/repo"}`),
			},
			decodeType: ToolUseBlock{},
		},
		{
			name:       "tool result",
			block:      ToolResultBlock{ToolUseID: "tool-1", Content: "ok", IsError: true},
			decodeType: ToolResultBlock{},
		},
		{
			name: "diff",
			block: DiffBlock{
				FilePath: "pkg/compozy/events/bus.go",
				Diff:     "@@ -1 +1 @@",
				OldText:  &oldText,
				NewText:  "new",
			},
			decodeType: DiffBlock{},
		},
		{
			name:       "terminal output",
			block:      TerminalOutputBlock{Command: "make verify", Output: "ok", ExitCode: 0, TerminalID: "term-1"},
			decodeType: TerminalOutputBlock{},
		},
		{
			name:       "image",
			block:      ImageBlock{Data: "data:image/png;base64,AA==", MimeType: "image/png", URI: &uri},
			decodeType: ImageBlock{},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			content, err := NewContentBlock(tc.block)
			if err != nil {
				t.Fatalf("new content block: %v", err)
			}

			data, err := json.Marshal(content)
			if err != nil {
				t.Fatalf("marshal content block: %v", err)
			}

			var decoded ContentBlock
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("unmarshal content block: %v", err)
			}

			if decoded.Type != content.Type {
				t.Fatalf("unexpected block type: %q", decoded.Type)
			}
			if !bytes.Equal(decoded.Data, content.Data) {
				t.Fatalf("unexpected block payload: %s", string(decoded.Data))
			}

			value, err := decoded.Decode()
			if err != nil {
				t.Fatalf("decode content block: %v", err)
			}
			if reflect.TypeOf(value) != reflect.TypeOf(tc.decodeType) {
				t.Fatalf("unexpected decoded type: %T", value)
			}

			switch tc.block.(type) {
			case TextBlock:
				if _, err := decoded.AsText(); err != nil {
					t.Fatalf("decode as text: %v", err)
				}
			case ToolUseBlock:
				if _, err := decoded.AsToolUse(); err != nil {
					t.Fatalf("decode as tool use: %v", err)
				}
			case ToolResultBlock:
				if _, err := decoded.AsToolResult(); err != nil {
					t.Fatalf("decode as tool result: %v", err)
				}
			case DiffBlock:
				if _, err := decoded.AsDiff(); err != nil {
					t.Fatalf("decode as diff: %v", err)
				}
			case TerminalOutputBlock:
				if _, err := decoded.AsTerminalOutput(); err != nil {
					t.Fatalf("decode as terminal output: %v", err)
				}
			case ImageBlock:
				if _, err := decoded.AsImage(); err != nil {
					t.Fatalf("decode as image: %v", err)
				}
			}
		})
	}
}

func TestContentBlockValidationErrors(t *testing.T) {
	t.Parallel()

	var nilText *TextBlock
	if _, err := NewContentBlock(nil); err == nil {
		t.Fatal("expected nil block error")
	}
	if _, err := NewContentBlock(nilText); err == nil {
		t.Fatal("expected nil pointer block error")
	}
	if _, err := NewContentBlock(struct{}{}); err == nil {
		t.Fatal("expected unsupported block error")
	}

	if _, err := (ContentBlock{}).MarshalJSON(); err == nil {
		t.Fatal("expected marshal error for missing type and data")
	}
	if _, err := (ContentBlock{Type: BlockText}).MarshalJSON(); err == nil {
		t.Fatal("expected marshal error for missing data")
	}

	var missingType ContentBlock
	if err := json.Unmarshal([]byte(`{"text":"missing type"}`), &missingType); err == nil {
		t.Fatal("expected missing type error")
	}

	var invalidType ContentBlock
	if err := json.Unmarshal([]byte(`{"type":"nope"}`), &invalidType); err == nil {
		t.Fatal("expected unsupported type error")
	}

	if err := validateContentBlock(ContentBlockType("invalid"), []byte(`{}`)); err == nil {
		t.Fatal("expected decode error for invalid type")
	}
}

func TestSessionUpdateRoundTripsJSON(t *testing.T) {
	t.Parallel()

	block, err := NewContentBlock(TextBlock{Text: "hello"})
	if err != nil {
		t.Fatalf("create text block: %v", err)
	}

	update := SessionUpdate{
		Kind:          UpdateKindAgentMessageChunk,
		ToolCallID:    "tool-1",
		ToolCallState: ToolCallStateCompleted,
		Blocks:        []ContentBlock{block},
		ThoughtBlocks: []ContentBlock{block},
		PlanEntries:   []SessionPlanEntry{{Content: "finish task", Priority: "high", Status: "done"}},
		AvailableCommands: []SessionAvailableCommand{
			{Name: "/help", Description: "Show help", ArgumentHint: "[topic]"},
		},
		CurrentModeID: "default",
		Usage:         Usage{InputTokens: 2, OutputTokens: 3, TotalTokens: 5},
		Status:        StatusCompleted,
	}

	data, err := json.Marshal(update)
	if err != nil {
		t.Fatalf("marshal update: %v", err)
	}

	var decoded SessionUpdate
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal update: %v", err)
	}

	roundTrip, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("marshal round-tripped update: %v", err)
	}
	if !bytes.Equal(data, roundTrip) {
		t.Fatalf("update changed after round trip:\noriginal: %s\nroundtrip: %s", string(data), string(roundTrip))
	}
}
