package agent

import (
	"testing"

	acp "github.com/coder/acp-go-sdk"
)

func TestNormalizeACPToolName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		kind  acp.ToolKind
		input map[string]any
		want  string
	}{
		{
			name: "Should treat ref-only input as open URL",
			kind: acp.ToolKindSearch,
			input: map[string]any{
				"ref_id": "turn0search0",
			},
			want: toolNameOpenURL,
		},
		{
			name: "Should treat url-only search input as web fetch",
			kind: acp.ToolKindSearch,
			input: map[string]any{
				"url": "https://example.com",
			},
			want: toolNameWebFetch,
		},
		{
			name: "Should treat search query input as web search",
			kind: acp.ToolKindSearch,
			input: map[string]any{
				"search_query": []map[string]any{
					{"q": "agent client protocol docs"},
				},
			},
			want: toolNameWebSearch,
		},
		{
			name: "Should keep click precedence over ref-only open",
			kind: acp.ToolKindSearch,
			input: map[string]any{
				"ref_id": "turn0search0",
				"id":     17,
			},
			want: toolNameClick,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := normalizeACPToolName("", "", tt.kind, tt.input); got != tt.want {
				t.Fatalf("normalizeACPToolName() = %q, want %q", got, tt.want)
			}
		})
	}
}
