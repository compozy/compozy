---
status: pending
parallelizable: false
blocked_by: ["3.0", "5.0", "6.0", "7.0", "10.0", "11.0", "12.0", "13.0", "14.0"]
---

<task_context>
<domain>test/integration/knowledge</domain>
<type>testing</type>
<scope>integration</scope>
<complexity>high</complexity>
<dependencies>database|http_server|external_apis</dependencies>
<unblocks>"—"</unblocks>
</task_context>

# Task 15.0: Integration & E2E Tests

## Overview

Add container‑backed tests for pgvector/qdrant, router API contract tests, CLI golden tests, and workflow binding integration per `_tests.md`.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Gate with `KNOWLEDGE_E2E=1` or build tag; do not run by default.
- Use `testcontainers-go` for pgvector/qdrant. Keep tests deterministic.
- Include Swagger parity golden generation and comparison.
- Run `make fmt && make lint && make test` (plus E2E gate) before completion.
</requirements>

## Subtasks

- [ ] 15.1 `test/integration/knowledge/pgvector_test.go`
- [ ] 15.2 `test/integration/knowledge/qdrant_test.go` (P1)
- [ ] 15.3 `test/integration/knowledge/workflow_binding_test.go`
- [ ] 15.4 `test/integration/knowledge/cli_test.go` with golden JSON

## Sequencing

- Blocked by: 3.0, 5.0, 6.0, 7.0, 10.0, 11.0, 12.0, 13.0, 14.0
- Unblocks: —
- Parallelizable: No

## Implementation Details

Start/stop containers reliably with retries; validate idempotency and latency; compare Swagger to golden.

### Relevant Files

- `test/integration/knowledge/*`

### Dependent Files

- —

## Success Criteria

- All integration tests pass under gate; goldens stable; API and CLI parity verified.
