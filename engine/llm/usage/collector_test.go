package usage_test

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm/usage"
	"github.com/stretchr/testify/require"
)

type stubRepo struct {
	rows []*usage.Row
	err  error
}

func (s *stubRepo) Upsert(_ context.Context, row *usage.Row) error {
	if s.err != nil {
		return s.err
	}
	s.rows = append(s.rows, row)
	return nil
}

func (s *stubRepo) GetByTaskExecID(context.Context, core.ID) (*usage.Row, error) {
	panic("not implemented")
}

func (s *stubRepo) GetByWorkflowExecID(context.Context, core.ID) (*usage.Row, error) {
	panic("not implemented")
}

func (s *stubRepo) SummarizeByWorkflowExecID(context.Context, core.ID) (*usage.Row, error) {
	return nil, usage.ErrNotFound
}

func TestCollector_RecordAndFinalize(t *testing.T) {
	t.Parallel()

	repo := &stubRepo{}
	meta := usage.Metadata{
		Component:      core.ComponentAgent,
		WorkflowExecID: core.MustNewID(),
		TaskExecID:     core.MustNewID(),
	}
	collector := usage.NewCollector(repo, meta)
	require.NotNil(t, collector)

	ctx := context.Background()
	ctx = usage.ContextWithCollector(ctx, collector)

	firstReasoning := 7
	secondReasoning := 3
	cached := 2
	inputAudio := 5
	outputAudio := 9

	collector.Record(ctx, &usage.Snapshot{
		Provider:         "openai",
		Model:            "gpt-4o-mini",
		PromptTokens:     10,
		CompletionTokens: 4,
		TotalTokens:      14,
		ReasoningTokens:  &firstReasoning,
	})
	collector.Record(ctx, &usage.Snapshot{
		Provider:           "openai",
		Model:              "gpt-4o-mini",
		PromptTokens:       6,
		CompletionTokens:   8,
		TotalTokens:        15,
		ReasoningTokens:    &secondReasoning,
		CachedPromptTokens: &cached,
		InputAudioTokens:   &inputAudio,
		OutputAudioTokens:  &outputAudio,
	})

	err := collector.Finalize(ctx, core.StatusSuccess)
	require.NoError(t, err)

	require.Len(t, repo.rows, 1)
	row := repo.rows[0]
	require.Equal(t, "openai", row.Provider)
	require.Equal(t, "gpt-4o-mini", row.Model)
	require.Equal(t, 16, row.PromptTokens)
	require.Equal(t, 12, row.CompletionTokens)
	require.Equal(t, 29, row.TotalTokens)

	require.NotNil(t, row.ReasoningTokens)
	require.Equal(t, 10, *row.ReasoningTokens)
	require.NotNil(t, row.CachedPromptTokens)
	require.Equal(t, cached, *row.CachedPromptTokens)
	require.NotNil(t, row.InputAudioTokens)
	require.Equal(t, inputAudio, *row.InputAudioTokens)
	require.NotNil(t, row.OutputAudioTokens)
	require.Equal(t, outputAudio, *row.OutputAudioTokens)
}

func TestCollector_FinalizeWithoutProviderSkipsPersistence(t *testing.T) {
	t.Parallel()

	repo := &stubRepo{}
	collector := usage.NewCollector(repo, usage.Metadata{
		Component: core.ComponentTask,
	})
	require.NotNil(t, collector)

	err := collector.Finalize(context.Background(), core.StatusFailed)
	require.NoError(t, err)
	require.Empty(t, repo.rows)
}
