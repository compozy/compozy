# SDK Knowledge Package

Functional options implementation for knowledge base configuration in Compozy SDK v2.

## Overview

This package provides a type-safe, functional options pattern for configuring knowledge bases, embedders, vector databases, and knowledge bindings.

## Constructors

### NewBase

Creates a knowledge base configuration with functional options.

```go
cfg, err := knowledge.NewBase(
    ctx,
    "my-knowledge-base",
    knowledge.WithEmbedder("my-embedder"),
    knowledge.WithVectorDB("my-vectordb"),
    knowledge.WithSources([]engineknowledge.SourceConfig{
        {Type: "file", Path: "/path/to/docs"},
    }),
)
```

**Required options:**

- `WithEmbedder(id)` - Embedder configuration ID
- `WithVectorDB(id)` - Vector database configuration ID
- `WithSources(sources)` - At least one source must be provided

**Optional options:**

- `WithDescription(desc)` - Knowledge base description
- `WithIngest(mode)` - Ingest mode (manual or on_start)
- `WithChunking(config)` - Chunking configuration
- `WithPreprocess(config)` - Preprocessing configuration
- `WithRetrieval(config)` - Retrieval configuration
- `WithMetadata(config)` - Metadata configuration

### NewBinding

Creates a knowledge binding for attaching knowledge bases to agents.

```go
binding, err := knowledge.NewBinding(
    ctx,
    "my-knowledge-base",
    knowledge.WithBindingTopK(&topK),
    knowledge.WithBindingMinScore(&minScore),
)
```

**Optional options:**

- `WithBindingTopK(*int)` - Override retrieval top-k
- `WithBindingMinScore(*float64)` - Override minimum score threshold
- `WithBindingMaxTokens(*int)` - Override maximum tokens
- `WithBindingInjectAs(string)` - How to inject retrieved context
- `WithBindingFallback(string)` - Fallback message when no results
- `WithBindingFilters(map[string]string)` - Metadata filters

### NewEmbedder

Creates an embedder configuration for converting text to vectors.

```go
cfg, err := knowledge.NewEmbedder(
    ctx,
    "my-embedder",
    "openai",
    "text-embedding-ada-002",
    knowledge.WithDimension(1536),
    knowledge.WithAPIKey(os.Getenv("OPENAI_API_KEY")),
)
```

**Supported providers:**

- `openai` - OpenAI embeddings
- `google` - Google embeddings
- `azure` - Azure OpenAI embeddings
- `cohere` - Cohere embeddings
- `ollama` - Ollama embeddings

**Required options:**

- `WithDimension(int)` - Vector dimension

**Optional options:**

- `WithAPIKey(string)` - API key for provider
- `WithBatchSize(int)` - Batch size for embedding operations
- `WithMaxConcurrentWorkers(int)` - Maximum concurrent workers

### NewVectorDB

Creates a vector database configuration for storing embeddings.

```go
cfg, err := knowledge.NewVectorDB(
    ctx,
    "my-vectordb",
    "pgvector",
    knowledge.WithDSN("postgres://localhost/mydb"),
    knowledge.WithVectorDBDimension(1536),
)
```

**Supported types:**

- `pgvector` - PostgreSQL with pgvector extension
- `qdrant` - Qdrant vector database
- `redis` - Redis with vector search
- `filesystem` - Local filesystem storage

**Type-specific requirements:**

**PGVector:**

- `WithDSN(string)` - PostgreSQL connection string (required)
- `WithVectorDBDimension(int)` - Vector dimension (required)
- `WithPGVector(*PGVectorConfig)` - Advanced PGVector configuration (optional)

**Qdrant:**

- `WithDSN(string)` - Qdrant URL (required)
- `WithCollection(string)` - Collection name (required)
- `WithVectorDBDimension(int)` - Vector dimension (required)

**Redis:**

- `WithDSN(string)` - Redis connection string (optional)
- `WithVectorDBDimension(int)` - Vector dimension (required)

**Filesystem:**

- `WithPath(string)` - Storage path (optional)
- `WithVectorDBDimension(int)` - Vector dimension (required)

## Design Notes

### Multi-Type Package Structure

Unlike other SDK packages that have a single config type, the knowledge package manages **4 different config types**:

1. `BaseConfig` - Knowledge base configuration
2. `KnowledgeBinding` - Agent binding configuration
3. `EmbedderConfig` - Text embedding configuration
4. `VectorDBConfig` - Vector storage configuration

To avoid type collisions when all types live in the same package, we use:

- Distinct option type names: `BaseOption`, `BindingOption`, `EmbedderOption`, `VectorDBOption`
- Prefixed function names where needed: `WithBindingTopK`, `WithVectorDBDimension`
- Manual option files (not code-generated) for better control

### Validation Strategy

All constructors follow a comprehensive validation pattern:

1. Context validation (nil check)
2. ID validation (empty check + format)
3. Apply functional options
4. Type-specific validation (provider, db type, etc.)
5. Apply defaults from global config
6. Range validation (chunk size, top-k, scores, etc.)
7. Cross-field validation (overlap < size, etc.)
8. Deep copy before return (immutability)

Errors are collected and returned as a `BuildError` containing all validation failures, not fail-fast.

### Testing Coverage

All constructors have comprehensive test coverage including:

- Minimal configuration (happy path)
- Input normalization (trim, lowercase)
- Nil context validation
- Empty/invalid ID validation
- Required field validation
- Type-specific validation (provider, db type)
- Range validation (dimensions, top-k, scores)
- Default value application

## Migration from Builder Pattern

The knowledge package was migrated from the old builder pattern to functional options:

**Old pattern (builder):**

```go
kb := knowledge.NewBuilder("my-kb").
    SetEmbedder("embedder-id").
    SetVectorDB("vectordb-id").
    AddSource(source).
    Build(ctx)
```

**New pattern (functional options):**

```go
kb, err := knowledge.NewBase(
    ctx,
    "my-kb",
    knowledge.WithEmbedder("embedder-id"),
    knowledge.WithVectorDB("vectordb-id"),
    knowledge.WithSources([]Source{source}),
)
```

Key improvements:

- Context-first API (required for all constructors)
- Immutable configs (deep copy on return)
- Better error aggregation (all errors reported at once)
- Type safety (compile-time option validation)
- Consistent patterns across SDK

## Related Packages

- `engine/knowledge` - Core knowledge base engine types
- `engine/core` - Core types including `KnowledgeBinding`
- `sdk/v2/internal/errors` - Build error aggregation
- `sdk/v2/internal/validate` - Validation utilities
