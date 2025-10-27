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

func TestNewVectorDBNormalizesInput(t *testing.T) {
	t.Parallel()

	builder := NewVectorDB("  vector-1  ", VectorDBType(" PgVector "))
	require.NotNil(t, builder)
	assert.Equal(t, "vector-1", builder.config.ID)
	assert.Equal(t, VectorDBType("pgvector"), builder.config.Type)
}

func TestBuildPGVectorSuccess(t *testing.T) {
	t.Parallel()

	builder := NewVectorDB("pgvector-main", engineknowledge.VectorDBTypePGVector).
		WithDSN("${PGVECTOR_DSN}").
		WithCollection(" documents ").
		WithPGVectorIndex(" IVFfLaT ", 128).
		WithPGVectorPool(2, 10)

	cfg, err := builder.Build(t.Context())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.NotSame(t, builder.config, cfg)
	assert.Equal(t, "${PGVECTOR_DSN}", cfg.Config.DSN)
	assert.Equal(t, "documents", cfg.Config.Collection)
	assert.Equal(t, engineknowledge.VectorDBTypePGVector, cfg.Type)
	require.NotNil(t, cfg.Config.PGVector)
	require.NotNil(t, cfg.Config.PGVector.Index)
	assert.Equal(t, "ivfflat", cfg.Config.PGVector.Index.Type)
	assert.Equal(t, 128, cfg.Config.PGVector.Index.Lists)
	require.NotNil(t, cfg.Config.PGVector.Pool)
	assert.Equal(t, int32(2), cfg.Config.PGVector.Pool.MinConns)
	assert.Equal(t, int32(10), cfg.Config.PGVector.Pool.MaxConns)
}

func TestBuildChromaSuccess(t *testing.T) {
	t.Parallel()

	builder := NewVectorDB("chroma-store", VectorDBTypeChroma).
		WithPath(" /tmp/chroma ")

	cfg, err := builder.Build(t.Context())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "/tmp/chroma", cfg.Config.Path)
}

func TestBuildQdrantSuccess(t *testing.T) {
	t.Parallel()

	builder := NewVectorDB("qdrant-store", engineknowledge.VectorDBTypeQdrant).
		WithDSN("https://localhost:6333").
		WithCollection(" docs ")

	cfg, err := builder.Build(t.Context())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "https://localhost:6333", cfg.Config.DSN)
	assert.Equal(t, "docs", cfg.Config.Collection)
	assert.Equal(t, engineknowledge.VectorDBTypeQdrant, cfg.Type)
}

func TestBuildWeaviateSuccess(t *testing.T) {
	t.Parallel()

	builder := NewVectorDB("weaviate-store", VectorDBTypeWeaviate).
		WithDSN("https://weaviate.local").
		WithCollection(" knowledge ")

	cfg, err := builder.Build(t.Context())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "https://weaviate.local", cfg.Config.DSN)
	assert.Equal(t, "knowledge", cfg.Config.Collection)
	assert.Equal(t, VectorDBTypeWeaviate, cfg.Type)
}

func TestBuildMilvusSuccess(t *testing.T) {
	t.Parallel()

	builder := NewVectorDB("milvus-store", VectorDBTypeMilvus).
		WithDSN("https://milvus.local").
		WithCollection(" embeddings ")

	cfg, err := builder.Build(t.Context())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "https://milvus.local", cfg.Config.DSN)
	assert.Equal(t, "embeddings", cfg.Config.Collection)
	assert.Equal(t, VectorDBTypeMilvus, cfg.Type)
}

func TestBuildInvalidTypeFails(t *testing.T) {
	t.Parallel()

	builder := NewVectorDB("invalid", VectorDBType("unsupported"))

	cfg, err := builder.Build(t.Context())
	require.Error(t, err)
	assert.Nil(t, cfg)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.Contains(t, err.Error(), "not supported")
}

func TestBuildPGVectorRequiresDSN(t *testing.T) {
	t.Parallel()

	builder := NewVectorDB("missing-dsn", engineknowledge.VectorDBTypePGVector)

	cfg, err := builder.Build(t.Context())
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "requires config.dsn")
}

func TestBuildChromaRequiresPath(t *testing.T) {
	t.Parallel()

	builder := NewVectorDB("missing-path", VectorDBTypeChroma)

	cfg, err := builder.Build(t.Context())
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "config.path")
}

func TestBuildQdrantRequiresCollection(t *testing.T) {
	t.Parallel()

	builder := NewVectorDB("missing-collection", engineknowledge.VectorDBTypeQdrant).
		WithDSN("https://localhost:6333")

	cfg, err := builder.Build(t.Context())
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "config.collection")
}

func TestBuildPGVectorIndexValidation(t *testing.T) {
	t.Parallel()

	builder := NewVectorDB("bad-index", engineknowledge.VectorDBTypePGVector).
		WithDSN("${PGVECTOR_DSN}").
		WithPGVectorIndex("ivfflat", 0)

	cfg, err := builder.Build(t.Context())
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "pgvector.index.lists")
}

func TestBuildPGVectorPoolValidation(t *testing.T) {
	t.Parallel()

	builder := NewVectorDB("bad-pool", engineknowledge.VectorDBTypePGVector).
		WithDSN("${PGVECTOR_DSN}").
		WithPGVectorPool(10, 5)

	cfg, err := builder.Build(t.Context())
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "min_conns cannot exceed")
}

func TestVectorDBBuildUsesLoggerFromContext(t *testing.T) {
	t.Parallel()

	recorder := &recordingLogger{}
	ctx := logger.ContextWithLogger(t.Context(), recorder)

	builder := NewVectorDB("pgvector-main", engineknowledge.VectorDBTypePGVector).
		WithDSN("${PGVECTOR_DSN}")

	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.NotEmpty(t, recorder.debugMessages)
	assert.Contains(t, recorder.debugMessages[0], "vector db configuration")
}

func TestVectorDBBuildReturnsBuildErrorInstance(t *testing.T) {
	t.Parallel()

	builder := NewVectorDB("", VectorDBType(""))

	cfg, err := builder.Build(t.Context())
	require.Error(t, err)
	assert.Nil(t, cfg)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.True(t, errors.Is(err, buildErr))
}
