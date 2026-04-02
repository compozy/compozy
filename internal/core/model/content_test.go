package model_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/core/model"
)

func TestContentBlockRoundTrip(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		payload any
		assert  func(t *testing.T, block model.ContentBlock)
	}{
		{
			name:    "text",
			payload: model.TextBlock{Text: "hello"},
			assert: func(t *testing.T, block model.ContentBlock) {
				t.Helper()
				got, err := block.AsText()
				if err != nil {
					t.Fatalf("decode text block: %v", err)
				}
				want := model.TextBlock{Type: model.BlockText, Text: "hello"}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("unexpected text block: %#v", got)
				}
			},
		},
		{
			name: "tool_use",
			payload: model.ToolUseBlock{
				ID:    "tool-1",
				Name:  "read_file",
				Input: json.RawMessage(`{"path":"README.md"}`),
			},
			assert: func(t *testing.T, block model.ContentBlock) {
				t.Helper()
				got, err := block.AsToolUse()
				if err != nil {
					t.Fatalf("decode tool use block: %v", err)
				}
				want := model.ToolUseBlock{
					Type:  model.BlockToolUse,
					ID:    "tool-1",
					Name:  "read_file",
					Input: json.RawMessage(`{"path":"README.md"}`),
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("unexpected tool use block: %#v", got)
				}
			},
		},
		{
			name:    "tool_result",
			payload: model.ToolResultBlock{ToolUseID: "tool-1", Content: "done", IsError: true},
			assert: func(t *testing.T, block model.ContentBlock) {
				t.Helper()
				got, err := block.AsToolResult()
				if err != nil {
					t.Fatalf("decode tool result block: %v", err)
				}
				want := model.ToolResultBlock{
					Type:      model.BlockToolResult,
					ToolUseID: "tool-1",
					Content:   "done",
					IsError:   true,
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("unexpected tool result block: %#v", got)
				}
			},
		},
		{
			name:    "diff",
			payload: model.DiffBlock{FilePath: "README.md", Diff: "--- old\n+++ new", NewText: "new"},
			assert: func(t *testing.T, block model.ContentBlock) {
				t.Helper()
				got, err := block.AsDiff()
				if err != nil {
					t.Fatalf("decode diff block: %v", err)
				}
				want := model.DiffBlock{
					Type:     model.BlockDiff,
					FilePath: "README.md",
					Diff:     "--- old\n+++ new",
					NewText:  "new",
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("unexpected diff block: %#v", got)
				}
			},
		},
		{
			name:    "terminal_output",
			payload: model.TerminalOutputBlock{Command: "go test", Output: "ok", ExitCode: 0, TerminalID: "term-1"},
			assert: func(t *testing.T, block model.ContentBlock) {
				t.Helper()
				got, err := block.AsTerminalOutput()
				if err != nil {
					t.Fatalf("decode terminal output block: %v", err)
				}
				want := model.TerminalOutputBlock{
					Type:       model.BlockTerminalOutput,
					Command:    "go test",
					Output:     "ok",
					ExitCode:   0,
					TerminalID: "term-1",
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("unexpected terminal output block: %#v", got)
				}
			},
		},
		{
			name:    "image",
			payload: model.ImageBlock{Data: "ZmFrZQ==", MimeType: "image/png"},
			assert: func(t *testing.T, block model.ContentBlock) {
				t.Helper()
				got, err := block.AsImage()
				if err != nil {
					t.Fatalf("decode image block: %v", err)
				}
				want := model.ImageBlock{
					Type:     model.BlockImage,
					Data:     "ZmFrZQ==",
					MimeType: "image/png",
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("unexpected image block: %#v", got)
				}
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			block, err := model.NewContentBlock(tc.payload)
			if err != nil {
				t.Fatalf("new content block: %v", err)
			}

			encoded, err := json.Marshal(block)
			if err != nil {
				t.Fatalf("marshal content block: %v", err)
			}

			var decoded model.ContentBlock
			if err := json.Unmarshal(encoded, &decoded); err != nil {
				t.Fatalf("unmarshal content block: %v", err)
			}

			if decoded.Type != block.Type {
				t.Fatalf("unexpected decoded type: got %q want %q", decoded.Type, block.Type)
			}

			tc.assert(t, decoded)

			payload, err := decoded.Decode()
			if err != nil {
				t.Fatalf("decode generic payload: %v", err)
			}
			if payload == nil {
				t.Fatal("expected decoded payload")
			}
		})
	}
}

