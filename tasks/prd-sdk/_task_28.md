## status: pending

<task_context>
<domain>sdk/knowledge</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>engine/knowledge, sdk/internal</dependencies>
</task_context>

# Task 28.0: Knowledge: Source (S)

## Overview

Implement SourceBuilder in `sdk/knowledge/source.go` to configure knowledge ingestion sources (files, directories, URLs, APIs) for knowledge base population.

<critical>
- **ALWAYS READ** @.cursor/rules/go-coding-standards.mdc before start
- **ALWAYS READ** tasks/prd-sdk/02-architecture.md (Context-first patterns)
- **ALWAYS READ** tasks/prd-sdk/03-sdk-entities.md (Knowledge section 6.3)
</critical>

<requirements>
- Support 4 source types: file, directory, URL, API
- Type-specific constructors for clarity
- Validate source paths/URLs are non-empty
- Follow error accumulation pattern (BuildError)
- Use logger.FromContext(ctx) in Build method
</requirements>

## Subtasks

- [ ] 28.1 Create sdk/knowledge/source.go with SourceBuilder
- [ ] 28.2 Implement NewFileSource(path), NewDirectorySource(paths...), NewURLSource(urls...), NewAPISource(provider)
- [ ] 28.3 Add source type validation in Build(ctx)
- [ ] 28.4 Implement Build(ctx) with path/URL validation
- [ ] 28.5 Add unit tests for all source types

## Implementation Details

Per 03-sdk-entities.md section 6.3:

```go
type SourceBuilder struct {
    config *knowledge.SourceConfig
    errors []error
}

// Constructors for different source types
func NewFileSource(path string) *SourceBuilder
func NewDirectorySource(paths ...string) *SourceBuilder
func NewURLSource(urls ...string) *SourceBuilder
func NewAPISource(provider string) *SourceBuilder

func (b *SourceBuilder) Build(ctx context.Context) (*knowledge.SourceConfig, error)
```

Source types: file, directory, url, api

Validation:
- File source: path exists and is readable
- Directory source: at least one path provided
- URL source: valid HTTP(S) URLs
- API source: provider is supported

### Relevant Files

- `sdk/knowledge/source.go` (new)
- `engine/knowledge/source_config.go` (reference)
- `sdk/internal/errors/build_error.go` (existing)

### Dependent Files

- `sdk/knowledge/base.go` (consumer)

## Deliverables

- sdk/knowledge/source.go with complete SourceBuilder
- Unit tests for all 4 source types
- Path/URL validation with clear error messages
- GoDoc comments for all constructors
- Type-safe source type handling

## Tests

Unit tests from _tests.md (Knowledge section):

- [ ] Valid file source builds successfully
- [ ] Valid directory source with multiple paths builds successfully
- [ ] Valid URL source with multiple URLs builds successfully
- [ ] Valid API source with provider builds successfully
- [ ] File source with empty path returns BuildError
- [ ] Directory source with empty paths array returns error
- [ ] URL source with invalid URL format returns error
- [ ] API source with unsupported provider returns error
- [ ] Build(ctx) propagates context to validation
- [ ] logger.FromContext(ctx) used in Build method
- [ ] Each source type sets correct SourceType enum

## Success Criteria

- All unit tests pass with >95% coverage
- make lint passes
- Type-specific constructors make API intuitive
- Error messages specify which source failed validation
- URL validation uses standard library (net/url)
