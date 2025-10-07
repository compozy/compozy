---
status: completed
parallelizable: true
blocked_by: []
---

<task_context>
<domain>engine/knowledge</domain>
<type>documentation|testing</type>
<scope>prework|tooling</scope>
<complexity>low</complexity>
<dependencies>external_apis</dependencies>
<unblocks>"1.0","2.0","3.0","4.0","5.0","6.0","7.0","8.0","9.0","10.0","11.0","12.0","13.0","14.0","15.0"</unblocks>
</task_context>

# Task 0.0: Pre‑work — External Libs Verification + Test Utilities Baseline

## Overview

Verify embeddings/vector APIs in `github.com/tmc/langchaingo v0.1.13` and scaffold shared test helpers to enforce repository standards for subsequent unit tests.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Use Perplexity + Context7 to confirm current `EmbedDocuments`/`EmbedQuery` shapes and supported vector store clients.
- Document any provider/dimension/batching constraints that impact adapters.
- Create shared test helpers; runtime code MUST not use globals and MUST derive `logger`/`config` from context.
- Run `make fmt && make lint && make test` before completion.
</requirements>

## Subtasks

- [x] 0.1 Verify `langchaingo` embeddings/vector APIs (Perplexity + Context7 notes committed under `tasks/prd-embedding/notes/`)
- [x] 0.2 Add `test/helpers/context.go` with `NewTestContext(t)` using `logger.FromContext(ctx)` and `config.FromContext(ctx)`
- [x] 0.3 Add `test/helpers/golden.go` utilities
- [x] 0.4 Confirm `go.mod` pins `github.com/tmc/langchaingo v0.1.13`

## Sequencing

- Blocked by: —
- Unblocks: 1.0–15.0
- Parallelizable: Yes

## Implementation Details

Record exact method signatures and any notable provider quirks (batch size, dimensions, timeouts). Keep notes small and scoped to MVP needs.

### Relevant Files

- `go.mod`
- `test/helpers/context.go`
- `test/helpers/golden.go`

### Dependent Files

- All adapter and pipeline unit tests

## Success Criteria

- Notes captured; helpers in place; `make lint` and `make test` pass.
