package memory

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
)

type recordingLogger struct {
	debugMessages []string
}

func (l *recordingLogger) Debug(msg string, _ ...any) {
	l.debugMessages = append(l.debugMessages, msg)
}

func (l *recordingLogger) Info(string, ...any)  {}
func (l *recordingLogger) Warn(string, ...any)  {}
func (l *recordingLogger) Error(string, ...any) {}
func (l *recordingLogger) With(...any) logger.Logger {
	return l
}

func TestNewInitializesDefaults(t *testing.T) {
	t.Parallel()

	builder := New("  conversation-memory  ")
	require.NotNil(t, builder)
	require.Equal(t, "conversation-memory", builder.config.ID)
	require.Equal(t, memcore.TokenBasedMemory, builder.config.Type)
	require.Equal(t, memcore.InMemoryPersistence, builder.config.Persistence.Type)
}

func TestBuildValidConfig(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	builder := New("support-memory").
		WithProvider("OpenAI").
		WithModel("gpt-4o").
		WithMaxTokens(2000).
		WithFIFOFlush(25)

	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "support-memory", cfg.ID)
	require.Equal(t, 2000, cfg.MaxTokens)
	require.Equal(t, 25, cfg.MaxMessages)
	require.NotNil(t, cfg.Flushing)
	require.Equal(t, memcore.SimpleFIFOFlushing, cfg.Flushing.Type)
	require.NotNil(t, cfg.TokenProvider)
	require.Equal(t, "openai", cfg.TokenProvider.Provider)
	require.Equal(t, "gpt-4o", cfg.TokenProvider.Model)
}

func TestWithFlushStrategyCopiesValue(t *testing.T) {
	t.Parallel()

	original := FlushStrategy{Kind: FlushStrategyFIFO, MaxMessages: 10}
	builder := New("copy-test")
	builder.WithFlushStrategy(original)
	original.MaxMessages = 1
	require.Equal(t, 10, builder.flushStrategy.MaxMessages)
}

func TestBuildEmptyIDReturnsError(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	builder := New("   ").
		WithProvider("openai").
		WithModel("gpt-4o").
		WithMaxTokens(1000)

	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.Len(t, buildErr.Errors, 1)
}

func TestBuildEmptyProviderReturnsError(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	builder := New("memory-id").
		WithProvider("   ").
		WithModel("gpt-4").
		WithMaxTokens(1500)

	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.Contains(t, buildErr.Error(), "provider")
}

func TestBuildEmptyModelReturnsError(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	builder := New("memory-id").
		WithProvider("openai").
		WithModel("   ").
		WithMaxTokens(1500)

	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.Contains(t, buildErr.Error(), "model")
}

func TestBuildInvalidMaxTokensReturnsError(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	builder := New("memory-id").
		WithProvider("openai").
		WithModel("gpt-4").
		WithMaxTokens(0)

	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.Contains(t, buildErr.Error(), "max tokens")
}

func TestWithFIFOFlushInvalidMessages(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	builder := New("memory-id").
		WithProvider("openai").
		WithModel("gpt-4").
		WithMaxTokens(1000).
		WithFIFOFlush(0)

	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.Contains(t, buildErr.Error(), "fifo flush requires")
}

func TestBuildSummarizationFlushConfig(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	builder := New("memory-id").
		WithProvider("openai").
		WithModel("gpt-4o").
		WithMaxTokens(1500).
		WithSummarizationFlush("OpenAI", "gpt-4o-mini", 600)

	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.NotNil(t, cfg.Flushing)
	require.Equal(t, memcore.HybridSummaryFlushing, cfg.Flushing.Type)
	require.Equal(t, 600, cfg.Flushing.SummaryTokens)
	require.InDelta(t, defaultSummarizeThreshold, cfg.Flushing.SummarizeThreshold, 0.0001)
	require.Zero(t, cfg.MaxMessages)
}

func TestBuildSummarizationFlushValidatesInputs(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	builder := New("memory-id").
		WithProvider("openai").
		WithModel("gpt-4o").
		WithMaxTokens(1500).
		WithSummarizationFlush("  ", " ", 0)

	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.Contains(t, buildErr.Error(), "summarization provider")
	require.Contains(t, buildErr.Error(), "summarization model")
	require.Contains(t, buildErr.Error(), "summary tokens")
}

func TestSummarizationFlushOverridesPreviousStrategy(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	builder := New("memory-id").
		WithProvider("openai").
		WithModel("gpt-4o").
		WithMaxTokens(1500).
		WithFIFOFlush(25).
		WithSummarizationFlush("openai", "gpt-4o", 700)

	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.NotNil(t, cfg.Flushing)
	require.Equal(t, memcore.HybridSummaryFlushing, cfg.Flushing.Type)
	require.Equal(t, 700, cfg.Flushing.SummaryTokens)
	require.Zero(t, cfg.MaxMessages)
}

func TestBuildUsesLoggerFromContext(t *testing.T) {
	t.Parallel()

	rec := &recordingLogger{}
	ctx := logger.ContextWithLogger(t.Context(), rec)

	builder := New("memory-id").
		WithProvider("openai").
		WithModel("gpt-4o").
		WithMaxTokens(1200)

	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.NotEmpty(t, rec.debugMessages)
	require.Contains(t, rec.debugMessages[0], "building memory configuration")
}

func TestBuildAggregatesMultipleErrors(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	builder := New(" ").
		WithProvider(" ").
		WithModel(" ").
		WithMaxTokens(0).
		WithFIFOFlush(0)

	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.GreaterOrEqual(t, len(buildErr.Errors), 3)
	require.True(t, errors.Is(err, buildErr))
}

func TestBuildProducesDeepCopy(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	builder := New("memory-id").
		WithProvider("openai").
		WithModel("gpt-4o").
		WithMaxTokens(1500)

	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	cfg.MaxTokens = 10
	require.NotEqual(t, cfg.MaxTokens, builder.config.MaxTokens)
}

func TestBuildDoesNotMutateReturnedConfig(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	builder := New("memory-id").
		WithProvider("openai").
		WithModel("gpt-4").
		WithMaxTokens(1200)

	first, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotNil(t, first)

	second, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotNil(t, second)

	require.NotSame(t, first, second)
}
