---
status: completed
---

<task_context>
<domain>engine/auth</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>database,redis</dependencies>
</task_context>

# Task 2.0: Repository & Service Layer

## Overview

Implement Postgres repository methods and the use-case service that power key validation, generation, listing and revocation.

<requirements>
- Follow interface signatures in Tech Spec §3.4.
- Use dependency injection via constructors.
- Ensure all DB operations use context and propagate errors per unified strategy.
- Wrap bcrypt hash verification in repository for isolation.
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

- [x] 2.1 Define repository interface in `engine/auth/uc/repository.go`
- [x] 2.2 Implement Postgres repository in `engine/auth/infra/postgres/repository.go`
- [x] 2.3 Implement Redis cache wrapper for `GetByKeyHash`
- [x] 2.4 Implement service struct in `engine/auth/uc/service.go`
- [x] 2.5 Unit tests for service using testcontainers and mini redis

## Implementation Details

Refer Tech Spec §3.2 (GenerateKey) and §3.4.

### Relevant Files

- `engine/auth/uc/repository.go`
- `engine/auth/uc/service.go`
- `engine/auth/infra/postgres/repository.go`
- `engine/auth/infra/redis/cache.go`

## Success Criteria

- `go test ./engine/auth/...` passes.
- Service validates, generates & revokes keys correctly against Postgres & Redis.
