package knowledge

import (
	"context"
	"errors"
	"testing"

	engineknowledge "github.com/compozy/compozy/engine/knowledge"
	sdkerrors "github.com/compozy/compozy/sdk/v2/internal/errors"
)

func TestNewBase(t *testing.T) {
	t.Run("Should create knowledge base with minimal configuration", func(t *testing.T) {
		ctx := context.Background()
		sources := []engineknowledge.SourceConfig{{Type: "file", Path: "/tmp/test.txt"}}
		cfg, err := NewBase(
			ctx,
			"test-kb",
			WithEmbedder("test-embedder"),
			WithVectorDB("test-vectordb"),
			WithSources(sources),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg == nil {
			t.Fatal("expected config, got nil")
		}
		if cfg.ID != "test-kb" {
			t.Errorf("expected ID 'test-kb', got '%s'", cfg.ID)
		}
		if cfg.Embedder != "test-embedder" {
			t.Errorf("expected embedder 'test-embedder', got '%s'", cfg.Embedder)
		}
		if cfg.VectorDB != "test-vectordb" {
			t.Errorf("expected vectordb 'test-vectordb', got '%s'", cfg.VectorDB)
		}
	})
	t.Run("Should trim whitespace from ID", func(t *testing.T) {
		ctx := context.Background()
		sources := []engineknowledge.SourceConfig{{Type: "file", Path: "/tmp/test.txt"}}
		cfg, err := NewBase(
			ctx,
			"  test-kb  ",
			WithEmbedder("test-embedder"),
			WithVectorDB("test-vectordb"),
			WithSources(sources),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.ID != "test-kb" {
			t.Errorf("expected trimmed ID 'test-kb', got '%s'", cfg.ID)
		}
	})
	t.Run("Should fail when context is nil", func(t *testing.T) {
		_, err := NewBase(nil, "test-kb")
		if err == nil {
			t.Fatal("expected error for nil context")
		}
		if err.Error() != "context is required" {
			t.Errorf("unexpected error message: %v", err)
		}
	})
	t.Run("Should fail when ID is empty", func(t *testing.T) {
		ctx := context.Background()
		_, err := NewBase(ctx, "", WithEmbedder("test-embedder"), WithVectorDB("test-vectordb"))
		if err == nil {
			t.Fatal("expected error for empty ID")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should fail when embedder is missing", func(t *testing.T) {
		ctx := context.Background()
		_, err := NewBase(ctx, "test-kb", WithVectorDB("test-vectordb"))
		if err == nil {
			t.Fatal("expected error for missing embedder")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should fail when vectordb is missing", func(t *testing.T) {
		ctx := context.Background()
		_, err := NewBase(ctx, "test-kb", WithEmbedder("test-embedder"))
		if err == nil {
			t.Fatal("expected error for missing vectordb")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should fail when no sources provided", func(t *testing.T) {
		ctx := context.Background()
		_, err := NewBase(ctx, "test-kb", WithEmbedder("test-embedder"), WithVectorDB("test-vectordb"))
		if err == nil {
			t.Fatal("expected error for no sources")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should apply defaults for optional fields", func(t *testing.T) {
		ctx := context.Background()
		sources := []engineknowledge.SourceConfig{{Type: "file", Path: "/tmp/test.txt"}}
		cfg, err := NewBase(
			ctx,
			"test-kb",
			WithEmbedder("test-embedder"),
			WithVectorDB("test-vectordb"),
			WithSources(sources),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Ingest != engineknowledge.IngestManual {
			t.Errorf("expected default ingest mode 'manual', got '%s'", cfg.Ingest)
		}
		if cfg.Chunking.Strategy != engineknowledge.ChunkStrategyRecursiveTextSplitter {
			t.Errorf("expected default chunking strategy, got '%s'", cfg.Chunking.Strategy)
		}
		if cfg.Chunking.Size == 0 {
			t.Error("expected non-zero chunk size from defaults")
		}
	})
	t.Run("Should validate chunk size range", func(t *testing.T) {
		ctx := context.Background()
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
		if err == nil {
			t.Fatal("expected error for chunk size below minimum")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should validate retrieval topk range", func(t *testing.T) {
		ctx := context.Background()
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
		if err == nil {
			t.Fatal("expected error for topk above maximum")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
}

func TestNewBinding(t *testing.T) {
	t.Run("Should create binding with minimal configuration", func(t *testing.T) {
		ctx := context.Background()
		binding, err := NewBinding(ctx, "test-kb")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if binding == nil {
			t.Fatal("expected binding, got nil")
		}
		if binding.ID != "test-kb" {
			t.Errorf("expected ID 'test-kb', got '%s'", binding.ID)
		}
	})
	t.Run("Should trim whitespace from ID", func(t *testing.T) {
		ctx := context.Background()
		binding, err := NewBinding(ctx, "  test-kb  ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if binding.ID != "test-kb" {
			t.Errorf("expected trimmed ID 'test-kb', got '%s'", binding.ID)
		}
	})
	t.Run("Should fail when context is nil", func(t *testing.T) {
		_, err := NewBinding(nil, "test-kb")
		if err == nil {
			t.Fatal("expected error for nil context")
		}
		if err.Error() != "context is required" {
			t.Errorf("unexpected error message: %v", err)
		}
	})
	t.Run("Should fail when ID is empty", func(t *testing.T) {
		ctx := context.Background()
		_, err := NewBinding(ctx, "")
		if err == nil {
			t.Fatal("expected error for empty ID")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should accept optional topk override", func(t *testing.T) {
		ctx := context.Background()
		topk := 10
		binding, err := NewBinding(ctx, "test-kb", WithBindingTopK(&topk))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if binding.TopK == nil || *binding.TopK != 10 {
			t.Error("expected topk override to be set")
		}
	})
	t.Run("Should fail when topk override is invalid", func(t *testing.T) {
		ctx := context.Background()
		topk := -1
		_, err := NewBinding(ctx, "test-kb", WithBindingTopK(&topk))
		if err == nil {
			t.Fatal("expected error for invalid topk")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should validate min score range", func(t *testing.T) {
		ctx := context.Background()
		minScore := 1.5
		_, err := NewBinding(ctx, "test-kb", WithBindingMinScore(&minScore))
		if err == nil {
			t.Fatal("expected error for min score out of range")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
}

func TestNewEmbedder(t *testing.T) {
	t.Run("Should create embedder with minimal configuration", func(t *testing.T) {
		ctx := context.Background()
		cfg, err := NewEmbedder(ctx, "test-embedder", "openai", "text-embedding-ada-002", WithDimension(1536))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg == nil {
			t.Fatal("expected config, got nil")
		}
		if cfg.ID != "test-embedder" {
			t.Errorf("expected ID 'test-embedder', got '%s'", cfg.ID)
		}
		if cfg.Provider != "openai" {
			t.Errorf("expected provider 'openai', got '%s'", cfg.Provider)
		}
		if cfg.Model != "text-embedding-ada-002" {
			t.Errorf("expected model 'text-embedding-ada-002', got '%s'", cfg.Model)
		}
	})
	t.Run("Should trim and normalize provider", func(t *testing.T) {
		ctx := context.Background()
		cfg, err := NewEmbedder(ctx, "test-embedder", "  OpenAI  ", "text-embedding-ada-002", WithDimension(1536))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Provider != "openai" {
			t.Errorf("expected normalized provider 'openai', got '%s'", cfg.Provider)
		}
	})
	t.Run("Should fail when context is nil", func(t *testing.T) {
		_, err := NewEmbedder(nil, "test-embedder", "openai", "text-embedding-ada-002", WithDimension(1536))
		if err == nil {
			t.Fatal("expected error for nil context")
		}
		if err.Error() != "context is required" {
			t.Errorf("unexpected error message: %v", err)
		}
	})
	t.Run("Should fail when ID is empty", func(t *testing.T) {
		ctx := context.Background()
		_, err := NewEmbedder(ctx, "", "openai", "text-embedding-ada-002", WithDimension(1536))
		if err == nil {
			t.Fatal("expected error for empty ID")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should fail when provider is invalid", func(t *testing.T) {
		ctx := context.Background()
		_, err := NewEmbedder(ctx, "test-embedder", "invalid-provider", "model", WithDimension(1536))
		if err == nil {
			t.Fatal("expected error for invalid provider")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should fail when model is empty", func(t *testing.T) {
		ctx := context.Background()
		_, err := NewEmbedder(ctx, "test-embedder", "openai", "", WithDimension(1536))
		if err == nil {
			t.Fatal("expected error for empty model")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should fail when dimension is invalid", func(t *testing.T) {
		ctx := context.Background()
		_, err := NewEmbedder(ctx, "test-embedder", "openai", "text-embedding-ada-002", WithDimension(0))
		if err == nil {
			t.Fatal("expected error for invalid dimension")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should apply defaults for batch size and workers", func(t *testing.T) {
		ctx := context.Background()
		cfg, err := NewEmbedder(ctx, "test-embedder", "openai", "text-embedding-ada-002", WithDimension(1536))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Config.BatchSize == 0 {
			t.Error("expected non-zero batch size from defaults")
		}
		if cfg.Config.MaxConcurrentWorkers == 0 {
			t.Error("expected non-zero max workers from defaults")
		}
	})
}

func TestNewVectorDB(t *testing.T) {
	t.Run("Should create pgvector with minimal configuration", func(t *testing.T) {
		ctx := context.Background()
		cfg, err := NewVectorDB(
			ctx,
			"test-vectordb",
			"pgvector",
			WithDSN("postgres://localhost/test"),
			WithVectorDBDimension(1536),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg == nil {
			t.Fatal("expected config, got nil")
		}
		if cfg.ID != "test-vectordb" {
			t.Errorf("expected ID 'test-vectordb', got '%s'", cfg.ID)
		}
		if cfg.Type != engineknowledge.VectorDBTypePGVector {
			t.Errorf("expected type 'pgvector', got '%s'", cfg.Type)
		}
	})
	t.Run("Should normalize database type", func(t *testing.T) {
		ctx := context.Background()
		cfg, err := NewVectorDB(
			ctx,
			"test-vectordb",
			"  PGVector  ",
			WithDSN("postgres://localhost/test"),
			WithVectorDBDimension(1536),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Type != engineknowledge.VectorDBTypePGVector {
			t.Errorf("expected normalized type 'pgvector', got '%s'", cfg.Type)
		}
	})
	t.Run("Should fail when context is nil", func(t *testing.T) {
		_, err := NewVectorDB(
			nil,
			"test-vectordb",
			"pgvector",
			WithDSN("postgres://localhost/test"),
			WithVectorDBDimension(1536),
		)
		if err == nil {
			t.Fatal("expected error for nil context")
		}
		if err.Error() != "context is required" {
			t.Errorf("unexpected error message: %v", err)
		}
	})
	t.Run("Should fail when ID is empty", func(t *testing.T) {
		ctx := context.Background()
		_, err := NewVectorDB(ctx, "", "pgvector", WithDSN("postgres://localhost/test"), WithVectorDBDimension(1536))
		if err == nil {
			t.Fatal("expected error for empty ID")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should fail when type is invalid", func(t *testing.T) {
		ctx := context.Background()
		_, err := NewVectorDB(
			ctx,
			"test-vectordb",
			"invalid-type",
			WithDSN("postgres://localhost/test"),
			WithVectorDBDimension(1536),
		)
		if err == nil {
			t.Fatal("expected error for invalid type")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should fail when pgvector DSN is missing", func(t *testing.T) {
		ctx := context.Background()
		_, err := NewVectorDB(ctx, "test-vectordb", "pgvector", WithVectorDBDimension(1536))
		if err == nil {
			t.Fatal("expected error for missing DSN")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should fail when dimension is invalid", func(t *testing.T) {
		ctx := context.Background()
		_, err := NewVectorDB(
			ctx,
			"test-vectordb",
			"pgvector",
			WithDSN("postgres://localhost/test"),
			WithVectorDBDimension(0),
		)
		if err == nil {
			t.Fatal("expected error for invalid dimension")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should create qdrant with collection", func(t *testing.T) {
		ctx := context.Background()
		cfg, err := NewVectorDB(
			ctx,
			"test-vectordb",
			"qdrant",
			WithDSN("http://localhost:6333"),
			WithCollection("test-collection"),
			WithVectorDBDimension(1536),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Type != engineknowledge.VectorDBTypeQdrant {
			t.Errorf("expected type 'qdrant', got '%s'", cfg.Type)
		}
		if cfg.Config.Collection != "test-collection" {
			t.Errorf("expected collection 'test-collection', got '%s'", cfg.Config.Collection)
		}
	})
	t.Run("Should fail when qdrant DSN is invalid URL", func(t *testing.T) {
		ctx := context.Background()
		_, err := NewVectorDB(
			ctx,
			"test-vectordb",
			"qdrant",
			WithDSN("not-a-valid-url"),
			WithCollection("test-collection"),
			WithVectorDBDimension(1536),
		)
		if err == nil {
			t.Fatal("expected error for invalid qdrant URL")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should create filesystem vectordb", func(t *testing.T) {
		ctx := context.Background()
		cfg, err := NewVectorDB(
			ctx,
			"test-vectordb",
			"filesystem",
			WithPath("/tmp/vectors"),
			WithVectorDBDimension(1536),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Type != engineknowledge.VectorDBTypeFilesystem {
			t.Errorf("expected type 'filesystem', got '%s'", cfg.Type)
		}
	})
	t.Run("Should create redis vectordb", func(t *testing.T) {
		ctx := context.Background()
		cfg, err := NewVectorDB(ctx, "test-vectordb", "redis", WithVectorDBDimension(1536))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Type != engineknowledge.VectorDBTypeRedis {
			t.Errorf("expected type 'redis', got '%s'", cfg.Type)
		}
	})
}
