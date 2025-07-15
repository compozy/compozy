---
status: completed
---

<task_context>
<domain>engine/auth/middleware</domain>
<type>implementation</type>
<scope>middleware</scope>
<complexity>medium</complexity>
<dependencies>database,redis,rate-limiter</dependencies>
</task_context>

# Task 4.0: Authentication Middleware

## Overview

Develop Gin middleware to validate API keys, enforce rate limits, inject user into context, and handle error responses.

<requirements>
- Use `logger.FromContext(ctx)` for logging.
- Return JSON error format per API standards.
- Cache hits first check Redis, fallback to repo on miss.
- Respect `Authorization: Bearer <key>` header only.
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

- [x] 4.1 Implement `AuthMiddleware` in `auth.go`
- [x] 4.2 Implement helper `AdminOnly()`
- [x] 4.3 Unit tests (happy path, invalid, rate-limited)

## Implementation Details

Tech Spec §3.3.

### Relevant Files

- `engine/auth/middleware/auth.go`

## Success Criteria

- Middleware adds ≤ 5 ms overhead in benchmark.
- All tests pass.
