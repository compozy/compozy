package kinds

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
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
	tests := []struct {
		name        string
		run         func() error
		wantMessage string
	}{
		{
			name: "nil payload",
			run: func() error {
				_, err := NewContentBlock(nil)
				return err
			},
			wantMessage: "marshal content block: nil payload",
		},
		{
			name: "nil typed pointer payload",
			run: func() error {
				_, err := NewContentBlock(nilText)
				return err
			},
			wantMessage: "marshal content block: nil *kinds.TextBlock",
		},
		{
			name: "unsupported payload type",
			run: func() error {
				_, err := NewContentBlock(struct{}{})
				return err
			},
			wantMessage: "marshal content block: unsupported payload type struct {}",
		},
		{
			name: "marshal missing type and data",
			run: func() error {
				_, err := (ContentBlock{}).MarshalJSON()
				return err
			},
			wantMessage: "marshal content block: missing type",
		},
		{
			name: "marshal missing data",
			run: func() error {
				_, err := (ContentBlock{Type: BlockText}).MarshalJSON()
				return err
			},
			wantMessage: "marshal text block: missing data",
		},
		{
			name: "unmarshal missing type",
			run: func() error {
				var missingType ContentBlock
				return json.Unmarshal([]byte(`{"text":"missing type"}`), &missingType)
			},
			wantMessage: "decode content block envelope: missing type",
		},
		{
			name: "unmarshal invalid type",
			run: func() error {
				var invalidType ContentBlock
				return json.Unmarshal([]byte(`{"type":"nope"}`), &invalidType)
			},
			wantMessage: `decode content block: unsupported type "nope"`,
		},
		{
			name: "validate invalid type",
			run: func() error {
				return validateContentBlock(ContentBlockType("invalid"), []byte(`{}`))
			},
			wantMessage: `decode content block: unsupported type "invalid"`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.run()
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantMessage) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantMessage)
			}
		})
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

func TestContentBlockMarshalUsesSnakeCaseJSONTags(t *testing.T) {
	t.Parallel()

	block, err := NewContentBlock(ToolResultBlock{
		ToolUseID: "tool-7",
		Content:   "ok",
		IsError:   true,
	})
	if err != nil {
		t.Fatalf("new content block: %v", err)
	}

	data, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("marshal content block: %v", err)
	}

	encoded := string(data)
	required := []string{`"tool_use_id":"tool-7"`, `"is_error":true`}
	for _, field := range required {
		if !strings.Contains(encoded, field) {
			t.Fatalf("expected snake_case field %q in %s", field, encoded)
		}
	}

	forbidden := []string{`"toolUseId"`, `"isError"`}
	for _, field := range forbidden {
		if strings.Contains(encoded, field) {
			t.Fatalf("did not expect camelCase field %q in %s", field, encoded)
		}
	}
}
