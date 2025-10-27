package knowledge

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	engineknowledge "github.com/compozy/compozy/engine/knowledge"
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

func TestNewEmbedderNormalizesInput(t *testing.T) {
	t.Parallel()

	builder := NewEmbedder("  embedder-1  ", "  OpenAI  ", "  text-embedding-3-small  ")
	require.NotNil(t, builder)
	assert.Equal(t, "embedder-1", builder.config.ID)
	assert.Equal(t, "openai", builder.config.Provider)
	assert.Equal(t, "text-embedding-3-small", builder.config.Model)
}

func TestBuildReturnsClonedConfig(t *testing.T) {
	t.Parallel()

	builder := NewEmbedder("embedder-1", "openai", "text-embedding-3-small").
		WithAPIKey("  key ").
		WithDimension(1536).
		WithBatchSize(128).
		WithMaxConcurrentWorkers(3)

	cfg, err := builder.Build(t.Context())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.NotSame(t, builder.config, cfg)
	assert.Equal(t, "key", cfg.APIKey)
	assert.Equal(t, 1536, cfg.Config.Dimension)
	assert.Equal(t, 128, cfg.Config.BatchSize)
	assert.Equal(t, 3, cfg.Config.MaxConcurrentWorkers)

	cfg.Config.BatchSize = 999
	assert.Equal(t, 128, builder.config.Config.BatchSize)
}

func TestBuildFailsWithInvalidProvider(t *testing.T) {
	t.Parallel()

	builder := NewEmbedder("embedder-1", "unsupported", "text-embedding-3-small").
		WithDimension(512)

	cfg, err := builder.Build(t.Context())
	require.Error(t, err)
	assert.Nil(t, cfg)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.Len(t, buildErr.Errors, 1)
	assert.Contains(t, buildErr.Error(), "provider")
}

func TestBuildFailsWithEmptyModel(t *testing.T) {
	t.Parallel()

	builder := NewEmbedder("embedder-1", "openai", " ").
		WithDimension(1536)

	cfg, err := builder.Build(t.Context())
	require.Error(t, err)
	assert.Nil(t, cfg)
}

func TestBuildFailsWithInvalidDimension(t *testing.T) {
	t.Parallel()

	builder := NewEmbedder("embedder-1", "openai", "text-embedding-3-small").
		WithDimension(-1)

	cfg, err := builder.Build(t.Context())
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "dimension")
}

func TestBuildFailsWithInvalidBatchSize(t *testing.T) {
	t.Parallel()

	builder := NewEmbedder("embedder-1", "openai", "text-embedding-3-small").
		WithDimension(1536).
		WithBatchSize(-10)

	cfg, err := builder.Build(t.Context())
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "batch_size")
}

func TestBuildAppliesDefaultsWhenOptionalFieldsMissing(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	defaults := engineknowledge.DefaultsFromContext(ctx)

	builder := NewEmbedder("embedder-1", "openai", "text-embedding-3-small").
		WithDimension(1536)

	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, defaults.EmbedderBatchSize, cfg.Config.BatchSize)
	assert.Equal(t, defaultMaxConcurrentWorkers, cfg.Config.MaxConcurrentWorkers)
}

func TestBuildAggregatesMultipleErrors(t *testing.T) {
	t.Parallel()

	builder := NewEmbedder(" ", "invalid", " ").
		WithDimension(0).
		WithBatchSize(-1).
		WithMaxConcurrentWorkers(-2)

	cfg, err := builder.Build(t.Context())
	require.Error(t, err)
	assert.Nil(t, cfg)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.GreaterOrEqual(t, len(buildErr.Errors), 4)
}

func TestBuildUsesLoggerFromContext(t *testing.T) {
	t.Parallel()

	recLogger := &recordingLogger{}
	ctx := logger.ContextWithLogger(t.Context(), recLogger)

	builder := NewEmbedder("embedder-1", "openai", "text-embedding-3-small").
		WithDimension(1536)

	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.NotEmpty(t, recLogger.debugMessages)
	assert.Contains(t, recLogger.debugMessages[0], "building embedder configuration")
}

func TestBuildFailsWithNilContext(t *testing.T) {
	t.Parallel()

	builder := NewEmbedder("embedder-1", "openai", "text-embedding-3-small").
		WithDimension(1536)

	cfg, err := builder.Build(nil)
	require.Error(t, err)
	assert.Nil(t, cfg)
}

func TestWithMaxConcurrentWorkersInvalid(t *testing.T) {
	t.Parallel()

	builder := NewEmbedder("embedder-1", "openai", "text-embedding-3-small").
		WithDimension(1536).
		WithMaxConcurrentWorkers(-5)

	cfg, err := builder.Build(t.Context())
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "max_concurrent_workers")
}

func TestBuildReturnsBuildErrorInstance(t *testing.T) {
	t.Parallel()

	builder := NewEmbedder("", "", "").
		WithDimension(0)

	cfg, err := builder.Build(t.Context())
	require.Error(t, err)
	assert.Nil(t, cfg)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.True(t, errors.Is(err, buildErr))
}
