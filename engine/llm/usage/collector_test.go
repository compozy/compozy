package usage_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm/usage"
)

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

func TestCollector_FinalizeAggregatesSummary(t *testing.T) {
	t.Parallel()

	meta := usage.Metadata{
		Component:      core.ComponentAgent,
		TaskExecID:     core.MustNewID(),
		WorkflowExecID: core.MustNewID(),
	}
	metrics := &stubMetrics{}
	collector := usage.NewCollector(metrics, meta)
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

	finalized, err := collector.Finalize(ctx, core.StatusSuccess)
	require.NoError(t, err)
	require.NotNil(t, finalized)
	require.Equal(t, meta, finalized.Metadata)
	require.NotNil(t, finalized.Summary)
	require.Len(t, finalized.Summary.Entries, 1)

	entry := finalized.Summary.Entries[0]
	assert.Equal(t, "openai", entry.Provider)
	assert.Equal(t, "gpt-4o-mini", entry.Model)
	assert.Equal(t, 16, entry.PromptTokens)
	assert.Equal(t, 12, entry.CompletionTokens)
	assert.Equal(t, 29, entry.TotalTokens)
	require.NotNil(t, entry.ReasoningTokens)
	assert.Equal(t, 10, *entry.ReasoningTokens)
	require.NotNil(t, entry.CachedPromptTokens)
	assert.Equal(t, cached, *entry.CachedPromptTokens)
	require.NotNil(t, entry.InputAudioTokens)
	assert.Equal(t, inputAudio, *entry.InputAudioTokens)
	require.NotNil(t, entry.OutputAudioTokens)
	assert.Equal(t, outputAudio, *entry.OutputAudioTokens)
	assert.Equal(t, string(usage.SourceTask), entry.Source)

	assert.Equal(t, 1, metrics.successes)
	assert.Equal(t, 0, metrics.failures)
	assert.Equal(t, core.ComponentAgent, metrics.last.component)
	assert.Equal(t, "openai", metrics.last.provider)
	assert.Equal(t, "gpt-4o-mini", metrics.last.model)
	assert.Equal(t, 16, metrics.last.prompt)
	assert.Equal(t, 12, metrics.last.completion)

	// Collector should reset internal state after finalize.
	repeat, repeatErr := collector.Finalize(ctx, core.StatusSuccess)
	require.NoError(t, repeatErr)
	assert.Nil(t, repeat)
}

func TestCollector_FinalizeWithoutSnapshotsReturnsNil(t *testing.T) {
	t.Parallel()

	collector := usage.NewCollector(&stubMetrics{}, usage.Metadata{
		Component: core.ComponentTask,
	})
	require.NotNil(t, collector)

	finalized, err := collector.Finalize(context.Background(), core.StatusSuccess)
	require.NoError(t, err)
	assert.Nil(t, finalized)
}

func TestCollector_FinalizeRecordsFailureMetricForNonSuccessStatus(t *testing.T) {
	t.Parallel()

	metrics := &stubMetrics{}
	collector := usage.NewCollector(metrics, usage.Metadata{
		Component: core.ComponentWorkflow,
	})
	require.NotNil(t, collector)

	collector.Record(context.Background(), &usage.Snapshot{
		Provider:         "anthropic",
		Model:            "claude-3",
		PromptTokens:     5,
		CompletionTokens: 2,
	})

	finalized, err := collector.Finalize(context.Background(), core.StatusFailed)
	require.NoError(t, err)
	require.NotNil(t, finalized)
	assert.Equal(t, 1, metrics.successes)
	assert.Equal(t, 1, metrics.failures)
	assert.Equal(t, core.ComponentWorkflow, metrics.last.component)
	assert.Equal(t, "anthropic", metrics.last.provider)
	assert.Equal(t, "claude-3", metrics.last.model)
}
