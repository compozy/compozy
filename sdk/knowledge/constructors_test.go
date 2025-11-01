package knowledge

import (
	"context"
	"testing"

	engineknowledge "github.com/compozy/compozy/engine/knowledge"
	sdkerrors "github.com/compozy/compozy/sdk/v2/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBase(t *testing.T) {
	t.Run("Should create knowledge base with minimal configuration", func(t *testing.T) {
		ctx := t.Context()
		sources := []engineknowledge.SourceConfig{{Type: "file", Path: "/tmp/test.txt"}}
		cfg, err := NewBase(
			ctx,
			"test-kb",
			WithEmbedder("test-embedder"),
			WithVectorDB("test-vectordb"),
			WithSources(sources),
		)
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Equal(t, "test-kb", cfg.ID)
		assert.Equal(t, "test-embedder", cfg.Embedder)
		assert.Equal(t, "test-vectordb", cfg.VectorDB)
	})
	t.Run("Should trim whitespace from ID", func(t *testing.T) {
		ctx := t.Context()
		sources := []engineknowledge.SourceConfig{{Type: "file", Path: "/tmp/test.txt"}}
		cfg, err := NewBase(
			ctx,
			"  test-kb  ",
			WithEmbedder("test-embedder"),
			WithVectorDB("test-vectordb"),
			WithSources(sources),
		)
		require.NoError(t, err)
		assert.Equal(t, "test-kb", cfg.ID)
	})
	t.Run("Should fail when context is nil", func(t *testing.T) {
		var nilCtx context.Context
		_, err := NewBase(nilCtx, "test-kb")
		require.Error(t, err, "expected error for nil context")
		assert.Equal(t, "context is required", err.Error())
	})
	t.Run("Should fail when ID is empty", func(t *testing.T) {
		ctx := t.Context()
		_, err := NewBase(ctx, "", WithEmbedder("test-embedder"), WithVectorDB("test-vectordb"))
		require.Error(t, err, "expected error for empty ID")
		var buildErr *sdkerrors.BuildError
		require.ErrorAs(t, err, &buildErr)
	})
	t.Run("Should fail when embedder is missing", func(t *testing.T) {
		ctx := t.Context()
		_, err := NewBase(ctx, "test-kb", WithVectorDB("test-vectordb"))
		require.Error(t, err, "expected error for missing embedder")
		var buildErr *sdkerrors.BuildError
		require.ErrorAs(t, err, &buildErr)
	})
	t.Run("Should fail when vectordb is missing", func(t *testing.T) {
		ctx := t.Context()
		_, err := NewBase(ctx, "test-kb", WithEmbedder("test-embedder"))
		require.Error(t, err, "expected error for missing vectordb")
		var buildErr *sdkerrors.BuildError
		require.ErrorAs(t, err, &buildErr)
	})
	t.Run("Should fail when no sources provided", func(t *testing.T) {
		ctx := t.Context()
		_, err := NewBase(ctx, "test-kb", WithEmbedder("test-embedder"), WithVectorDB("test-vectordb"))
		require.Error(t, err, "expected error for no sources")
		var buildErr *sdkerrors.BuildError
		require.ErrorAs(t, err, &buildErr)
	})
	t.Run("Should apply defaults for optional fields", func(t *testing.T) {
		ctx := t.Context()
		sources := []engineknowledge.SourceConfig{{Type: "file", Path: "/tmp/test.txt"}}
		cfg, err := NewBase(
			ctx,
			"test-kb",
			WithEmbedder("test-embedder"),
			WithVectorDB("test-vectordb"),
			WithSources(sources),
		)
		require.NoError(t, err)
		assert.Equal(t, engineknowledge.IngestManual, cfg.Ingest)
		assert.Equal(t, engineknowledge.ChunkStrategyRecursiveTextSplitter, cfg.Chunking.Strategy)
		assert.NotZero(t, cfg.Chunking.Size)
	})
	t.Run("Should validate chunk size range", func(t *testing.T) {
		ctx := t.Context()
		sources := []engineknowledge.SourceConfig{{Type: "file", Path: "/tmp/test.txt"}}
		chunking := engineknowledge.ChunkingConfig{Size: 10}
		_, err := NewBase(
			ctx,
			"test-kb",
			WithEmbedder("test-embedder"),
			WithVectorDB("test-vectordb"),
			WithSources(sources),
			WithChunking(chunking),
		)
		require.Error(t, err, "expected error for chunk size below minimum")
		var buildErr *sdkerrors.BuildError
		require.ErrorAs(t, err, &buildErr)
	})
	t.Run("Should validate retrieval topk range", func(t *testing.T) {
		ctx := t.Context()
		sources := []engineknowledge.SourceConfig{{Type: "file", Path: "/tmp/test.txt"}}
		retrieval := engineknowledge.RetrievalConfig{TopK: 100}
		_, err := NewBase(
			ctx,
			"test-kb",
			WithEmbedder("test-embedder"),
			WithVectorDB("test-vectordb"),
			WithSources(sources),
			WithRetrieval(&retrieval),
		)
		require.Error(t, err, "expected error for topk above maximum")
		var buildErr *sdkerrors.BuildError
		require.ErrorAs(t, err, &buildErr)
	})
}

