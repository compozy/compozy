---
status: pending
---

<task_context>
<domain>test/integration/auth</domain>
<type>testing</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>database,redis,http_server</dependencies>
</task_context>

# Task 8.0: Integration & Unit Tests

## Overview

Ensure reliable unit and integration test coverage for the entire auth stack.

<requirements>
- Follow testing standards (t.Run pattern, testify).
- Spin up Postgres & Redis containers via test/helpers.
- Cover happy path, invalid key, revoked key, rate-limit exceeded.
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

- [ ] 8.1 Unit tests for service & middleware
- [ ] 8.2 Integration test harness setup
- [ ] 8.3 Endpoint tests with httptest server
- [ ] 8.4 Race condition check with `-race`

## Success Criteria

- Coverage ≥ 80 % in `engine/auth`.
- Integration suite green on CI.
