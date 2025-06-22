---
status: completed
---

<task_context>
<domain>engine/memory</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>task_1</dependencies>
</task_context>

# Task 2.0: Implement Token Management and Flushing System

## Overview

Build the comprehensive token management system with FIFO eviction and hybrid flushing strategy. This system manages token budgets through accurate counting, intelligent eviction, and rule-based summarization to maintain conversation context while respecting memory constraints.

## Subtasks

- [ ] 2.1 Create TokenMemoryManager with standard FIFO eviction
- [ ] 2.2 Integrate tiktoken-go for accurate token counting
- [ ] 2.3 Build HybridFlushingStrategy with rule-based summarization
- [ ] 2.4 Implement optimized flush checking using message count estimates
- [ ] 2.5 Create Temporal activities for background flush processing
- [ ] 2.6 Support configurable token allocation ratios and flushing parameters

## Implementation Details

Create a unified token management and flushing system that:

**Token Management**:

- Count tokens accurately using tiktoken-go library
- Support multiple model encodings (o200k_base, cl100k_base, p50k_base)
- Evict oldest messages first when token limit is reached
- Support configurable token allocation ratios (system, short_term, long_term)
- Cache token counts per message to avoid re-tokenization

**Hybrid Flushing Strategy**:

- Implement rule-based summarization (first message + N recent messages)
- Use optimized flush checking based on message count estimates
- Summarize oldest X% of messages when threshold is reached
- Preserve recent context for conversation continuity
- Calculate token savings from summarization

**Background Processing**:

- **NO ASYNQ**: Use Temporal activities for async flush operations
- Follow existing Temporal patterns from `engine/workflow/activities/`
- Use workflow.ExecuteActivity for background processing with proper context
- Activity heartbeats for long-running flush operations
- Temporal retry policies for fault tolerance
- Avoid blocking main memory operations

**Library Integration**:

- Use **tiktoken-go** for all token counting operations (not duplicated)
- Leverage existing Temporal infrastructure for background tasks
- Follow existing activity patterns for error handling and retries
- Use `temporal.NewNonRetryableApplicationError` for permanent failures

# Relevant Files

## Core Implementation Files

- `engine/memory/token_manager.go` - Token-based memory management and FIFO eviction
- `engine/memory/flush_strategy.go` - Hybrid flushing with rule-based summarization
- `engine/memory/activities/flush.go` - Temporal activity for background flushing
- `engine/memory/types.go` - TokenAllocation and FlushingStrategy data models

## Test Files

- `engine/memory/token_manager_test.go` - Token allocation and FIFO eviction tests
- `engine/memory/flush_strategy_test.go` - Hybrid flushing and summarization tests
- `engine/memory/activities/flush_test.go` - Temporal activity tests

## Success Criteria

- FIFO eviction works correctly under token pressure scenarios
- Token counting provides accurate model-specific calculations using tiktoken-go
- Hybrid flushing maintains context continuity through intelligent summarization
- Rule-based summarization provides deterministic, cost-effective context preservation
- Optimized flush checking avoids performance bottlenecks on high-volume scenarios
- Temporal activities handle background flush operations without blocking
- Token allocation ratios correctly distribute available token budget
- Summary quality preserves essential context (first + recent messages)
- Token savings calculations accurately reflect flush effectiveness
- Configurable parameters allow tuning for different conversation patterns

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