func TestNewBinding(t *testing.T) {
	t.Run("Should create binding with minimal configuration", func(t *testing.T) {
		ctx := t.Context()
		binding, err := NewBinding(ctx, "test-kb")
		require.NoError(t, err)
		require.NotNil(t, binding)
		assert.Equal(t, "test-kb", binding.ID)
	})
	t.Run("Should trim whitespace from ID", func(t *testing.T) {
		ctx := t.Context()
		binding, err := NewBinding(ctx, "  test-kb  ")
		require.NoError(t, err)
		assert.Equal(t, "test-kb", binding.ID)
	})
	t.Run("Should fail when context is nil", func(t *testing.T) {
		var nilCtx context.Context
		_, err := NewBinding(nilCtx, "test-kb")
		require.Error(t, err, "expected error for nil context")
		assert.Equal(t, "context is required", err.Error())
	})
	t.Run("Should fail when ID is empty", func(t *testing.T) {
		ctx := t.Context()
		_, err := NewBinding(ctx, "")
		require.Error(t, err, "expected error for empty ID")
		var buildErr *sdkerrors.BuildError
		require.ErrorAs(t, err, &buildErr)
	})
	t.Run("Should accept optional topk override", func(t *testing.T) {
		ctx := t.Context()
		topk := 10
		binding, err := NewBinding(ctx, "test-kb", WithBindingTopK(&topk))
		require.NoError(t, err)
		require.NotNil(t, binding.TopK)
		assert.Equal(t, 10, *binding.TopK)
	})
	t.Run("Should fail when topk override is invalid", func(t *testing.T) {
		ctx := t.Context()
		topk := -1
		_, err := NewBinding(ctx, "test-kb", WithBindingTopK(&topk))
		require.Error(t, err, "expected error for invalid topk")
		var buildErr *sdkerrors.BuildError
		require.ErrorAs(t, err, &buildErr)
	})
	t.Run("Should validate min score range", func(t *testing.T) {
		ctx := t.Context()
		minScore := 1.5
		_, err := NewBinding(ctx, "test-kb", WithBindingMinScore(&minScore))
		require.Error(t, err, "expected error for min score out of range")
		var buildErr *sdkerrors.BuildError
		require.ErrorAs(t, err, &buildErr)
	})
}

func TestNewEmbedder(t *testing.T) {
	t.Run("Should create embedder with minimal configuration", func(t *testing.T) {
		ctx := t.Context()
		cfg, err := NewEmbedder(ctx, "test-embedder", "openai", "text-embedding-ada-002", WithDimension(1536))
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Equal(t, "test-embedder", cfg.ID)
		assert.Equal(t, "openai", cfg.Provider)
		assert.Equal(t, "text-embedding-ada-002", cfg.Model)
	})
	t.Run("Should trim and normalize provider", func(t *testing.T) {
		ctx := t.Context()
		cfg, err := NewEmbedder(ctx, "test-embedder", "  OpenAI  ", "text-embedding-ada-002", WithDimension(1536))
		require.NoError(t, err)
		assert.Equal(t, "openai", cfg.Provider)
	})
	t.Run("Should fail when context is nil", func(t *testing.T) {
		var nilCtx context.Context
		_, err := NewEmbedder(nilCtx, "test-embedder", "openai", "text-embedding-ada-002", WithDimension(1536))
		require.Error(t, err, "expected error for nil context")
		assert.Equal(t, "context is required", err.Error())
	})
	t.Run("Should fail when ID is empty", func(t *testing.T) {
		ctx := t.Context()
		_, err := NewEmbedder(ctx, "", "openai", "text-embedding-ada-002", WithDimension(1536))
		require.Error(t, err, "expected error for empty ID")
		var buildErr *sdkerrors.BuildError
		require.ErrorAs(t, err, &buildErr)
	})
	t.Run("Should fail when provider is invalid", func(t *testing.T) {
		ctx := t.Context()
		_, err := NewEmbedder(ctx, "test-embedder", "invalid-provider", "model", WithDimension(1536))
		require.Error(t, err, "expected error for invalid provider")
		var buildErr *sdkerrors.BuildError
		require.ErrorAs(t, err, &buildErr)
	})
	t.Run("Should fail when model is empty", func(t *testing.T) {
		ctx := t.Context()
		_, err := NewEmbedder(ctx, "test-embedder", "openai", "", WithDimension(1536))
		require.Error(t, err, "expected error for empty model")
		var buildErr *sdkerrors.BuildError
		require.ErrorAs(t, err, &buildErr)
	})
	t.Run("Should fail when dimension is invalid", func(t *testing.T) {
		ctx := t.Context()
		_, err := NewEmbedder(ctx, "test-embedder", "openai", "text-embedding-ada-002", WithDimension(0))
		require.Error(t, err, "expected error for invalid dimension")
		var buildErr *sdkerrors.BuildError
		require.ErrorAs(t, err, &buildErr)
	})
	t.Run("Should apply defaults for batch size and workers", func(t *testing.T) {
		ctx := t.Context()
		cfg, err := NewEmbedder(ctx, "test-embedder", "openai", "text-embedding-ada-002", WithDimension(1536))
		require.NoError(t, err)
		assert.NotZero(t, cfg.Config.BatchSize)
		assert.NotZero(t, cfg.Config.MaxConcurrentWorkers)
	})
}

