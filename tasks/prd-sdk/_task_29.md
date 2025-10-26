## status: pending

<task_context>
<domain>sdk/knowledge</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>engine/knowledge, sdk/knowledge, sdk/internal</dependencies>
</task_context>

# Task 29.0: Knowledge: Base (S)

## Overview

Implement BaseBuilder in `sdk/knowledge/base.go` to configure complete knowledge bases with embedders, vector DBs, sources, chunking strategies, and retrieval parameters.

<critical>
- **ALWAYS READ** @.cursor/rules/go-coding-standards.mdc before start
- **ALWAYS READ** tasks/prd-sdk/02-architecture.md (Context-first patterns)
- **ALWAYS READ** tasks/prd-sdk/03-sdk-entities.md (Knowledge section 6.4)
</critical>

<requirements>
- Configure embedder and vectorDB references
- Add multiple sources for ingestion
- Configure chunking strategy (size, overlap)
- Configure retrieval parameters (topK, minScore, maxTokens)
- Validate embedder and vectorDB IDs are non-empty
- Follow error accumulation pattern (BuildError)
</requirements>

## Subtasks

- [ ] 29.1 Create sdk/knowledge/base.go with BaseBuilder
- [ ] 29.2 Implement NewBase(id) constructor
- [ ] 29.3 Implement WithEmbedder, WithVectorDB, AddSource methods
- [ ] 29.4 Implement WithChunking, WithPreprocess, WithIngestMode methods
- [ ] 29.5 Implement WithRetrieval method and Build(ctx) with validation

## Implementation Details

Per 03-sdk-entities.md section 6.4:

```go
type BaseBuilder struct {
    config *knowledge.BaseConfig
    errors []error
}

func NewBase(id string) *BaseBuilder
func (b *BaseBuilder) WithDescription(desc string) *BaseBuilder
func (b *BaseBuilder) WithEmbedder(embedderID string) *BaseBuilder
func (b *BaseBuilder) WithVectorDB(vectorDBID string) *BaseBuilder

// Ingestion
func (b *BaseBuilder) AddSource(source *knowledge.SourceConfig) *BaseBuilder
func (b *BaseBuilder) WithChunking(strategy knowledge.ChunkStrategy, size, overlap int) *BaseBuilder
func (b *BaseBuilder) WithPreprocess(dedupe, removeHTML bool) *BaseBuilder
func (b *BaseBuilder) WithIngestMode(mode knowledge.IngestMode) *BaseBuilder

// Retrieval
func (b *BaseBuilder) WithRetrieval(topK int, minScore float64, maxTokens int) *BaseBuilder

func (b *BaseBuilder) Build(ctx context.Context) (*knowledge.BaseConfig, error)
```

Validation:
- ID is required and non-empty
- EmbedderID and VectorDBID are required
- At least one source must be added
- Chunk size > overlap
- TopK > 0, minScore in [0.0, 1.0], maxTokens > 0

### Relevant Files

- `sdk/knowledge/base.go` (new)
- `engine/knowledge/base_config.go` (reference)
- `sdk/internal/errors/build_error.go` (existing)

### Dependent Files

- `sdk/knowledge/embedder.go` (dependency)
- `sdk/knowledge/vectordb.go` (dependency)
- `sdk/knowledge/source.go` (dependency)
- `sdk/project/builder.go` (consumer)

## Deliverables

- sdk/knowledge/base.go with complete BaseBuilder
- Unit tests for all configuration methods
- Validation for required dependencies (embedder, vectorDB)
- GoDoc comments with usage examples
- Default values for optional parameters

## Tests

Unit tests from _tests.md (Knowledge section):

- [ ] Valid knowledge base with all fields builds successfully
- [ ] Knowledge base without embedder returns BuildError
- [ ] Knowledge base without vectorDB returns BuildError
- [ ] Knowledge base without sources returns BuildError
- [ ] Chunking with size <= overlap returns validation error
- [ ] Retrieval with topK <= 0 returns validation error
- [ ] Retrieval with minScore < 0 or > 1 returns validation error
- [ ] Multiple sources can be added successfully
- [ ] Default chunking strategy applied when not specified
- [ ] Default retrieval parameters applied when not specified
- [ ] Build(ctx) propagates context to validation
- [ ] logger.FromContext(ctx) used in Build method

## Success Criteria

- All unit tests pass with >95% coverage
- make lint passes
- Sensible defaults for chunking and retrieval
- Error messages specify missing dependencies
- Documentation includes complete usage example
