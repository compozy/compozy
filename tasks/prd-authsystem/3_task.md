---
status: completed
---

<task_context>
<domain>engine/infra/server/middleware</domain>
<type>configuration</type>
<scope>middleware</scope>
<complexity>low</complexity>
<dependencies>gin</dependencies>
</task_context>

# Task 3.0: Configure Per-Key Rate Limiting

## Overview

Update the existing rate-limiting middleware in `engine/infra/server` to support per-key rate limiting, with a default of 100 requests per minute per key.

<requirements>
- The middleware must extract the API key from the request context.
- The rate limit (100 req/min) should be configurable.
- Ensure the `rate_limit_blocks_total` Prometheus counter is correctly incremented when a request is blocked.
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

- [x] 3.1 Modify the rate-limit middleware to identify requests by API key.
- [x] 3.2 Update the server configuration to enable and configure per-key rate limiting.
- [x] 3.3 Write integration tests to verify that the per-key rate limiting is working correctly.

## Implementation Details

See Tech Spec §2.2 component overview.

### Relevant Files

- `engine/infra/server/middleware/ratelimit/`
- `engine/infra/server/mod.go`
- `engine/infra/server/config.go`

## Success Criteria

- Blocking behavior passes unit tests.
- Metric increments on block.