func TestNewVectorDB(t *testing.T) {
	t.Run("Should create pgvector with minimal configuration", func(t *testing.T) {
		ctx := t.Context()
		cfg, err := NewVectorDB(
			ctx,
			"test-vectordb",
			"pgvector",
			WithDSN("postgres://localhost/test"),
			WithVectorDBDimension(1536),
		)
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Equal(t, "test-vectordb", cfg.ID)
		assert.Equal(t, engineknowledge.VectorDBTypePGVector, cfg.Type)
	})
	t.Run("Should normalize database type", func(t *testing.T) {
		ctx := t.Context()
		cfg, err := NewVectorDB(
			ctx,
			"test-vectordb",
			"  PGVector  ",
			WithDSN("postgres://localhost/test"),
			WithVectorDBDimension(1536),
		)
		require.NoError(t, err)
		assert.Equal(t, engineknowledge.VectorDBTypePGVector, cfg.Type)
	})
	t.Run("Should fail when context is nil", func(t *testing.T) {
		var nilCtx context.Context
		_, err := NewVectorDB(
			nilCtx,
			"test-vectordb",
			"pgvector",
			WithDSN("postgres://localhost/test"),
			WithVectorDBDimension(1536),
		)
		require.Error(t, err, "expected error for nil context")
		assert.Equal(t, "context is required", err.Error())
	})
	t.Run("Should fail when ID is empty", func(t *testing.T) {
		ctx := t.Context()
		_, err := NewVectorDB(ctx, "", "pgvector", WithDSN("postgres://localhost/test"), WithVectorDBDimension(1536))
		require.Error(t, err, "expected error for empty ID")
		var buildErr *sdkerrors.BuildError
		require.ErrorAs(t, err, &buildErr)
	})
	t.Run("Should fail when type is invalid", func(t *testing.T) {
		ctx := t.Context()
		_, err := NewVectorDB(
			ctx,
			"test-vectordb",
			"invalid-type",
			WithDSN("postgres://localhost/test"),
			WithVectorDBDimension(1536),
		)
		require.Error(t, err, "expected error for invalid type")
		var buildErr *sdkerrors.BuildError
		require.ErrorAs(t, err, &buildErr)
	})
	t.Run("Should fail when pgvector DSN is missing", func(t *testing.T) {
		ctx := t.Context()
		_, err := NewVectorDB(ctx, "test-vectordb", "pgvector", WithVectorDBDimension(1536))
		require.Error(t, err, "expected error for missing DSN")
		var buildErr *sdkerrors.BuildError
		require.ErrorAs(t, err, &buildErr)
	})
	t.Run("Should fail when dimension is invalid", func(t *testing.T) {
		ctx := t.Context()
		_, err := NewVectorDB(
			ctx,
			"test-vectordb",
			"pgvector",
			WithDSN("postgres://localhost/test"),
			WithVectorDBDimension(0),
		)
		require.Error(t, err, "expected error for invalid dimension")
		var buildErr *sdkerrors.BuildError
		require.ErrorAs(t, err, &buildErr)
	})
	t.Run("Should create qdrant with collection", func(t *testing.T) {
		ctx := t.Context()
		cfg, err := NewVectorDB(
			ctx,
			"test-vectordb",
			"qdrant",
			WithDSN("http://localhost:6333"),
			WithCollection("test-collection"),
			WithVectorDBDimension(1536),
		)
		require.NoError(t, err)
		assert.Equal(t, engineknowledge.VectorDBTypeQdrant, cfg.Type)
		assert.Equal(t, "test-collection", cfg.Config.Collection)
	})
	t.Run("Should fail when qdrant DSN is invalid URL", func(t *testing.T) {
		ctx := t.Context()
		_, err := NewVectorDB(
			ctx,
			"test-vectordb",
			"qdrant",
			WithDSN("not-a-valid-url"),
			WithCollection("test-collection"),
			WithVectorDBDimension(1536),
		)
		require.Error(t, err, "expected error for invalid qdrant URL")
		var buildErr *sdkerrors.BuildError
		require.ErrorAs(t, err, &buildErr)
	})
	t.Run("Should create filesystem vectordb", func(t *testing.T) {
		ctx := t.Context()
		cfg, err := NewVectorDB(
			ctx,
			"test-vectordb",
			"filesystem",
			WithPath("/tmp/vectors"),
			WithVectorDBDimension(1536),
		)
		require.NoError(t, err)
		assert.Equal(t, engineknowledge.VectorDBTypeFilesystem, cfg.Type)
	})
	t.Run("Should create redis vectordb", func(t *testing.T) {
		ctx := t.Context()
		cfg, err := NewVectorDB(ctx, "test-vectordb", "redis", WithVectorDBDimension(1536))
		require.NoError(t, err)
		assert.Equal(t, engineknowledge.VectorDBTypeRedis, cfg.Type)
	})
}
