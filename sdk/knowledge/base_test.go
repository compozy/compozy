package knowledge

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	engineknowledge "github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/test/helpers"
)

type baseRecordingLogger struct {
	debugMessages []string
}

func (l *baseRecordingLogger) Debug(msg string, _ ...any) {
	l.debugMessages = append(l.debugMessages, msg)
}

func (l *baseRecordingLogger) Info(string, ...any)  {}
func (l *baseRecordingLogger) Warn(string, ...any)  {}
func (l *baseRecordingLogger) Error(string, ...any) {}

func (l *baseRecordingLogger) With(...any) logger.Logger {
	return l
}

func TestBaseBuilderBuildSuccess(t *testing.T) {
	t.Parallel()

	ctx := helpers.NewTestContext(t)
	sourceOne, err := NewURLSource("https://example.com/docs").Build(ctx)
	require.NoError(t, err)
	sourceTwo, err := NewURLSource("https://example.com/faq").Build(ctx)
	require.NoError(t, err)

	builder := NewBase(" docs ").
		WithDescription("  Example Knowledge Base ").
		WithEmbedder(" embed-1 ").
		WithVectorDB(" vector-1 ").
		AddSource(sourceOne).
		AddSource(sourceTwo).
		WithChunking(ChunkStrategyRecursiveTextSplitter, 900, 100).
		WithPreprocess(true, true).
		WithIngestMode(IngestModeOnStart).
		WithRetrieval(7, 0.5, 1500)

	cfg, buildErr := builder.Build(ctx)
	require.NoError(t, buildErr)
	require.NotNil(t, cfg)

	assert.Equal(t, "docs", cfg.ID)
	assert.Equal(t, "Example Knowledge Base", cfg.Description)
	assert.Equal(t, "embed-1", cfg.Embedder)
	assert.Equal(t, "vector-1", cfg.VectorDB)
	assert.Equal(t, engineknowledge.IngestOnStart, cfg.Ingest)
	assert.Len(t, cfg.Sources, 2)
	assert.Equal(t, 900, cfg.Chunking.Size)
	assert.Equal(t, 100, cfg.Chunking.OverlapValue())
	assert.NotNil(t, cfg.Preprocess.Deduplicate)
	assert.True(t, *cfg.Preprocess.Deduplicate)
	assert.True(t, cfg.Preprocess.RemoveHTML)
	assert.Equal(t, 7, cfg.Retrieval.TopK)
	assert.InDelta(t, 0.5, cfg.Retrieval.MinScoreValue(), 0.0001)
	assert.Equal(t, 1500, cfg.Retrieval.MaxTokens)

	cfg.Sources[0].Path = "mutated"
	assert.NotEqual(t, "mutated", builder.config.Sources[0].Path)
}

func TestBaseBuilderRequiresEmbedder(t *testing.T) {
	t.Parallel()

	ctx := helpers.NewTestContext(t)
	source, err := NewURLSource("https://example.com/docs").Build(ctx)
	require.NoError(t, err)

	cfg, buildErr := NewBase("kb").
		WithVectorDB("vector").
		AddSource(source).
		Build(ctx)

	require.Error(t, buildErr)
	assert.Nil(t, cfg)

	var aggErr *sdkerrors.BuildError
	require.ErrorAs(t, buildErr, &aggErr)
	assert.NotEmpty(t, aggErr.Errors)
}

func TestBaseBuilderRequiresVectorDB(t *testing.T) {
	t.Parallel()

	ctx := helpers.NewTestContext(t)
	source, err := NewURLSource("https://example.com/docs").Build(ctx)
	require.NoError(t, err)

	cfg, buildErr := NewBase("kb").
		WithEmbedder("embedder").
		AddSource(source).
		Build(ctx)

	require.Error(t, buildErr)
	assert.Nil(t, cfg)
}

func TestBaseBuilderRequiresSources(t *testing.T) {
	t.Parallel()

	cfg, buildErr := NewBase("kb").
		WithEmbedder("embedder").
		WithVectorDB("vector").
		Build(helpers.NewTestContext(t))

	require.Error(t, buildErr)
	assert.Nil(t, cfg)
	assert.Contains(t, buildErr.Error(), "source")
}

func TestBaseBuilderValidatesChunkingOverlap(t *testing.T) {
	t.Parallel()

	ctx := helpers.NewTestContext(t)
	source, err := NewURLSource("https://example.com/docs").Build(ctx)
	require.NoError(t, err)

	cfg, buildErr := NewBase("kb").
		WithEmbedder("embedder").
		WithVectorDB("vector").
		AddSource(source).
		WithChunking(ChunkStrategyRecursiveTextSplitter, 128, 128).
		Build(ctx)

	require.Error(t, buildErr)
	assert.Nil(t, cfg)
	assert.Contains(t, buildErr.Error(), "overlap")
}

