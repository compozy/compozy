## status: completed

<task_context>
<domain>sdk/memory</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>engine/memory, sdk/internal</dependencies>
</task_context>

# Task 31.0: Memory: Config — core (S)

## Overview

Implement ConfigBuilder in `sdk/memory/config.go` to configure memory resources with core features: provider, model, max tokens, and basic flush strategies. This task lays the foundation for full memory system features (Tasks 32-35 add privacy, persistence, token counting).

<critical>
- **ALWAYS READ** @.cursor/rules/go-coding-standards.mdc before start
- **ALWAYS READ** tasks/prd-sdk/02-architecture.md (Context-first patterns, Memory section)
- **ALWAYS READ** tasks/prd-sdk/03-sdk-entities.md (Memory section 7.1)
</critical>

<requirements>
- Configure provider and model for memory operations
- Set max tokens limit
- Support basic flush strategies (FIFO)
- Follow error accumulation pattern (BuildError)
- Use logger.FromContext(ctx) in Build method
- Validate provider and model are non-empty
</requirements>

## Subtasks

- [x] 31.1 Create sdk/memory/config.go with ConfigBuilder
- [x] 31.2 Implement New(id) constructor
- [x] 31.3 Implement WithProvider, WithModel, WithMaxTokens methods
- [x] 31.4 Implement WithFlushStrategy and WithFIFOFlush methods
- [x] 31.5 Add unit tests for core configuration and FIFO flush

## Implementation Details

Per 03-sdk-entities.md section 7.1 (core subset for this task):

```go
type ConfigBuilder struct {
    config *memory.Config
    errors []error
}

func New(id string) *ConfigBuilder

// Core configuration
func (b *ConfigBuilder) WithProvider(provider string) *ConfigBuilder
func (b *ConfigBuilder) WithModel(model string) *ConfigBuilder
func (b *ConfigBuilder) WithMaxTokens(max int) *ConfigBuilder

// Flush strategies (basic)
func (b *ConfigBuilder) WithFlushStrategy(strategy memory.FlushStrategy) *ConfigBuilder
func (b *ConfigBuilder) WithFIFOFlush(maxMessages int) *ConfigBuilder

func (b *ConfigBuilder) Build(ctx context.Context) (*memory.Config, error)
```

Validation:
- ID is required and non-empty
- Provider is required
- Model is required
- MaxTokens > 0
- FIFO maxMessages > 0

Note: Advanced features (summarization flush, privacy, expiration, persistence, token counting, distributed locking) will be added in Tasks 32-35.

### Relevant Files

- `sdk/memory/config.go` (new)
- `engine/memory/config.go` (reference)
- `sdk/internal/errors/build_error.go` (existing)

### Dependent Files

- `sdk/project/builder.go` (consumer)
- `sdk/memory/reference.go` (Task 32, consumer)

## Deliverables

- sdk/memory/config.go with ConfigBuilder (core features only)
- Unit tests for provider/model/maxTokens/FIFO flush
- Validation for required fields
- GoDoc comments for all methods
- Clear preparation for advanced features in follow-up tasks

## Tests

Unit tests from _tests.md (Memory section - core subset):

- [x] Valid memory config with provider/model/maxTokens builds successfully
- [x] Memory config with empty ID returns BuildError
- [x] Memory config with empty provider returns BuildError
- [x] Memory config with empty model returns BuildError
- [x] Memory config with maxTokens <= 0 returns validation error
- [x] FIFO flush with maxMessages <= 0 returns validation error
- [x] FIFO flush strategy sets correct enum value
- [x] Build(ctx) propagates context to validation
- [x] logger.FromContext(ctx) used in Build method
- [x] Multiple validation errors aggregate in BuildError

## Success Criteria

- All unit tests pass with >95% coverage
- make lint passes
- Core memory configuration works end-to-end
- Error messages are specific and actionable
- Code structure allows easy extension for Tasks 32-35
