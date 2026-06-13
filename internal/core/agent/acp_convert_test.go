package agent

import (
	"testing"

	acp "github.com/coder/acp-go-sdk"

	"github.com/compozy/compozy/internal/core/model"
)

func TestConvertACPUsage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input acp.Usage
		want  model.Usage
	}{
		{
			name: "all fields populated including thought tokens",
			input: acp.Usage{
				InputTokens:       100,
				OutputTokens:      50,
				TotalTokens:       150,
				CachedReadTokens:  acp.Ptr(20),
				CachedWriteTokens: acp.Ptr(5),
				ThoughtTokens:     acp.Ptr(10),
			},
			want: model.Usage{
				InputTokens:  100,
				OutputTokens: 60,
				TotalTokens:  150,
				CacheReads:   20,
				CacheWrites:  5,
			},
		},
		{
			name: "no thought tokens",
			input: acp.Usage{
				InputTokens:  100,
				OutputTokens: 50,
				TotalTokens:  150,
			},
			want: model.Usage{
				InputTokens:  100,
				OutputTokens: 50,
				TotalTokens:  150,
			},
		},
		{
			name:  "zero value usage",
			input: acp.Usage{},
			want:  model.Usage{},
		},
		{
			name: "cache fields only",
			input: acp.Usage{
				CachedReadTokens:  acp.Ptr(10),
				CachedWriteTokens: acp.Ptr(3),
			},
			want: model.Usage{
				CacheReads:  10,
				CacheWrites: 3,
			},
		},
		{
			name: "thought tokens only with no base output tokens",
			input: acp.Usage{
				ThoughtTokens: acp.Ptr(15),
			},
			want: model.Usage{
				OutputTokens: 15,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := convertACPUsage(tt.input)
			t.Logf(
				"input:  InputTokens=%d OutputTokens=%d TotalTokens=%d ThoughtTokens=%v CacheReads=%v CacheWrites=%v",
				tt.input.InputTokens,
				tt.input.OutputTokens,
				tt.input.TotalTokens,
				tt.input.ThoughtTokens,
				tt.input.CachedReadTokens,
				tt.input.CachedWriteTokens,
			)
			t.Logf("output: InputTokens=%d OutputTokens=%d TotalTokens=%d CacheReads=%d CacheWrites=%d",
				got.InputTokens, got.OutputTokens, got.TotalTokens, got.CacheReads, got.CacheWrites)
			if got != tt.want {
				t.Errorf("convertACPUsage(%+v) = %+v, want %+v", tt.input, got, tt.want)
			}
		})
	}
}

func TestConvertACPAvailableCommandsArgumentHintNilGuards(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		commands []acp.AvailableCommand
		wantHint string
	}{
		{
			name: "nil Input field leaves ArgumentHint empty",
			commands: []acp.AvailableCommand{
				{Name: "run", Description: "Run task", Input: nil},
			},
			wantHint: "",
		},
		{
			name: "non-nil Input but nil Unstructured leaves ArgumentHint empty",
			commands: []acp.AvailableCommand{
				{Name: "run", Description: "Run task", Input: &acp.AvailableCommandInput{Unstructured: nil}},
			},
			wantHint: "",
		},
		{
			name: "non-nil Unstructured populates ArgumentHint",
			commands: []acp.AvailableCommand{
				{
					Name:        "run",
					Description: "Run task",
					Input: &acp.AvailableCommandInput{
						Unstructured: &acp.UnstructuredCommandInput{Hint: "--fast"},
					},
				},
			},
			wantHint: "--fast",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := convertACPAvailableCommands(tt.commands)
			if len(got) != 1 {
				t.Fatalf("convertACPAvailableCommands() len = %d, want 1", len(got))
			}
			if got[0].ArgumentHint != tt.wantHint {
				t.Errorf("ArgumentHint = %q, want %q", got[0].ArgumentHint, tt.wantHint)
			}
		})
	}
}
