package usage

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSummaryValidateSuccess(t *testing.T) {
	t.Run("Should normalize valid entry", func(t *testing.T) {
		now := time.Now().UTC()
		summary := &Summary{
			Entries: []Entry{{
				Provider:           " openai ",
				Model:              " gpt-4o-mini ",
				PromptTokens:       10,
				CompletionTokens:   5,
				TotalTokens:        0,
				ReasoningTokens:    intPtr(1),
				CachedPromptTokens: intPtr(2),
				AgentIDs:           []string{" agent-a ", "agent-a"},
				CapturedAt:         &now,
			}},
		}

		require.NoError(t, summary.Validate())

		entry := summary.Entries[0]
		require.Equal(t, "openai", entry.Provider)
		require.Equal(t, "gpt-4o-mini", entry.Model)
		require.Equal(t, 15, entry.TotalTokens)
		require.Equal(t, []string{"agent-a"}, entry.AgentIDs)
		require.NotZero(t, entry.CapturedAt)
	})
}

func TestSummaryValidateErrors(t *testing.T) {
	tests := []struct {
		name      string
		entry     Entry
		wantError string
	}{
		{
			name: "Should fail when provider missing",
			entry: Entry{
				Model:            "valid",
				PromptTokens:     1,
				CompletionTokens: 2,
			},
			wantError: "provider is required",
		},
		{
			name: "Should fail when model missing",
			entry: Entry{
				Provider:         "openai",
				PromptTokens:     1,
				CompletionTokens: 2,
			},
			wantError: "model is required",
		},
		{
			name: "Should fail when prompt tokens negative",
			entry: Entry{
				Provider:         "openai",
				Model:            "gpt-4o-mini",
				PromptTokens:     -1,
				CompletionTokens: 2,
			},
			wantError: "prompt_tokens must be non-negative",
		},
		{
			name: "Should fail when total tokens negative",
			entry: Entry{
				Provider:         "openai",
				Model:            "gpt-4o-mini",
				PromptTokens:     1,
				CompletionTokens: 2,
				TotalTokens:      -3,
			},
			wantError: "total_tokens must be non-negative",
		},
		{
			name: "Should fail when reasoning tokens negative",
			entry: Entry{
				Provider:         "openai",
				Model:            "gpt-4o-mini",
				PromptTokens:     1,
				CompletionTokens: 2,
				ReasoningTokens:  intPtr(-1),
			},
			wantError: "reasoning_tokens must be non-negative",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			summary := &Summary{Entries: []Entry{tc.entry}}
			err := summary.Validate()
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantError)
		})
	}
}

func intPtr(v int) *int {
	return &v
}
