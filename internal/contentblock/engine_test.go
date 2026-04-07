package contentblock

import (
	"encoding/json"
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
