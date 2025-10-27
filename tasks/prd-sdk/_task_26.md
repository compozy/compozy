## status: completed

<task_context>
<domain>sdk/knowledge</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>engine/knowledge, sdk/internal</dependencies>
</task_context>

# Task 26.0: Knowledge: Embedder (S)

## Overview

Implement EmbedderBuilder in `sdk/knowledge/embedder.go` to configure embedding providers (OpenAI, Google, etc.) for knowledge base ingestion and retrieval.

<critical>
- **ALWAYS READ** @.cursor/rules/go-coding-standards.mdc before start
- **ALWAYS READ** tasks/prd-sdk/02-architecture.md (Context-first patterns)
- **ALWAYS READ** tasks/prd-sdk/03-sdk-entities.md (Knowledge section 6.1)
</critical>

<requirements>
- Support provider/model configuration (OpenAI, Google, etc.)
- Configure dimension, batch size, and concurrency
- Follow error accumulation pattern (BuildError)
- Use logger.FromContext(ctx) in Build method
- Validate provider and model are non-empty
</requirements>

## Subtasks

- [x] 26.1 Create sdk/knowledge/embedder.go with EmbedderBuilder
- [x] 26.2 Implement NewEmbedder(id, provider, model)
- [x] 26.3 Implement WithAPIKey, WithDimension, WithBatchSize, WithMaxConcurrentWorkers
- [x] 26.4 Implement Build(ctx) with validation and error aggregation
- [x] 26.5 Add unit tests for all methods and error cases

## Implementation Details

Per 03-sdk-entities.md section 6.1:

```go
type EmbedderBuilder struct {
    config *knowledge.EmbedderConfig
    errors []error
}

func NewEmbedder(id, provider, model string) *EmbedderBuilder
func (b *EmbedderBuilder) WithAPIKey(key string) *EmbedderBuilder
func (b *EmbedderBuilder) WithDimension(dim int) *EmbedderBuilder
func (b *EmbedderBuilder) WithBatchSize(size int) *EmbedderBuilder
func (b *EmbedderBuilder) WithMaxConcurrentWorkers(max int) *EmbedderBuilder
func (b *EmbedderBuilder) Build(ctx context.Context) (*knowledge.EmbedderConfig, error)
```

Supported providers: openai, google, azure, cohere, ollama

### Relevant Files

- `sdk/knowledge/embedder.go` (new)
- `engine/knowledge/embedder_config.go` (reference)
- `sdk/internal/errors/build_error.go` (existing)

### Dependent Files

- `sdk/knowledge/base.go` (consumer)
- `sdk/project/builder.go` (consumer)

## Deliverables

- sdk/knowledge/embedder.go with complete EmbedderBuilder
- Unit tests in sdk/knowledge/embedder_test.go
- Error validation for empty provider/model
- GoDoc comments for all public methods
- Sensible defaults (e.g., batch size=100, workers=4)

## Tests

Unit tests from _tests.md (Knowledge section):

- [x] Valid embedder with all fields builds successfully
- [x] Embedder with empty provider returns BuildError
- [x] Embedder with empty model returns BuildError
- [x] Embedder with invalid dimension (<0) returns error
- [x] Embedder with invalid batch size (<=0) returns error
- [x] Default values applied when optional fields omitted
- [x] Build(ctx) propagates context to validation
- [x] logger.FromContext(ctx) used in Build method
- [x] Multiple validation errors aggregate in BuildError

## Success Criteria

- All unit tests pass with >95% coverage
- make lint passes
- Builder follows fluent API pattern
- Error messages are provider-specific and actionable
- Defaults match engine expectations
