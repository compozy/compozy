package contentblock

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestMarshalEnvelopeJSONValidatesRawPayloadShape(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		blockType string
		data      json.RawMessage
		wantErr   string
	}{
		{
			name:      "Should reject malformed JSON payloads",
			blockType: "text",
			data:      json.RawMessage(`{"type":"text"`),
			wantErr:   "invalid data",
		},
		{
			name:      "Should reject mismatched embedded types",
			blockType: "text",
			data:      json.RawMessage(`{"type":"tool_use","text":"hello"}`),
			wantErr:   `unexpected type "tool_use"`,
		},
		{
			name:      "Should preserve validated payloads",
			blockType: "text",
			data:      json.RawMessage(`{"type":"text","text":"hello"}`),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := MarshalEnvelopeJSON(tc.blockType, tc.data)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatal("expected validation error")
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("marshal envelope JSON: %v", err)
			}
			if string(got) != string(tc.data) {
				t.Fatalf("expected payload to be preserved\nwant: %s\ngot:  %s", tc.data, got)
			}
		})
	}
}

func TestUnmarshalEnvelopeJSONValidatesDecoderHooks(t *testing.T) {
	t.Parallel()

	t.Run("Should reject a missing validator", func(t *testing.T) {
		t.Parallel()

		_, err := UnmarshalEnvelopeJSON[string]([]byte(`{"type":"text","text":"hello"}`), nil)
		if err == nil {
			t.Fatal("expected missing validator error")
		}
		if !strings.Contains(err.Error(), "missing validator") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("Should wrap validator failures with the block type", func(t *testing.T) {
		t.Parallel()

		validateErr := errors.New("validator exploded")
		_, err := UnmarshalEnvelopeJSON[string](
			[]byte(`{"type":"text","text":"hello"}`),
			func(string, []byte) error {
				return validateErr
			},
		)
		if err == nil {
			t.Fatal("expected validator failure")
		}
		if !errors.Is(err, validateErr) {
			t.Fatalf("expected validator error to be wrapped, got %v", err)
		}
		if !strings.Contains(err.Error(), "decode text block") {
			t.Fatalf("expected block type context, got %v", err)
		}
	})
}

func TestDecodeBlockValidatesTypeExtractor(t *testing.T) {
	t.Parallel()

	t.Run("Should reject a missing type extractor", func(t *testing.T) {
		t.Parallel()

		_, err := DecodeBlock[map[string]string](
			[]byte(`{"type":"text","text":"hello"}`),
			"text",
			nil,
			nil,
		)
		if err == nil {
			t.Fatal("expected missing type extractor error")
		}
		if !strings.Contains(err.Error(), "missing type extractor") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