func TestContentBlockValidJSONDecodesTypedStructs(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"type":"tool_use","id":"tool-7","name":"write_file","input":{"path":"main.go"}}`)

	var block model.ContentBlock
	if err := json.Unmarshal(raw, &block); err != nil {
		t.Fatalf("unmarshal content block: %v", err)
	}

	toolUse, err := block.AsToolUse()
	if err != nil {
		t.Fatalf("decode tool use block: %v", err)
	}
	if toolUse.ID != "tool-7" {
		t.Fatalf("unexpected tool id: %q", toolUse.ID)
	}
	if string(toolUse.Input) != `{"path":"main.go"}` {
		t.Fatalf("unexpected tool input: %s", string(toolUse.Input))
	}
}

func TestContentBlockMalformedJSONReturnsDescriptiveError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{
			name:    "missing type",
			raw:     `{"text":"hello"}`,
			wantErr: "missing type",
		},
		{
			name:    "invalid payload",
			raw:     `{"type":"text","text":123}`,
			wantErr: "decode text block",
		},
		{
			name:    "unsupported type",
			raw:     `{"type":"resource"}`,
			wantErr: "unsupported type",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var block model.ContentBlock
			err := json.Unmarshal([]byte(tc.raw), &block)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("unexpected error %q", err)
			}
		})
	}
}

func TestUsageAdd(t *testing.T) {
	t.Parallel()

	var usage model.Usage
	usage.Add(model.Usage{
		InputTokens:  10,
		OutputTokens: 4,
		TotalTokens:  14,
		CacheReads:   2,
		CacheWrites:  1,
	})
	usage.Add(model.Usage{
		InputTokens:  3,
		OutputTokens: 7,
		TotalTokens:  10,
		CacheReads:   5,
		CacheWrites:  6,
	})

	want := model.Usage{
		InputTokens:  13,
		OutputTokens: 11,
		TotalTokens:  24,
		CacheReads:   7,
		CacheWrites:  7,
	}
	if !reflect.DeepEqual(usage, want) {
		t.Fatalf("unexpected usage: %#v", usage)
	}
}

func TestContentBlockConstructorErrorsAndPointerVariants(t *testing.T) {
	t.Parallel()

	text := &model.TextBlock{Text: "pointer"}
	block, err := model.NewContentBlock(text)
	if err != nil {
		t.Fatalf("new content block from pointer: %v", err)
	}
	decoded, err := block.AsText()
	if err != nil {
		t.Fatalf("decode pointer text block: %v", err)
	}
	if decoded.Text != "pointer" {
		t.Fatalf("unexpected decoded text: %q", decoded.Text)
	}

	if _, err := model.NewContentBlock((*model.TextBlock)(nil)); err == nil {
		t.Fatal("expected nil pointer error")
	}
	if _, err := model.NewContentBlock(struct{}{}); err == nil {
		t.Fatal("expected unsupported payload error")
	}
}

func TestContentBlockMarshalErrors(t *testing.T) {
	t.Parallel()

	if _, err := json.Marshal(model.ContentBlock{}); err == nil {
		t.Fatal("expected missing type error")
	}

	if _, err := json.Marshal(model.ContentBlock{Type: model.BlockText}); err == nil {
		t.Fatal("expected missing data error")
	}
}

func TestContentBlockPointerVariants(t *testing.T) {
	t.Parallel()

	cases := []any{
		&model.ToolUseBlock{ID: "tool-1", Name: "read"},
		&model.ToolResultBlock{ToolUseID: "tool-1", Content: "ok"},
		&model.DiffBlock{FilePath: "README.md", Diff: "diff"},
		&model.TerminalOutputBlock{Command: "go test", Output: "ok"},
		&model.ImageBlock{Data: "ZmFrZQ==", MimeType: "image/png"},
	}

	for _, payload := range cases {
		if _, err := model.NewContentBlock(payload); err != nil {
			t.Fatalf("new content block from %T: %v", payload, err)
		}
	}
}
