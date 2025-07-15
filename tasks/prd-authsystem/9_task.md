---
status: pending
---

<task_context>
<domain>docs</domain>
<type>documentation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>swagger</dependencies>
</task_context>

# Task 9.0: Documentation Updates

## Overview

Add API reference and user guide for auth endpoints and CLI commands.

<requirements>
- Update Swagger annotations; regenerate `/swagger/index.html`.
- Write docs page `docs/api/auth.mdx` with examples.
- Include CLI examples with curl & compozy commands.
</requirements>

<critical>
**MANDATORY REQUIREMENTS:**
- **ALWAYS** check dependent files APIs before write tests to avoid write wrong code
- **ALWAYS** verify against PRD and tech specs - NEVER make assumptions
- **NEVER** use workarounds, especially in tests - implement proper solutions
- **MUST** follow all established project standards:
    - Architecture patterns: @.cursor/rules/architecture.mdc
    - Go coding standards: @.cursor/rules/go-coding-standards.mdc
    - Testing requirements: @.cursor/rules/test-standard.mdc
    - API standards: @.cursor/rules/api-standards.mdc
    - Security & quality: @.cursor/rules/quality-security.md
    - GoGraph MCP tools: @.cursor/rules/gograph.mdc
    - No Backwards Compatibility: @.cursor/rules/backwards-compatibility.mdc
- **MUST** use `logger.FromContext(ctx)` - NEVER use logger as function parameter or dependency injection
- **MUST** run `make lint` and `make test` before completing ANY subtask
- **MUST** follow @.cursor/rules/task-review.mdc workflow for parent tasks
**Enforcement:** Violating these standards results in immediate task rejection.
</critical>

## Subtasks

- [ ] 9.1 Swagger annotations & generation
- [ ] 9.2 MDX docs page
- [ ] 9.3 Review by PM

## Success Criteria

- Docs site builds with new page.
- Examples tested and functional.
