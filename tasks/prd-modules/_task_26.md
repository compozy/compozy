## status: pending

<task_context>
<domain>v2/knowledge</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>engine/knowledge, v2/internal</dependencies>
</task_context>

# Task 26.0: Knowledge: Embedder (S)

## Overview

Implement EmbedderBuilder in `v2/knowledge/embedder.go` to configure embedding providers (OpenAI, Google, etc.) for knowledge base ingestion and retrieval.

<critical>
- **ALWAYS READ** @.cursor/rules/go-coding-standards.mdc before start
- **ALWAYS READ** tasks/prd-modules/02-architecture.md (Context-first patterns)
- **ALWAYS READ** tasks/prd-modules/03-sdk-entities.md (Knowledge section 6.1)
</critical>

<requirements>
- Support provider/model configuration (OpenAI, Google, etc.)
- Configure dimension, batch size, and concurrency
- Follow error accumulation pattern (BuildError)
- Use logger.FromContext(ctx) in Build method
- Validate provider and model are non-empty
</requirements>

## Subtasks

- [ ] 26.1 Create v2/knowledge/embedder.go with EmbedderBuilder
- [ ] 26.2 Implement NewEmbedder(id, provider, model)
- [ ] 26.3 Implement WithAPIKey, WithDimension, WithBatchSize, WithMaxConcurrentWorkers
- [ ] 26.4 Implement Build(ctx) with validation and error aggregation
- [ ] 26.5 Add unit tests for all methods and error cases

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

- `v2/knowledge/embedder.go` (new)
- `engine/knowledge/embedder_config.go` (reference)
- `v2/internal/errors/build_error.go` (existing)

### Dependent Files

- `v2/knowledge/base.go` (consumer)
- `v2/project/builder.go` (consumer)

## Deliverables

- v2/knowledge/embedder.go with complete EmbedderBuilder
- Unit tests in v2/knowledge/embedder_test.go
- Error validation for empty provider/model
- GoDoc comments for all public methods
- Sensible defaults (e.g., batch size=100, workers=4)

## Tests

Unit tests from _tests.md (Knowledge section):

- [ ] Valid embedder with all fields builds successfully
- [ ] Embedder with empty provider returns BuildError
- [ ] Embedder with empty model returns BuildError
- [ ] Embedder with invalid dimension (<0) returns error
- [ ] Embedder with invalid batch size (<=0) returns error
- [ ] Default values applied when optional fields omitted
- [ ] Build(ctx) propagates context to validation
- [ ] logger.FromContext(ctx) used in Build method
- [ ] Multiple validation errors aggregate in BuildError

## Success Criteria

- All unit tests pass with >95% coverage
- make lint passes
- Builder follows fluent API pattern
- Error messages are provider-specific and actionable
- Defaults match engine expectations