func TestBaseBuilderValidatesRetrievalParameters(t *testing.T) {
	t.Parallel()

	ctx := helpers.NewTestContext(t)
	source, err := NewURLSource("https://example.com/docs").Build(ctx)
	require.NoError(t, err)

	builder := NewBase("kb").
		WithEmbedder("embedder").
		WithVectorDB("vector").
		AddSource(source)

	_, errTopK := builder.
		WithRetrieval(0, 0.2, 800).
		Build(ctx)
	require.Error(t, errTopK)
	assert.Contains(t, errTopK.Error(), "top_k")

	_, errMinScore := builder.
		WithRetrieval(3, 1.5, 800).
		Build(ctx)
	require.Error(t, errMinScore)
	assert.Contains(t, errMinScore.Error(), "min_score")

	_, errMaxTokens := builder.
		WithRetrieval(3, 0.2, 0).
		Build(ctx)
	require.Error(t, errMaxTokens)
	assert.Contains(t, errMaxTokens.Error(), "max_tokens")
}

func TestBaseBuilderAddsMultipleSources(t *testing.T) {
	t.Parallel()

	ctx := helpers.NewTestContext(t)
	sourceOne, err := NewURLSource("https://example.com/docs").Build(ctx)
	require.NoError(t, err)
	sourceTwo, err := NewURLSource("https://example.com/guide").Build(ctx)
	require.NoError(t, err)

	cfg, buildErr := NewBase("kb").
		WithEmbedder("embedder").
		WithVectorDB("vector").
		AddSource(sourceOne).
		AddSource(sourceTwo).
		Build(ctx)

	require.NoError(t, buildErr)
	require.NotNil(t, cfg)
	assert.Len(t, cfg.Sources, 2)
}

func TestBaseBuilderAppliesDefaults(t *testing.T) {
	t.Parallel()

	ctx := helpers.NewTestContext(t)
	source, err := NewURLSource("https://example.com/docs").Build(ctx)
	require.NoError(t, err)

	cfg, buildErr := NewBase("kb").
		WithEmbedder("embedder").
		WithVectorDB("vector").
		AddSource(source).
		Build(ctx)

	require.NoError(t, buildErr)
	require.NotNil(t, cfg)

	defaults := engineknowledge.DefaultsFromContext(ctx)
	assert.Equal(t, defaults.ChunkSize, cfg.Chunking.Size)
	assert.Equal(t, defaults.ChunkOverlap, cfg.Chunking.OverlapValue())
	assert.Equal(t, defaults.RetrievalTopK, cfg.Retrieval.TopK)
	assert.InDelta(t, defaults.RetrievalMinScore, cfg.Retrieval.MinScoreValue(), 0.0001)
	assert.Equal(t, defaultRetrievalMaxTokens, cfg.Retrieval.MaxTokens)
	assert.True(t, *cfg.Preprocess.Deduplicate)
	assert.Equal(t, engineknowledge.IngestManual, cfg.Ingest)
}

func TestBaseBuilderUsesContextDefaults(t *testing.T) {
	t.Parallel()

	baseCtx := t.Context()
	baseCtx = logger.ContextWithLogger(baseCtx, logger.NewForTests())
	manager := config.NewManager(baseCtx, config.NewService())
	providers := []config.Source{
		config.NewDefaultProvider(),
		config.NewCLIProvider(map[string]any{
			"knowledge-chunk-size":          1024,
			"knowledge-chunk-overlap":       64,
			"knowledge-retrieval-top-k":     9,
			"knowledge-retrieval-min-score": 0.4,
		}),
	}
	_, err := manager.Load(baseCtx, providers...)
	require.NoError(t, err)
	ctx := config.ContextWithManager(baseCtx, manager)
	t.Cleanup(func() {
		require.NoError(t, manager.Close(ctx))
	})

	source, srcErr := NewURLSource("https://example.com/docs").Build(ctx)
	require.NoError(t, srcErr)

	cfg, buildErr := NewBase("kb").
		WithEmbedder("embedder").
		WithVectorDB("vector").
		AddSource(source).
		Build(ctx)

	require.NoError(t, buildErr)
	require.NotNil(t, cfg)
	assert.Equal(t, 1024, cfg.Chunking.Size)
	assert.Equal(t, 64, cfg.Chunking.OverlapValue())
	assert.Equal(t, 9, cfg.Retrieval.TopK)
	assert.InDelta(t, 0.4, cfg.Retrieval.MinScoreValue(), 0.0001)
}

func TestBaseBuilderUsesLoggerFromContext(t *testing.T) {
	t.Parallel()

	recLogger := &baseRecordingLogger{}
	ctx := logger.ContextWithLogger(helpers.NewTestContext(t), recLogger)
	source, err := NewURLSource("https://example.com/docs").Build(ctx)
	require.NoError(t, err)

	cfg, buildErr := NewBase("kb").
		WithEmbedder("embedder").
		WithVectorDB("vector").
		AddSource(source).
		Build(ctx)

	require.NoError(t, buildErr)
	require.NotNil(t, cfg)
	assert.NotEmpty(t, recLogger.debugMessages)
	found := false
	for _, msg := range recLogger.debugMessages {
		if strings.Contains(msg, "knowledge base configuration") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected knowledge base debug message, got %v", recLogger.debugMessages)
}

func TestBaseBuilderRejectsNilContext(t *testing.T) {
	t.Parallel()

	ctx := helpers.NewTestContext(t)
	source, err := NewURLSource("https://example.com/docs").Build(ctx)
	require.NoError(t, err)

	cfg, buildErr := NewBase("kb").
		WithEmbedder("embedder").
		WithVectorDB("vector").
		AddSource(source).
		Build(nil)

	require.Error(t, buildErr)
	assert.Nil(t, cfg)
}
