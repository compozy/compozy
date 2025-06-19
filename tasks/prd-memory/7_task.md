---
status: pending
---

<task_context>
<domain>engine/memory</domain>
<type>integration</type>
<scope>middleware</scope>
<complexity>medium</complexity>
<dependencies>task_5,task_6</dependencies>
</task_context>

# Task 7.0: Create Memory Manager Factory and Template Engine Integration

## Overview

Build the central memory management system with key template evaluation and instance lifecycle. This factory orchestrates memory instance creation, template processing, and lifecycle management while enforcing access controls and providing proper error handling.

## Subtasks

- [ ] 7.1 Implement MemoryManager as factory for memory instances with template evaluation
- [ ] 7.2 Integrate tplengine for memory key template processing
- [ ] 7.3 Add instance caching and lifecycle management scoped to workflow execution
- [ ] 7.4 Implement access mode enforcement (read-only prevents modifications)
- [ ] 7.5 Create error handling that returns proper errors instead of panics

## Implementation Details

Implement `MemoryManager` as the central factory that:

- Evaluates key templates using `tplengine` with workflow context variables
- Creates or fetches `AsyncSafeMemoryInstance` objects with caching
- Enforces access mode restrictions (read-only vs read-write)
- Manages instance lifecycle scoped to workflow execution
- Initializes memory from persistence on first access

Template variables supported:

- `{{ .workflow.id }}`, `{{ .workflow.input.* }}`
- `{{ .user.id }}`, `{{ .session.id }}`, `{{ .project.id }}`
- `{{ .agent.id }}`, `{{ .timestamp }}`

The manager must return proper errors instead of panics and provide safe concurrent access to memory instances.

## Success Criteria

- Template evaluation works correctly with all workflow context variables
- Instance caching provides performance benefits while maintaining isolation
- Lifecycle management ensures proper cleanup at workflow completion
- Access mode enforcement prevents unauthorized modifications in read-only mode
- Error handling provides meaningful messages without panics
- Memory initialization from persistence works on first access

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
