package usage_test

import (
	"context"
	"errors"
	"testing"
	"time"

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

func (s *stubRepo) SummariesByWorkflowExecIDs(
	context.Context,
	[]core.ID,
) (map[core.ID]*usage.Row, error) {
	return map[core.ID]*usage.Row{}, nil
}

type stubMetrics struct {
	successes int
	failures  int
	last      struct {
		component  core.ComponentType
		provider   string
		model      string
		prompt     int
		completion int
		latency    time.Duration
	}
}

func (m *stubMetrics) RecordSuccess(
	_ context.Context,
	component core.ComponentType,
	provider string,
	model string,
	promptTokens int,
	completionTokens int,
	latency time.Duration,
) {
	m.successes++
	m.last.component = component
	m.last.provider = provider
	m.last.model = model
	m.last.prompt = promptTokens
	m.last.completion = completionTokens
	m.last.latency = latency
}

func (m *stubMetrics) RecordFailure(
	_ context.Context,
	component core.ComponentType,
	provider string,
	model string,
	latency time.Duration,
) {
	m.failures++
	m.last.component = component
	m.last.provider = provider
	m.last.model = model
	m.last.latency = latency
}

func TestCollector_Finalize(t *testing.T) {
	t.Parallel()

	t.Run("Should record and finalize usage snapshot aggregate", func(t *testing.T) {
		t.Parallel()
		repo := &stubRepo{}
		metrics := &stubMetrics{}
		meta := usage.Metadata{
			Component:      core.ComponentAgent,
			WorkflowExecID: core.MustNewID(),
			TaskExecID:     core.MustNewID(),
		}
		collector := usage.NewCollector(repo, metrics, meta)
		require.NotNil(t, collector)

		ctx := usage.ContextWithCollector(context.Background(), collector)

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

		require.Equal(t, 1, metrics.successes)
		require.Equal(t, 0, metrics.failures)
		require.Equal(t, "openai", metrics.last.provider)
		require.Equal(t, "gpt-4o-mini", metrics.last.model)
		require.Equal(t, 16, metrics.last.prompt)
		require.Equal(t, 12, metrics.last.completion)
		require.Equal(t, core.ComponentAgent, metrics.last.component)
	})

	t.Run("Should skip persistence when required identifiers missing", func(t *testing.T) {
		t.Parallel()
		repo := &stubRepo{}
		metrics := &stubMetrics{}
		collector := usage.NewCollector(repo, metrics, usage.Metadata{
			Component: core.ComponentTask,
		})
		require.NotNil(t, collector)

		err := collector.Finalize(context.Background(), core.StatusFailed)
		require.NoError(t, err)
		require.Empty(t, repo.rows)
		require.Zero(t, metrics.successes)
		require.Zero(t, metrics.failures)
	})

	t.Run("Should record failure metric when persistence fails", func(t *testing.T) {
		t.Parallel()
		repo := &stubRepo{err: errors.New("boom")}
		metrics := &stubMetrics{}
		meta := usage.Metadata{
			Component: core.ComponentWorkflow,
		}
		collector := usage.NewCollector(repo, metrics, meta)
		require.NotNil(t, collector)

		collector.Record(context.Background(), &usage.Snapshot{
			Provider: "openai",
			Model:    "gpt-4o-mini",
		})

		err := collector.Finalize(context.Background(), core.StatusFailed)
		require.Error(t, err)
		require.Equal(t, 0, metrics.successes)
		require.Equal(t, 1, metrics.failures)
		require.Equal(t, core.ComponentWorkflow, metrics.last.component)
	})
}
