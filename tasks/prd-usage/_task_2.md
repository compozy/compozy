## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>engine/llm/adapter</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>external_apis</dependencies>
</task_context>

# Task 2.0: LangChain Adapter Usage Extraction

## Overview

Update the LangChain adapter to populate `LLMResponse.Usage` for synchronous and streaming responses, ensuring downstream collectors receive consistent token counts.

<critical>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</critical>

<requirements>
- Parse `ContentResponse` `GenerationInfo` fields into `LLMResponse.Usage` (prompt, completion, total, reasoning, cached tokens).
- Request streaming usage chunks via `stream_options.include_usage=true` when supported by providers.
- Handle providers that omit usage gracefully (return `nil`, log structured warning).
- Maintain compliance with `.cursor/rules/architecture.mdc` (no globals, context usage).
</requirements>

## Subtasks

- [ ] 2.1 Update `engine/llm/adapter/langchain_adapter.go` to map usage metadata
- [ ] 2.2 Ensure streaming requests set `stream_options.include_usage=true`
- [ ] 2.3 Add/refresh unit tests in `langchain_adapter_test.go`
- [ ] 2.4 Verify logging for missing usage metadata (redaction rules)

## Implementation Details

- Reference “Collection” section in `USAGE_FEATURE.md` and “Component Overview” in `_techspec.md`.
- Keep validation mode behavior unchanged; just enrich usage handling.
- Log warnings using `logger.FromContext(ctx)` with redacted errors.

### Relevant Files

- `engine/llm/adapter/langchain_adapter.go`
- `engine/llm/adapter/langchain_adapter_test.go`

### Dependent Files

- `engine/llm/usage/collector.go`
- `engine/llm/orchestrator/loop.go`

## Deliverables

- Adapter populates `LLMResponse.Usage` for sync and streaming flows
- Warnings emitted when usage missing without noisy duplication
- Updated unit tests demonstrating populated and nil usage cases

## Tests

- Unit tests mapped from `_tests.md` for this feature:
  - [ ] `engine/llm/adapter/langchain_adapter_test.go` – metadata present
  - [ ] `engine/llm/adapter/langchain_adapter_test.go` – metadata absent logging

## Success Criteria

- Adapter compiles and returns enriched usage for providers supporting metadata
- Tests cover success and fallback scenarios
- No regressions in existing adapter functionality (tool calls, structured output)
