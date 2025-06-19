---
status: pending
---

<task_context>
<domain>engine/llm</domain>
<type>integration</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>task_6,task_8</dependencies>
</task_context>

# Task 9.0: Update LLM Orchestrator for Async Memory Operations

## Overview

Modify the LLM orchestrator to use async memory operations and inject conversation history. This integration enables the orchestrator to work with the enhanced memory system while maintaining optimal performance through async patterns.

## Subtasks

- [ ] 9.1 Update LLM orchestrator to accept Memory interface via dependency injection
- [ ] 9.2 Convert memory operations to async patterns with proper error handling
- [ ] 9.3 Implement conversation history loading before prompt generation
- [ ] 9.4 Add message appending after LLM responses
- [ ] 9.5 Add memory operation tracing and metrics collection

## Implementation Details

Update the LLM orchestrator to accept `Memory` interface instances via dependency injection from the agent runtime. Convert all memory operations to async patterns with proper context propagation and error handling.

Key integration points:

- Load conversation history using `ReadAsync()` before prompt generation
- Append user messages and LLM responses using `AppendAsync()`
- Support multiple memory instances per agent with different access modes
- Integrate with existing prompt building and response handling
- Add memory operation tracing for observability

The orchestrator must handle async operation timeouts, errors, and cancellation scenarios gracefully while maintaining performance.

## Success Criteria

- Memory interface integration works via dependency injection
- Async operations handle errors and timeouts correctly
- Conversation history loads properly before prompt generation
- Message appending works for both user and LLM responses
- Multiple memory instances per agent work with access mode restrictions
- Memory operation tracing provides observability into performance
- Performance tests show <50ms overhead for memory operations

<critical>
**MANDATORY REQUIREMENTS:**

- **ALWAYS** verify against PRD and tech specs - NEVER make assumptions
- **NEVER** use workarounds, especially in tests - implement proper solutions
- **MUST** follow all established project standards:
    - Architecture patterns: `.cursor/rules/architecture.mdc`
    - Go coding standards: `.cursor/rules/go-coding-standards.mdc`
    - Testing requirements: `.cursor/rules/testing-standards.mdc`
    - API standards: `.cursor/rules/api-standards.mdc`
    - Security & quality: `.cursor/rules/quality-security.mdc`
- **MUST** run `make lint` and `make test` before completing parent tasks
- **MUST** follow `.cursor/rules/task-review.mdc` workflow for parent tasks

**Enforcement:** Violating these standards results in immediate task rejection.
</critical>
