---
status: completed
parallelizable: false
blocked_by: []
---

<task_context>
<domain>engine/infra/server/router</domain>
<type>implementation</type>
<scope>middleware</scope>
<complexity>medium</complexity>
<dependencies>webhook,redis,http_server</dependencies>
<unblocks>3.0, 4.0, 5.0</unblocks>
</task_context>

# Task 2.0: Implement API idempotency helper (router wrapper)

## Overview

Create a small helper that provides API-level idempotency for execution endpoints using the existing webhook idempotency service. The helper derives a key from `X-Idempotency-Key` or a stable hash of `(method + route + normalized JSON body)` with a 24h TTL.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Add `engine/infra/server/router/idempotency.go` with an `APIIdempotency` interface and implementation.
- Provide `CheckAndSet(ctx, c, namespace, body, ttl)` returning `(unique, reason, err)`.
- In-flight duplicate → respond `409 Conflict` without blocking.
- Use `config.FromContext(ctx)` and `logger.FromContext(ctx)` only; no globals.
- Unit tests for header precedence, hashing, TTL, and duplicate detection.
</requirements>

## Subtasks

- [x] 2.1 Define interface and key derivation (header vs. body hash)
- [x] 2.2 Implement Redis-backed calls via webhook service wrapper
- [x] 2.3 Unit tests for success/duplicate/error paths

## Sequencing

- Blocked by: —
- Unblocks: 3.0, 4.0, 5.0
- Parallelizable: No (foundation)

## Implementation Details

See Tech Spec “System Architecture → Component Overview” and “Technical Considerations → Key Decisions”. Namespace keys under `idempotency:api:execs:`.

### Relevant Files

- `engine/infra/server/router/idempotency.go`

### Dependent Files

- `engine/webhook/service.go`
- `engine/infra/server/router/helpers.go`

## Success Criteria

- Helper integrated by downstream handlers; duplicate requests prevented
- Tests validate key semantics and TTL
- Lints/tests pass
