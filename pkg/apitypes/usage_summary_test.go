package apitypes

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUsageSummaryMarshalJSONEmptyEmitsArray(t *testing.T) {
	summary := &UsageSummary{}
	data, err := summary.MarshalJSON()
	require.NoError(t, err)
	require.Equal(t, []byte("[]"), data)
}

func TestUsageSummaryUnmarshalJSONTrimsWhitespace(t *testing.T) {
	summary := &UsageSummary{
		Entries: []UsageEntry{{
			Provider:         "openai",
			Model:            "gpt-4o-mini",
			PromptTokens:     1,
			CompletionTokens: 2,
			TotalTokens:      3,
			CapturedAt:       timePtr(time.Now()),
		}},
	}
	require.NoError(t, summary.UnmarshalJSON([]byte("  null  ")))
	require.Nil(t, summary.Entries)
}

func TestUsageSummaryUnmarshalJSONArray(t *testing.T) {
	input := []byte(`[{"provider":"openai","model":"gpt-4o","prompt_tokens":4,"completion_tokens":3,"total_tokens":7}]`)
	summary := &UsageSummary{}
	require.NoError(t, summary.UnmarshalJSON(input))
	require.Len(t, summary.Entries, 1)
	require.Equal(t, "openai", summary.Entries[0].Provider)
	require.Equal(t, 7, summary.Entries[0].TotalTokens)
}

func timePtr(value time.Time) *time.Time {
	return &value
}
