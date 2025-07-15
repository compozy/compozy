---
status: completed
---

<task_context>
<domain>engine/auth</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>database</dependencies>
</task_context>

# Task 1.0: Database Migrations & Data Models

## Overview

Create Goose migrations and Go structs for `users` and `api_keys` tables as defined in the Tech Spec.

<requirements>
- Follow existing Goose version scheme (reserve versions 0001 & 0002).
- Hash stored keys with bcrypt.
- Include indexes on `hash` and `user_id`.
- Add models in `engine/auth/model` with proper tags.
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

- [x] 1.1 Author `0001_create_users.sql` ✅ COMPLETED
- [x] 1.2 Author `0002_create_api_keys.sql` ✅ COMPLETED
- [x] 1.3 Implement `User` and `APIKey` structs ✅ COMPLETED
- [x] 1.4 Run `goose up` in local dev and verify ✅ COMPLETED
- [x] 1.5 Unit test struct tags with schemagen utility ✅ COMPLETED

## Implementation Details

See Tech Spec §3.1, §3.6.

### Relevant Files

- `engine/auth/migrations/0001_create_users.sql`
- `engine/auth/migrations/0002_create_api_keys.sql`
- `engine/auth/model/user.go`
- `engine/auth/model/apikey.go`

## Success Criteria

- Goose migrations apply cleanly on fresh DB.
- `go test ./...` passes with new models.
- Indexes visible in `pg_indexes`.
