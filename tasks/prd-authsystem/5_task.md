---
status: completed
---

<task_context>
<domain>engine/auth/router</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>middleware,service</dependencies>
</task_context>

# Task 5.0: HTTP Handlers & Router Registration ✅ COMPLETED

## Overview

Expose REST endpoints for key management and user CRUD, registering them under `/api/v0`.

<requirements>
- Follow paths & verbs in PRD §4.2.
- Use middleware groups for auth/admin separation.
- Ensure Swagger annotations for every endpoint.
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

- [x] 5.1 Implement handlers in `handlers.go`
- [x] 5.2 Register routes in `router.go`
- [x] 5.3 Add Swagger annotations + regenerate docs

### Relevant Files

- `engine/auth/router/handlers.go`
- `engine/auth/router/router.go`

## Success Criteria

- Endpoints return expected status codes with integration tests.
- Swagger UI shows new routes.
