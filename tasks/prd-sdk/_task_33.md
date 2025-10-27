## status: completed

<task_context>
<domain>sdk/memory</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>sdk/memory/config.go, engine/memory</dependencies>
</task_context>

# Task 33.0: Memory: Flush Strategies (S)

## Overview

Extend memory ConfigBuilder with flush strategy methods (FIFO and Summarization modes).

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** tasks/prd-sdk/02-architecture.md and tasks/prd-sdk/03-sdk-entities.md
</critical>

<requirements>
- Flush strategy configuration methods
- FIFO flush with max messages
- Summarization flush with provider/model
- Context-first validation
- Error accumulation
</requirements>

## Subtasks

- [x] 33.1 Add WithFlushStrategy(strategy) method to ConfigBuilder
- [x] 33.2 Implement WithFIFOFlush(maxMessages int) helper
- [x] 33.3 Implement WithSummarizationFlush(provider, model string) helper
- [x] 33.4 Update Build(ctx) validation for flush strategies
- [x] 33.5 Add unit tests for flush strategy methods

## Implementation Details

Reference from 03-sdk-entities.md section 7.1:

```go
// Flush strategies
func (b *ConfigBuilder) WithFlushStrategy(strategy memory.FlushStrategy) *ConfigBuilder
func (b *ConfigBuilder) WithFIFOFlush(maxMessages int) *ConfigBuilder
func (b *ConfigBuilder) WithSummarizationFlush(provider, model string, maxTokens int) *ConfigBuilder
```

Engine flush strategies from engine/memory:
- FlushStrategyNone
- FlushStrategyFIFO
- FlushStrategySummarization

### Relevant Files

- `sdk/memory/config.go` (extend existing)
- `engine/memory/types.go` (flush strategy types)

### Dependent Files

- Task 31.0 output (memory ConfigBuilder base)
- Future memory examples

## Deliverables

- Flush strategy methods in ConfigBuilder
- FIFO and Summarization helper methods
- Validation for flush configuration
- Updated package documentation

## Tests

Unit tests mapped from `_tests.md`:

- [x] WithFlushStrategy sets strategy correctly
- [x] WithFIFOFlush configures FIFO with max messages
- [x] WithSummarizationFlush configures summarization with provider/model
- [x] Build(ctx) validates flush strategy requirements
- [x] Error cases: invalid max messages, missing provider/model
- [x] Edge cases: conflicting flush strategies

## Success Criteria

- Flush strategy methods follow builder pattern
- All unit tests pass
- make lint and make test pass
- Flush strategies ready for memory examples
