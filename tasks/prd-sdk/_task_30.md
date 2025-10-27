## status: completed

<task_context>
<domain>sdk/knowledge</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>engine/knowledge, sdk/internal</dependencies>
</task_context>

# Task 30.0: Knowledge: Binding (S)

## Overview

Implement BindingBuilder in `sdk/knowledge/binding.go` to configure agent-knowledge base attachments with retrieval parameters override capabilities.

<critical>
- **ALWAYS READ** @.cursor/rules/go-coding-standards.mdc before start
- **ALWAYS READ** tasks/prd-sdk/02-architecture.md (Context-first patterns)
- **ALWAYS READ** tasks/prd-sdk/03-sdk-entities.md (Knowledge section 6.5)
</critical>

<requirements>
- Reference knowledge base by ID
- Allow retrieval parameter overrides (topK, minScore, maxTokens)
- Validate knowledge base ID is non-empty
- Follow error accumulation pattern (BuildError)
- Use logger.FromContext(ctx) in Build method
</requirements>

## Subtasks

- [x] 30.1 Create sdk/knowledge/binding.go with BindingBuilder
- [x] 30.2 Implement NewBinding(knowledgeBaseID) constructor
- [x] 30.3 Implement WithTopK, WithMinScore, WithMaxTokens methods
- [x] 30.4 Implement Build(ctx) with validation
- [x] 30.5 Add unit tests for all methods and overrides

## Implementation Details

Per 03-sdk-entities.md section 6.5:

```go
type BindingBuilder struct {
    config *knowledge.BindingConfig
    errors []error
}

func NewBinding(knowledgeBaseID string) *BindingBuilder

// Retrieval parameter overrides (optional)
func (b *BindingBuilder) WithTopK(topK int) *BindingBuilder
func (b *BindingBuilder) WithMinScore(score float64) *BindingBuilder
func (b *BindingBuilder) WithMaxTokens(max int) *BindingBuilder

func (b *BindingBuilder) Build(ctx context.Context) (*knowledge.BindingConfig, error)
```

Validation:
- Knowledge base ID is required and non-empty
- If topK provided: topK > 0
- If minScore provided: minScore in [0.0, 1.0]
- If maxTokens provided: maxTokens > 0

Defaults:
- Use knowledge base's retrieval settings if not overridden

### Relevant Files

- `sdk/knowledge/binding.go` (new)
- `engine/knowledge/binding_config.go` (reference)
- `sdk/internal/errors/build_error.go` (existing)

### Dependent Files

- `sdk/agent/builder.go` (consumer via WithKnowledge)
- `sdk/knowledge/base.go` (referenced resource)

## Deliverables

- sdk/knowledge/binding.go with complete BindingBuilder
- Unit tests for binding with and without overrides
- Validation for knowledge base ID
- GoDoc comments explaining override behavior
- Clear distinction between binding and base retrieval settings

## Tests

Unit tests from _tests.md (Knowledge section):

- [x] Valid binding with knowledge base ID builds successfully
- [x] Binding with empty knowledge base ID returns BuildError
- [x] Binding with topK override validates topK > 0
- [x] Binding with minScore override validates range [0.0, 1.0]
- [x] Binding with maxTokens override validates maxTokens > 0
- [x] Binding without overrides uses nil values (engine uses base defaults)
- [x] Multiple parameter overrides can be combined
- [x] Build(ctx) propagates context to validation
- [x] logger.FromContext(ctx) used in Build method
- [x] Error messages specify which override failed validation

## Success Criteria

- All unit tests pass with >95% coverage
- make lint passes
- Override semantics clearly documented
- Error messages distinguish base vs binding validation failures
- Agent integration example in documentation
