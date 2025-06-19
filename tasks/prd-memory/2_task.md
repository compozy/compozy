---
status: pending
---

<task_context>
<domain>engine/memory</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>task_1</dependencies>
</task_context>

# Task 2.0: Implement Priority-Based Token Management System

## Overview

Build the priority-aware token counting and eviction system with token allocation constraints. This system ensures critical content (priority 0) is preserved while intelligently managing token budgets through configurable priority levels and allocation ratios.

## Subtasks

- [ ] 2.1 Create PriorityMemoryManager with EvictWithPriority method
- [ ] 2.2 Implement calculateEffectiveTokenLimits for priority/allocation interaction
- [ ] 2.3 Build message grouping by priority and eviction algorithms
- [ ] 2.4 Add token counting integration with model-specific tokenizers
- [ ] 2.5 Support optional priority configuration with FIFO fallback

## Implementation Details

Create `PriorityMemoryManager` that enforces the PRD rule: "enforce the lower of the two values" when both token_allocation ratios and priority block max_tokens are configured.

The system must:

- Preserve priority 0 (critical) content regardless of token pressure
- Group messages by priority levels (0=critical, 1=important, 2+=optional)
- Calculate effective token limits by comparing ratio-based budgets vs fixed max_tokens
- Evict from lowest priority first while respecting effective limits
- Map content types to allocation categories (system, short_term, long_term)

Integrate with existing model registry for accurate token calculations using model-specific tokenizers.

## Success Criteria

- Priority-based eviction preserves critical content under all token pressure scenarios
- Effective token limits correctly implement "lower of two values" rule
- Message grouping by priority works with configurable priority blocks
- Token counting integration provides accurate model-specific calculations
- Optional priority configuration falls back to standard FIFO eviction
- Edge cases where ratios conflict with max_tokens are handled correctly

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
