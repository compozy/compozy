---
status: pending
---

<task_context>
<domain>cluster</domain>
<type>integration</type>
<scope>configuration</scope>
<complexity>low</complexity>
<dependencies>redis,server,migrations</dependencies>
</task_context>

# Task 10.0: Deployment & Configuration

## Overview

Update docker-compose, Helm charts, and environment configs to enable auth system in all environments.

<requirements>
- Apply migrations on start-up (`goose up`).
- Add `AUTH_RATE_LIMIT=100` env.
- Ensure Redis service available in dev/prod stacks.
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

- [ ] 10.1 Extend `docker-compose.yml` healthcheck for Redis
- [ ] 10.2 Helm chart updates (values & secrets)
- [ ] 10.3 CI job: run migrations

## Success Criteria

- Staging stack starts with auth enabled.
- Health checks green.
